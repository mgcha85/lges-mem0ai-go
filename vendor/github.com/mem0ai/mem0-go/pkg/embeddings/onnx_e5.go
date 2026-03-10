package embeddings

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	ort "github.com/yalue/onnxruntime_go"
)

// OnnxE5Embedder implements Embedder using a local ONNX model with tokenizer.json.
// Designed for models like intfloat/multilingual-e5-small that use SentencePiece/Unigram.
type OnnxE5Embedder struct {
	tokenizer *HFTokenizer
	session   *ort.DynamicAdvancedSession
	dim       int
}

// NewOnnxE5Embedder creates a new ONNX embedder from a model directory.
// modelDir should contain: model.onnx and tokenizer.json
func NewOnnxE5Embedder(modelDir string, dim int) (*OnnxE5Embedder, error) {
	if !ortInitialized {
		libPath := getOrtLibPath(modelDir)
		ort.SetSharedLibraryPath(libPath)
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("failed to initialize onnx environment: %w", err)
		}
		ortInitialized = true
	}

	// Load tokenizer
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")
	tok, err := NewHFTokenizer(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer: %w", err)
	}

	// Create ONNX session
	modelPath := filepath.Join(modelDir, "model.onnx")
	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"}

	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	session, err := ort.NewDynamicAdvancedSession(
		modelPath,
		inputNames,
		outputNames,
		options,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create onnx session: %w", err)
	}

	return &OnnxE5Embedder{
		tokenizer: tok,
		session:   session,
		dim:       dim,
	}, nil
}

// Embed generates an embedding for the given text.
// For E5 models, query texts should be prefixed with "query: " and
// passage texts with "passage: " for best results.
func (e *OnnxE5Embedder) Embed(text string) ([]float32, error) {
	// Tokenize
	inputIDs, attentionMask := e.tokenizer.Encode(text, 512)
	seqLen := int64(len(inputIDs))
	shape := []int64{1, seqLen}

	// Prepare inputs
	inputTensor, err := ort.NewTensor(ort.NewShape(shape...), inputIDs)
	if err != nil {
		return nil, fmt.Errorf("input_ids tensor error: %w", err)
	}
	defer inputTensor.Destroy()

	maskTensor, err := ort.NewTensor(ort.NewShape(shape...), attentionMask)
	if err != nil {
		return nil, fmt.Errorf("attention_mask tensor error: %w", err)
	}
	defer maskTensor.Destroy()

	// token_type_ids: all zeros for RoBERTa/E5
	tokenTypeIDs := make([]int64, seqLen)
	typeTensor, err := ort.NewTensor(ort.NewShape(shape...), tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("token_type_ids tensor error: %w", err)
	}
	defer typeTensor.Destroy()

	// Prepare output
	outputData := make([]float32, seqLen*int64(e.dim))
	outputTensor, err := ort.NewTensor(ort.NewShape(1, seqLen, int64(e.dim)), outputData)
	if err != nil {
		return nil, fmt.Errorf("output tensor error: %w", err)
	}
	defer outputTensor.Destroy()

	// Run inference
	err = e.session.Run(
		[]ort.Value{inputTensor, maskTensor, typeTensor},
		[]ort.Value{outputTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("onnx inference failed: %w", err)
	}

	// Mean Pooling with attention mask
	embedding := make([]float32, e.dim)
	var sumWeight float32

	for i := 0; i < int(seqLen); i++ {
		if attentionMask[i] == 1 {
			offset := i * e.dim
			for j := 0; j < e.dim; j++ {
				embedding[j] += outputData[offset+j]
			}
			sumWeight += 1.0
		}
	}

	if sumWeight > 0 {
		for i := range embedding {
			embedding[i] /= sumWeight
		}
	}

	// L2 normalization
	var norm float64
	for _, v := range embedding {
		norm += float64(v * v)
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range embedding {
			embedding[i] /= float32(norm)
		}
	}

	return embedding, nil
}

// Close cleans up the ONNX session.
func (e *OnnxE5Embedder) Close() error {
	if e.session != nil {
		e.session.Destroy()
	}
	return nil
}

// getOrtLibPath finds the ONNX Runtime shared library.
func getOrtLibPath(modelDir string) string {
	// Check in model directory first
	var libName string
	if runtime.GOOS == "darwin" {
		libName = "libonnxruntime.dylib"
	} else {
		libName = "libonnxruntime.so"
	}

	// Check common locations
	candidates := []string{
		filepath.Join(modelDir, "..", "..", "lib", libName),
		filepath.Join(modelDir, "..", "lib", libName),
		libName, // system path
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}

	return libName // fallback to system path
}

// === HFTokenizer: reads tokenizer.json from HuggingFace ===

// HFTokenizer implements tokenization using HuggingFace's tokenizer.json format.
// Supports Unigram (SentencePiece) models used by XLM-RoBERTa / E5.
type HFTokenizer struct {
	vocab      map[string]int   // token -> id
	vocabList  []vocabPiece     // sorted by score for unigram
	bosID      int              // <s> token id
	eosID      int              // </s> token id
	padID      int              // <pad> token id
	unkID      int              // <unk> token id
	addPrefix  bool             // whether to add ▁ prefix
}

type vocabPiece struct {
	Token string
	Score float64
	ID    int
}

// tokenizerJSON represents the structure of tokenizer.json
type tokenizerJSON struct {
	Model struct {
		Type  string `json:"type"`
		Vocab [][]interface{} `json:"vocab"` // [[token, score], ...]
	} `json:"model"`
	AddedTokens []struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
		Special bool   `json:"special"`
	} `json:"added_tokens"`
	PreTokenizer  *json.RawMessage `json:"pre_tokenizer"`
	Normalizer    *json.RawMessage `json:"normalizer"`
	PostProcessor *json.RawMessage `json:"post_processor"`
}

