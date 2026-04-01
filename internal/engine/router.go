package engine

import (
	"path/filepath"
	"strings"

	"syncghost/internal/config"
	"syncghost/internal/logger"
	"syncghost/internal/state"
)

type EventRouter struct {
	osEventsChan    <-chan OSFileEvent
	cloudEventsChan <-chan CloudFileEvent
	accountQueues   map[string]*TaskQueue
	downChans       map[string]chan DownTaskEvent
	filters         map[string]*SyncFilter // LocalPath -> Filter
}

func NewEventRouter(osEvents <-chan OSFileEvent, cloudEvents <-chan CloudFileEvent, queues map[string]*TaskQueue) *EventRouter {
	r := &EventRouter{
		osEventsChan:    osEvents,
		cloudEventsChan: cloudEvents,
		accountQueues:   queues,
		downChans:       make(map[string]chan DownTaskEvent),
		filters:         make(map[string]*SyncFilter),
	}
	// Pre-load filters for each task
	for _, task := range config.GlobalConfig.SyncTasks {
		r.filters[task.LocalPath] = NewSyncFilter(task.LocalPath)
	}
	return r
}

// RegisterDownChan assigns a download channel to an account
func (r *EventRouter) RegisterDownChan(accountID string, ch chan DownTaskEvent) {
	r.downChans[accountID] = ch
}

func (r *EventRouter) Start() {
	// Goroutine 1: OS -> Cloud (UP)
	go func() {
		for osEvent := range r.osEventsChan {
			r.handleOSEvent(osEvent)
		}
		for _, q := range r.accountQueues {
			q.Stop()
		}
	}()

	// Goroutine 2: Cloud -> OS (DOWN)
	go func() {
		if r.cloudEventsChan == nil {
			return
		}
		for cloudEvent := range r.cloudEventsChan {
			r.handleCloudEvent(cloudEvent)
		}
		for _, ch := range r.downChans {
			close(ch)
		}
	}()
}

func (r *EventRouter) handleOSEvent(osEvent OSFileEvent) {
	for _, task := range config.GlobalConfig.SyncTasks {
		// Phase 61: Only process upload events if UP is enabled for this task
		if !task.Up.Enable {
			continue
		}

		// Normalize paths for comparison (especially on Windows)
		cleanEventPath := strings.ToLower(filepath.Clean(osEvent.LocalPath))
		cleanTaskPath := strings.ToLower(filepath.Clean(task.LocalPath))

		relPath, err := filepath.Rel(cleanTaskPath, cleanEventPath)
		if err != nil {
			continue
		}

		// Check original case relPath for actual usage
		actualRelPath, err := filepath.Rel(filepath.Clean(task.LocalPath), filepath.Clean(osEvent.LocalPath))
		if err == nil && !strings.HasPrefix(relPath, "..") && relPath != "." {
			// Phase 68: Robust Ignore logic
			if f, ok := r.filters[task.LocalPath]; ok && f.ShouldIgnore(actualRelPath) {
				continue
			}

			// Compute Remote
			normalizedRelPath := filepath.ToSlash(actualRelPath)
			remotePath := filepath.ToSlash(filepath.Join(task.RemotePath, normalizedRelPath))
			remotePath = strings.ReplaceAll(remotePath, "//", "/")

			// Compute Drive
			driveType := "unknown"
			for _, acc := range config.GlobalConfig.Accounts {
				if acc.ID == task.AccountID {
					driveType = acc.Type
					break
				}
			}

			// Phase 71: Media Filtering for Yike and similar drives
			// We need to check capabilities. Since router doesn't have plugin instances,
			// we might need to pass them or just check by driveType.
			// However, a cleaner way is to let the TaskQueue or Engine handle it,
			// but the design says Router should intercept to avoid I/O.

			// For now, I'll use a hardcoded check for 'yike' in Router or
			// better: retrieve the plugin from a global registry if available.
			// But since we want to follow the design doc's "Router intercept",
			// I'll add the check here.
			if driveType == "yike" && !osEvent.IsDir {
				ext := strings.ToLower(filepath.Ext(osEvent.LocalPath))
				allowed := []string{".jpg", ".jpeg", ".png", ".gif", ".heic", ".bmp", ".webp", ".mp4", ".mov", ".avi", ".mkv"}
				isMedia := false
				for _, a := range allowed {
					if a == ext {
						isMedia = true
						break
					}
				}
				if !isMedia {
					// logger.LogInfo("Router: Skipping non-media file %s for Yike account %s", osEvent.LocalPath, task.AccountID)
					continue
				}
			}

			syncEvent := UpTaskEvent{
				OSFileEvent:  osEvent,
				AccountID:    task.AccountID,
				DriveType:    driveType,
				LocalRoot:    task.LocalPath,
				RemotePath:   remotePath,
				IsDir:        osEvent.IsDir,
				OnConflict:   task.Up.OnConflict,
				SyncDeletion: task.Up.SyncDeletion,
			}

			// Dispatch
			if q, exists := r.accountQueues[task.AccountID]; exists {
				logger.LogInfo("Router: Dispatching %s action on Local %s for %s (Remote: %s)", osEvent.Action, osEvent.LocalPath, task.AccountID, remotePath)
				q.EventChan <- syncEvent
			}
		}
	}
}

func (r *EventRouter) handleCloudEvent(cloudEvent CloudFileEvent) {
	for _, task := range config.GlobalConfig.SyncTasks {
		if task.AccountID != cloudEvent.AccountID || !task.Down.Enable {
			continue
		}

		// Check if remote path is within task scope
		remoteRoot := task.RemotePath
		if !strings.HasSuffix(remoteRoot, "/") {
			remoteRoot += "/"
		}

		if !strings.HasPrefix(cloudEvent.RemotePath, remoteRoot) {
			continue
		}

		// Resolve local path
		relPath := strings.TrimPrefix(cloudEvent.RemotePath, remoteRoot)
		localPath := filepath.Join(task.LocalPath, filepath.FromSlash(relPath))

		// Phase 63: Echo Prevention (The Core Filter)
		driveType := "unknown"
		for _, acc := range config.GlobalConfig.Accounts {
			if acc.ID == task.AccountID {
				driveType = acc.Type
				break
			}
		}

		st, err := state.GetFileState(task.AccountID, driveType, localPath)
		if err == nil && st != nil {
			// If FsID and MD5 match exactly, this is a loopback event from our own upload
			if st.FileID == cloudEvent.FsID && strings.EqualFold(st.MD5, cloudEvent.MD5) {
				logger.LogInfo("Router: Echo Intercepted for %s (already synced)", cloudEvent.RemotePath)
				continue
			}
		}

		// Phase 68: Unified Ignore check
		if f, ok := r.filters[task.LocalPath]; ok && f.ShouldIgnore(relPath) {
			continue
		}

		logger.LogInfo("Router: Accepted cloud event for %s -> %s", cloudEvent.RemotePath, localPath)

		// Dispatch to DownEngine
		if ch, exists := r.downChans[task.AccountID]; exists {
			ch <- DownTaskEvent{
				CloudFileEvent: cloudEvent,
				LocalPath:      localPath,
				LocalRoot:      task.LocalPath,
				DriveType:      driveType,
				OnConflict:     task.Down.OnConflict,
				SyncDeletion:   task.Down.SyncDeletion,
			}
		}
	}
}
