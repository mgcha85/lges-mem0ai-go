package embeddings

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIEmbedder implements the Embedder interface using OpenAI's embedding API.
type OpenAIEmbedder struct {
	client *openai.Client
	model  openai.EmbeddingModel
}

// NewOpenAIEmbedder creates a new OpenAI Embedder.
func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		client: openai.NewClient(apiKey),
		model:  openai.EmbeddingModel(model),
	}
}

// Embed generates an embedding vector for the given text.
func (e *OpenAIEmbedder) Embed(text string) ([]float32, error) {
	resp, err := e.client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Input: []string{text},
		Model: e.model,
	})
	if err != nil {
		return nil, fmt.Errorf("openai embedding error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai returned no embedding data")
	}

	return resp.Data[0].Embedding, nil
}

// Close is a no-op for OpenAI embedder.
func (e *OpenAIEmbedder) Close() error {
	return nil
}
