package memory

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mem0ai/mem0-go/pkg/config"
	"github.com/mem0ai/mem0-go/pkg/embeddings"
	"github.com/mem0ai/mem0-go/pkg/llm"
	"github.com/mem0ai/mem0-go/pkg/prompts"
	"github.com/mem0ai/mem0-go/pkg/store"
	"github.com/mem0ai/mem0-go/pkg/vectorstore"
)

// MemoryResult represents a single memory item returned from operations.
type MemoryResult struct {
	ID             string  `json:"id"`
	Memory         string  `json:"memory"`
	Hash           string  `json:"hash,omitempty"`
	CreatedAt      string  `json:"created_at,omitempty"`
	UpdatedAt      string  `json:"updated_at,omitempty"`
	UserID         string  `json:"user_id,omitempty"`
	Score          float32 `json:"score,omitempty"`
	Event          string  `json:"event,omitempty"`
	PreviousMemory string  `json:"previous_memory,omitempty"`
}

// AddResponse is the result of an Add operation.
type AddResponse struct {
	Results []MemoryResult `json:"results"`
}

// SearchResponse is the result of a Search operation.
type SearchResponse struct {
	Results []MemoryResult `json:"results"`
}

// Message represents a chat message.
type Message = llm.Message

// Memory is the core memory management system.
type Memory struct {
	llm         llm.LLM
	embedder    embeddings.Embedder
	vectorStore vectorstore.VectorStore
	historyDB   *store.SQLiteManager
	config      config.MemoryConfig
}

// New creates a new Memory instance with the given configuration.
func New(cfg config.MemoryConfig) (*Memory, error) {
	llmClient := llm.NewOpenAILLM(cfg.LLMAPIKey, cfg.LLMModel)

	var embedder embeddings.Embedder
	var err error

	if cfg.EmbeddingProvider == "onnx" {
		embedder, err = embeddings.NewOnnxEmbedder(cfg.OnnxModelPath, cfg.OnnxVocabPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create onnx embedder: %w", err)
		}
	} else {
		embedder = embeddings.NewOpenAIEmbedder(cfg.LLMAPIKey, cfg.EmbeddingModel)
	}

	vs, err := vectorstore.NewQdrantStore(cfg.QdrantHost, cfg.QdrantPort, cfg.CollectionName, cfg.EmbeddingDims)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	db, err := store.NewSQLiteManager(cfg.HistoryDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create history db: %w", err)
	}

	return &Memory{
		llm:         llmClient,
		embedder:    embedder,
		vectorStore: vs,
		historyDB:   db,
		config:      cfg,
	}, nil
}

// NewWithDeps creates a Memory with injected dependencies (for testing).
func NewWithDeps(l llm.LLM, e embeddings.Embedder, vs vectorstore.VectorStore, db *store.SQLiteManager) *Memory {
	return &Memory{
		llm:         l,
		embedder:    e,
		vectorStore: vs,
		historyDB:   db,
	}
}

