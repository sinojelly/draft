package status

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStatusConcurrency(t *testing.T) {
	// Reset state
	StartTime = time.Now()
	
	const goroutines = 10
	const iterations = 100
	
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)
	
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				taskID := fmt.Sprintf("task-%d-%d", id, j)
				AddActiveTask(taskID, "Downloading test file")
				UpdateTaskProgress(taskID, int64(j), 100)
			}
		}(i)
		
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				UpdateMetrics(1, 1, 1)
				AddActivity(fmt.Sprintf("Activity %d-%d", id, j))
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify state
	StateMu.Lock()
	if GlobalState.TotalUp != int64(goroutines*iterations) {
		t.Errorf("Expected TotalUp %d, got %d", goroutines*iterations, GlobalState.TotalUp)
	}
	StateMu.Unlock()
	
	// Test EncodeState
	data := EncodeState()
	var decoded DashboardState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to decode state: %v", err)
	}
	
	if len(decoded.RecentActivity) != 20 {
		t.Errorf("Expected 20 recent activities, got %d", len(decoded.RecentActivity))
	}
}

func TestTriggerUpdate(t *testing.T) {
	// Drain channel
	select {
	case <-StateChanged:
	default:
	}
	
	TriggerUpdate()
	select {
	case <-StateChanged:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("TriggerUpdate did not signal StateChanged")
	}
	
	// Test non-blocking
	TriggerUpdate()
	TriggerUpdate() // Should not block
}

func TestTaskDirections(t *testing.T) {
	AddActiveTask("up-1", "Uploading file")
	AddActiveTask("down-1", "Downloading file")
	
	if _, ok := ActiveUpMap.Load("up-1"); !ok {
		t.Error("Expected up-1 in ActiveUpMap")
	}
	if _, ok := ActiveDownMap.Load("down-1"); !ok {
		t.Error("Expected down-1 in ActiveDownMap")
	}
	
	RemoveActiveTask("up-1")
	if _, ok := ActiveUpMap.Load("up-1"); ok {
		t.Error("Expected up-1 to be removed from ActiveUpMap")
	}
}
