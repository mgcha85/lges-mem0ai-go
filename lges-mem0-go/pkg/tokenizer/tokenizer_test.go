package tokenizer

import (
	"os"
	"reflect"
	"testing"
)

func createTestVocab(t *testing.T) string {
	content := `[PAD]
[unused0]
[unused1]
[unused2]
[unused3]
[unused4]
[unused5]
[unused6]
[unused7]
[unused8]
[unused9]
[UNK]
[CLS]
[SEP]
[MASK]
hello
world
,
!
em
##bed
##ding
##s
`
	f, err := os.CreateTemp("", "vocab.txt")
	if err != nil {
		t.Fatalf("Failed to create temp vocab: %v", err)
	}
	defer f.Close()
	f.WriteString(content)
	return f.Name()
}

func TestTokenizer(t *testing.T) {
	vocabPath := createTestVocab(t)
	defer os.Remove(vocabPath)

	tok, err := NewTokenizer(vocabPath)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "basic",
			input:    "Hello world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "punctuation",
			input:    "Hello, world!",
			expected: []string{"hello", ",", "world", "!"},
		},
		{
			name:     "wordpiece",
			input:    "embeddings",
			expected: []string{"em", "##bed", "##ding", "##s"},
		},
		{
			name:     "mixed",
			input:    "Hello, embeddings!",
			expected: []string{"hello", ",", "em", "##bed", "##ding", "##s", "!"},
		},
		{
			name:     "unknown",
			input:    "unknownword",
			expected: []string{"[UNK]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tok.Tokenize(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	vocabPath := createTestVocab(t)
	defer os.Remove(vocabPath)

	tok, _ := NewTokenizer(vocabPath)

	inputIDs, mask, typeIDs := tok.Encode("Hello world", 10)

	// Expected: [CLS] hello world [SEP] [PAD]...
	// IDs: [CLS]=101 (in real bert), here: 12. hello=15, world=16, SEP=13, PAD=0
	// My mock vocab indices:
	// 0: [PAD]
	// 11: [UNK]
	// 12: [CLS]
	// 13: [SEP]
	// 14: [MASK]
	// 15: hello
	// 16: world

	expectedIDs := []int64{12, 15, 16, 13, 0, 0, 0, 0, 0, 0}
	expectedMask := []int64{1, 1, 1, 1, 0, 0, 0, 0, 0, 0}
	expectedType := []int64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	if !reflect.DeepEqual(inputIDs, expectedIDs) {
		t.Errorf("InputIDs = %v, want %v", inputIDs, expectedIDs)
	}
	if !reflect.DeepEqual(mask, expectedMask) {
		t.Errorf("AttentionMask = %v, want %v", mask, expectedMask)
	}
	if !reflect.DeepEqual(typeIDs, expectedType) {
		t.Errorf("TokenTypeIDs = %v, want %v", typeIDs, expectedType)
	}
}
