package engine

import (
	"os"
	"sync"
	"syncghost/internal/config"
	"testing"
	"time"
)

func TestTaskQueueDebounce(t *testing.T) {
	// Setup dummy config
	config.GlobalConfig = &config.Config{
		Performance: config.PerformanceConfig{
			MaxBatchCount:      10,
			DebounceDurationMs: 100, // 100ms debounce
		},
	}

	q := NewTaskQueue(100, 100*time.Millisecond)
	q.Start()

	var collectedBatches [][]UpTaskEvent
	var mu sync.Mutex

	// Monitor outbound batches
	go func() {
		for batch := range q.BatchChan {
			mu.Lock()
			collectedBatches = append(collectedBatches, batch)
			mu.Unlock()
		}
	}()

	// Simulate burst of events for the same file
	tmpFile := "test_debounce.txt"
	os.WriteFile(tmpFile, []byte("test"), 0644)
	defer os.Remove(tmpFile)

	for i := 0; i < 5; i++ {
		q.EventChan <- UpTaskEvent{
			OSFileEvent: OSFileEvent{LocalPath: tmpFile, Action: "modify"},
		}
		time.Sleep(10 * time.Millisecond) // interval < 100ms debounce
	}

	// Wait enough time for debounce timer to fire > 100ms
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(collectedBatches) != 1 {
		t.Fatalf("Expected exactly 1 batch due to debouncing and same file, got %d", len(collectedBatches))
	}
	if len(collectedBatches[0]) != 1 {
		t.Fatalf("Expected exactly 1 event in batch (deduplicated), got %d", len(collectedBatches[0]))
	}
	if collectedBatches[0][0].OSFileEvent.LocalPath != tmpFile {
		t.Errorf("Wrong file propagated, expected %s, got %s", tmpFile, collectedBatches[0][0].OSFileEvent.LocalPath)
	}
}
