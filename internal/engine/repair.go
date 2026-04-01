package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"syncghost/internal/config"
	"syncghost/internal/drive"
	"syncghost/internal/logger"
	"syncghost/internal/state"
	"syncghost/internal/status"
)

// PerformRepairScan performs a deep MD5-based consistency check between local and cloud.
func PerformRepairScan(task config.SyncTask, driveType string, plugin drive.CloudDrive, queue *TaskQueue) error {
	logger.LogInfo("[%s] REPAIR: Starting deep MD5 consistency scan for %s", task.AccountID, task.LocalPath)
	status.AddActivity(fmt.Sprintf("[%s] Deep Repair Started...", task.AccountID))
	
	filter := NewSyncFilter(task.LocalPath)
	count := 0
	mismatches := 0

	err := filepath.Walk(task.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip hidden .syncghost dir
		if strings.Contains(path, ".syncghost") {
			return nil
		}

		relPath, _ := filepath.Rel(task.LocalPath, path)
		if filter.ShouldIgnore(relPath) {
			return nil
		}

		count++
		if count%50 == 0 {
			logger.LogInfo("[%s] REPAIR: Audited %d files...", task.AccountID, count)
		}

		// Deep check: Compare Local with Cloud
		remoteDir := task.RemotePath
		remotePath := filepath.ToSlash(filepath.Join(remoteDir, filepath.ToSlash(relPath)))

		// 1. Get Local MD5 from DB (Optimistic)
		saved, err := state.GetFileState(task.AccountID, driveType, path)
		
		// 2. Fetch Live Cloud Info
		// Optimization: Only fetch if needed or for critical repair. 
		// For a "Speed Repair", we might just check if local modtime > sync time.
		// For "Deep Repair", we call the API.
		_, cloudMD5, fsID, err := plugin.GetFileInfo(remotePath)
		if err != nil {
			// File doesn't exist on cloud - repair by uploading
			logger.LogInfo("[%s] REPAIR: File missing on cloud: %s", task.AccountID, relPath)
			mismatches++
			enqueueRepair(task, driveType, path, remotePath, queue)
			return nil
		}

		// 3. Compare MD5
		// We calculate local MD5 only if we have cloud info to compare against
		localMD5, err := drive.CalculateMD5(path)
		if err != nil {
			return nil
		}

		if !strings.EqualFold(localMD5, cloudMD5) {
			logger.LogInfo("[%s] REPAIR: Hash mismatch for %s (Local: %s, Cloud: %s)", 
				task.AccountID, relPath, localMD5, cloudMD5)
			mismatches++
			enqueueRepair(task, driveType, path, remotePath, queue)
		} else if saved == nil || saved.FileID != fsID {
			// State DB is out of sync but files are identical - repair DB only
			logger.LogInfo("[%s] REPAIR: DB out of sync for %s (updating ID)", task.AccountID, relPath)
			state.SaveFileState(task.AccountID, driveType, path, state.FileState{
				Size:     info.Size(),
				SyncTime: info.ModTime().Unix(),
				FileID:   fsID,
				MD5:      localMD5,
			})
		}

		return nil
	})

	logger.LogInfo("[%s] REPAIR COMPLETE: Audited %d files, fixed %d mismatches.", task.AccountID, count, mismatches)
	status.AddActivity(fmt.Sprintf("[%s] Repair Complete (%d fixes)", task.AccountID, mismatches))
	return err
}

func enqueueRepair(task config.SyncTask, driveType string, localPath string, remotePath string, queue *TaskQueue) {
	stat, _ := os.Stat(localPath)
	queue.EventChan <- UpTaskEvent{
		OSFileEvent: OSFileEvent{
			Action:    "create",
			LocalPath: localPath,
			Size:      stat.Size(),
			ModTime:   stat.ModTime().Unix(),
		},
		RemotePath:   remotePath,
		AccountID:    task.AccountID,
		DriveType:    driveType,
		LocalRoot:    task.LocalPath,
		OnConflict:   task.Up.OnConflict,
		SyncDeletion: task.Up.SyncDeletion,
		Force:        true,
	}
}
