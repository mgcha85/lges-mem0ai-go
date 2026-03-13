package vectorstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements VectorStore using SQLite and in-memory cosine similarity.
type SQLiteStore struct {
	db        *sql.DB
	tableName string
	dimension int
}

// NewSQLiteStore creates a new SQLite-based vector store.
func NewSQLiteStore(dbPath string, tableName string, dimension int) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	store := &SQLiteStore{
		db:        db,
		tableName: tableName,
		dimension: dimension,
	}

	if err := store.initTable(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) initTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS "%s" (
			id TEXT PRIMARY KEY,
			vector_json TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			user_id TEXT
		)
	`, s.tableName)

	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
	}

	// Create index on user_id for fast filtering
	indexQuery := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS "idx_%s_user_id" ON "%s" (user_id)`, s.tableName, s.tableName)
	if _, err := s.db.Exec(indexQuery); err != nil {
		return fmt.Errorf("failed to create index on user_id: %w", err)
	}

	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Insert adds vectors with their IDs and payloads.
func (s *SQLiteStore) Insert(vectors [][]float32, ids []string, payloads []map[string]interface{}) error {
	if len(vectors) != len(ids) || len(vectors) != len(payloads) {
		return fmt.Errorf("mismatched lengths: vectors(%d), ids(%d), payloads(%d)", len(vectors), len(ids), len(payloads))
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`INSERT OR REPLACE INTO "%s" (id, vector_json, payload_json, user_id) VALUES (?, ?, ?, ?)`, s.tableName)
	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i := range vectors {
		vectorJSON, err := json.Marshal(vectors[i])
		if err != nil {
			return fmt.Errorf("failed to marshal vector at index %d: %w", i, err)
		}

		payloadJSON, err := json.Marshal(payloads[i])
		if err != nil {
			return fmt.Errorf("failed to marshal payload at index %d: %w", i, err)
		}

		// Extract user_id from payload for indexing
		var userID string
		if uid, ok := payloads[i]["user_id"].(string); ok {
			userID = uid
		}

		_, err = stmt.Exec(ids[i], string(vectorJSON), string(payloadJSON), userID)
		if err != nil {
			return fmt.Errorf("failed to insert record %s: %w", ids[i], err)
		}
	}

	return tx.Commit()
}

// Search finds the most similar vectors to the query.
// It filters by user_id if provided in filters, loads matching vectors into memory,
// computes cosine similarity, and returns the top 'limit' results.
func (s *SQLiteStore) Search(query string, vector []float32, limit int, filters map[string]string) ([]SearchResult, error) {
	// Normalize the query vector for cosine similarity (dot product of normalized vectors)
	qNorm := normalize(vector)
	if qNorm == nil {
		return nil, fmt.Errorf("query vector contains only zeros or invalid values")
	}

	// Build the SQL query with filters
	sqlQuery := fmt.Sprintf(`SELECT id, vector_json, payload_json FROM "%s" WHERE 1=1`, s.tableName)
	var args []interface{}

	if userID, ok := filters["user_id"]; ok {
		sqlQuery += ` AND user_id = ?`
		args = append(args, userID)
	}

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query records: %w", err)
	}
	defer rows.Close()

	var results []SearchResult

	for rows.Next() {
		var id, vecStr, payloadStr string
		if err := rows.Scan(&id, &vecStr, &payloadStr); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse vector
		var vec []float32
		if err := json.Unmarshal([]byte(vecStr), &vec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal vector for %s: %w", id, err)
		}

		// Calculate cosine similarity
		// Assuming stored vectors are not normalized, we normalize them here.
		// For E5 onnx embedder, they are already L2 normalized, so calculating dot product is just as good,
		// but let's normalize to be safe with any embedder.
		vNorm := normalize(vec)
		if vNorm == nil {
			continue // skip invalid vectors
		}

		score := dotProduct(qNorm, vNorm)

		// Parse payload
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload for %s: %w", id, err)
		}

		results = append(results, SearchResult{
			ID:      id,
			Score:   score,
			Payload: payload,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Get retrieves a single vector record by ID.
func (s *SQLiteStore) Get(id string) (*VectorRecord, error) {
	query := fmt.Sprintf(`SELECT id, payload_json FROM "%s" WHERE id = ?`, s.tableName)
	row := s.db.QueryRow(query, id)

	var recID, payloadStr string
	if err := row.Scan(&recID, &payloadStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("record not found: %s", id)
		}
		return nil, fmt.Errorf("failed to fetch record %s: %w", id, err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload for %s: %w", id, err)
	}

	return &VectorRecord{
		ID:      recID,
		Payload: payload,
	}, nil
}

// List returns all records matching the given filters.
func (s *SQLiteStore) List(filters map[string]string, limit int) ([]VectorRecord, error) {
	query := fmt.Sprintf(`SELECT id, payload_json FROM "%s" WHERE 1=1`, s.tableName)
	var args []interface{}

	if userID, ok := filters["user_id"]; ok {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}

	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	defer rows.Close()

	var records []VectorRecord
	for rows.Next() {
		var id, payloadStr string
		if err := rows.Scan(&id, &payloadStr); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload for %s: %w", id, err)
		}

		records = append(records, VectorRecord{
			ID:      id,
			Payload: payload,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return records, nil
}

// Update updates a vector record's vector and/or payload.
func (s *SQLiteStore) Update(id string, vector []float32, payload map[string]interface{}) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var updates []string
	var args []interface{}

	if vector != nil {
		vectorJSON, err := json.Marshal(vector)
		if err != nil {
			return fmt.Errorf("failed to marshal vector: %w", err)
		}
		updates = append(updates, "vector_json = ?")
		args = append(args, string(vectorJSON))
	}

	if payload != nil {
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		updates = append(updates, "payload_json = ?")
		args = append(args, string(payloadJSON))

		// Check if user_id needs updating
		if uid, ok := payload["user_id"].(string); ok {
			updates = append(updates, "user_id = ?")
			args = append(args, uid)
		}
	}

	if len(updates) == 0 {
		return nil // Nothing to update
	}

	query := fmt.Sprintf(`UPDATE "%s" SET %s WHERE id = ?`, s.tableName, strings.Join(updates, ", "))
	args = append(args, id)

	res, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update record %s: %w", id, err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record not found: %s", id)
	}

	return tx.Commit()
}

// Delete removes a vector record by ID.
func (s *SQLiteStore) Delete(id string) error {
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE id = ?`, s.tableName)
	res, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete record %s: %w", id, err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record not found: %s", id)
	}

	return nil
}

// Reset deletes and recreates the collection table.
func (s *SQLiteStore) Reset() error {
	dropQuery := fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, s.tableName)
	if _, err := s.db.Exec(dropQuery); err != nil {
		return fmt.Errorf("failed to drop table %s: %w", s.tableName, err)
	}

	return s.initTable()
}

// --- Helper Functions ---

// normalize applies L2 normalization to a vector inline.
// Returns normalized vector copy or nil if magnitude is 0.
func normalize(v []float32) []float32 {
	if len(v) == 0 {
		return nil
	}

	var sumSq float64
	for _, val := range v {
		sumSq += float64(val * val)
	}

	if sumSq == 0 {
		return nil
	}

	norm := float32(math.Sqrt(sumSq))
	res := make([]float32, len(v))
	for i, val := range v {
		res[i] = val / norm
	}

	return res
}

// dotProduct computes the dot product of two vectors.
func dotProduct(v1, v2 []float32) float32 {
	if len(v1) != len(v2) {
		return 0
	}

	var dot float32
	for i := range v1 {
		dot += v1[i] * v2[i]
	}
	return dot
}
