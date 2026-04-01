package upstream

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"syncghost/internal/config"
	"syncghost/internal/drive"
	"syncghost/internal/engine"
	"syncghost/internal/logger"
	"syncghost/internal/state"
	"syncghost/internal/status"
)

type UpEngine struct {
	plugin          drive.CloudDrive
	batchChan       chan []engine.UpTaskEvent
	workers         int
	wg              sync.WaitGroup
	processingFiles sync.Map // Track files currently being processed to avoid concurrent duplicates
}

func NewUpEngine(plugin drive.CloudDrive, batchChan chan []engine.UpTaskEvent) *UpEngine {
	workers := config.GlobalConfig.Performance.MaxConcurrency
	if workers <= 0 {
		workers = 5
	}
	return &UpEngine{
		plugin:    plugin,
		batchChan: batchChan,
		workers:   workers,
	}
}

func (e *UpEngine) Start() {
	// Start fixed number of workers to process batches
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
	logger.LogInfo("UpEngine started with %d workers.", e.workers)
}

func (e *UpEngine) Wait() {
	e.wg.Wait()
}

func (e *UpEngine) worker(id int) {
	defer e.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(fmt.Sprintf("WorkerPatch:%d", id), fmt.Errorf("panic recovered: %v", r))
			status.AddError(fmt.Sprintf("Worker %d crashed and recovered from critical error", id))
		}
	}()
	for batch := range e.batchChan {
		startTime := time.Now()
		var totalBytes int64 = 0
		
		// Optimization: Topological deduplication for deletions
		originalCount := len(batch)
		batch = e.deduplicateBatch(batch)
		if len(batch) < originalCount {
			logger.LogInfo("UpEngine: Worker %d optimized batch from %d to %d events", id, originalCount, len(batch))
		}

		logger.LogInfo("UpEngine: Worker %d processing batch of %d events", id, len(batch))
		
		for _, event := range batch {
			// Note: processEvent could be improved to return bytes handled
			// for more accurate throughput (skipping errors/skips)
			e.processEvent(event)
			
			// Simple byte counting for throughput estimation
			if event.Action != "delete" {
				if stat, err := os.Stat(event.LocalPath); err == nil {
					totalBytes += stat.Size()
				}
			}
		}

		duration := time.Since(startTime)
		if duration.Seconds() > 0 && totalBytes > 0 {
			throughput := float64(totalBytes) / 1024 / 1024 / duration.Seconds()
			logger.LogInfo("UpEngine: Batch completed: %d files, %.2f MB in %v (%.2f MB/s)", 
				len(batch), float64(totalBytes)/1024/1024, duration.Round(10*time.Millisecond), throughput)
			status.UpdateLastBatchStats(len(batch), duration.Seconds(), throughput)
		}
	}
}

