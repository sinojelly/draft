package yike

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"syncghost/internal/drive"

	"golang.org/x/time/rate"
)

var (
	ErrNotImplemented   = errors.New("operation not supported for yike (up-only mode)")
	ErrAntiBotTriggered = errors.New("baidu anti-bot triggered (CAPTCHA required)")
)

type YikePlugin struct {
	mu         sync.RWMutex
	BDUSS      string
	SToken     string
	bdstoken   string // 【新增】：从主页自动嗅探的防御令牌
	limiter    *rate.Limiter
	albumCache map[string]string
	httpClient *http.Client
}

func NewYikePlugin(bduss, stoken string) *YikePlugin {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &YikePlugin{
		BDUSS:      bduss,
		SToken:     stoken,
		limiter:    rate.NewLimiter(rate.Limit(2), 3),
		albumCache: make(map[string]string),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

func (p *YikePlugin) GetCapabilities() drive.DriveCapabilities {
	return drive.DriveCapabilities{
		MaxFileSize:    20 * 1024 * 1024 * 1024,
		MaxConcurrency: 3,
		SupportChunked: true,
		MediaTypeOnly:  true,
		AllowedExts:    []string{".jpg", ".jpeg", ".png", ".gif", ".heic", ".bmp", ".webp", ".mp4", ".mov", ".avi", ".mkv"},
	}
}

// --- Stubs for Down-sync operations ---

func (p *YikePlugin) GetDirID(remotePath string) (string, error) {
	return remotePath, nil
}
func (p *YikePlugin) GetIncrementalChanges(cursor string) ([]drive.CloudChange, string, bool, error) {
	return nil, "", false, ErrNotImplemented
}
func (p *YikePlugin) GetLatestCursor() (string, error) {
	return "", ErrNotImplemented
}
func (p *YikePlugin) Download(remotePath string, localPath string, reporter drive.ProgressReporter) error {
	return ErrNotImplemented
}
func (p *YikePlugin) ListDir(remotePath string) ([]drive.CloudChange, error) {
	return p.ApiListDir(remotePath)
}
