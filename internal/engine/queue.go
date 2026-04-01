package engine

import (
	"os"
	"sync"
	"time"

	"syncghost/internal/config"
	"syncghost/internal/logger"
)

// TaskQueue implements the funnel model (debounce & batching)
type TaskQueue struct {
	EventChan chan UpTaskEvent  // Inbound routed events
	BatchChan chan []UpTaskEvent // Outbound batched events

	activeEvents map[string]UpTaskEvent
	mu           sync.Mutex
	timer        *time.Timer
	debounce     time.Duration
	wg           sync.WaitGroup
	closeOnce    sync.Once
}

func NewTaskQueue(maxBatch int, debounce time.Duration) *TaskQueue {
	return &TaskQueue{
		EventChan:    make(chan UpTaskEvent, 10000),
		BatchChan:    make(chan []UpTaskEvent, 100),
		activeEvents: make(map[string]UpTaskEvent),
		debounce:     debounce,
	}
}

func (q *TaskQueue) Start() {
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		debounceDuration := q.debounce
		maxBatch := config.GlobalConfig.Performance.MaxBatchCount
		if maxBatch <= 0 {
			maxBatch = 100
		}

		q.timer = time.NewTimer(debounceDuration)
		// Stop the timer initially so it doesn't fire immediately
		if !q.timer.Stop() {
			<-q.timer.C
		}

		var timerActive bool
		for {
			select {
			case event, ok := <-q.EventChan:
				if !ok {
					// Flush remaining before exit
					q.mu.Lock()
					q.flush()
					q.mu.Unlock()
					close(q.BatchChan)
					return
				}
				q.mu.Lock()
				
				// Phase 56: Event Collapsing
				// If we have a 'create' event followed by a 'delete' in the same debounce window,
				// they collapse into nothing (file was temporary and never reached the cloud).
				if oldEvent, exists := q.activeEvents[event.LocalPath]; exists {
					if oldEvent.Action == "create" && event.Action == "delete" {
						delete(q.activeEvents, event.LocalPath)
						logger.LogInfo("Queue: Collapsed ephemeral create-then-delete for %s", event.LocalPath)
						q.mu.Unlock()
						continue
					}
				}

				q.activeEvents[event.LocalPath] = event
				
				if len(q.activeEvents) >= maxBatch {
					if timerActive {
						if !q.timer.Stop() {
							select {
							case <-q.timer.C:
							default:
							}
						}
						timerActive = false
					}
					q.flush()
				} else {
					// Every time an event arrives, reset the timer to extend the window (true debounce)
					if timerActive {
						if !q.timer.Stop() {
							select {
							case <-q.timer.C:
							default:
							}
						}
					}
					q.timer.Reset(debounceDuration)
					timerActive = true
				}
				q.mu.Unlock()

			case <-q.timer.C:
				q.mu.Lock()
				timerActive = false
				q.flush()
				q.mu.Unlock()
			}
		}
	}()
	logger.LogInfo("Task Queue (Funnel) started.")
}

func (q *TaskQueue) flush() {
	if len(q.activeEvents) == 0 {
		return
	}

	batch := make([]UpTaskEvent, 0, len(q.activeEvents))
	for path, evt := range q.activeEvents {
		// Optimization: Check if file still exists before batching (unless it's a delete event)
		if evt.Action != "delete" {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// File was a temporary/short-lived file that's already gone. Skip.
				continue
			}
		}
		batch = append(batch, evt)
	}

	// Reset map
	q.activeEvents = make(map[string]UpTaskEvent)

	// Release lock temporarily to avoid deadlock when backpressuring
	q.mu.Unlock()
	q.BatchChan <- batch
	q.mu.Lock()
}

func (q *TaskQueue) Stop() {
	q.closeOnce.Do(func() {
		close(q.EventChan)
		logger.LogInfo("TaskQueue stopped securely.")
	})
}

func (q *TaskQueue) Wait() {
	q.wg.Wait()
}