func (e *UpEngine) processEvent(event engine.UpTaskEvent) {
	// Account signature for db and error logging
	accountID := event.AccountID
	driveType := event.DriveType

	// 1. Remote-level concurrency lock: Skip if this remote path is already being processed
	// This prevents collisions if multiple local tasks map to the same cloud folder.
	lockKey := accountID + ":" + event.RemotePath
	if _, loaded := e.processingFiles.LoadOrStore(lockKey, true); loaded {
		logger.LogInfo("[%s] Remote path %s is already being processed, skipping concurrent event", accountID, event.RemotePath)
		return
	}
	defer e.processingFiles.Delete(lockKey)

	status.UpdateMetrics(0, 0, 1)        // increase workers for this task
	defer status.UpdateMetrics(0, 0, -1) // decrease workers when done

	// 2. Track specific task progress for the dashboard
	taskDesc := fmt.Sprintf("[%s] Processing %s", driveType, filepath.Base(event.LocalPath))
	status.AddActiveTask(event.LocalPath, taskDesc)
	defer status.RemoveActiveTask(event.LocalPath)

	if event.Action == "delete" {
		if event.SyncDeletion {
			if engine.GlobalDeletionGuard.AllowDeletion(accountID) {
				logger.LogInfo("[%s] SyncDeletion enabled: Deleting remote for %s", accountID, event.LocalPath)
				var err error
				if event.IsDir {
					err = e.plugin.DeleteDir(event.RemotePath)
				} else {
					err = e.plugin.Delete(event.RemotePath)
				}

				if err != nil {
					logger.LogInfo("[%s] Remote delete skipped or failed for %s: %v", accountID, event.LocalPath, err)
				}
			} else {
				logger.LogInfo("[%s] SAFETY: Remote delete blocked by DeletionGuard for %s", accountID, event.LocalPath)
			}
		} else {
			logger.LogInfo("[%s] SyncDeletion disabled: Skipping remote delete for %s", accountID, event.LocalPath)
		}

		err := state.DeleteFileState(accountID, driveType, event.LocalPath)
		if err != nil {
			logger.LogError(fmt.Sprintf("%s_%s:Delete", accountID, driveType), err)
			status.AddError(fmt.Sprintf("[%s] DeleteState failed for %s: %v", accountID, event.LocalPath, err))
		} else {
			status.AddActivity(fmt.Sprintf("[%s] Purged %s", accountID, filepath.Base(event.LocalPath)))
		}
		return
	}

	// For create or modify, check if file still exists
	// Handle special markers
	if event.Action == "MARK_DONE" {
		logger.LogInfo("[%s] Finalizing initial sync state for %s", accountID, event.LocalPath)
		state.MarkInitialSyncDone(accountID, event.LocalPath)
		return
	}

	stat, err := os.Stat(event.LocalPath)
	if err != nil {
		// File might have been deleted right after creation
		return
	}

	// Read state to check if already synced and unchanged (unless forced)
	if !event.Force {
		savedState, err := state.GetFileState(accountID, driveType, event.LocalPath)
		if err == nil && savedState != nil {
			if savedState.Size == stat.Size() && savedState.SyncTime >= stat.ModTime().Unix() {
				logger.LogInfo("[%s] Skipping unchanged file: %s", accountID, event.LocalPath)
				status.AddActivity(fmt.Sprintf("[%s] Skipped (identical): %s", accountID, filepath.Base(event.LocalPath)))
				return
			}
		}
	}

	// File genuinely needs upload or server verification. Try exclusive open to avoid "The process cannot access the file" due to write lock
	// Optimization: Non-blocking check to avoid worker starvation.
	// If the file is locked (e.g., by a long-running rendering process), we skip it now.
	// The next OS modify/close event will naturally trigger another attempt.
	f, err := os.OpenFile(event.LocalPath, os.O_RDONLY, 0)
	if err != nil {
		logger.LogInfo("[%s] File is currently locked by another process, skipping for now: %s", accountID, event.LocalPath)
		return
	}
	f.Close()

	logger.LogInfo("[%s] Uploading %s -> %s (Conflict policy: %s)", accountID, event.LocalPath, event.RemotePath, event.OnConflict)
	status.AddActivity(fmt.Sprintf("[%s] Syncing %s...", accountID, filepath.Base(event.LocalPath)))

	remoteDir := path.Dir(event.RemotePath)
	remoteFileID, err := e.plugin.Upload(event.LocalPath, remoteDir, event.OnConflict, func(transferred, total int64) {
		status.UpdateTaskProgress(event.LocalPath, transferred, total)
	})

	if err != nil {
		// Resilience: If the file was deleted/moved DURING the upload process, ignore the error
		if os.IsNotExist(err) || strings.Contains(err.Error(), "The system cannot find the file specified") {
			logger.LogInfo("[%s] File vanished during upload, skipping: %s", accountID, event.LocalPath)
			return
		}
		
		// Phase 70: Track conflicts if skipped by policy
		if strings.Contains(err.Error(), "skipping upload due to 'skip' conflict policy") {
			status.AddConflict()
			status.AddActivity(fmt.Sprintf("[%s] Conflict (Ignored): %s", accountID, filepath.Base(event.LocalPath)))
			return
		}

		logger.LogError(fmt.Sprintf("%s_%s:Upload", accountID, driveType), err)
		status.AddError(fmt.Sprintf("[%s] Upload failed for %s: %v", accountID, event.LocalPath, err))

		state.RecordFailure(accountID, driveType, event.LocalPath, event.RemotePath, err.Error(), "up", "", "", stat.Size())
		return
	}

	logger.LogInfo("[%s] Successfully uploaded to %s, fs_id: %s", accountID, event.RemotePath, remoteFileID)
	status.AddActivity(fmt.Sprintf("[%s] Synced %s", accountID, filepath.Base(event.LocalPath)))
	status.UpdateMetrics(1, 0, 0) // successfully synced a file (UP)

	newState := state.FileState{
		Size:     stat.Size(),
		SyncTime: time.Now().Unix(),
		FileID:   remoteFileID,
	}
	
	err = state.SaveFileState(accountID, driveType, event.LocalPath, newState)
	if err != nil {
		logger.LogError(fmt.Sprintf("%s_%s:SaveState", accountID, driveType), err)
	}
	// Phase 55: Clear failure records from DB and live Dashboard upon success
	state.ClearFailure(accountID, event.LocalPath)
	status.RemoveFailure(event.LocalPath)
}

// deduplicateBatch removes redundant delete events (e.g., deleting a file inside a directory already being deleted)
func (e *UpEngine) deduplicateBatch(batch []engine.UpTaskEvent) []engine.UpTaskEvent {
	if len(batch) <= 1 {
		return batch
	}

	// 1. Identify all directory deletions
	dirDeletes := make(map[string]bool)
	for _, event := range batch {
		if event.Action == "delete" && event.IsDir {
			// Ensure path ends with / for prefix matching
			p := event.RemotePath
			if !strings.HasSuffix(p, "/") {
				p += "/"
			}
			dirDeletes[p] = true
		}
	}

	if len(dirDeletes) == 0 {
		return batch
	}

	// 2. Filter batch
	var result []engine.UpTaskEvent
	for _, event := range batch {
		if event.Action == "delete" && !event.IsDir {
			// Check if this file resides in a deleted directory
			isRedundant := false
			for dirPath := range dirDeletes {
				if strings.HasPrefix(event.RemotePath, dirPath) {
					isRedundant = true
					break
				}
			}
			if isRedundant {
				continue
			}
		}
		
		// If it's a directory delete itself, check if it's a sub-directory of another directory delete
		if event.Action == "delete" && event.IsDir {
			isRedundant := false
			for dirPath := range dirDeletes {
				// Don't compare with self
				p := event.RemotePath
				if !strings.HasSuffix(p, "/") { p += "/" }
				
				if dirPath != p && strings.HasPrefix(p, dirPath) {
					isRedundant = true
					break
				}
			}
			if isRedundant {
				continue
			}
		}

		result = append(result, event)
	}

	return result
}
