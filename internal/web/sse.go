package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"syncghost/internal/logger"
	"syncghost/internal/status"
	"time"
)

//go:embed web/*
var webFiles embed.FS

// serveStatic mounts the embedded web directory to /
func ServeStatic() {
	publicFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		logger.ErrorLog.Fatalf("Failed to sub embedded web files: %v", err)
	}

	http.Handle("/", http.FileServer(http.FS(publicFS)))
}

// OpenBrowser opens the given URL in the default browser of the system.
func OpenBrowser(url string) {
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

var (
	clients  = make(map[chan []byte]bool)
	clientMu sync.Mutex
)

func dispatch(data []byte) {
	clientMu.Lock()
	defer clientMu.Unlock()
	for c := range clients {
		select {
		case c <- data:
		default:
			// Full, skip
		}
	}
}

// SSEHandler serves Server-Sent Events
func SSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	c := make(chan []byte, 10)
	clientMu.Lock()
	clients[c] = true
	clientMu.Unlock()

	defer func() {
		clientMu.Lock()
		delete(clients, c)
		clientMu.Unlock()
		close(c)
		logger.LogInfo("SSE Client disconnected.")
	}()

	logger.LogInfo("SSE Client connected from %s", r.RemoteAddr)

	// Send initial state
	initData := status.EncodeState()
	fmt.Fprintf(w, "data: %s\n\n", string(initData))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Wait for updates
	notify := r.Context().Done()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-notify:
			return
		case <-heartbeat.C:
			// Heartbeat comment to keep connection alive
			fmt.Fprintf(w, ": heartbeat\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case data := <-c:
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

// StartServer starts the internal web server for telemetry
func StartServer(port int) {
	ServeStatic()
	http.HandleFunc("/api/stream", SSEHandler)
	http.HandleFunc("/api/accounts", HandleListAccounts)
	http.HandleFunc("/api/auth/baidu/qr", handleBaiduQR)
	http.HandleFunc("/api/auth/baidu/poll", handleBaiduPoll)
	http.HandleFunc("/api/config", HandleConfig)

	// Register self as a log sink to capture all system logs
	logger.RegisterSink(func(level, msg string) {
		if level == "ERROR" {
			status.AddError(msg)
		} else {
			status.AddActivity(msg)
		}
	})

	go func() {
		addr := fmt.Sprintf("http://localhost:%d", port)
		logger.LogInfo("Starting Web Dashboard Server at %s", addr)

		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			logger.LogError("WebServer", err)
		}
	}()

	// Event-driven broadcast loop
	go func() {
		uptimeTicker := time.NewTicker(60 * time.Second)
		defer uptimeTicker.Stop()

		for {
			select {
			case <-status.StateChanged:
				dispatch(status.EncodeState())
			case <-uptimeTicker.C:
				dispatch(status.EncodeState())
			}
		}
	}()
}
