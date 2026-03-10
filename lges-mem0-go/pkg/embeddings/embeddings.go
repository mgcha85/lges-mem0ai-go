package embeddings

// Embedder is the interface for generating text embeddings.
type Embedder interface {
	// Embed converts text into a vector embedding.
	Embed(text string) ([]float32, error)

	// Close cleans up resources.
	Close() error
}
