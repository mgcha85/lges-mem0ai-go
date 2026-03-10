package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// UserRecord represents a row in the users table.
type UserRecord struct {
	EmployeeID string  `json:"employee_id"`
	Name       string  `json:"name"`
	Position   *string `json:"position,omitempty"`
	CreatedAt  *string `json:"created_at,omitempty"`
	UpdatedAt  *string `json:"updated_at,omitempty"`
}

// SessionRecord represents a row in the sessions table.
type SessionRecord struct {
	SessionID    string  `json:"session_id"`
	EmployeeID   string  `json:"employee_id"`
	CreatedAt    *string `json:"created_at,omitempty"`
	LastActivity *string `json:"last_activity,omitempty"`
}

// Database manages user and session data in SQLite.
type Database struct {
	db *sql.DB
}

// New creates a new Database instance and initializes the schema.
func New(dbPath string) (*Database, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	d := &Database{db: db}
	if err := d.initDB(); err != nil {
		db.Close()
		return nil, err
	}

	return d, nil
}

func (d *Database) initDB() error {
	// Enable WAL mode
	if _, err := d.db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to set WAL mode: %w", err)
	}
	if _, err := d.db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	// Create users table
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			employee_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			position TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create sessions table
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			session_id TEXT PRIMARY KEY,
			employee_id TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (employee_id) REFERENCES users(employee_id)
		)
	`); err != nil {
		return fmt.Errorf("failed to create sessions table: %w", err)
	}

	// Create index
	if _, err := d.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_sessions_employee ON sessions(employee_id)
	`); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// GetOrCreateUser gets an existing user or creates a new one.
func (d *Database) GetOrCreateUser(employeeID, name, position string) (*UserRecord, error) {
	row := d.db.QueryRow("SELECT employee_id, name, position, created_at, updated_at FROM users WHERE employee_id = ?", employeeID)

	var u UserRecord
	err := row.Scan(&u.EmployeeID, &u.Name, &u.Position, &u.CreatedAt, &u.UpdatedAt)

	if err == sql.ErrNoRows {
		// Create new user
		now := time.Now().Format(time.RFC3339)
		if name == "" {
			name = "Unknown"
		}
		pos := position
		_, err = d.db.Exec(
			"INSERT INTO users (employee_id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			employeeID, name, pos, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		return &UserRecord{
			EmployeeID: employeeID,
			Name:       name,
			Position:   &pos,
			CreatedAt:  &now,
			UpdatedAt:  &now,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// Update if new info provided
	now := time.Now().Format(time.RFC3339)
	if name != "" && name != u.Name {
		d.db.Exec("UPDATE users SET name = ?, updated_at = ? WHERE employee_id = ?", name, now, employeeID)
		u.Name = name
	}
	if position != "" && (u.Position == nil || position != *u.Position) {
		d.db.Exec("UPDATE users SET position = ?, updated_at = ? WHERE employee_id = ?", position, now, employeeID)
		u.Position = &position
	}

	return &u, nil
}

// GetUser retrieves a user by employee_id.
func (d *Database) GetUser(employeeID string) (*UserRecord, error) {
	row := d.db.QueryRow("SELECT employee_id, name, position, created_at, updated_at FROM users WHERE employee_id = ?", employeeID)
	var u UserRecord
	err := row.Scan(&u.EmployeeID, &u.Name, &u.Position, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &u, nil
}

// GetOrCreateSession gets an existing session or creates a new one.
func (d *Database) GetOrCreateSession(sessionID, employeeID string) (*SessionRecord, error) {
	row := d.db.QueryRow("SELECT session_id, employee_id, created_at, last_activity FROM sessions WHERE session_id = ?", sessionID)

	var s SessionRecord
	err := row.Scan(&s.SessionID, &s.EmployeeID, &s.CreatedAt, &s.LastActivity)

	now := time.Now().Format(time.RFC3339)

	if err == sql.ErrNoRows {
		_, err = d.db.Exec(
			"INSERT INTO sessions (session_id, employee_id, created_at, last_activity) VALUES (?, ?, ?, ?)",
			sessionID, employeeID, now, now,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		return &SessionRecord{
			SessionID:    sessionID,
			EmployeeID:   employeeID,
			CreatedAt:    &now,
			LastActivity: &now,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	// Update last_activity
	d.db.Exec("UPDATE sessions SET last_activity = ? WHERE session_id = ?", now, sessionID)
	s.LastActivity = &now

	return &s, nil
}

// GetSession retrieves a session by session_id.
func (d *Database) GetSession(sessionID string) (*SessionRecord, error) {
	row := d.db.QueryRow("SELECT session_id, employee_id, created_at, last_activity FROM sessions WHERE session_id = ?", sessionID)
	var s SessionRecord
	err := row.Scan(&s.SessionID, &s.EmployeeID, &s.CreatedAt, &s.LastActivity)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &s, nil
}

// GetUserSessions retrieves all sessions for a user.
func (d *Database) GetUserSessions(employeeID string) ([]SessionRecord, error) {
	rows, err := d.db.Query(
		"SELECT session_id, employee_id, created_at, last_activity FROM sessions WHERE employee_id = ? ORDER BY last_activity DESC",
		employeeID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionRecord
	for rows.Next() {
		var s SessionRecord
		if err := rows.Scan(&s.SessionID, &s.EmployeeID, &s.CreatedAt, &s.LastActivity); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// ListAllUsers lists all users.
func (d *Database) ListAllUsers() ([]UserRecord, error) {
	rows, err := d.db.Query("SELECT employee_id, name, position, created_at, updated_at FROM users ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.EmployeeID, &u.Name, &u.Position, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// --- Singleton ---

var (
	dbInstance *Database
	dbOnce     sync.Once
)

// GetDatabase returns the singleton database instance.
func GetDatabase(dbPath string) (*Database, error) {
	var initErr error
	dbOnce.Do(func() {
		dbInstance, initErr = New(dbPath)
	})
	if initErr != nil {
		return nil, initErr
	}
	return dbInstance, nil
}
