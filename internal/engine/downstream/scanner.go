package downstream

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"syncghost/internal/config"
	"syncghost/internal/drive"
	"syncghost/internal/engine"
	"syncghost/internal/logger"
	"syncghost/internal/state"
)

// PerformInitialDownScan recursively walks the cloud directory and enqueues missing/outdated files for download.
// Optimization (Phase 66): Introduction of parallel directory listing.
func PerformInitialDownScan(task config.SyncTask, driveType string, plugin drive.CloudDrive, downChan chan engine.DownTaskEvent) error {
	logger.LogInfo("[%s] Starting concurrent cloud-to-local scan for %s", task.AccountID, task.RemotePath)
	filter := engine.NewSyncFilter(task.LocalPath)
	count := 0
	var countMu sync.Mutex
	
	// Limit concurrency to avoid secondary Baidu rate limits (Error 31034)
	sem := make(chan struct{}, 5) 
	var wg sync.WaitGroup

	// 1. Initialize Orphan Detection (Phase 76 Reliability)
	// Get all tracked files for this account to find things deleted on cloud
	orphans := make(map[string]bool)
	if task.Down.SyncDeletion {
		dbStates, err := state.GetFileStatesForAccount(task.AccountID, driveType)
		if err == nil {
			logger.LogInfo("[%s] Orphan Detection: Found %d total states in DB", task.AccountID, len(dbStates))
			for path := range dbStates {
				// Only consider orphans that belong to the current task's local path
				if strings.HasPrefix(strings.ToLower(path), strings.ToLower(task.LocalPath)) {
					logger.LogInfo("[%s] Orphan Candidate: %s", task.AccountID, path)
					orphans[path] = true
				}
			}
		} else {
			logger.LogError("Orphan Detection:GetFileStates", err)
		}
	}
	var orphansMu sync.Mutex

	var walk func(remoteDir string)
	walk = func(remoteDir string) {
		defer wg.Done()

		sem <- struct{}{}
		items, err := plugin.ListDir(remoteDir)
		<-sem

		if err != nil {
			logger.LogError("InitialDownScan:List", fmt.Errorf("dir %s: %v", remoteDir, err))
			return
		}

		for _, item := range items {
			rel, err := filepath.Rel(task.RemotePath, item.Path)
			if err != nil {
				continue
			}

			// Phase 68: Unified filtering
			if filter.ShouldIgnore(rel) {
				continue
			}

			if item.IsDir {
				wg.Add(1)
				go walk(item.Path)
				continue
			}

			localPath := filepath.Join(task.LocalPath, filepath.FromSlash(rel))
			
			// Mark as "not an orphan"
			orphansMu.Lock()
			delete(orphans, localPath)
			orphansMu.Unlock()

			if shouldDownSync(task.AccountID, driveType, localPath, item) {
				downChan <- engine.DownTaskEvent{
					CloudFileEvent: engine.CloudFileEvent{
						Action:     "create",
						RemotePath: item.Path,
						FsID:       item.FsID,
						MD5:        item.MD5,
						Size:       item.Size,
						ModTime:    item.ModTime,
						AccountID:  task.AccountID,
					},
					LocalPath:    localPath,
					LocalRoot:    task.LocalPath,
					DriveType:    driveType,
					OnConflict:   task.Down.OnConflict,
					SyncDeletion: task.Down.SyncDeletion,
				}
				countMu.Lock()
				count++
				countMu.Unlock()
			}
		}
	}

	wg.Add(1)
	walk(task.RemotePath)
	wg.Wait()

	// 2. Enqueue deletions for orphans (Phase 76)
	if task.Down.SyncDeletion && len(orphans) > 0 {
		logger.LogInfo("[%s] Found %d orphaned files. Enqueuing local deletions.", task.AccountID, len(orphans))
		for path := range orphans {
			downChan <- engine.DownTaskEvent{
				CloudFileEvent: engine.CloudFileEvent{
					Action:     "delete",
					AccountID:  task.AccountID,
				},
				LocalPath:    path,
				LocalRoot:    task.LocalPath,
				DriveType:    driveType,
				SyncDeletion: true,
			}
		}
	}

	logger.LogInfo("[%s] Concurrent initial down-scan completed. %d files enqueued for download.", task.AccountID, count)
	return nil
}

// shouldDownSync determines if a cloud file needs to be downloaded based on local state
func shouldDownSync(accountID, driveType, localPath string, cloudItem drive.CloudChange) bool {
	// If local file doesn't exist, we definitely need it
	stat, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return true
	}

	// If it exists, check the state database
	saved, err := state.GetFileState(accountID, driveType, localPath)
	if err != nil || saved == nil {
		// No record of this file being synced, but it exists locally.
		// For safety, if size matches, we might skip, but better to trust cloud version if MD5 differs.
		if stat.Size() == cloudItem.Size {
			// Optimization: if size matches and no record, assume they might be same for now
			return false 
		}
		return true
	}

	// We have a record. Check if cloud version is DIFFERENT from what we last synced.
	if saved.FileID != cloudItem.FsID {
		return true
	}
	
	// Optional: MD5 check if available
	if cloudItem.MD5 != "" && saved.MD5 != "" && cloudItem.MD5 != saved.MD5 {
		return true
	}

	return false
}
