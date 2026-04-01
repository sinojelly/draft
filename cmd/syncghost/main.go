package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"path/filepath"
	"syncghost/internal/config"
	"syncghost/internal/drive"
	"syncghost/internal/drive/baidu"
	"syncghost/internal/drive/yike"
	"syncghost/internal/engine"
	"syncghost/internal/engine/downstream"
	"syncghost/internal/engine/upstream"
	"syncghost/internal/logger"
	"syncghost/internal/state"
	"syncghost/internal/web"

	"github.com/joho/godotenv"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("CRITICAL PANIC RECOVERED: %v\n", r)
			time.Sleep(5 * time.Second)
			os.Exit(1)
		}
	}()
	// 0. Parse Flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	doTrashClear := flag.Bool("trash-clear", false, "Clear all .syncghost/trash directories and exit")
	doConfigCheck := flag.Bool("config-check", false, "Validate configuration and permissions then exit")
	doRepair := flag.Bool("repair", false, "Perform deep MD5 consistency repair and exit")
	showVersion := flag.Bool("version", false, "Show version information and exit")

	flag.Parse()

	if *showVersion {
		fmt.Println("SyncGhost v1.2.0-STABLE (Phase 69)")
		return
	}

	_ = godotenv.Load()

	// 1. Initial Config Load for Global Settings
	err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("FATAL ERROR: Failed to load configuration: %v\n", err)
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}

	if *doConfigCheck {
		fmt.Printf("Configuration [%s] is VALID.\n", *configPath)
		return
	}

	if *doTrashClear {
		fmt.Println("Cleaning up .syncghost/trash directories...")
		for _, task := range config.GlobalConfig.SyncTasks {
			trashPath := filepath.Join(task.LocalPath, ".syncghost", "trash")
			if _, err := os.Stat(trashPath); err == nil {
				os.RemoveAll(trashPath)
			}
		}
		return
	}

	// 2. Initialize Permanent Components (Logger, DB, Web Server)
	logger.InitLogger(config.GlobalConfig.Global.LogDir, config.GlobalConfig.Global.LogLevel)
	state.InitDB(".sync_state.db")
	defer state.CloseDB()
	
	web.StartServer(config.GlobalConfig.Global.WebPort)
	logger.LogInfo("SyncGhost Daemon started.")

	// Auto-open browser if configured
	if config.GlobalConfig.Global.AutoOpenBrowser {
		addr := fmt.Sprintf("http://localhost:%d", config.GlobalConfig.Global.WebPort)
		go openBrowser(addr)
	}

	// Listen for System Exit Signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 3. Main Daemon Loop for Hot-Reloading
	for {
		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup

		// Reload configuration before each start (to pick up changes from Web UI)
		if err := config.LoadConfig(*configPath); err != nil {
			logger.LogError("ConfigReload", err)
			// Wait for reload signal or exit
		} else {
			// Start engine components for this generation
			startEngineComponents(ctx, &wg, *doRepair)
		}

		select {
		case <-engine.ReloadChan:
			logger.LogInfo("[Daemon] 收到热加载信号，正在优雅关闭当前同步任务...")
			cancel()
			wg.Wait()
			logger.LogInfo("[Daemon] 旧任务已清理完毕，正在重新拉起引擎...")
			continue // Restart the loop

		case sig := <-sigChan:
			logger.LogInfo("[Daemon] 收到退出信号 (%v)，正在安全停机...", sig)
			cancel()
			wg.Wait()
			logger.LogInfo("SyncGhost 已完全退出.")
			return
		}
	}
}