// Add processes messages to extract facts and store them as memories.
func (m *Memory) Add(messages []Message, userID string) (*AddResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Parse messages into text
	parsedMessages := parseMessages(messages)

	// Step 1: Extract facts via LLM
	systemPrompt := prompts.UserMemoryExtractionPrompt()
	userPrompt := fmt.Sprintf("Input:\n%s", parsedMessages)

	response, err := m.llm.GenerateResponse([]Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, true)
	if err != nil {
		return nil, fmt.Errorf("fact extraction LLM error: %w", err)
	}

	response = removeCodeBlocks(response)

	var factsResp struct {
		Facts []string `json:"facts"`
	}
	if err := json.Unmarshal([]byte(response), &factsResp); err != nil {
		// Try to extract JSON from response
		extracted := extractJSON(response)
		if err2 := json.Unmarshal([]byte(extracted), &factsResp); err2 != nil {
			log.Printf("Failed to parse facts response: %v, raw: %s", err2, response)
			return &AddResponse{Results: []MemoryResult{}}, nil
		}
	}

	if len(factsResp.Facts) == 0 {
		log.Println("No new facts retrieved from input. Skipping memory update.")
		return &AddResponse{Results: []MemoryResult{}}, nil
	}

	// Step 2: Search for existing memories matching each new fact
	retrievedOldMemory := make([]map[string]string, 0)
	newMessageEmbeddings := make(map[string][]float32)
	filters := map[string]string{"user_id": userID}

	for _, fact := range factsResp.Facts {
		embedding, err := m.embedder.Embed(fact)
		if err != nil {
			log.Printf("Embedding error for fact %q: %v", fact, err)
			continue
		}
		newMessageEmbeddings[fact] = embedding

		existingMemories, err := m.vectorStore.Search(fact, embedding, 5, filters)
		if err != nil {
			log.Printf("Vector search error: %v", err)
			continue
		}

		for _, mem := range existingMemories {
			data, _ := mem.Payload["data"].(string)
			retrievedOldMemory = append(retrievedOldMemory, map[string]string{
				"id":   mem.ID,
				"text": data,
			})
		}
	}

	// Deduplicate old memories by ID
	uniqueMemories := make(map[string]map[string]string)
	for _, mem := range retrievedOldMemory {
		uniqueMemories[mem["id"]] = mem
	}
	retrievedOldMemory = make([]map[string]string, 0, len(uniqueMemories))
	for _, mem := range uniqueMemories {
		retrievedOldMemory = append(retrievedOldMemory, mem)
	}
	log.Printf("Total existing memories: %d", len(retrievedOldMemory))

	// Step 3: Map UUIDs to integer IDs (to avoid LLM hallucinating UUIDs)
	tempUUIDMapping := make(map[string]string)
	for idx, item := range retrievedOldMemory {
		tempUUIDMapping[fmt.Sprintf("%d", idx)] = item["id"]
		retrievedOldMemory[idx]["id"] = fmt.Sprintf("%d", idx)
	}

	// Step 4: Ask LLM to decide ADD/UPDATE/DELETE/NONE
	updatePrompt := prompts.GetUpdateMemoryMessages(retrievedOldMemory, factsResp.Facts)
	updateResponse, err := m.llm.GenerateResponse([]Message{
		{Role: "user", Content: updatePrompt},
	}, true)
	if err != nil {
		log.Printf("Memory update LLM error: %v", err)
		return &AddResponse{Results: []MemoryResult{}}, nil
	}

	updateResponse = removeCodeBlocks(updateResponse)

	var memoryActions struct {
		Memory []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			Event     string `json:"event"`
			OldMemory string `json:"old_memory,omitempty"`
		} `json:"memory"`
	}
	if err := json.Unmarshal([]byte(updateResponse), &memoryActions); err != nil {
		extracted := extractJSON(updateResponse)
		if err2 := json.Unmarshal([]byte(extracted), &memoryActions); err2 != nil {
			log.Printf("Failed to parse memory actions: %v, raw: %s", err2, updateResponse)
			return &AddResponse{Results: []MemoryResult{}}, nil
		}
	}

	// Step 5: Apply actions
	var results []MemoryResult
	metadata := map[string]interface{}{"user_id": userID}

	for _, action := range memoryActions.Memory {
		if action.Text == "" {
			continue
		}

		switch action.Event {
		case "ADD":
			memID, err := m.createMemory(action.Text, newMessageEmbeddings, metadata)
			if err != nil {
				log.Printf("Error creating memory: %v", err)
				continue
			}
			results = append(results, MemoryResult{
				ID:     memID,
				Memory: action.Text,
				Event:  "ADD",
			})

		case "UPDATE":
			realID, ok := tempUUIDMapping[action.ID]
			if !ok {
				log.Printf("UUID mapping not found for ID %s", action.ID)
				continue
			}
			err := m.updateMemory(realID, action.Text, newMessageEmbeddings, metadata)
			if err != nil {
				log.Printf("Error updating memory: %v", err)
				continue
			}
			results = append(results, MemoryResult{
				ID:             realID,
				Memory:         action.Text,
				Event:          "UPDATE",
				PreviousMemory: action.OldMemory,
			})

		case "DELETE":
			realID, ok := tempUUIDMapping[action.ID]
			if !ok {
				log.Printf("UUID mapping not found for ID %s", action.ID)
				continue
			}
			err := m.deleteMemory(realID)
			if err != nil {
				log.Printf("Error deleting memory: %v", err)
				continue
			}
			results = append(results, MemoryResult{
				ID:     realID,
				Memory: action.Text,
				Event:  "DELETE",
			})

		case "NONE":
			log.Println("NOOP for Memory.")
		}
	}

	return &AddResponse{Results: results}, nil
}

