package engine

import (
	"sync"
	"syncghost/internal/logger"
)

var (
	// ReloadChan 传递热加载信号，缓冲设为 1 防止阻塞
	ReloadChan = make(chan struct{}, 1)
	reloadMu   sync.Mutex
)

// TriggerReload 由 Web 控制台或授权回调调用，通知主程序重启引擎
func TriggerReload() {
	reloadMu.Lock()
	defer reloadMu.Unlock()

	select {
	case ReloadChan <- struct{}{}:
		logger.LogInfo("[Engine Manager] 触发了热加载信号，准备重启引擎...")
	default:
		// 通道已有信号，无需重复触发
	}
}
