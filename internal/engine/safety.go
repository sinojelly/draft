package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syncghost/internal/logger"
	"time"
)

// DeletionGuard tracks and limits mass deletion events for safety
type DeletionGuard struct {
	mu        sync.Mutex
	deletions map[string][]time.Time // accountID -> timestamps
	threshold int                    // max deletions in window
	window    time.Duration
}

// GlobalDeletionGuard is the system-wide safety switch
var GlobalDeletionGuard = &DeletionGuard{
	deletions: make(map[string][]time.Time),
	threshold: 50, // Limit to 50 deletions per 5 minutes
	window:    5 * time.Minute,
}

// AllowDeletion checks if a deletion is safe or if it looks like a mass-delete accident
func (dg *DeletionGuard) AllowDeletion(accountID string) bool {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-dg.window)

	// Clean old entries
	valid := []time.Time{}
	for _, t := range dg.deletions[accountID] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	
	if len(valid) >= dg.threshold {
		logger.LogInfo("SAFETY GUARD TRIGGERED: Mass deletion detected for %s. Blocking further deletions for %v.", accountID, dg.window)
		return false
	}

	dg.deletions[accountID] = append(valid, now)
	return true
}

// TrashLocal provides a "Soft Delete" by moving files to a hidden .syncghost/trash folder
func TrashLocal(taskRoot, localPath string) error {
	trashDir := filepath.Join(taskRoot, ".syncghost", "trash")
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return err
	}

	rel, err := filepath.Rel(taskRoot, localPath)
	if err != nil {
		return err
	}

	dest := filepath.Join(trashDir, rel)
	// Ensure subdirectories in trash exist
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// If destination exists, add timestamp to avoid collisions
	if _, err := os.Stat(dest); err == nil {
		timestamp := time.Now().Format("20060102_150405")
		dest = fmt.Sprintf("%s.trash_%s", dest, timestamp)
	}

	logger.LogInfo("Soft Delete: Moving %s to trash.", localPath)
	return os.Rename(localPath, dest)
}
