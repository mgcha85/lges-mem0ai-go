package tokenizer

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// Tokenizer is a simple WordPiece tokenizer.
type Tokenizer struct {
	vocab map[string]int64
	unk   int64
	cls   int64
	sep   int64
	mask  int64
	pad   int64
}

// NewTokenizer creates a new Tokenizer from a vocab file.
func NewTokenizer(vocabPath string) (*Tokenizer, error) {
	file, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open vocab file: %w", err)
	}
	defer file.Close()

	vocab := make(map[string]int64)
	scanner := bufio.NewScanner(file)
	var idx int64
	for scanner.Scan() {
		token := scanner.Text()
		vocab[token] = idx
		idx++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read vocab file: %w", err)
	}

	t := &Tokenizer{
		vocab: vocab,
		unk:   vocab["[UNK]"],
		cls:   vocab["[CLS]"],
		sep:   vocab["[SEP]"],
		mask:  vocab["[MASK]"],
		pad:   vocab["[PAD]"],
	}
	return t, nil
}

// Tokenize splits text into tokens using WordPiece logic.
func (t *Tokenizer) Tokenize(text string) []string {
	text = strings.ToLower(text)
	text = cleanText(text)

	var basicTokens []string
	// Basic tokenization: split by whitespace and punctuation
	current := ""
	for _, r := range text {
		if unicode.IsSpace(r) {
			if current != "" {
				basicTokens = append(basicTokens, current)
				current = ""
			}
		} else if isPunctuation(r) {
			if current != "" {
				basicTokens = append(basicTokens, current)
				current = ""
			}
			basicTokens = append(basicTokens, string(r))
		} else {
			current += string(r)
		}
	}
	if current != "" {
		basicTokens = append(basicTokens, current)
	}

	var tokens []string
	for _, token := range basicTokens {
		subTokens := t.wordPieceTokenize(token)
		tokens = append(tokens, subTokens...)
	}
	return tokens
}

// Encode converts text to input IDs, attention mask, and token type IDs.
func (t *Tokenizer) Encode(text string, maxLen int) ([]int64, []int64, []int64) {
	tokens := t.Tokenize(text)

	// Truncate (reserve 2 for CLS and SEP)
	if len(tokens) > maxLen-2 {
		tokens = tokens[:maxLen-2]
	}

	var ids []int64
	ids = append(ids, t.cls) // [CLS]
	for _, token := range tokens {
		if id, ok := t.vocab[token]; ok {
			ids = append(ids, id)
		} else {
			ids = append(ids, t.unk) // [UNK]
		}
	}
	ids = append(ids, t.sep) // [SEP]

	inputIDs := make([]int64, maxLen)
	attentionMask := make([]int64, maxLen)
	tokenTypeIDs := make([]int64, maxLen)

	for i := 0; i < len(ids); i++ {
		inputIDs[i] = ids[i]
		attentionMask[i] = 1
		tokenTypeIDs[i] = 0
	}

	// Padding
	for i := len(ids); i < maxLen; i++ {
		inputIDs[i] = t.pad
		attentionMask[i] = 0
		tokenTypeIDs[i] = 0
	}

	return inputIDs, attentionMask, tokenTypeIDs
}

func (t *Tokenizer) wordPieceTokenize(word string) []string {
	if len(word) > 100 {
		return []string{"[UNK]"}
	}

	var subTokens []string
	runes := []rune(word)
	start := 0

	for start < len(runes) {
		end := len(runes)
		curSubToken := ""
		found := false

		for end > start {
			sub := string(runes[start:end])
			if start > 0 {
				sub = "##" + sub
			}
			if _, ok := t.vocab[sub]; ok {
				curSubToken = sub
				found = true
				break
			}
			end--
		}

		if !found {
			return []string{"[UNK]"}
		}

		subTokens = append(subTokens, curSubToken)
		start = end
	}
	return subTokens
}

func cleanText(text string) string {
	// Remove control characters using regex or basic rune filtering
	// Using basic rune filtering here
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, text)
}

func isPunctuation(r rune) bool {
	// Basic punctuation check based on unicode category
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}
