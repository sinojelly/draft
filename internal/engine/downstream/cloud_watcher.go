package downstream

import (
	"errors"
	"time"

	"syncghost/internal/config"
	"syncghost/internal/drive"
	"syncghost/internal/engine"
	"syncghost/internal/logger"
	"syncghost/internal/state"
)

// CloudWatcher polls the cloud drive for changes using incremental cursor APIs
type CloudWatcher struct {
	AccountID string
	Plugin    drive.CloudDrive
	Tasks     []config.SyncTask
	EventChan chan engine.CloudFileEvent
	DownChan  chan engine.DownTaskEvent
	Interval  time.Duration
	stopChan  chan struct{}
}

func NewCloudWatcher(accountID string, plugin drive.CloudDrive, tasks []config.SyncTask, downChan chan engine.DownTaskEvent) *CloudWatcher {
	return &CloudWatcher{
		AccountID: accountID,
		Plugin:    plugin,
		Tasks:     tasks,
		EventChan: make(chan engine.CloudFileEvent, 1000),
		DownChan:  downChan,
		Interval:  10 * time.Second,
		stopChan:  make(chan struct{}),
	}
}

func (cw *CloudWatcher) Start() {
	go func() {
		logger.LogInfo("[%s] CloudWatcher started.", cw.AccountID)
		
		// 1. Load or Initialize Cursor
		cursor := state.GetCloudCursor(cw.AccountID)
		if cursor == "" {
			var err error
			cursor, err = cw.Plugin.GetLatestCursor()
			if err != nil {
				logger.ErrorLog.Printf("[%s] CloudWatcher: Initial cursor fetch failed: %v", cw.AccountID, err)
			} else {
				state.SaveCloudCursor(cw.AccountID, cursor)
				logger.LogInfo("[%s] CloudWatcher: Initialized cursor: %s", cw.AccountID, cursor)
			}
		}

		ticker := time.NewTicker(cw.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-cw.stopChan:
				return
			case <-ticker.C:
				cw.poll(cursor)
				// Update local cursor reference from state in case it changed externally
				cursor = state.GetCloudCursor(cw.AccountID)
			}
		}
	}()
}

func (cw *CloudWatcher) poll(cursor string) {
	if cursor == "" {
		// If we still don't have a cursor, try to fetch it one last time
		newCursor, err := cw.Plugin.GetLatestCursor()
		if err != nil || newCursor == "" {
			logger.LogInfo("[%s] CloudWatcher: Still no valid cursor. Triggering full-re-sync fallback.", cw.AccountID)
			for _, task := range cw.Tasks {
				if task.Down.Enable {
					go PerformInitialDownScan(task, "baidu", cw.Plugin, cw.DownChan)
				}
			}
			return
		}
		cursor = newCursor
		state.SaveCloudCursor(cw.AccountID, cursor)
	}

	for {
		changes, nextCursor, hasMore, err := cw.Plugin.GetIncrementalChanges(cursor)
		if err != nil {
			if errors.Is(err, drive.ErrCursorInvalid) {
				logger.ErrorLog.Printf("[%s] CloudWatcher: Cursor invalid. Triggering fallback full scan.", cw.AccountID)
				// Re-initialize cursor and trigger full scan
				for _, task := range cw.Tasks {
					if task.Down.Enable {
						go func(t config.SyncTask) {
							// Drive type logic could be refined, but for now we assume Baidu/generic
							// In a real app we'd pass the actual drive type from config
							PerformInitialDownScan(t, "baidu", cw.Plugin, cw.DownChan)
						}(task)
					}
				}
				// Reset cursor in state to force re-initialization on next poll if needed
				state.SaveCloudCursor(cw.AccountID, "")
			} else {
				logger.ErrorLog.Printf("[%s] CloudWatcher poll failed: %v", cw.AccountID, err)
			}
			return
		}

		// Process entries
		for _, change := range changes {
			cw.EventChan <- engine.CloudFileEvent{
				RemotePath: change.Path,
				Action:     change.Action,
				FsID:       change.FsID,
				Size:       change.Size,
				MD5:        change.MD5,
				IsDir:      change.IsDir,
				ModTime:    change.ModTime,
				AccountID:  cw.AccountID,
			}
		}

		// Update cursor
		cursor = nextCursor
		state.SaveCloudCursor(cw.AccountID, cursor)

		if !hasMore {
			break
		}
	}
}

func (cw *CloudWatcher) Stop() {
	close(cw.stopChan)
	close(cw.EventChan)
}
