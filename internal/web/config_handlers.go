package web

import (
	"encoding/json"
	"net/http"
	"os"

	"syncghost/internal/config"
	"syncghost/internal/engine"
	"syncghost/internal/logger"

	"gopkg.in/yaml.v3" // 确保你的项目引入了 yaml 库
)

// HandleConfig 处理配置的获取与更新
func HandleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config.GlobalConfig)
		return
	}

	if r.Method == "POST" {
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, "Invalid JSON data", http.StatusBadRequest)
			return
		}

		// 1. 内存覆写 (将前端传来的新配置更新到内存)
		config.GlobalConfig.Global = newCfg.Global
		config.GlobalConfig.SyncTasks = newCfg.SyncTasks

		// 2. 落盘持久化
		data, err := yaml.Marshal(config.GlobalConfig)
		if err == nil {
			os.WriteFile(config.ConfigPath, data, 0644)
		}

		// 3. 原子级热拔插生效
		// 3.1 动态修改日志级别
		if config.GlobalConfig.Global.LogLevel != "" {
			logger.SetLevel(config.GlobalConfig.Global.LogLevel)
		}

		// 3.2 触发引擎优雅重启，挂载新的同步任务
		engine.TriggerReload()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
		return
	}
}
