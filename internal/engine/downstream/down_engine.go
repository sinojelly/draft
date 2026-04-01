package downstream

import (
	"fmt"
	"os"
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

// DownEngine handles cloud-to-local synchronization tasks
type DownEngine struct {
	plugin          drive.CloudDrive
	eventChan       chan engine.DownTaskEvent
	workers         int
	wg              sync.WaitGroup
	processingFiles sync.Map
}

func NewDownEngine(plugin drive.CloudDrive, eventChan chan engine.DownTaskEvent) *DownEngine {
	workers := config.GlobalConfig.Performance.MaxConcurrency
	if workers <= 0 {
		workers = 5
	}
	return &DownEngine{
		plugin:    plugin,
		eventChan: eventChan,
		workers:   workers,
	}
}

func (e *DownEngine) Start() {
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
	logger.LogInfo("DownEngine started with %d workers.", e.workers)
}

func (e *DownEngine) worker(id int) {
	defer e.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(fmt.Sprintf("DownWorker:%d", id), fmt.Errorf("panic recovered: %v", r))
			status.AddError(fmt.Sprintf("DownWorker %d crashed and recovered", id))
		}
	}()

	for event := range e.eventChan {
		e.processEvent(event)
	}
}

func (e *DownEngine) processEvent(event engine.DownTaskEvent) {
	accountID := event.AccountID
	driveType := event.DriveType

	// 1. Concurrency Lock
	lockKey := accountID + ":" + event.LocalPath
	if _, loaded := e.processingFiles.LoadOrStore(lockKey, true); loaded {
		return
	}
	defer e.processingFiles.Delete(lockKey)

	status.UpdateMetrics(0, 0, 1)        // increase workers
	defer status.UpdateMetrics(0, 0, -1) // decrease workers when done

	// 2. Telemetry
	taskDesc := fmt.Sprintf("[%s] Downloading %s", driveType, filepath.Base(event.RemotePath))
	status.AddActiveTask(event.LocalPath, taskDesc)
	defer status.RemoveActiveTask(event.LocalPath)

	// 3. Handle Deletion
	if event.Action == "delete" {
		if event.SyncDeletion {
			if engine.GlobalDeletionGuard.AllowDeletion(accountID) {
				logger.LogInfo("[%s] DownSync: Soft-Deleting local file %s", accountID, event.LocalPath)
				if err := engine.TrashLocal(event.LocalRoot, event.LocalPath); err != nil && !os.IsNotExist(err) {
					logger.LogError("DownSync:TrashLocal", err)
				}
			} else {
				logger.LogInfo("[%s] SAFETY: Local delete blocked by DeletionGuard for %s", accountID, event.LocalPath)
			}
		}
		state.DeleteFileState(accountID, driveType, event.LocalPath)
		return
	}

	// 4. Handle Conflict (Overwrite/Rename)
	finalLocalPath := event.LocalPath
	if _, err := os.Stat(finalLocalPath); err == nil {
		if event.OnConflict == "skip" {
			logger.LogInfo("[%s] Conflict: Skip policy for %s", accountID, event.LocalPath)
			status.AddConflict()
			return
		}
		if event.OnConflict == "rename" {
			finalLocalPath = generateUniqueLocalPath(finalLocalPath)
			if finalLocalPath != event.LocalPath {
				status.AddConflict()
			}
		}
		// overwrite is default (handled by plugin.Download)
	}

	// 5. Execute Download with Atomic Rename
	tempLocalPath := finalLocalPath + ".sgdownload"
	logger.LogInfo("[%s] Downloading %s -> %s (Temp: %s)", accountID, event.RemotePath, finalLocalPath, tempLocalPath)
	
	downloadStartTime := time.Now()
	err := e.plugin.Download(event.RemotePath, tempLocalPath, func(transferred, total int64) {
		status.UpdateTaskProgress(event.LocalPath, transferred, total)
	})
	
	if err != nil {
		logger.LogError("DownSync:Download", err)
		status.AddError(fmt.Sprintf("[%s] Download failed: %v", accountID, err))
		state.RecordFailure(accountID, driveType, event.LocalPath, event.RemotePath, err.Error(), "down", fmt.Sprintf("%v", event.FsID), event.MD5, event.Size)
		// Cleanup partial download
		os.Remove(tempLocalPath)
		return
	}

	// Double check MD5/Size before rename if possible
	stat, _ := os.Stat(tempLocalPath)
	if event.Size > 0 && stat.Size() != event.Size {
		logger.LogInfo("[%s] DownSync: Size mismatch for %s (Expected %d, got %d). Retrying...", 
			accountID, event.LocalPath, event.Size, stat.Size())
		os.Remove(tempLocalPath)
		return
	}

	// Atomic Rename with Retries (Phase 80 Windows Hardening)
	if err := atomicRename(tempLocalPath, finalLocalPath); err != nil {
		logger.LogError("DownSync:Rename", err)
		status.AddError(fmt.Sprintf("[%s] Rename failed after retries: %v", accountID, err))
		return
	}

	duration := time.Since(downloadStartTime)
	throughput := 0.0
	if duration.Seconds() > 0 {
		throughput = float64(stat.Size()) / 1024 / 1024 / duration.Seconds()
	}

	// 6. Finalize State
	logger.LogInfo("[%s] Successfully downloaded %s (%.2f MB/s)", accountID, event.RemotePath, throughput)
	status.AddActivity(fmt.Sprintf("[%s] Pulled %s", accountID, filepath.Base(event.RemotePath)))
	status.UpdateMetrics(0, 1, 0) // successfully synced a file (DOWN)
	status.UpdateLastBatchStats(1, duration.Seconds(), throughput)
	
	newState := state.FileState{
		Size:     stat.Size(),
		SyncTime: time.Now().Unix(),
		FileID:   fmt.Sprintf("%v", event.FsID),
		MD5:      event.MD5,
	}
	state.SaveFileState(accountID, driveType, event.LocalPath, newState)
	state.ClearFailure(accountID, event.LocalPath)
}

func (e *DownEngine) Stop() {
	// eventChan should be closed by Router
	e.wg.Wait()
}

func generateUniqueLocalPath(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; i < 100; i++ {
		newPath := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
	return path + ".conflict"
}

// atomicRename handles Windows-specific file lock contention with exponential backoff.
func atomicRename(oldpath, newpath string) error {
	var err error
	for i := 0; i < 3; i++ {
		err = os.Rename(oldpath, newpath)
		if err == nil {
			return nil
		}
		if i < 2 {
			logger.LogInfo("Rename attempt %d failed: %v. Retrying in %ds...", i+1, err, 1<<i)
			time.Sleep(time.Duration(1<<i) * time.Second) // 1s, 2s
		}
	}
	return err
}
