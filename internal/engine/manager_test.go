package engine

import (
	"testing"
	"time"
)

func TestTriggerReload(t *testing.T) {
	// Drain channel
	select {
	case <-ReloadChan:
	default:
	}
	
	TriggerReload()
	select {
	case <-ReloadChan:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("TriggerReload did not signal ReloadChan")
	}
	
	// Test non-blocking behavior
	TriggerReload()
	TriggerReload() // Should not block the caller
}
