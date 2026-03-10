package llm

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAILLM implements the LLM interface using OpenAI's API.
type OpenAILLM struct {
	client *openai.Client
	model  string
}

// NewOpenAILLM creates a new OpenAI LLM client.
func NewOpenAILLM(apiKey, model string) *OpenAILLM {
	return &OpenAILLM{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

// GenerateResponse sends messages to OpenAI and returns the response.
func (o *OpenAILLM) GenerateResponse(messages []Message, jsonMode bool) (string, error) {
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
		return "", fmt.Errorf("openai chat completion error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}

	return resp.Choices[0].Message.Content, nil
}
