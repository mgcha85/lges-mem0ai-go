package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mem0ai/mem0-go/pkg/llm"
	"github.com/mem0ai/mem0-go/pkg/store"
	"github.com/mem0ai/mem0-go/pkg/vectorstore"
)

// --- Mock implementations ---

// MockLLM simulates LLM responses for testing.
type MockLLM struct {
	Responses []string
	CallIndex int
	Calls     [][]llm.Message
}

func (m *MockLLM) GenerateResponse(messages []llm.Message, jsonMode bool) (string, error) {
	m.Calls = append(m.Calls, messages)
	if m.CallIndex >= len(m.Responses) {
		return `{"facts": []}`, nil
	}
	resp := m.Responses[m.CallIndex]
	m.CallIndex++
	return resp, nil
}

// MockEmbedder returns deterministic embeddings for testing.
type MockEmbedder struct {
	EmbedCalls []string
}

func (m *MockEmbedder) Embed(text string) ([]float32, error) {
	m.EmbedCalls = append(m.EmbedCalls, text)
	// Generate a simple deterministic embedding based on text hash
	vec := make([]float32, 4)
	for i, c := range text {
		vec[i%4] += float32(c) / 1000.0
	}
	return vec, nil
}

func (m *MockEmbedder) Close() error {
	return nil
}

// MockVectorStore provides an in-memory vector store for testing.
type MockVectorStore struct {
	Data map[string]mockEntry
}

type mockEntry struct {
	Vector  []float32
	Payload map[string]interface{}
}

func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{Data: make(map[string]mockEntry)}
}

func (m *MockVectorStore) Insert(vectors [][]float32, ids []string, payloads []map[string]interface{}) error {
	for i := range ids {
		m.Data[ids[i]] = mockEntry{Vector: vectors[i], Payload: payloads[i]}
	}
	return nil
}