// Search finds memories relevant to a query.
func (m *Memory) Search(query string, userID string, limit int) (*SearchResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if limit <= 0 {
		limit = 100
	}

	embedding, err := m.embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embedding error: %w", err)
	}

	filters := map[string]string{"user_id": userID}
	searchResults, err := m.vectorStore.Search(query, embedding, limit, filters)
	if err != nil {
		return nil, fmt.Errorf("vector search error: %w", err)
	}

	var results []MemoryResult
	for _, r := range searchResults {
		data, _ := r.Payload["data"].(string)
		hash, _ := r.Payload["hash"].(string)
		createdAt, _ := r.Payload["created_at"].(string)
		updatedAt, _ := r.Payload["updated_at"].(string)

		results = append(results, MemoryResult{
			ID:        r.ID,
			Memory:    data,
			Hash:      hash,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			Score:     r.Score,
		})
	}

	return &SearchResponse{Results: results}, nil
}

// Get retrieves a single memory by ID.
func (m *Memory) Get(memoryID string) (*MemoryResult, error) {
	record, err := m.vectorStore.Get(memoryID)
	if err != nil {
		return nil, err
	}

	data, _ := record.Payload["data"].(string)
	hash, _ := record.Payload["hash"].(string)
	createdAt, _ := record.Payload["created_at"].(string)
	updatedAt, _ := record.Payload["updated_at"].(string)
	userID, _ := record.Payload["user_id"].(string)

	return &MemoryResult{
		ID:        record.ID,
		Memory:    data,
		Hash:      hash,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		UserID:    userID,
	}, nil
}

// GetAll retrieves all memories for a user.
func (m *Memory) GetAll(userID string, limit int) ([]MemoryResult, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if limit <= 0 {
		limit = 100
	}

	filters := map[string]string{"user_id": userID}
	records, err := m.vectorStore.List(filters, limit)
	if err != nil {
		return nil, fmt.Errorf("vector store list error: %w", err)
	}

	var results []MemoryResult
	for _, rec := range records {
		data, _ := rec.Payload["data"].(string)
		hash, _ := rec.Payload["hash"].(string)
		createdAt, _ := rec.Payload["created_at"].(string)
		updatedAt, _ := rec.Payload["updated_at"].(string)

		results = append(results, MemoryResult{
			ID:        rec.ID,
			Memory:    data,
			Hash:      hash,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			UserID:    userID,
		})
	}

	return results, nil
}

// Delete removes a memory by ID.
func (m *Memory) Delete(memoryID string) error {
	return m.deleteMemory(memoryID)
}

// Reset clears all memories and history.
func (m *Memory) Reset() error {
	if err := m.vectorStore.Reset(); err != nil {
		return fmt.Errorf("vector store reset error: %w", err)
	}
	if err := m.historyDB.Reset(); err != nil {
		return fmt.Errorf("history db reset error: %w", err)
	}
	return nil
}

// History returns change history for a memory.
func (m *Memory) History(memoryID string) ([]store.HistoryRecord, error) {
	return m.historyDB.GetHistory(memoryID)
}

// Close cleans up resources.
func (m *Memory) Close() error {
	if qs, ok := m.vectorStore.(*vectorstore.QdrantStore); ok {
		qs.Close()
	}
	if m.historyDB != nil {
		m.historyDB.Close()
	}
	return nil
}

// --- Internal helpers ---

