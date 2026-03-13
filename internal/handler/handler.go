package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/mgcha85/lges-mem0ai-go/internal/config"
	"github.com/mgcha85/lges-mem0ai-go/internal/database"
	"github.com/mgcha85/lges-mem0ai-go/internal/models"
	"github.com/mgcha85/lges-mem0ai-go/internal/service"

	"github.com/mgcha85/lges-mem0ai-go/pkg/llm"
	openai "github.com/sashabaranov/go-openai"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	service *service.MemoryService
	db      *database.Database
	cfg     *config.Config
}

// New creates a new Handler.
func New(svc *service.MemoryService, db *database.Database, cfg *config.Config) *Handler {
	return &Handler{
		service: svc,
		db:      db,
		cfg:     cfg,
	}
}

// RegisterRoutes registers all HTTP routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.HealthCheck)
	mux.HandleFunc("GET /users", h.ListUsers)
	mux.HandleFunc("GET /users/{employee_id}", h.GetUser)
	mux.HandleFunc("GET /sessions/{session_id}", h.GetSession) // Context in manual
	mux.HandleFunc("POST /memory", h.AddMemory)
	mux.HandleFunc("GET /memory/{employee_id}", h.GetUserAllMemories)
	mux.HandleFunc("GET /memory/{employee_id}/{session_id}", h.GetAllMemories)
	mux.HandleFunc("POST /memory/search", h.SearchMemory)
	mux.HandleFunc("POST /chat", h.Chat)
	mux.HandleFunc("DELETE /memory/{employee_id}/{session_id}", h.DeleteSessionMemories)
	mux.HandleFunc("DELETE /users/{employee_id}/memories", h.DeleteUserAllMemories)
}

// buildMemoryKey creates a unique key for session isolation.
func buildMemoryKey(employeeID, sessionID string) string {
	return fmt.Sprintf("%s_%s", employeeID, sessionID)
}

