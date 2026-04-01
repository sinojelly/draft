package upstream

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"syncghost/internal/config"
	"syncghost/internal/engine"
	"syncghost/internal/logger"
	"syncghost/internal/state"

	"github.com/fsnotify/fsnotify"
)

type LocalWatcher struct {
	watcher   *fsnotify.Watcher
	EventChan chan engine.OSFileEvent
	filters   map[string]*engine.SyncFilter // rootPath -> filter
}

func NewLocalWatcher() (*LocalWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &LocalWatcher{
		watcher:   watcher,
		EventChan: make(chan engine.OSFileEvent, 10000), // Buffer for burst events
		filters:   make(map[string]*engine.SyncFilter),
	}, nil
}

func (w *LocalWatcher) Start() {
	go func() {
		for {
			select {
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}
				// 【注意】：此处绝不能有任何阻塞逻辑！
				w.handleRawEvent(event)
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				logger.LogError("WatcherError", err)
			}
		}
	}()

	// Add all sync paths
	for _, task := range config.GlobalConfig.SyncTasks {
		filter := engine.NewSyncFilter(task.LocalPath)
		w.filters[task.LocalPath] = filter
		// 【修复3】：这里必须传入 task.LocalPath 作为 taskRoot 基准
		err := w.addRecursive(task.LocalPath, filter, false, task.LocalPath)
		if err != nil {
			logger.LogError("WatcherAdd", err)
		}
	}
	logger.LogInfo("File system watcher started with .syncghostignore support.")
}

func (w *LocalWatcher) addRecursive(dir string, filter *engine.SyncFilter, emitEvents bool, taskRoot string) error {
	return filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip unavailable paths
		}

		// 【修复3】：计算相对于 taskRoot (即根监控目录) 的正确相对路径
		rel, err := filepath.Rel(taskRoot, p)
		if err != nil {
			return nil
		}

		// 统一前置过滤拦截 (无论是文件还是文件夹，命中规则一律无视)
		if rel != "." && filter.ShouldIgnore(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			w.watcher.Add(p) // 确保多层深度的目录也被成功挂载监听

			// 如果是运行中新增的目录，触发事件与入库
			if emitEvents && p != dir {
				accountID, driveType := "", ""
				for _, task := range config.GlobalConfig.SyncTasks {
					if task.LocalPath == taskRoot {
						accountID = task.AccountID
						for _, acc := range config.GlobalConfig.Accounts {
							if acc.ID == accountID {
								driveType = acc.Type
								break
							}
						}
						break
					}
				}
				if accountID != "" {
					state.SaveFileState(accountID, driveType, p, state.FileState{IsDir: true, SyncTime: time.Now().Unix()})
					logger.LogInfo("Watcher: Auto-discovered subdirectory %s", p)
					w.EventChan <- engine.OSFileEvent{
						LocalPath: p,
						Action:    "create",
						IsDir:     true,
						ModTime:   info.ModTime().Unix(),
					}
				}
			}
		} else if emitEvents {
			// Found a file in a new directory, emit create event
			w.EventChan <- engine.OSFileEvent{
				LocalPath: p,
				Action:    "create",
				IsDir:     false,
				Size:      info.Size(),
				ModTime:   info.ModTime().Unix(),
			}
			logger.LogInfo("Watcher: Auto-discovered file %s", p)
		}
		return nil
	})
}

func (w *LocalWatcher) handleRawEvent(event fsnotify.Event) {
	var taskRoot string
	for root := range w.filters {
		if strings.HasPrefix(event.Name, root) {
			taskRoot = root
			break
		}
	}

	if taskRoot == "" {
		return
	}

	filter := w.filters[taskRoot]
	rel, _ := filepath.Rel(taskRoot, event.Name)

	if filter.ShouldIgnore(rel) {
		return
	}

	filename := filepath.Base(event.Name)
	if strings.HasPrefix(filename, "~") {
		return // Skip temp office files
	}

	action := "modify"
	var isDir bool
	if event.Has(fsnotify.Create) {
		action = "create"
		stat, err := os.Stat(event.Name)
		if err == nil && stat.IsDir() {
			isDir = true

			// 【修复1】：立刻添加监听，尽早挂载后续事件
			w.watcher.Add(event.Name)

			// 快速存入 DB
			for _, task := range config.GlobalConfig.SyncTasks {
				if task.LocalPath == taskRoot {
					driveType := ""
					for _, acc := range config.GlobalConfig.Accounts {
						if acc.ID == task.AccountID {
							driveType = acc.Type
							break
						}
					}
					state.SaveFileState(task.AccountID, driveType, event.Name, state.FileState{IsDir: true, SyncTime: time.Now().Unix()})
					break
				}
			}

			// 【修复1 & 2核心】：异步防抖回填 (Asynchronous Recursive Backfill)
			// 让出主线程！延时 150ms 保证 xcopy 等爆发性写入指令完成落盘，再进行收割
			go func(newDir string, f *engine.SyncFilter, tr string) {
				time.Sleep(150 * time.Millisecond)
				err := w.addRecursive(newDir, f, true, tr)
				if err != nil {
					logger.LogError("Watcher: addRecursive backfill failed", err)
				}
			}(event.Name, filter, taskRoot)
		}
	} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		action = "delete"
	}

	var size int64 = 0
	var modTime int64 = 0
	if action != "delete" {
		stat, err := os.Stat(event.Name)
		if err == nil && !stat.IsDir() {
			size = stat.Size()
			modTime = stat.ModTime().Unix()
		} else if isDir {
			// Directory handled above
		} else {
			return // Ignore directories for file events unless it was just created
		}
	}

	// Echo Loop Defense
	for _, task := range config.GlobalConfig.SyncTasks {
		if task.LocalPath == taskRoot {
			driveType := ""
			for _, acc := range config.GlobalConfig.Accounts {
				if acc.ID == task.AccountID {
					driveType = acc.Type
					break
				}
			}
			savedState, err := state.GetFileState(task.AccountID, driveType, event.Name)
			if err != nil {
				logger.LogDebug("Watcher: DB lookup failed for %s: %v", event.Name, err)
			}
			if savedState != nil {
				logger.LogDebug("Watcher: Found DB state for %s: isDir=%v", event.Name, savedState.IsDir)
				if action == "delete" && savedState.IsDir {
					isDir = true
				}

				if !isDir && savedState.Size == size && savedState.SyncTime == modTime {
					return
				}
			}
			break
		}
	}

	logger.LogInfo("Watcher: Detected %s action on %s (isDir: %v)", action, event.Name, isDir)

	w.EventChan <- engine.OSFileEvent{
		LocalPath: event.Name,
		Action:    action,
		IsDir:     isDir,
		Size:      size,
		ModTime:   modTime,
	}
}

func (w *LocalWatcher) Stop() {
	if w.watcher != nil {
		w.watcher.Close()
	}
	close(w.EventChan)
}