func (m *MockVectorStore) Search(query string, vector []float32, limit int, filters map[string]string) ([]vectorstore.SearchResult, error) {
	var results []vectorstore.SearchResult
	for id, entry := range m.Data {
		// Check filters
		match := true
		for k, v := range filters {
			payloadVal, _ := entry.Payload[k].(string)
			if payloadVal != v {
				match = false
				break
			}
		}
		if !match {
			continue
		}

		results = append(results, vectorstore.SearchResult{
			ID:      id,
			Score:   0.9,
			Payload: entry.Payload,
		})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (m *MockVectorStore) Get(id string) (*vectorstore.VectorRecord, error) {
	entry, ok := m.Data[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return &vectorstore.VectorRecord{ID: id, Payload: entry.Payload}, nil
}

func (m *MockVectorStore) List(filters map[string]string, limit int) ([]vectorstore.VectorRecord, error) {
	var records []vectorstore.VectorRecord
	for id, entry := range m.Data {
		match := true
		for k, v := range filters {
			payloadVal, _ := entry.Payload[k].(string)
			if payloadVal != v {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		records = append(records, vectorstore.VectorRecord{ID: id, Payload: entry.Payload})
		if len(records) >= limit {
			break
		}
	}
	return records, nil
}

func (m *MockVectorStore) Update(id string, vector []float32, payload map[string]interface{}) error {
	entry, ok := m.Data[id]
	if !ok {
		return fmt.Errorf("not found: %s", id)
	}
	if vector != nil {
		entry.Vector = vector
	}
	if payload != nil {
		entry.Payload = payload
	}
	m.Data[id] = entry
	return nil
}

func (m *MockVectorStore) Delete(id string) error {
	delete(m.Data, id)
	return nil
}

func (m *MockVectorStore) Reset() error {
	m.Data = make(map[string]mockEntry)
	return nil
}

// --- Helper ---

func newTestMemory(t *testing.T, mockLLM *MockLLM) (*Memory, *MockEmbedder, *MockVectorStore) {
	t.Helper()
	embedder := &MockEmbedder{}
	vs := NewMockVectorStore()
	dir := t.TempDir()
	db, err := store.NewSQLiteManager(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mem := NewWithDeps(mockLLM, embedder, vs, db)
	return mem, embedder, vs
}

// --- Tests ---

func TestAdd_NewFacts(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			// First call: fact extraction
			`{"facts": ["Name is John", "Is a software engineer"]}`,
			// Second call: memory update decision (no existing memories)
			`{"memory": [{"id": "0", "text": "Name is John", "event": "ADD"}, {"id": "1", "text": "Is a software engineer", "event": "ADD"}]}`,
		},
	}

	mem, embedder, vs := newTestMemory(t, mockLLM)

	messages := []Message{
		{Role: "user", Content: "Hi, my name is John. I am a software engineer."},
		{Role: "assistant", Content: "Nice to meet you, John!"},
	}

	resp, err := mem.Add(messages, "user1")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Should have 2 ADD results
	if len(resp.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(resp.Results))
	}

	for _, r := range resp.Results {
		if r.Event != "ADD" {
			t.Errorf("Expected event ADD, got %s", r.Event)
		}
	}

	// Check embedder was called for both facts
	if len(embedder.EmbedCalls) < 2 {
		t.Errorf("Expected at least 2 embed calls, got %d", len(embedder.EmbedCalls))
	}

	// Check vector store has 2 entries
	if len(vs.Data) != 2 {
		t.Errorf("Expected 2 entries in vector store, got %d", len(vs.Data))
	}

	// Verify stored data
	for _, entry := range vs.Data {
		data, _ := entry.Payload["data"].(string)
		if data != "Name is John" && data != "Is a software engineer" {
			t.Errorf("Unexpected stored data: %s", data)
		}
		uid, _ := entry.Payload["user_id"].(string)
		if uid != "user1" {
			t.Errorf("Expected user_id user1, got %s", uid)
		}
	}
}

func TestAdd_EmptyFacts(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": []}`,
		},
	}

	mem, _, _ := newTestMemory(t, mockLLM)

	messages := []Message{
		{Role: "user", Content: "Hi"},
	}

	resp, err := mem.Add(messages, "user1")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if len(resp.Results) != 0 {
		t.Errorf("Expected 0 results for empty facts, got %d", len(resp.Results))
	}
}

func TestAdd_UpdateExistingMemory(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			// Initial add
			`{"facts": ["Likes pizza"]}`,
			`{"memory": [{"id": "0", "text": "Likes pizza", "event": "ADD"}]}`,
		},
	}

	mem, _, vs := newTestMemory(t, mockLLM)

	// First: add a memory
	resp1, _ := mem.Add([]Message{{Role: "user", Content: "I like pizza"}}, "user1")
	if len(resp1.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resp1.Results))
	}
	firstID := resp1.Results[0].ID

	// Now update via new conversation
	mockLLM.CallIndex = 0
	mockLLM.Responses = []string{
		`{"facts": ["Loves pizza and pasta"]}`,
		`{"memory": [{"id": "0", "text": "Loves pizza and pasta", "event": "UPDATE", "old_memory": "Likes pizza"}]}`,
	}

	resp2, err := mem.Add([]Message{{Role: "user", Content: "I also love pasta"}}, "user1")
	if err != nil {
		t.Fatalf("Add (update) failed: %v", err)
	}

	if len(resp2.Results) != 1 {
		t.Fatalf("Expected 1 update result, got %d", len(resp2.Results))
	}

	r := resp2.Results[0]
	if r.Event != "UPDATE" {
		t.Errorf("Expected UPDATE event, got %s", r.Event)
	}
	if r.ID != firstID {
		t.Errorf("Expected updated ID %s, got %s", firstID, r.ID)
	}
	if r.Memory != "Loves pizza and pasta" {
		t.Errorf("Expected updated memory text, got %s", r.Memory)
	}

	// Verify vector store has the updated data
	entry, ok := vs.Data[firstID]
	if !ok {
		t.Fatal("Expected entry in vector store")
	}
	data, _ := entry.Payload["data"].(string)
	if data != "Loves pizza and pasta" {
		t.Errorf("Expected updated data in vector store, got %s", data)
	}
}