func (m *Memory) createMemory(data string, existingEmbeddings map[string][]float32, metadata map[string]interface{}) (string, error) {
	var emb []float32
	var err error
	if cached, ok := existingEmbeddings[data]; ok {
		emb = cached
	} else {
		emb, err = m.embedder.Embed(data)
		if err != nil {
			return "", fmt.Errorf("embedding error: %w", err)
		}
	}

	memID := uuid.New().String()
	meta := copyMetadata(metadata)
	meta["data"] = data
	meta["hash"] = fmt.Sprintf("%x", md5.Sum([]byte(data)))
	createdAt := time.Now().Format(time.RFC3339)
	meta["created_at"] = createdAt

	if err := m.vectorStore.Insert([][]float32{emb}, []string{memID}, []map[string]interface{}{meta}); err != nil {
		return "", fmt.Errorf("vector store insert error: %w", err)
	}

	newVal := data
	m.historyDB.AddHistory(memID, nil, &newVal, "ADD", &createdAt, nil, 0)
	return memID, nil
}

func (m *Memory) updateMemory(memoryID, data string, existingEmbeddings map[string][]float32, metadata map[string]interface{}) error {
	existing, err := m.vectorStore.Get(memoryID)
	if err != nil {
		return fmt.Errorf("memory not found for update: %w", err)
	}

	prevValue, _ := existing.Payload["data"].(string)

	var emb []float32
	if cached, ok := existingEmbeddings[data]; ok {
		emb = cached
	} else {
		emb, err = m.embedder.Embed(data)
		if err != nil {
			return fmt.Errorf("embedding error: %w", err)
		}
	}

	meta := copyMetadata(metadata)
	meta["data"] = data
	meta["hash"] = fmt.Sprintf("%x", md5.Sum([]byte(data)))
	if ca, ok := existing.Payload["created_at"].(string); ok {
		meta["created_at"] = ca
	}
	updatedAt := time.Now().Format(time.RFC3339)
	meta["updated_at"] = updatedAt

	// Preserve user_id from existing if not in metadata
	if _, ok := meta["user_id"]; !ok {
		if uid, ok := existing.Payload["user_id"]; ok {
			meta["user_id"] = uid
		}
	}

	if err := m.vectorStore.Update(memoryID, emb, meta); err != nil {
		return fmt.Errorf("vector store update error: %w", err)
	}

	createdAt, _ := existing.Payload["created_at"].(string)
	m.historyDB.AddHistory(memoryID, &prevValue, &data, "UPDATE", &createdAt, &updatedAt, 0)
	return nil
}

func (m *Memory) deleteMemory(memoryID string) error {
	existing, err := m.vectorStore.Get(memoryID)
	if err != nil {
		return fmt.Errorf("memory not found for delete: %w", err)
	}

	prevValue, _ := existing.Payload["data"].(string)

	if err := m.vectorStore.Delete(memoryID); err != nil {
		return fmt.Errorf("vector store delete error: %w", err)
	}

	m.historyDB.AddHistory(memoryID, &prevValue, nil, "DELETE", nil, nil, 1)
	return nil
}

// --- Utility functions ---

func parseMessages(messages []Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			sb.WriteString(fmt.Sprintf("system: %s\n", msg.Content))
		case "user":
			sb.WriteString(fmt.Sprintf("user: %s\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("assistant: %s\n", msg.Content))
		}
	}
	return sb.String()
}

func removeCodeBlocks(content string) string {
	content = strings.TrimSpace(content)
	re := regexp.MustCompile("(?s)^```[a-zA-Z0-9]*\n(.*?)\n```$")
	match := re.FindStringSubmatch(content)
	if match != nil {
		content = strings.TrimSpace(match[1])
	}
	// Remove <think> blocks
	thinkRe := regexp.MustCompile("(?s)<think>.*?</think>")
	content = thinkRe.ReplaceAllString(content, "")
	return strings.TrimSpace(content)
}

func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	re := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
	match := re.FindStringSubmatch(text)
	if match != nil {
		return match[1]
	}
	return text
}

func copyMetadata(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}
