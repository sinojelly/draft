package upstream

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"syncghost/internal/config"
	"syncghost/internal/engine"
	"syncghost/internal/logger"
	"syncghost/internal/state"
)

// PerformInitialScan recursively walks the local directory and enqueues all files for synchronization.
func PerformInitialScan(task config.SyncTask, driveType string, queue *engine.TaskQueue) error {
	logger.LogInfo("[%s] Starting parallel initial full scan for %s", task.AccountID, task.LocalPath)
	filter := engine.NewSyncFilter(task.LocalPath)
	
	count := 0
	var countMu sync.Mutex
	sem := make(chan struct{}, 8) // Parallel scan tokens
	var wg sync.WaitGroup

	var walk func(localDir string)
	walk = func(localDir string) {
		defer wg.Done()

		entries, err := os.ReadDir(localDir)
		if err != nil {
			logger.LogError("InitialScan:ReadDir", fmt.Errorf("dir %s: %v", localDir, err))
			return
		}

		for _, entry := range entries {
			fullPath := filepath.Join(localDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}

			relPath, _ := filepath.Rel(task.LocalPath, fullPath)
			if filter.ShouldIgnore(relPath) {
				continue
			}

			if entry.IsDir() {
				// Avoid walking hidden .syncghost directory
				if entry.Name() == ".syncghost" {
					continue
				}
				// Phase 75: Record directory existence for future deletion detection
				logger.LogInfo("[%s] Tracking folder for deletion support: %s", task.AccountID, relPath)
				state.SaveFileState(task.AccountID, driveType, fullPath, state.FileState{IsDir: true, SyncTime: info.ModTime().Unix()})
				
				wg.Add(1)
				go walk(fullPath)
				continue
			}

			// Incremental Scan Optimization (Phase 72)
			// Only enqueue if the file actually changed since last sync
			if shouldUpSync(task.AccountID, driveType, fullPath, info) {
				remotePath := filepath.ToSlash(filepath.Join(task.RemotePath, relPath))
				
				sem <- struct{}{} // Wait for a worker slot before enqueuing to prevent overflow? 
				// Actually just queue directly, the channel has 10000 capacity.
				
				queue.EventChan <- engine.UpTaskEvent{
					OSFileEvent: engine.OSFileEvent{
						Action:    "create",
						LocalPath: fullPath,
						Size:      info.Size(),
						ModTime:   info.ModTime().Unix(),
					},
					RemotePath:   remotePath,
					AccountID:    task.AccountID,
					DriveType:    driveType,
					LocalRoot:    task.LocalPath,
					OnConflict:   task.Up.OnConflict,
					SyncDeletion: task.Up.SyncDeletion,
					Force:        true,
				}
				<-sem

				countMu.Lock()
				count++
				countMu.Unlock()
			}
		}
	}

	wg.Add(1)
	walk(task.LocalPath)
	wg.Wait()
	
	// Enqueue a final marker event to persist the "Done" state after all files are processed
	queue.EventChan <- engine.UpTaskEvent{
		OSFileEvent: engine.OSFileEvent{
			LocalPath: task.LocalPath,
			Action:    "MARK_DONE",
		},
		AccountID: task.AccountID,
		DriveType: driveType,
	}

	logger.LogInfo("[%s] Initial scan completed. %d files enqueued for processing.", task.AccountID, count)
	return nil
}

// shouldUpSync determines if a local file needs to be checked by the cloud during initial scan
func shouldUpSync(accountID, driveType, localPath string, info os.FileInfo) bool {
	saved, err := state.GetFileState(accountID, driveType, localPath)
	if err != nil || saved == nil {
		return true // New file
	}

	// If size or modification time differs, it needs a check
	// ModTime is stored in seconds precision.
	if info.Size() != saved.Size || info.ModTime().Unix() != saved.SyncTime {
		return true
	}

	return false
}
