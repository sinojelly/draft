package state

import (
	"os"
	"testing"
)

func TestStateDB(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test_state_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbPath := tempFile.Name()
	tempFile.Close()
	os.Remove(dbPath) // Bolt needs to create it or open it

	defer os.Remove(dbPath)

	err = InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	accountID := "acc1"
	driveType := "baidu"
	localPath := "/data/test.txt"

	// 1. Get non-existent
	state, err := GetFileState(accountID, driveType, localPath)
	if err != nil {
		t.Errorf("Expected no error for missing file, got %v", err)
	}
	if state != nil {
		t.Errorf("Expected nil state, got %v", state)
	}

	// 2. Save state
	newState := FileState{
		FileID:   "f1",
		MD5:      "abc",
		Size:     100,
		SyncTime: 123456,
	}
	err = SaveFileState(accountID, driveType, localPath, newState)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// 3. Get existing
	state, err = GetFileState(accountID, driveType, localPath)
	if err != nil || state == nil {
		t.Fatalf("Failed to get state: %v, %v", err, state)
	}
	if state.MD5 != "abc" || state.Size != 100 {
		t.Errorf("State mismatch: %+v", state)
	}

	// 4. Delete state
	err = DeleteFileState(accountID, driveType, localPath)
	if err != nil {
		t.Errorf("Failed to delete state: %v", err)
	}

	state, _ = GetFileState(accountID, driveType, localPath)
	if state != nil {
		t.Errorf("State should be deleted")
	}
}
