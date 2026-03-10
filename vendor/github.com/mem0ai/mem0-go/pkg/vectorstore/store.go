package vectorstore

// SearchResult represents a single result from a vector search.
type SearchResult struct {
	ID      string
	Score   float32
	Payload map[string]interface{}
}

// VectorRecord represents a stored vector with its metadata.
type VectorRecord struct {
	ID      string
	Payload map[string]interface{}
}

// VectorStore is the interface for vector storage operations.
type VectorStore interface {
	// Insert adds vectors with their IDs and payloads.
	Insert(vectors [][]float32, ids []string, payloads []map[string]interface{}) error

	// Search finds the most similar vectors to the query.
	Search(query string, vector []float32, limit int, filters map[string]string) ([]SearchResult, error)

	// Get retrieves a single vector record by ID.
	Get(id string) (*VectorRecord, error)

	// List returns all records matching the given filters.
	List(filters map[string]string, limit int) ([]VectorRecord, error)

	// Update updates a vector record's vector and/or payload.
	Update(id string, vector []float32, payload map[string]interface{}) error

	// Delete removes a vector record by ID.
	Delete(id string) error

	// Reset deletes and recreates the collection.
	Reset() error
}