func TestAdd_DeleteMemory(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Likes cheese pizza"]}`,
			`{"memory": [{"id": "0", "text": "Likes cheese pizza", "event": "ADD"}]}`,
		},
	}

	mem, _, vs := newTestMemory(t, mockLLM)

	// Add memory
	resp1, _ := mem.Add([]Message{{Role: "user", Content: "I like cheese pizza"}}, "user1")
	firstID := resp1.Results[0].ID

	// Delete via contradiction
	mockLLM.CallIndex = 0
	mockLLM.Responses = []string{
		`{"facts": ["Dislikes cheese pizza"]}`,
		`{"memory": [{"id": "0", "text": "Likes cheese pizza", "event": "DELETE"}]}`,
	}

	resp2, err := mem.Add([]Message{{Role: "user", Content: "Actually I hate cheese pizza"}}, "user1")
	if err != nil {
		t.Fatalf("Add (delete) failed: %v", err)
	}

	if len(resp2.Results) != 1 {
		t.Fatalf("Expected 1 delete result, got %d", len(resp2.Results))
	}

	if resp2.Results[0].Event != "DELETE" {
		t.Errorf("Expected DELETE event, got %s", resp2.Results[0].Event)
	}

	// Verify vector store no longer has the entry
	if _, ok := vs.Data[firstID]; ok {
		t.Error("Expected entry to be deleted from vector store")
	}
}

func TestAdd_RequiresUserID(t *testing.T) {
	mockLLM := &MockLLM{}
	mem, _, _ := newTestMemory(t, mockLLM)

	_, err := mem.Add([]Message{{Role: "user", Content: "test"}}, "")
	if err == nil {
		t.Error("Expected error for empty user_id")
	}
}

func TestSearch_Basic(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Name is Alice"]}`,
			`{"memory": [{"id": "0", "text": "Name is Alice", "event": "ADD"}]}`,
		},
	}

	mem, _, _ := newTestMemory(t, mockLLM)

	// Add a memory
	mem.Add([]Message{{Role: "user", Content: "My name is Alice"}}, "user1")

	// Search
	results, err := mem.Search("What is the user's name?", "user1", 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results.Results))
	}

	if results.Results[0].Memory != "Name is Alice" {
		t.Errorf("Expected 'Name is Alice', got %s", results.Results[0].Memory)
	}

	if results.Results[0].Score <= 0 {
		t.Errorf("Expected positive score, got %f", results.Results[0].Score)
	}
}

func TestSearch_NoResults(t *testing.T) {
	mockLLM := &MockLLM{}
	mem, _, _ := newTestMemory(t, mockLLM)

	results, err := mem.Search("anything", "user1", 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results.Results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results.Results))
	}
}

func TestSearch_RequiresUserID(t *testing.T) {
	mockLLM := &MockLLM{}
	mem, _, _ := newTestMemory(t, mockLLM)

	_, err := mem.Search("test", "", 3)
	if err == nil {
		t.Error("Expected error for empty user_id")
	}
}

func TestGet_ExistingMemory(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Hobby is gardening"]}`,
			`{"memory": [{"id": "0", "text": "Hobby is gardening", "event": "ADD"}]}`,
		},
	}

	mem, _, _ := newTestMemory(t, mockLLM)

	resp, _ := mem.Add([]Message{{Role: "user", Content: "I love gardening"}}, "user1")
	memID := resp.Results[0].ID

	result, err := mem.Get(memID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if result.Memory != "Hobby is gardening" {
		t.Errorf("Expected 'Hobby is gardening', got %s", result.Memory)
	}
	if result.ID != memID {
		t.Errorf("Expected ID %s, got %s", memID, result.ID)
	}
}

func TestGet_NotFound(t *testing.T) {
	mockLLM := &MockLLM{}
	mem, _, _ := newTestMemory(t, mockLLM)

	_, err := mem.Get("nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent memory")
	}
}

func TestGetAll(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Name is Bob", "Age is 30"]}`,
			`{"memory": [{"id": "0", "text": "Name is Bob", "event": "ADD"}, {"id": "1", "text": "Age is 30", "event": "ADD"}]}`,
		},
	}

	mem, _, _ := newTestMemory(t, mockLLM)

	mem.Add([]Message{{Role: "user", Content: "I'm Bob, 30 years old"}}, "user1")

	results, err := mem.GetAll("user1", 10)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	memories := map[string]bool{}
	for _, r := range results {
		memories[r.Memory] = true
	}
	if !memories["Name is Bob"] {
		t.Error("Expected 'Name is Bob' in results")
	}
	if !memories["Age is 30"] {
		t.Error("Expected 'Age is 30' in results")
	}
}

