package llm

// Message represents a chat message with a role and content.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLM is the interface for language model interactions.
type LLM interface {
	// GenerateResponse sends messages to the LLM and returns the response text.
	// If jsonMode is true, the response should be valid JSON.
	GenerateResponse(messages []Message, jsonMode bool) (string, error)
}
