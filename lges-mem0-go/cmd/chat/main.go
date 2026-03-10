package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mem0ai/mem0-go/pkg/config"
	"github.com/mem0ai/mem0-go/pkg/llm"
	"github.com/mem0ai/mem0-go/pkg/memory"
	openai "github.com/sashabaranov/go-openai"
)

func chatWithMemories(openaiClient *openai.Client, mem *memory.Memory, message, userID string) (string, error) {
	// 1. Retrieve relevant memories
	searchResp, err := mem.Search(message, userID, 3)
	if err != nil {
		return "", fmt.Errorf("memory search error: %w", err)
	}

	var memoriesStr string
	for _, entry := range searchResp.Results {
		memoriesStr += fmt.Sprintf("- %s\n", entry.Memory)
	}

	// 2. Generate assistant response using OpenAI
	systemPrompt := fmt.Sprintf("You are a helpful AI. Answer the question based on query and memories.\nUser Memories:\n%s", memoriesStr)

	resp, err := openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "gpt-4.1-nano-2025-04-14",
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: message},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("openai chat error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}
	assistantResponse := resp.Choices[0].Message.Content

	// 3. Create new memories from the conversation
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: message},
		{Role: "assistant", Content: assistantResponse},
	}
	_, err = mem.Add(messages, userID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to add memory: %v\n", err)
	}

	return assistantResponse, nil
}

func main() {
	cfg := config.DefaultConfig()
	if cfg.LLMAPIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: OPENAI_API_KEY environment variable is required")
		os.Exit(1)
	}

	mem, err := memory.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing memory: %v\n", err)
		os.Exit(1)
	}
	defer mem.Close()

	openaiClient := openai.NewClient(cfg.LLMAPIKey)

	userID := "default_user"
	fmt.Println("Chat with AI (type 'exit' to quit)")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		userInput := strings.TrimSpace(scanner.Text())
		if strings.ToLower(userInput) == "exit" {
			fmt.Println("Goodbye!")
			break
		}
		if userInput == "" {
			continue
		}

		response, err := chatWithMemories(openaiClient, mem, userInput, userID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Printf("AI: %s\n", response)
	}
}
