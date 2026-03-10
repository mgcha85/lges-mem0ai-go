package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// HistoryRecord represents a single history entry.
type HistoryRecord struct {
	ID        string
	MemoryID  string
	PrevValue *string
	NewValue  *string
	Event     string
	CreatedAt string
	UpdatedAt *string
	IsDeleted int
}

// SQLiteManager manages memory history in SQLite.
type SQLiteManager struct {
	db *sql.DB
}

// NewSQLiteManager creates a new SQLite manager with the given database path.
func NewSQLiteManager(dbPath string) (*SQLiteManager, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db at %s: %w", dbPath, err)
	}

	mgr := &SQLiteManager{db: db}
	if err := mgr.createTable(); err != nil {
		db.Close()
		return nil, err
	}

	return mgr, nil
}

func (s *SQLiteManager) createTable() error {
	query := `CREATE TABLE IF NOT EXISTS history (
		id TEXT PRIMARY KEY,
		memory_id TEXT NOT NULL,
		prev_value TEXT,
		new_value TEXT,
		event TEXT NOT NULL,
		created_at TEXT,
		updated_at TEXT,
		is_deleted INTEGER DEFAULT 0
	)`
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create history table: %w", err)
	}
	return nil
}

// AddHistory records a memory change event.
func (s *SQLiteManager) AddHistory(memoryID string, prevValue, newValue *string, event string, createdAt, updatedAt *string, isDeleted int) error {
	id := fmt.Sprintf("%s_%d", memoryID, time.Now().UnixNano())

	if createdAt == nil {
		now := time.Now().Format(time.RFC3339)
		createdAt = &now
	}

	query := `INSERT INTO history (id, memory_id, prev_value, new_value, event, created_at, updated_at, is_deleted)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, id, memoryID, prevValue, newValue, event, *createdAt, updatedAt, isDeleted)
	if err != nil {
		return fmt.Errorf("failed to add history: %w", err)
	}
	return nil
}

// GetHistory retrieves all history entries for a given memory ID.
func (s *SQLiteManager) GetHistory(memoryID string) ([]HistoryRecord, error) {
	query := `SELECT id, memory_id, prev_value, new_value, event, created_at, updated_at, is_deleted
	           FROM history WHERE memory_id = ? ORDER BY created_at ASC`
	rows, err := s.db.Query(query, memoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var records []HistoryRecord
	for rows.Next() {
		var r HistoryRecord
		err := rows.Scan(&r.ID, &r.MemoryID, &r.PrevValue, &r.NewValue, &r.Event, &r.CreatedAt, &r.UpdatedAt, &r.IsDeleted)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history row: %w", err)
		}
		records = append(records, r)
	}
	return records, nil
}

// Reset drops and recreates the history table.
func (s *SQLiteManager) Reset() error {
	_, err := s.db.Exec("DROP TABLE IF EXISTS history")
	if err != nil {
		return fmt.Errorf("failed to drop history table: %w", err)
	}
	return s.createTable()
}

// Close closes the database connection.
func (s *SQLiteManager) Close() error {
	return s.db.Close()
}
