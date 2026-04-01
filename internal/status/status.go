package status

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
	"syncghost/internal/state"
)

type TaskProgress struct {
	Transferred int64   `json:"transferred"`
	Total       int64   `json:"total"`
	Percentage  float64 `json:"percentage"`
}

type DashboardState struct {
	TotalUp          int64                    `json:"totalUp"`
	TotalDown        int64                    `json:"totalDown"`
	ActiveWorkers    int                      `json:"activeWorkers"`
	RecentErrors     []string                 `json:"recentErrors"`
	RecentActivity   []string                 `json:"recentActivity"`
	ActiveUpTasks    map[string]string        `json:"activeUpTasks"`
	ActiveDownTasks  map[string]string        `json:"activeDownTasks"`
	UpProgress       map[string]TaskProgress  `json:"upProgress"`
	DownProgress     map[string]TaskProgress  `json:"downProgress"`
	PendingFailures  []state.FailureRecord    `json:"pendingFailures"`
	Conflicts        int64                    `json:"conflicts"`
	UptimeStr        string                   `json:"uptime"`
	LastBatchSize    int                      `json:"lastBatchSize"`
	LastBatchDuration float64                 `json:"lastBatchDuration"` // seconds
	Throughput       float64                  `json:"throughput"`        // MB/s
	LastSyncTime     int64                    `json:"lastSyncTime"`
	ThroughputHistory []float64               `json:"throughputHistory"`
}

var (
	GlobalState = DashboardState{
		RecentErrors:      []string{},
		RecentActivity:    []string{},
		PendingFailures:   []state.FailureRecord{},
		ThroughputHistory: []float64{},
	}
	StateMu         sync.Mutex
	StartTime       = time.Now()
	ActiveUpMap     sync.Map
	ActiveDownMap   sync.Map
	UpProgressMap   sync.Map
	DownProgressMap sync.Map
	StateChanged    = make(chan struct{}, 1)
)

func TriggerUpdate() {
	select {
	case StateChanged <- struct{}{}:
	default:
	}
}

func UpdateLastBatchStats(size int, durationSeconds, throughput float64) {
	StateMu.Lock()
	GlobalState.LastBatchSize = size
	GlobalState.LastBatchDuration = durationSeconds
	GlobalState.Throughput = throughput
	GlobalState.LastSyncTime = time.Now().Unix()
	
	GlobalState.ThroughputHistory = append(GlobalState.ThroughputHistory, throughput)
	if len(GlobalState.ThroughputHistory) > 60 {
		GlobalState.ThroughputHistory = GlobalState.ThroughputHistory[1:]
	}
	StateMu.Unlock()
	TriggerUpdate()
}

func AddFailures(failures []state.FailureRecord) {
	StateMu.Lock()
	GlobalState.PendingFailures = append(GlobalState.PendingFailures, failures...)
	StateMu.Unlock()
	TriggerUpdate()
}

func RemoveFailure(localPath string) {
	StateMu.Lock()
	newFailures := []state.FailureRecord{}
	for _, f := range GlobalState.PendingFailures {
		if f.LocalPath != localPath {
			newFailures = append(newFailures, f)
		}
	}
	GlobalState.PendingFailures = newFailures
	StateMu.Unlock()
	TriggerUpdate()
}

func AddActivity(msg string) {
	StateMu.Lock()
	GlobalState.RecentActivity = append([]string{msg}, GlobalState.RecentActivity...)
	if len(GlobalState.RecentActivity) > 20 {
		GlobalState.RecentActivity = GlobalState.RecentActivity[:20]
	}
	StateMu.Unlock()
	TriggerUpdate()
}

func AddError(errStr string) {
	StateMu.Lock()
	GlobalState.RecentErrors = append([]string{errStr}, GlobalState.RecentErrors...)
	if len(GlobalState.RecentErrors) > 10 {
		GlobalState.RecentErrors = GlobalState.RecentErrors[:10]
	}
	StateMu.Unlock()
	TriggerUpdate()
}

func UpdateMetrics(upDelta, downDelta int64, activeDelta int) {
	StateMu.Lock()
	GlobalState.TotalUp += upDelta
	GlobalState.TotalDown += downDelta
	GlobalState.ActiveWorkers += activeDelta
	StateMu.Unlock()
	TriggerUpdate()
}

func AddConflict() {
	StateMu.Lock()
	GlobalState.Conflicts++
	StateMu.Unlock()
	TriggerUpdate()
}

func AddActiveTask(id, desc string) {
	if strings.Contains(desc, "Downloading") {
		ActiveDownMap.Store(id, desc)
	} else {
		ActiveUpMap.Store(id, desc)
	}
	TriggerUpdate()
}

func RemoveActiveTask(id string) {
	ActiveUpMap.Delete(id)
	ActiveDownMap.Delete(id)
	UpProgressMap.Delete(id)
	DownProgressMap.Delete(id)
	TriggerUpdate()
}

func UpdateTaskProgress(id string, transferred, total int64) {
	percentage := 0.0
	if total > 0 {
		percentage = float64(transferred) / float64(total) * 100
	}
	progress := TaskProgress{
		Transferred: transferred,
		Total:       total,
		Percentage:  percentage,
	}

	if _, ok := ActiveDownMap.Load(id); ok {
		DownProgressMap.Store(id, progress)
	} else {
		UpProgressMap.Store(id, progress)
	}
}

func EncodeState() []byte {
	StateMu.Lock()
	defer StateMu.Unlock()
	
	GlobalState.UptimeStr = time.Since(StartTime).Round(time.Second).String()
	
	GlobalState.ActiveUpTasks = make(map[string]string)
	ActiveUpMap.Range(func(key, value any) bool {
		GlobalState.ActiveUpTasks[key.(string)] = value.(string)
		return true
	})

	GlobalState.ActiveDownTasks = make(map[string]string)
	ActiveDownMap.Range(func(key, value any) bool {
		GlobalState.ActiveDownTasks[key.(string)] = value.(string)
		return true
	})

	GlobalState.UpProgress = make(map[string]TaskProgress)
	UpProgressMap.Range(func(key, value any) bool {
		GlobalState.UpProgress[key.(string)] = value.(TaskProgress)
		return true
	})

	GlobalState.DownProgress = make(map[string]TaskProgress)
	DownProgressMap.Range(func(key, value any) bool {
		GlobalState.DownProgress[key.(string)] = value.(TaskProgress)
		return true
	})

	data, _ := json.Marshal(GlobalState)
	return data
}
