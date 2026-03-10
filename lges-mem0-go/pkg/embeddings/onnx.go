package embeddings

import (
	"fmt"
	"math"
	"runtime"

	"github.com/mem0ai/mem0-go/pkg/tokenizer"
	ort "github.com/yalue/onnxruntime_go"
)

// OnnxEmbedder implements Embedder using a local ONNX model.
type OnnxEmbedder struct {
	tokenizer *tokenizer.Tokenizer
	session   *ort.DynamicAdvancedSession
	dim       int
}

var ortInitialized bool

// NewOnnxEmbedder creates a new ONNX-based embedder.
// modelPath: path to .onnx file
// vocabPath: path to vocab.txt
func NewOnnxEmbedder(modelPath, vocabPath string) (*OnnxEmbedder, error) {
	if !ortInitialized {
		// Initialize ONNX Runtime
		// Assuming shared library is in a standard location or configured via environment
		ort.SetSharedLibraryPath(getSharedLibPath())
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("failed to initialize onnx environment: %w", err)
		}
		ortInitialized = true
	}

	tok, err := tokenizer.NewTokenizer(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create tokenizer: %w", err)
	}

	// Create session
	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"} // Adjust if model uses different output name

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

	// Check model dimension by running a dummy inference or just hardcode/config
	// For all-MiniLM-L6-v2 it is 384
	return &OnnxEmbedder{
		tokenizer: tok,
		session:   session,
		dim:       384, // Default for MiniLM
	}, nil
}

// Embed generates an embedding for the given text.
func (e *OnnxEmbedder) Embed(text string) ([]float32, error) {
	// 1. Tokenize
	// Limit sequence length to 512 for standard BERT
	inputIDs, attentionMask, tokenTypeIDs := e.tokenizer.Encode(text, 512)

	seqLen := int64(len(inputIDs))
	shape := []int64{1, seqLen}

	// 2. Prepare inputs
	inputTensor, err := ort.NewTensor(ort.NewShape(shape...), inputIDs)
	if err != nil {
		return nil, err
	}
	defer inputTensor.Destroy()

	maskTensor, err := ort.NewTensor(ort.NewShape(shape...), attentionMask)
	if err != nil {
		return nil, err
	}
	defer maskTensor.Destroy()

	typeTensor, err := ort.NewTensor(ort.NewShape(shape...), tokenTypeIDs)
	if err != nil {
		return nil, err
	}
	defer typeTensor.Destroy()

	// 3. Prepare output
	// Output shape: [1, seqLen, 384]
	outputSize := 1 * seqLen * int64(e.dim)
	outputData := make([]float32, outputSize)
	outputTensor, err := ort.NewTensor(ort.NewShape(1, seqLen, int64(e.dim)), outputData)
	if err != nil {
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// 4. Run inference
	err = e.session.Run(
		[]ort.Value{inputTensor, maskTensor, typeTensor},
		[]ort.Value{outputTensor},
	)
	if err != nil {
		return nil, fmt.Errorf("onnx inference failed: %w", err)
	}

	// 5. Mean Pooling
	// Output data is already updated in outputData
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

	// Average
	if sumWeight > 0 {
		for i := range embedding {
			embedding[i] /= sumWeight
		}
	}

	// 6. Normalization
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
func (e *OnnxEmbedder) Close() error {
	if e.session != nil {
		e.session.Destroy()
	}
	return nil
}

// Helper to guess shared library path based on OS
func getSharedLibPath() string {
	if runtime.GOOS == "darwin" {
		return "libonnxruntime.dylib" // Needs full path usually
	}
	return "libonnxruntime.so" // Linux default
}
