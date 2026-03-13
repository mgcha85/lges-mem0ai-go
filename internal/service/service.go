package service

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/mgcha85/lges-mem0ai-go/internal/config"

	"github.com/mgcha85/lges-mem0ai-go/pkg/embeddings"
	"github.com/mgcha85/lges-mem0ai-go/pkg/llm"
	"github.com/mgcha85/lges-mem0ai-go/pkg/memory"
	"github.com/mgcha85/lges-mem0ai-go/pkg/store"
	"github.com/mgcha85/lges-mem0ai-go/pkg/vectorstore"
	openai "github.com/sashabaranov/go-openai"
)

// MemoryService wraps mem0-go's Memory and provides session-aware operations.
type MemoryService struct {
	mem *memory.Memory
}

// New creates a new MemoryService from the application config.
func New(cfg *config.Config) (*MemoryService, error) {
	// 1. Create LLM client (with custom base URL support for DeepSeek)
	var llmClient llm.LLM
	if cfg.OpenAIAPIBase != "" {
		llmClient = NewOpenAILLMWithBaseURL(cfg.OpenAIAPIKey, cfg.OpenAIAPIBase, cfg.OpenAIModel)
	} else {
		llmClient = llm.NewOpenAILLM(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	}

	// 2. Create embedder
	var embedder embeddings.Embedder
	switch cfg.EmbeddingProvider {
	case "onnx":
		var err error
		if cfg.OnnxModelDir != "" {
			// New E5-style embedder using tokenizer.json
			embedder, err = embeddings.NewOnnxE5Embedder(cfg.OnnxModelDir, cfg.EmbeddingDims)
		} else {
			// Legacy WordPiece-based embedder
			embedder, err = embeddings.NewOnnxEmbedder(cfg.OnnxModelPath, cfg.OnnxVocabPath)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create onnx embedder: %w", err)
		}
	default:
		embedder = embeddings.NewOpenAIEmbedder(cfg.OpenAIAPIKey, cfg.EmbeddingModel)
	}

	// 3. Create vector store
	var vs vectorstore.VectorStore
	var err error
	switch cfg.VectorDBProvider {
	case "sqlite":
		vs, err = vectorstore.NewSQLiteStore(filepath.Join(cfg.DataDir, "vector.db"), cfg.CollectionName, cfg.EmbeddingDims)
		if err != nil {
			return nil, fmt.Errorf("failed to create sqlite vector store: %w", err)
		}
	default:
		// Default to sqlite
		vs, err = vectorstore.NewSQLiteStore(filepath.Join(cfg.DataDir, "vector.db"), cfg.CollectionName, cfg.EmbeddingDims)
		if err != nil {
			return nil, fmt.Errorf("failed to create sqlite vector store: %w", err)
		}
	}

	// 4. Create history DB
	historyDBPath := cfg.DataDir + "/history.db"
	historyDB, err := store.NewSQLiteManager(historyDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create history db: %w", err)
	}

	// 5. Assemble Memory with injected dependencies
	mem := memory.NewWithDeps(llmClient, embedder, vs, historyDB)

	// 6. Warm up AI model (ONNX) to ensure it is in memory
	if cfg.EmbeddingProvider == "onnx" {
		log.Println("[MemoryService] Warming up ONNX embedding model...")
		_, err := embedder.Embed("warmup")
		if err != nil {
			log.Printf("[MemoryService] Warning: model warmup failed: %v", err)
		} else {
			log.Println("[MemoryService] ONNX model warmed up successfully.")
		}
	}

	log.Printf("[MemoryService] Initialized (LLM: %s, Embedder: %s, VectorDB: %s)",
		cfg.OpenAIModel, cfg.EmbeddingProvider, cfg.VectorDBProvider)

	return &MemoryService{mem: mem}, nil
}

// AddMemory adds messages to memory for a given user ID.
func (s *MemoryService) AddMemory(messages []memory.Message, userID string) (*memory.AddResponse, error) {
	return s.mem.Add(messages, userID)
}

// GetAllMemories retrieves all memories for a user.
func (s *MemoryService) GetAllMemories(userID string) ([]memory.MemoryResult, error) {
	return s.mem.GetAll(userID, 100)
}

// SearchMemories searches memories relevant to a query.
func (s *MemoryService) SearchMemories(query, userID string, limit int) (*memory.SearchResponse, error) {
	return s.mem.Search(query, userID, limit)
}

// GetUserMemories retrieves all memories for a user across all sessions by matching the prefix.
func (s *MemoryService) GetUserMemories(userID string) ([]memory.MemoryResult, error) {
	// Since we use userID + "_" + sessionID as the key in mem0-go
	// we use Search with a broad filter if possible, or GetAll and filter manually.
	// mem0-go doesn't support prefix search directly in GetAll, but let's check
	// actually we can use GetAll and filter in our service.
	
	// Get a large number of memories and filter by user prefix
	all, err := s.mem.GetAll("", 1000) // "" returns all
	if err != nil {
		return nil, err
	}
	
	var userMemories []memory.MemoryResult
	prefix := userID + "_"
	for _, m := range all {
		if strings.HasPrefix(m.UserID, prefix) || m.UserID == userID {
			userMemories = append(userMemories, m)
		}
	}
	return userMemories, nil
}

// DeleteMemory deletes a specific memory by ID.
func (s *MemoryService) DeleteMemory(memoryID string) error {
	return s.mem.Delete(memoryID)
}

// DeleteAllMemories deletes all memories (via reset).
func (s *MemoryService) DeleteAllMemories(userID string) error {
	// mem0-go doesn't have a per-user delete, so we get all and delete individually
	memories, err := s.mem.GetAll(userID, 1000)
	if err != nil {
		return err
	}
	for _, m := range memories {
		if err := s.mem.Delete(m.ID); err != nil {
			log.Printf("Warning: failed to delete memory %s: %v", m.ID, err)
		}
	}
	return nil
}

// Close cleans up resources.
func (s *MemoryService) Close() error {
	return s.mem.Close()
}

// === Custom OpenAI LLM with Base URL support ===

// openAILLMCustom implements llm.LLM interface with custom base URL support.
type openAILLMCustom struct {
	client *openai.Client
	model  string
}

// NewOpenAILLMWithBaseURL creates an OpenAI LLM client pointing to a custom API endpoint.
func NewOpenAILLMWithBaseURL(apiKey, baseURL, model string) llm.LLM {
	clientCfg := openai.DefaultConfig(apiKey)
	clientCfg.BaseURL = baseURL
	client := openai.NewClientWithConfig(clientCfg)

	return &openAILLMCustom{
		client: client,
		model:  model,
	}
}

// GenerateResponse sends messages to the custom LLM endpoint.
func (o *openAILLMCustom) GenerateResponse(messages []llm.Message, jsonMode bool) (string, error) {
	var chatMessages []openai.ChatCompletionMessage
	for _, m := range messages {
		chatMessages = append(chatMessages, openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	req := openai.ChatCompletionRequest{
		Model:    o.model,
		Messages: chatMessages,
	}

	if jsonMode {
		req.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}

	resp, err := o.client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("llm chat completion error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}

	return resp.Choices[0].Message.Content, nil
}
