package config

import (
	"os"
	"testing"
)

func TestLoadConfig_NewStructure(t *testing.T) {
	content := `
accounts:
  - id: "test_acc"
    type: "baidu"
sync_tasks:
  - account_id: "test_acc"
    local_path: "."
    remote_path: "/remote"
    up:
      enable: true
      on_conflict: "overwrite"
    down:
      enable: true
      poll_interval_sec: 30
`
	tmpFile := "test_new_config.yaml"
	os.WriteFile(tmpFile, []byte(content), 0644)
	defer os.Remove(tmpFile)

	err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load new config: %v", err)
	}

	task := GlobalConfig.SyncTasks[0]
	if !task.Up.Enable || !task.Down.Enable {
		t.Error("Expected both Up and Down to be enabled")
	}
	if task.Up.OnConflict != "overwrite" {
		t.Errorf("Expected Up.OnConflict overwrite, got %s", task.Up.OnConflict)
	}
	if task.Down.PollIntervalSec != 30 {
		t.Errorf("Expected Down.PollIntervalSec 30, got %d", task.Down.PollIntervalSec)
	}
	// Verify defaults
	if task.Down.OnConflict != "rename" {
		t.Errorf("Expected Down.OnConflict rename (default), got %s", task.Down.OnConflict)
	}
}

func TestLoadConfig_Validation(t *testing.T) {
	content := `
accounts:
  - id: "test_acc"
    type: "baidu"
sync_tasks:
  - account_id: "test_acc"
    local_path: "."
    remote_path: "/remote"
    up:
      enable: true
      on_conflict: "invalid_strategy"
`
	tmpFile := "test_invalid_config.yaml"
	os.WriteFile(tmpFile, []byte(content), 0644)
	defer os.Remove(tmpFile)

	err := LoadConfig(tmpFile)
	if err == nil {
		t.Error("Expected error for invalid on_conflict strategy, but got nil")
	}
}

func TestIsPathInTask(t *testing.T) {
	cfg := &Config{
		SyncTasks: []SyncTask{
			{
				AccountID:  "acc1",
				LocalPath:  "D:/Sync/Docs",
				RemotePath: "/Apps/SyncGhost/Docs",
				Up:         UpConfig{Enable: true},
				Down:       DownConfig{Enable: true},
			},
		},
	}

	// Test Local (Up)
	if !cfg.IsPathInTask("acc1", "up", "D:/Sync/Docs/file.txt") {
		t.Error("Expected true for file inside local path")
	}
	if !cfg.IsPathInTask("acc1", "up", "D:/Sync/Docs/SubDir/file.txt") {
		t.Error("Expected true for file in sub-directory")
	}
	if cfg.IsPathInTask("acc1", "up", "D:/Other/file.txt") {
		t.Error("Expected false for file outside local path")
	}

	// Test Remote (Down)
	if !cfg.IsPathInTask("acc1", "down", "/Apps/SyncGhost/Docs/file.txt") {
		t.Error("Expected true for file inside remote path")
	}
	if cfg.IsPathInTask("acc1", "down", "/Apps/Other/file.txt") {
		t.Error("Expected false for file outside remote path")
	}

	// Test Account Mismatch
	if cfg.IsPathInTask("acc2", "up", "D:/Sync/Docs/file.txt") {
		t.Error("Expected false for account mismatch")
	}

	// Test Direction Mismatch (if disabled)
	cfg.SyncTasks[0].Up.Enable = false
	if cfg.IsPathInTask("acc1", "up", "D:/Sync/Docs/file.txt") {
		t.Error("Expected false when Up is disabled")
	}
}
