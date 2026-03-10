package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *SQLiteManager {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_history.db")
	mgr, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLiteManager: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })
	return mgr
}

func TestNewSQLiteManager(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	mgr, err := NewSQLiteManager(dbPath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer mgr.Close()

	// DB file should exist
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected DB file to be created")
	}
}

func TestAddHistory_And_GetHistory(t *testing.T) {
	mgr := tempDB(t)

	memID := "mem-001"
	newVal := "User likes pizza"
	createdAt := "2025-01-01T00:00:00Z"

	err := mgr.AddHistory(memID, nil, &newVal, "ADD", &createdAt, nil, 0)
	if err != nil {
		t.Fatalf("AddHistory failed: %v", err)
	}

	records, err := mgr.GetHistory(memID)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.MemoryID != memID {
		t.Errorf("Expected memory_id %s, got %s", memID, r.MemoryID)
	}
	if r.Event != "ADD" {
		t.Errorf("Expected event ADD, got %s", r.Event)
	}
	if r.NewValue == nil || *r.NewValue != newVal {
		t.Errorf("Expected new_value %s, got %v", newVal, r.NewValue)
	}
	if r.PrevValue != nil {
		t.Errorf("Expected nil prev_value, got %v", r.PrevValue)
	}
}

func TestAddHistory_Update(t *testing.T) {
	mgr := tempDB(t)

	memID := "mem-002"
	newVal := "User likes sushi"
	createdAt := "2025-01-01T00:00:00Z"

	// Add initial
	mgr.AddHistory(memID, nil, &newVal, "ADD", &createdAt, nil, 0)

	// Update
	prevVal := newVal
	updatedVal := "User loves sushi and ramen"
	updatedAt := "2025-01-02T00:00:00Z"
	err := mgr.AddHistory(memID, &prevVal, &updatedVal, "UPDATE", &createdAt, &updatedAt, 0)
	if err != nil {
		t.Fatalf("AddHistory UPDATE failed: %v", err)
	}

	records, err := mgr.GetHistory(memID)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	// Check update record
	r := records[1]
	if r.Event != "UPDATE" {
		t.Errorf("Expected event UPDATE, got %s", r.Event)
	}
	if r.PrevValue == nil || *r.PrevValue != prevVal {
		t.Errorf("Expected prev_value %s, got %v", prevVal, r.PrevValue)
	}
	if r.NewValue == nil || *r.NewValue != updatedVal {
		t.Errorf("Expected new_value %s, got %v", updatedVal, r.NewValue)
	}
}

func TestAddHistory_Delete(t *testing.T) {
	mgr := tempDB(t)

	memID := "mem-003"
	val := "Temporary memory"
	createdAt := "2025-01-01T00:00:00Z"

	mgr.AddHistory(memID, nil, &val, "ADD", &createdAt, nil, 0)

	err := mgr.AddHistory(memID, &val, nil, "DELETE", nil, nil, 1)
	if err != nil {
		t.Fatalf("AddHistory DELETE failed: %v", err)
	}

	records, err := mgr.GetHistory(memID)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(records))
	}

	r := records[1]
	if r.Event != "DELETE" {
		t.Errorf("Expected event DELETE, got %s", r.Event)
	}
	if r.IsDeleted != 1 {
		t.Errorf("Expected is_deleted 1, got %d", r.IsDeleted)
	}
}

func TestGetHistory_NotFound(t *testing.T) {
	mgr := tempDB(t)

	records, err := mgr.GetHistory("nonexistent")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Expected 0 records, got %d", len(records))
	}
}

func TestReset(t *testing.T) {
	mgr := tempDB(t)

	memID := "mem-004"
	val := "Something"
	createdAt := "2025-01-01T00:00:00Z"
	mgr.AddHistory(memID, nil, &val, "ADD", &createdAt, nil, 0)

	err := mgr.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	records, err := mgr.GetHistory(memID)
	if err != nil {
		t.Fatalf("GetHistory after reset failed: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Expected 0 records after reset, got %d", len(records))
	}
}

func TestMultipleMemories(t *testing.T) {
	mgr := tempDB(t)

	createdAt := "2025-01-01T00:00:00Z"
	for i := 0; i < 5; i++ {
		memID := "mem-multi-" + string(rune('A'+i))
		val := "Fact " + string(rune('A'+i))
		mgr.AddHistory(memID, nil, &val, "ADD", &createdAt, nil, 0)
	}

	// Each memory should have exactly 1 record
	for i := 0; i < 5; i++ {
		memID := "mem-multi-" + string(rune('A'+i))
		records, err := mgr.GetHistory(memID)
		if err != nil {
			t.Fatalf("GetHistory for %s failed: %v", memID, err)
		}
		if len(records) != 1 {
			t.Errorf("Expected 1 record for %s, got %d", memID, len(records))
		}
	}
}