// NewHFTokenizer loads a tokenizer from a tokenizer.json file.
func NewHFTokenizer(path string) (*HFTokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tokenizer.json: %w", err)
	}

	var tj tokenizerJSON
	if err := json.Unmarshal(data, &tj); err != nil {
		return nil, fmt.Errorf("failed to parse tokenizer.json: %w", err)
	}

	t := &HFTokenizer{
		vocab:     make(map[string]int),
		addPrefix: true, // XLM-RoBERTa adds ▁ prefix
	}

	// Load vocab
	for _, item := range tj.Model.Vocab {
		if len(item) < 2 {
			continue
		}
		token, ok := item[0].(string)
		if !ok {
			continue
		}
		score := 0.0
		switch v := item[1].(type) {
		case float64:
			score = v
		}
		id := len(t.vocabList)
		t.vocab[token] = id
		t.vocabList = append(t.vocabList, vocabPiece{Token: token, Score: score, ID: id})
	}

	// Add special tokens (these override vocab entries)
	for _, at := range tj.AddedTokens {
		t.vocab[at.Content] = at.ID
		switch at.Content {
		case "<s>":
			t.bosID = at.ID
		case "</s>":
			t.eosID = at.ID
		case "<pad>":
			t.padID = at.ID
		case "<unk>":
			t.unkID = at.ID
		}
	}

	return t, nil
}

// Encode tokenizes text and returns input_ids and attention_mask.
// Adds <s> and </s> special tokens.
func (t *HFTokenizer) Encode(text string, maxLen int) ([]int64, []int64) {
	// Normalize: NFC normalization + lowercase (optional for E5)
	text = strings.TrimSpace(text)

	// Tokenize using unigram
	tokens := t.tokenize(text)

	// Truncate (reserve 2 for BOS and EOS)
	if len(tokens) > maxLen-2 {
		tokens = tokens[:maxLen-2]
	}

	// Build input_ids: <s> + tokens + </s>
	ids := make([]int64, 0, len(tokens)+2)
	ids = append(ids, int64(t.bosID))
	for _, token := range tokens {
		if id, ok := t.vocab[token]; ok {
			ids = append(ids, int64(id))
		} else {
			ids = append(ids, int64(t.unkID))
		}
	}
	ids = append(ids, int64(t.eosID))

	// No padding for dynamic shapes
	attentionMask := make([]int64, len(ids))
	for i := range attentionMask {
		attentionMask[i] = 1
	}

	return ids, attentionMask
}