func TestGetAll_UserIsolation(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["User1 fact"]}`,
			`{"memory": [{"id": "0", "text": "User1 fact", "event": "ADD"}]}`,
		},
	}

	mem, _, _ := newTestMemory(t, mockLLM)

	mem.Add([]Message{{Role: "user", Content: "User1 info"}}, "user1")

	// User2 should have no memories
	results, err := mem.GetAll("user2", 10)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for user2, got %d", len(results))
	}
}

func TestDelete(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Temp fact"]}`,
			`{"memory": [{"id": "0", "text": "Temp fact", "event": "ADD"}]}`,
		},
	}

	mem, _, vs := newTestMemory(t, mockLLM)

	resp, _ := mem.Add([]Message{{Role: "user", Content: "temp"}}, "user1")
	memID := resp.Results[0].ID

	err := mem.Delete(memID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, ok := vs.Data[memID]; ok {
		t.Error("Expected memory to be deleted from vector store")
	}
}

func TestReset(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Something"]}`,
			`{"memory": [{"id": "0", "text": "Something", "event": "ADD"}]}`,
		},
	}

	mem, _, vs := newTestMemory(t, mockLLM)

	mem.Add([]Message{{Role: "user", Content: "something"}}, "user1")

	if len(vs.Data) == 0 {
		t.Fatal("Expected data in vector store before reset")
	}

	err := mem.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	if len(vs.Data) != 0 {
		t.Error("Expected vector store to be empty after reset")
	}
}

func TestHistory(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Likes cats"]}`,
			`{"memory": [{"id": "0", "text": "Likes cats", "event": "ADD"}]}`,
		},
	}

	mem, _, _ := newTestMemory(t, mockLLM)

	resp, _ := mem.Add([]Message{{Role: "user", Content: "I like cats"}}, "user1")
	memID := resp.Results[0].ID

	history, err := mem.History(memID)
	if err != nil {
		t.Fatalf("History failed: %v", err)
	}

	if len(history) != 1 {
		t.Fatalf("Expected 1 history entry, got %d", len(history))
	}

	if history[0].Event != "ADD" {
		t.Errorf("Expected ADD event, got %s", history[0].Event)
	}
}

func TestParseMessages(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	result := parseMessages(messages)
	if !strings.Contains(result, "system: You are helpful") {
		t.Error("Expected system message in parsed output")
	}
	if !strings.Contains(result, "user: Hello") {
		t.Error("Expected user message in parsed output")
	}
	if !strings.Contains(result, "assistant: Hi there") {
		t.Error("Expected assistant message in parsed output")
	}
}

func TestRemoveCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no code block",
			input:    `{"facts": []}`,
			expected: `{"facts": []}`,
		},
		{
			name:     "json code block",
			input:    "```json\n{\"facts\": []}\n```",
			expected: `{"facts": []}`,
		},
		{
			name:     "plain code block",
			input:    "```\n{\"facts\": []}\n```",
			expected: `{"facts": []}`,
		},
		{
			name:     "with think tags",
			input:    "<think>thinking...</think>{\"facts\": []}",
			expected: `{"facts": []}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeCodeBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "raw json",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json in code block",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON(tt.input)
			if strings.TrimSpace(result) != strings.TrimSpace(tt.expected) {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMultiTurnConversation(t *testing.T) {
	// Simulate a multi-turn conversation where information accumulates
	callIdx := 0
	responses := []string{
		// Turn 1: Extract name
		`{"facts": ["Name is Charlie"]}`,
		`{"memory": [{"id": "0", "text": "Name is Charlie", "event": "ADD"}]}`,
		// Turn 2: Extract job
		`{"facts": ["Works as a data scientist"]}`,
		`{"memory": [{"id": "0", "text": "Name is Charlie", "event": "NONE"}, {"id": "1", "text": "Works as a data scientist", "event": "ADD"}]}`,
		// Turn 3: Extract hobby
		`{"facts": ["Enjoys hiking on weekends"]}`,
		`{"memory": [{"id": "0", "text": "Name is Charlie", "event": "NONE"}, {"id": "1", "text": "Works as a data scientist", "event": "NONE"}, {"id": "2", "text": "Enjoys hiking on weekends", "event": "ADD"}]}`,
	}

	mockLLM := &MockLLM{
		Responses: responses,
	}

	mem, _, vs := newTestMemory(t, mockLLM)
	_ = callIdx

	// Turn 1
	resp1, err := mem.Add([]Message{
		{Role: "user", Content: "Hi, I'm Charlie"},
		{Role: "assistant", Content: "Nice to meet you, Charlie!"},
	}, "user1")
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	if len(resp1.Results) != 1 || resp1.Results[0].Memory != "Name is Charlie" {
		t.Errorf("Turn 1: Expected 'Name is Charlie', got %v", resp1.Results)
	}

	// Turn 2
	resp2, err := mem.Add([]Message{
		{Role: "user", Content: "I work as a data scientist"},
		{Role: "assistant", Content: "That's a great field!"},
	}, "user1")
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}
	if len(resp2.Results) != 1 || resp2.Results[0].Memory != "Works as a data scientist" {
		t.Errorf("Turn 2: Expected 'Works as a data scientist', got %v", resp2.Results)
	}

	// Turn 3
	resp3, err := mem.Add([]Message{
		{Role: "user", Content: "I enjoy hiking on weekends"},
		{Role: "assistant", Content: "Hiking is wonderful!"},
	}, "user1")
	if err != nil {
		t.Fatalf("Turn 3 failed: %v", err)
	}
	if len(resp3.Results) != 1 || resp3.Results[0].Memory != "Enjoys hiking on weekends" {
		t.Errorf("Turn 3: Expected 'Enjoys hiking on weekends', got %v", resp3.Results)
	}

	// Verify all 3 memories are stored
	if len(vs.Data) != 3 {
		t.Errorf("Expected 3 memories in store, got %d", len(vs.Data))
	}

	// Verify GetAll returns all 3
	allMemories, err := mem.GetAll("user1", 10)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(allMemories) != 3 {
		t.Errorf("Expected 3 memories, got %d", len(allMemories))
	}
}

func TestLLMCalledWithCorrectPrompts(t *testing.T) {
	mockLLM := &MockLLM{
		Responses: []string{
			`{"facts": ["Name is Test"]}`,
			`{"memory": [{"id": "0", "text": "Name is Test", "event": "ADD"}]}`,
		},
	}

	mem, _, _ := newTestMemory(t, mockLLM)

	mem.Add([]Message{
		{Role: "user", Content: "My name is Test"},
	}, "user1")

	// Verify fact extraction call
	if len(mockLLM.Calls) < 2 {
		t.Fatalf("Expected at least 2 LLM calls, got %d", len(mockLLM.Calls))
	}

	// First call should be fact extraction
	factCall := mockLLM.Calls[0]
	if len(factCall) != 2 {
		t.Fatalf("Expected 2 messages in fact extraction call, got %d", len(factCall))
	}
	if factCall[0].Role != "system" {
		t.Error("First message should be system role")
	}
	if !strings.Contains(factCall[0].Content, "Personal Information Organizer") {
		t.Error("System prompt should contain 'Personal Information Organizer'")
	}
	if factCall[1].Role != "user" {
		t.Error("Second message should be user role")
	}
	if !strings.Contains(factCall[1].Content, "My name is Test") {
		t.Error("User prompt should contain the user input")
	}

	// Second call should be memory update
	updateCall := mockLLM.Calls[1]
	if len(updateCall) != 1 {
		t.Fatalf("Expected 1 message in update call, got %d", len(updateCall))
	}
	if updateCall[0].Role != "user" {
		t.Error("Update call should be user role")
	}
	if !strings.Contains(updateCall[0].Content, "ADD") {
		t.Error("Update prompt should contain ADD operation")
	}
}

func TestAddResponse_JSONSerialization(t *testing.T) {
	resp := AddResponse{
		Results: []MemoryResult{
			{ID: "abc-123", Memory: "Test memory", Event: "ADD"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var decoded AddResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if len(decoded.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(decoded.Results))
	}
	if decoded.Results[0].ID != "abc-123" {
		t.Errorf("Expected ID abc-123, got %s", decoded.Results[0].ID)
	}
}

// TestIntegration_RealOpenAI is for testing with real OpenAI API.
// Skip if OPENAI_API_KEY is not set.
func TestIntegration_RealOpenAI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// This test requires a running Qdrant instance
	// Skip if not available
	t.Skip("Integration test requires running Qdrant and OpenAI API key")
}