// === Health Check ===

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// === User Management ===

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.db.ListAllUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]models.UserInfo, 0, len(users))
	for _, u := range users {
		result = append(result, models.UserInfo{
			EmployeeID: u.EmployeeID,
			Name:       u.Name,
			Position:   u.Position,
			CreatedAt:  u.CreatedAt,
			UpdatedAt:  u.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	employeeID := r.PathValue("employee_id")

	user, err := h.db.GetUser(employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}

	sessions, err := h.db.GetUserSessions(employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sessionInfos := make([]models.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		sessionInfos = append(sessionInfos, models.SessionInfo{
			SessionID:    s.SessionID,
			EmployeeID:   s.EmployeeID,
			CreatedAt:    s.CreatedAt,
			LastActivity: s.LastActivity,
		})
	}

	writeJSON(w, http.StatusOK, models.UserWithSessions{
		User: models.UserInfo{
			EmployeeID: user.EmployeeID,
			Name:       user.Name,
			Position:   user.Position,
			CreatedAt:  user.CreatedAt,
			UpdatedAt:  user.UpdatedAt,
		},
		Sessions: sessionInfos,
	})
}

// === Session Management ===

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")

	session, err := h.db.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	writeJSON(w, http.StatusOK, models.SessionInfo{
		SessionID:    session.SessionID,
		EmployeeID:   session.EmployeeID,
		CreatedAt:    session.CreatedAt,
		LastActivity: session.LastActivity,
	})
}

// === Memory Operations ===

func (h *Handler) AddMemory(w http.ResponseWriter, r *http.Request) {
	var req models.AddMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Ensure user and session exist
	if _, err := h.db.GetOrCreateUser(req.EmployeeID, "", ""); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := h.db.GetOrCreateSession(req.SessionID, req.EmployeeID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	memoryKey := buildMemoryKey(req.EmployeeID, req.SessionID)

	// Convert messages
	messages := make([]llm.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = llm.Message{Role: m.Role, Content: m.Content}
	}

	result, err := h.service.AddMemory(messages, memoryKey)
	if err != nil {
		log.Printf("Error adding memory: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Added memory for %s: %d results", memoryKey, len(result.Results))
	writeJSON(w, http.StatusOK, models.AddMemoryResponse{
		Message: "Memory added successfully",
	})
}

func (h *Handler) GetAllMemories(w http.ResponseWriter, r *http.Request) {
	employeeID := r.PathValue("employee_id")
	sessionID := r.PathValue("session_id")
	memoryKey := buildMemoryKey(employeeID, sessionID)

	memories, err := h.service.GetAllMemories(memoryKey)
	if err != nil {
		log.Printf("Error getting memories: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]models.MemoryItem, 0, len(memories))
	for _, m := range memories {
		items = append(items, models.MemoryItem{
			ID:     m.ID,
			Memory: m.Memory,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) GetUserAllMemories(w http.ResponseWriter, r *http.Request) {
	employeeID := r.PathValue("employee_id")

	memories, err := h.service.GetUserMemories(employeeID)
	if err != nil {
		log.Printf("Error getting user memories: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]models.MemoryItem, 0, len(memories))
	for _, m := range memories {
		items = append(items, models.MemoryItem{
			ID:     m.ID,
			Memory: m.Memory,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) SearchMemory(w http.ResponseWriter, r *http.Request) {
	var req models.SearchMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Limit == 0 {
		req.Limit = 5
	}

	memoryKey := buildMemoryKey(req.EmployeeID, req.SessionID)
	results, err := h.service.SearchMemories(req.Query, memoryKey, req.Limit)
	if err != nil {
		log.Printf("Error searching memory: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]models.MemoryItem, 0, len(results.Results))
	for _, r := range results.Results {
		score := r.Score
		items = append(items, models.MemoryItem{
			ID:     r.ID,
			Memory: r.Memory,
			Score:  &score,
		})
	}
	writeJSON(w, http.StatusOK, models.SearchMemoryResponse{Results: items})
}

// === Chat Endpoint ===

func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	var req models.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 1. Get or create user
	userName := ""
	userPosition := ""
	if req.UserName != nil {
		userName = *req.UserName
	}
	if req.UserPosition != nil {
		userPosition = *req.UserPosition
	}

	user, err := h.db.GetOrCreateUser(req.EmployeeID, userName, userPosition)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("User: %+v", user)

	// 2. Get or create session
	session, err := h.db.GetOrCreateSession(req.SessionID, req.EmployeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("Session: %+v", session)

	// 3. Build memory key
	memoryKey := buildMemoryKey(req.EmployeeID, req.SessionID)

	// 4. Search relevant memories
	relevantMemories, err := h.service.SearchMemories(req.Message, memoryKey, 3)
	if err != nil {
		log.Printf("Error searching memories: %v", err)
		relevantMemories = nil
	}

	// 5. Construct context
	var contextParts []string
	if relevantMemories != nil {
		for _, m := range relevantMemories.Results {
			contextParts = append(contextParts, fmt.Sprintf("- %s", m.Memory))
		}
	}
	contextStr := strings.Join(contextParts, "\n")

	// 6. User info context
	posStr := "Unknown"
	if user.Position != nil {
		posStr = *user.Position
	}
	userContext := fmt.Sprintf("사용자 정보: 이름=%s, 직급=%s", user.Name, posStr)
	fullContext := userContext
	if contextStr != "" {
		fullContext = userContext + "\n" + contextStr
	}

	// 7. Add current turn to memory
	addMessages := []llm.Message{
		{Role: "user", Content: req.Message},
	}
	addResult, err := h.service.AddMemory(addMessages, memoryKey)
	if err != nil {
		log.Printf("Warning: failed to add memory: %v", err)
	} else {
		log.Printf("Memory update result: %d results", len(addResult.Results))
	}

	// 8. Generate response with LLM
	systemPrompt := fmt.Sprintf(
		"You are a helpful assistant. Use the provided context to answer the user's question.\n\nContext:\n%s",
		fullContext,
	)

	responseText := h.generateChatResponse(systemPrompt, req.Message)

	// 9. Build response
	memoriesUsed := make([]models.MemoryItem, 0)
	if relevantMemories != nil {
		for _, m := range relevantMemories.Results {
			score := m.Score
			memoriesUsed = append(memoriesUsed, models.MemoryItem{
				ID:     m.ID,
				Memory: m.Memory,
				Score:  &score,
			})
		}
	}

	userInfo := map[string]interface{}{
		"employee_id": user.EmployeeID,
		"name":        user.Name,
	}
	if user.Position != nil {
		userInfo["position"] = *user.Position
	}

	writeJSON(w, http.StatusOK, models.ChatResponse{
		Response:     responseText,
		MemoriesUsed: memoriesUsed,
		UserInfo:     userInfo,
	})
}

func (h *Handler) generateChatResponse(systemPrompt, userMessage string) string {
	// Create OpenAI client with base URL support
	clientCfg := openai.DefaultConfig(h.cfg.OpenAIAPIKey)
	if h.cfg.OpenAIAPIBase != "" {
		clientCfg.BaseURL = h.cfg.OpenAIAPIBase
	}
	client := openai.NewClientWithConfig(clientCfg)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: h.cfg.OpenAIModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userMessage},
			},
		},
	)
	if err != nil {
		log.Printf("LLM Generation Failed: %v", err)
		return fmt.Sprintf("(Error generating response: %s)", err.Error())
	}

	if len(resp.Choices) == 0 {
		return "(Error: LLM returned no choices)"
	}

	return resp.Choices[0].Message.Content
}

// === Delete Endpoints ===

func (h *Handler) DeleteSessionMemories(w http.ResponseWriter, r *http.Request) {
	employeeID := r.PathValue("employee_id")
	sessionID := r.PathValue("session_id")
	memoryKey := buildMemoryKey(employeeID, sessionID)

	if err := h.service.DeleteAllMemories(memoryKey); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("All memories deleted for session %s", sessionID),
	})
}

func (h *Handler) DeleteUserAllMemories(w http.ResponseWriter, r *http.Request) {
	employeeID := r.PathValue("employee_id")

	sessions, err := h.db.GetUserSessions(employeeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	deletedCount := 0
	for _, session := range sessions {
		memoryKey := buildMemoryKey(employeeID, session.SessionID)
		if err := h.service.DeleteAllMemories(memoryKey); err != nil {
			log.Printf("Warning: failed to delete memories for session %s: %v", session.SessionID, err)
		}
		deletedCount++
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Deleted memories from %d sessions for user %s", deletedCount, employeeID),
	})
}

// === Helpers ===

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, models.ErrorResponse{Detail: detail})
}
