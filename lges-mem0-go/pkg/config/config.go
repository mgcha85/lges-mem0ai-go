package config

import (
	"os"
	"path/filepath"
)

// MemoryConfig holds all configuration for the Memory system.
type MemoryConfig struct {
	// LLM settings
	LLMModel  string // e.g., "gpt-4.1-nano-2025-04-14"
	LLMAPIKey string

	// Embedding settings
	EmbeddingProvider string // "openai" or "onnx"
	EmbeddingModel    string // e.g., "text-embedding-3-small"
	EmbeddingDims     int    // e.g., 1536

	// ONNX settings
	OnnxModelPath string
	OnnxVocabPath string

	// Vector store settings
	QdrantHost     string
	QdrantPort     int
	CollectionName string

	// History DB
	HistoryDBPath string
}

// DefaultConfig returns a MemoryConfig with sensible defaults.
func DefaultConfig() MemoryConfig {
	homeDir, _ := os.UserHomeDir()
	mem0Dir := os.Getenv("MEM0_DIR")
	if mem0Dir == "" {
		mem0Dir = filepath.Join(homeDir, ".mem0-go")
	}
	_ = os.MkdirAll(filepath.Join(mem0Dir, "models"), 0o755)

	apiKey := os.Getenv("OPENAI_API_KEY")

	embeddingProvider := os.Getenv("EMBEDDING_PROVIDER")
	if embeddingProvider == "" {
		embeddingProvider = "openai"
	}

	onnxModelPath := os.Getenv("ONNX_MODEL_PATH")
	if onnxModelPath == "" {
		onnxModelPath = filepath.Join(mem0Dir, "models", "all-MiniLM-L6-v2.onnx")
	}

	onnxVocabPath := os.Getenv("ONNX_VOCAB_PATH")
	if onnxVocabPath == "" {
		onnxVocabPath = filepath.Join(mem0Dir, "models", "vocab.txt")
	}

	return MemoryConfig{
		LLMModel:          "gpt-4.1-nano-2025-04-14",
		LLMAPIKey:         apiKey,
		EmbeddingProvider: embeddingProvider,
		EmbeddingModel:    "text-embedding-3-small",
		EmbeddingDims:     1536,
		OnnxModelPath:     onnxModelPath,
		OnnxVocabPath:     onnxVocabPath,
		QdrantHost:        "localhost",
		QdrantPort:        6334,
		CollectionName:    "mem0-go",
		HistoryDBPath:     filepath.Join(mem0Dir, "history.db"),
	}
}