func startEngineComponents(ctx context.Context, wg *sync.WaitGroup, isRepairMode bool) {
	// 1. Initialize Plugins
	plugins := make(map[string]drive.CloudDrive)
	for _, account := range config.GlobalConfig.Accounts {
		if account.Type == "baidu" {
			// Phase 70: Upgrade to BDUSS/SToken login via PCS Web API
			token, _ := state.GetAuthToken(account.ID, account.Type)
			if token == nil || token.AccessToken == "" {
				logger.LogInfo("[%s] Baidu credentials missing. Please use Web Dashboard.", account.ID)
				continue
			}
			baiduPlugin := baidu.NewBaiduPlugin(token.AccessToken, token.RefreshToken)
			plugins[account.ID] = baiduPlugin
			logger.LogInfo("Initialized Baidu PCS plugin for account %s", account.ID)
		} else if account.Type == "yike" {
			token, _ := state.GetAuthToken(account.ID, account.Type)
			if token == nil || token.AccessToken == "" {
				logger.LogInfo("[%s] Yike credentials missing. Please use Web Dashboard.", account.ID)
				continue
			}
			yikePlugin := yike.NewYikePlugin(token.AccessToken, token.RefreshToken)
			plugins[account.ID] = yikePlugin
			logger.LogInfo("Initialized Yike plugin for account %s", account.ID)
		}
	}

	// 2. Core Engine Components
	watcher, err := upstream.NewLocalWatcher()
	if err != nil {
		logger.LogError("WatcherInit", err)
		return
	}

	queues := make(map[string]*engine.TaskQueue)
	upEngines := make(map[string]*upstream.UpEngine)
	downEngines := make(map[string]*downstream.DownEngine)
	cloudWatchers := make(map[string]*downstream.CloudWatcher)
	cloudEventsChan := make(chan engine.CloudFileEvent, 10000)
	downChans := make(map[string]chan engine.DownTaskEvent)

	for _, account := range config.GlobalConfig.Accounts {
		if plugin, ok := plugins[account.ID]; ok {
			// Up-Sync
			debounce := time.Duration(config.GlobalConfig.Performance.DebounceDurationMs) * time.Millisecond
			if debounce <= 0 { debounce = 2 * time.Second }
			q := engine.NewTaskQueue(config.GlobalConfig.Performance.MaxBatchCount, debounce)
			ue := upstream.NewUpEngine(plugin, q.BatchChan)
			queues[account.ID] = q
			upEngines[account.ID] = ue
			
			q.Start()
			ue.Start()

			// Down-Sync
			downEnabled := false
			for _, task := range config.GlobalConfig.SyncTasks {
				if task.AccountID == account.ID && task.Down.Enable {
					downEnabled = true
					break
				}
			}
			if downEnabled {
				downChan := make(chan engine.DownTaskEvent, 1000)
				downChans[account.ID] = downChan
				de := downstream.NewDownEngine(plugin, downChan)
				downEngines[account.ID] = de
				de.Start()

				var accountTasks []config.SyncTask
				for _, t := range config.GlobalConfig.SyncTasks {
					if t.AccountID == account.ID { accountTasks = append(accountTasks, t) }
				}
				cw := downstream.NewCloudWatcher(account.ID, plugin, accountTasks, downChan)
				cloudWatchers[account.ID] = cw
				cw.Start()

				// Pipe events
				go func(events <-chan engine.CloudFileEvent) {
					for {
						select {
						case <-ctx.Done(): return
						case evt, ok := <-events:
							if !ok { return }
							select {
							case cloudEventsChan <- evt:
							default:
							}
						}
					}
				}(cw.EventChan)
			}
		}
	}

	watcher.Start()
	router := engine.NewEventRouter(watcher.EventChan, cloudEventsChan, queues)
	for accID, ch := range downChans { router.RegisterDownChan(accID, ch) }
	router.Start()

	retryManager := engine.NewRetryManager(queues, downChans)
	retryManager.Start()

	// 3. Bootstrap Scans
	if isRepairMode {
		fmt.Println("Entering Deep Repair Mode...")
		for _, task := range config.GlobalConfig.SyncTasks {
			if plugin, ok := plugins[task.AccountID]; ok {
				driveType := "unknown"
				for _, acc := range config.GlobalConfig.Accounts {
					if acc.ID == task.AccountID { driveType = acc.Type; break }
				}
				engine.PerformRepairScan(task, driveType, plugin, queues[task.AccountID])
			}
		}
		fmt.Println("Deep Repair Complete.")
	} else {
		for _, task := range config.GlobalConfig.SyncTasks {
			if _, ok := plugins[task.AccountID]; !ok { continue }
			
			if task.Up.Enable && task.Up.InitialSync && !state.IsInitialSyncDone(task.AccountID, task.LocalPath) {
				var driveType string
				for _, acc := range config.GlobalConfig.Accounts {
					if acc.ID == task.AccountID { driveType = acc.Type; break }
				}
				upstream.PerformInitialScan(task, driveType, queues[task.AccountID])
			}
			
			if task.Down.Enable && task.Down.InitialSync && !state.IsInitialDownSyncDone(task.AccountID, task.RemotePath) {
				var driveType string
				for _, acc := range config.GlobalConfig.Accounts {
					if acc.ID == task.AccountID { driveType = acc.Type; break }
				}
				downstream.PerformInitialDownScan(task, driveType, plugins[task.AccountID], downChans[task.AccountID])
				state.MarkInitialDownSyncDone(task.AccountID, task.RemotePath)
			}
		}
	}

	// 4. Lifecycle Management (Graceful Shutdown for this generation)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		logger.LogInfo("Shutting down engine components for reload...")
		
		watcher.Stop()
		retryManager.Stop() // Assume Stop exists or add it
		
		for _, ue := range upEngines { ue.Wait() }
		for _, q := range queues { q.Stop(); q.Wait() }
		for _, cw := range cloudWatchers { cw.Stop() }
		for _, de := range downEngines { de.Stop() }
	}()
}

func authorizeBaidu(account config.AccountConfig, baiduPlugin *baidu.BaiduPlugin) string {
	// Not used anymore due to Web Dashboard SSO, but kept for minimal compatibility if needed
	return ""
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		logger.LogInfo("Failed to open browser automatically. Please visit %s", url)
	}
}

