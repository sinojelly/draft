package downstream

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"syncghost/internal/config"
	"syncghost/internal/drive"
	eng "syncghost/internal/engine"
	"syncghost/internal/state"
)

// MockDrive implements drive.CloudDrive for testing
type MockDrive struct {
	DownloadFunc func(remotePath, localPath string, progress drive.ProgressReporter) error
}

func (m *MockDrive) GetCapabilities() drive.DriveCapabilities { return drive.DriveCapabilities{} }
func (m *MockDrive) Upload(localPath, remoteDir, onConflict string, progress drive.ProgressReporter) (string, error) {
	return "mock_id", nil
}
func (m *MockDrive) Download(remotePath, localPath string, progress drive.ProgressReporter) error {
	if m.DownloadFunc != nil {
		return m.DownloadFunc(remotePath, localPath, progress)
	}
	return os.WriteFile(localPath, []byte("hello"), 0644)
}
func (m *MockDrive) DeleteDir(remotePath string) error               { return nil }
func (m *MockDrive) Delete(remotePath string) error                  { return nil }
func (m *MockDrive) ListDir(remotePath string) ([]drive.CloudChange, error) { return nil, nil }
func (m *MockDrive) GetDirID(remotePath string) (string, error)      { return "dir_id", nil }
func (m *MockDrive) GetFileInfo(remotePath string) (int64, string, string, error) {
	return 0, "", "", nil
}
func (m *MockDrive) CheckExistence(remotePath string) (bool, error) { return true, nil }
func (m *MockDrive) GetIncrementalChanges(cursor string) ([]drive.CloudChange, string, bool, error) {
	return nil, "", false, nil
}
func (m *MockDrive) GetLatestCursor() (string, error) { return "", nil }

func TestDownEngineAtomicRename(t *testing.T) {
	config.GlobalConfig = &config.Config{
		Performance: config.PerformanceConfig{MaxConcurrency: 1},
	}

	tempDir, _ := os.MkdirTemp("", "down_test")
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	state.InitDB(dbPath)
	defer state.CloseDB()

	localPath := filepath.Join(tempDir, "target.txt")
	tempPath := localPath + ".sgdownload"

	mock := &MockDrive{
		DownloadFunc: func(remotePath, localPath string, progress drive.ProgressReporter) error {
			// Verify that it's downloading to the .sgdownload path
			if !filepath.HasPrefix(localPath, tempPath) && localPath != tempPath {
				t.Errorf("Plugin received wrong local path: got %s, want %s", localPath, tempPath)
			}
			return os.WriteFile(localPath, []byte("data"), 0644)
		},
	}

	eventChan := make(chan eng.DownTaskEvent, 1)
	downEngine := NewDownEngine(mock, eventChan)
	downEngine.Start()
	defer downEngine.Stop()

	event := eng.DownTaskEvent{
		CloudFileEvent: eng.CloudFileEvent{
			AccountID:  "test",
			RemotePath: "/cloud/target.txt",
			Action:     "create",
			Size:       4,
		},
		DriveType: "mock",
		LocalPath: localPath,
		LocalRoot: tempDir,
	}

	eventChan <- event
	close(eventChan)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// 1. Verify target file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		t.Errorf("Target file %s was not created", localPath)
	}

	// 2. Verify temp file is gone
	if _, err := os.Stat(tempPath); err == nil {
		t.Errorf("Temp file %s still exists", tempPath)
	}
}
