package engine

import (
	"context"
	"time"
	"syncghost/internal/config"
	"syncghost/internal/logger"
	"syncghost/internal/state"
)

// RetryManager periodically re-enqueues failed sync tasks across both Upstream and Downstream pipelines.
type RetryManager struct {
	queues    map[string]*TaskQueue
	downChans map[string]chan DownTaskEvent
	interval  time.Duration
	cancel    context.CancelFunc
}


// NewRetryManager creates a new failure recovery manager
func NewRetryManager(queues map[string]*TaskQueue, downChans map[string]chan DownTaskEvent) *RetryManager {
	return &RetryManager{
		queues:    queues,
		downChans: downChans,
		interval:  5 * time.Minute, // Background retry frequency
	}
}

// Start launches the background retry loop
func (rm *RetryManager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	rm.cancel = cancel
	
	go func() {
		logger.LogInfo("RetryManager: Background recovery loop started (interval: %v)", rm.interval)
		ticker := time.NewTicker(rm.interval)
		defer ticker.Stop()
		
		// Run once at start to recover from previous session's failures
		rm.ProcessRetries()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rm.ProcessRetries()
			}
		}
	}()
}

// Stop terminates the retry loop
func (rm *RetryManager) Stop() {
	if rm.cancel != nil {
		rm.cancel()
	}
}

// ProcessRetries scans the database for all recorded failures and re-enqueues them
func (rm *RetryManager) ProcessRetries() {
	failures := state.GetAllFailureDetails()
	if len(failures) == 0 {
		return
	}

	logger.LogInfo("RetryManager: Found %d pending failures, attempt re-synchronization...", len(failures))
	
	for _, f := range failures {
		// Phase 82: Pruning stale failures (those no longer in config)
		pathToCheck := f.LocalPath
		if f.Direction == "down" {
			pathToCheck = f.RemotePath
		}
		if !config.GlobalConfig.IsPathInTask(f.AccountID, f.Direction, pathToCheck) {
			logger.LogInfo("RetryManager: Pruning stale failure record for %s (Account: %s, Direction: %s)", pathToCheck, f.AccountID, f.Direction)
			state.ClearFailure(f.AccountID, f.LocalPath)
			continue
		}

		if f.Direction == "up" {
			if q, ok := rm.queues[f.AccountID]; ok {
				// Re-enqueue for upload
				q.EventChan <- UpTaskEvent{
					OSFileEvent: OSFileEvent{
						LocalPath: f.LocalPath,
						Action:    "modify", // Treat as modify to bypass potential "unchanged" skip
						Size:      f.Size,
					},
					RemotePath: f.RemotePath,
					AccountID:  f.AccountID,
					DriveType:  f.DriveType,
				}
			}
		} else if f.Direction == "down" {
			if ch, ok := rm.downChans[f.AccountID]; ok {
				// Re-enqueue for download
				ch <- DownTaskEvent{
					CloudFileEvent: CloudFileEvent{
						RemotePath: f.RemotePath,
						Action:     "modify",
						FsID:       f.FileID,
						MD5:        f.MD5,
						Size:       f.Size,
						AccountID:  f.AccountID,
					},
					LocalPath: f.LocalPath,
					DriveType: f.DriveType,
				}
			}
		}
	}
}