// tokenize splits text into subword tokens using unigram tokenization.
func (t *HFTokenizer) tokenize(text string) []string {
	if text == "" {
		return nil
	}

	// Pre-tokenize: split into words by whitespace/punctuation
	words := preTokenize(text)

	var allTokens []string
	for _, word := range words {
		tokens := t.tokenizeWord(word)
		allTokens = append(allTokens, tokens...)
	}
	return allTokens
}

// preTokenize splits text into word-level pieces.
// For XLM-RoBERTa, spaces are converted to ▁ (U+2581).
func preTokenize(text string) []string {
	// Replace spaces with ▁ and split
	// XLM-RoBERTa uses ▁ to denote word boundaries
	var pieces []string
	var current strings.Builder

	runes := []rune(text)
	for i, r := range runes {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				pieces = append(pieces, current.String())
				current.Reset()
			}
			// Add ▁ as prefix to next word
			current.WriteRune('▁')
		} else {
			if i == 0 {
				// First character always gets ▁ prefix
				current.WriteRune('▁')
			}
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		pieces = append(pieces, current.String())
	}

	return pieces
}

// tokenizeWord performs unigram tokenization on a single word.
// Uses the Viterbi algorithm for optimal segmentation.
func (t *HFTokenizer) tokenizeWord(word string) []string {
	runes := []rune(word)
	n := len(runes)
	if n == 0 {
		return nil
	}

	// Viterbi-style best segmentation
	type bestPath struct {
		score float64
		start int
		token string
	}

	// bestEndingAt[i] = best segmentation ending at position i
	bestEndingAt := make([]bestPath, n+1)
	bestEndingAt[0] = bestPath{score: 0, start: -1}
	for i := 1; i <= n; i++ {
		bestEndingAt[i] = bestPath{score: -1e18, start: -1}
	}

	for end := 1; end <= n; end++ {
		for start := 0; start < end; start++ {
			substr := string(runes[start:end])
			if _, ok := t.vocab[substr]; ok {
				// Find score
				score := bestEndingAt[start].score + t.getScore(substr)
				if score > bestEndingAt[end].score {
					bestEndingAt[end] = bestPath{
						score: score,
						start: start,
						token: substr,
					}
				}
			}
		}
		// If no valid segmentation found, try single character as unk
		if bestEndingAt[end].start == -1 && end > 0 {
			singleChar := string(runes[end-1 : end])
			score := bestEndingAt[end-1].score - 100.0 // penalty for unk
			bestEndingAt[end] = bestPath{
				score: score,
				start: end - 1,
				token: singleChar,
			}
		}
	}

	// Backtrack to find the best segmentation
	var tokens []string
	pos := n
	for pos > 0 {
		bp := bestEndingAt[pos]
		if bp.start == -1 {
			break
		}
		tokens = append(tokens, bp.token)
		pos = bp.start
	}

	// Reverse tokens (we backtracked from end to start)
	sort.SliceStable(tokens, func(i, j int) bool {
		return false // no-op, we'll reverse manually
	})
	for i, j := 0, len(tokens)-1; i < j; i, j = i+1, j-1 {
		tokens[i], tokens[j] = tokens[j], tokens[i]
	}

	return tokens
}

// getScore returns the unigram score for a token.
func (t *HFTokenizer) getScore(token string) float64 {
	if id, ok := t.vocab[token]; ok {
		if id < len(t.vocabList) {
			return t.vocabList[id].Score
		}
	}
	return -100.0 // very low score for unknown tokens
}
