package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	// OpenAI/LLM settings
	OpenAIAPIKey  string
	OpenAIAPIBase string
	OpenAIModel   string

	// Embedding settings
	EmbeddingProvider string // "openai" or "onnx"
	EmbeddingModel    string
	EmbeddingDims     int

	// ONNX settings (when EmbeddingProvider == "onnx")
	OnnxModelDir  string // directory containing model.onnx + tokenizer.json
	OnnxModelPath string // legacy: direct path to .onnx file
	OnnxVocabPath string // legacy: direct path to vocab.txt

	// VectorDB settings
	VectorDBProvider string
	VectorDBPath     string // For sqlite
	CollectionName   string

	// Server
	ServerPort int

	// Data directory
	DataDir string
}

// Load reads configuration from a .env file and environment variables.
// Environment variables take precedence over .env file values.
func Load(envFile string) *Config {
	// Load .env file (if exists)
	loadDotEnv(envFile)

	execDir := filepath.Dir(envFile)
	dataDir := filepath.Join(execDir, "data")
	_ = os.MkdirAll(dataDir, 0o755)

	homeDir, _ := os.UserHomeDir()
	mem0Dir := getEnv("MEM0_DIR", filepath.Join(homeDir, ".mem0-go"))
	_ = os.MkdirAll(filepath.Join(mem0Dir, "models"), 0o755)

	return &Config{
		OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
		OpenAIAPIBase: getEnv("OPENAI_API_BASE", ""),
		OpenAIModel:   getEnv("OPENAI_MODEL", "gpt-4o"),

		EmbeddingProvider: getEnv("EMBEDDING_PROVIDER", "openai"),
		EmbeddingModel:    getEnv("EMBEDDING_MODEL", "intfloat/multilingual-e5-small"),
		EmbeddingDims:     getEnvInt("EMBEDDING_DIMS", 384),

		OnnxModelDir:  getEnv("ONNX_MODEL_DIR", filepath.Join(execDir, "models", "multilingual-e5-small")),
		OnnxModelPath: getEnv("ONNX_MODEL_PATH", filepath.Join(mem0Dir, "models", "all-MiniLM-L6-v2.onnx")),
		OnnxVocabPath: getEnv("ONNX_VOCAB_PATH", filepath.Join(mem0Dir, "models", "vocab.txt")),

		VectorDBProvider: getEnv("VECTORDB_PROVIDER", "sqlite"),
		VectorDBPath:     getEnv("VECTORDB_PATH", filepath.Join(mem0Dir, "vector.db")),
		CollectionName:   getEnv("COLLECTION_NAME", "lges-mem0ai-go"),

		ServerPort: getEnvInt("SERVER_PORT", 8080),

		DataDir: dataDir,
	}
}

// loadDotEnv reads a .env file and sets environment variables (does NOT override existing env vars).
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env file not found is okay
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
