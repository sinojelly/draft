package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type GlobalOptions struct {
	LogLevel        string `yaml:"log_level"`
	LogDir          string `yaml:"log_dir"`
	WebPort         int    `yaml:"web_port"`
	AutoOpenBrowser bool   `yaml:"auto_open_browser"`
	LogRotation     struct {
		MaxSizeMb  int  `yaml:"max_size_mb"`
		MaxBackups int  `yaml:"max_backups"`
		MaxAgeDays int  `yaml:"max_age_days"`
		Compress   bool `yaml:"compress"`
	} `yaml:"log_rotation"`
}

type AccountConfig struct {
	ID                string `yaml:"id"`
	Type              string `yaml:"type"`
	AuthMode          string `yaml:"auth_mode"`
	AppKeyOverride    string `yaml:"app_key_override"`
	SecretKeyOverride string `yaml:"secret_key_override"`
}

type UpConfig struct {
	Enable       bool     `yaml:"enable"`
	Ignore       []string `yaml:"ignore"`
	InitialSync  bool     `yaml:"initial_sync"`
	OnConflict   string   `yaml:"on_conflict"` // overwrite, rename, skip
	SyncDeletion bool     `yaml:"sync_deletion"`
}

type DownConfig struct {
	Enable          bool     `yaml:"enable"`
	Ignore          []string `yaml:"ignore"`
	InitialSync     bool     `yaml:"initial_sync"`
	OnConflict      string   `yaml:"on_conflict"` // overwrite, rename
	SyncDeletion    bool     `yaml:"sync_deletion"`
	PollIntervalSec int      `yaml:"poll_interval_sec"`
}

type SyncTask struct {
	AccountID  string `yaml:"account_id"`
	LocalPath  string `yaml:"local_path"`
	RemotePath string `yaml:"remote_path"`

	// Structured configs
	Up   UpConfig   `yaml:"up"`
	Down DownConfig `yaml:"down"`
}

type PerformanceConfig struct {
	MaxConcurrency     int `yaml:"max_concurrency"`
	MaxBatchCount      int `yaml:"max_batch_count"`
	DebounceDurationMs int `yaml:"debounce_duration_ms"`
}

type Config struct {
	Global      GlobalOptions     `yaml:"global"`
	Accounts    []AccountConfig   `yaml:"accounts"`
	SyncTasks   []SyncTask        `yaml:"sync_tasks"`
	Performance PerformanceConfig `yaml:"performance"`
}

var (
	GlobalConfig *Config
	ConfigPath   string
)

func (c *Config) Validate() error {
	if len(c.Accounts) == 0 {
		return fmt.Errorf("at least one account must be configured")
	}

	accountMap := make(map[string]bool)
	for _, acc := range c.Accounts {
		if accountMap[acc.ID] {
			return fmt.Errorf("duplicate account id found: %s", acc.ID)
		}
		accountMap[acc.ID] = true
	}

	for _, task := range c.SyncTasks {
		if _, err := os.Stat(task.LocalPath); os.IsNotExist(err) {
			return fmt.Errorf("local_path %s does not exist", task.LocalPath)
		}

		// Validate Up section if enabled
		if task.Up.Enable {
			oc := task.Up.OnConflict
			if oc != "" && oc != "overwrite" && oc != "rename" && oc != "skip" {
				return fmt.Errorf("invalid up.on_conflict value '%s' for task %s", oc, task.LocalPath)
			}
		}

		// Validate Down section if enabled
		if task.Down.Enable {
			oc := task.Down.OnConflict
			if oc != "" && oc != "overwrite" && oc != "rename" {
				return fmt.Errorf("invalid down.on_conflict value '%s' for task %s", oc, task.LocalPath)
			}
		}
	}
	return nil
}

func LoadConfig(path string) error {
	ConfigPath = path
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	cfg := &Config{}
	// Set default to true before unmarshal
	cfg.Global.AutoOpenBrowser = true

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return err
	}

	// 1. Set Defaults
	for i := range cfg.SyncTasks {
		t := &cfg.SyncTasks[i]
		if t.Up.Enable {
			if t.Up.OnConflict == "" {
				t.Up.OnConflict = "rename"
			}
		}
		if t.Down.Enable {
			if t.Down.OnConflict == "" {
				t.Down.OnConflict = "rename"
			}
			if t.Down.PollIntervalSec <= 0 {
				t.Down.PollIntervalSec = 60 // Default 1 minute
			}
		}
	}

	if cfg.Global.LogLevel == "" {
		cfg.Global.LogLevel = "INFO"
	}
	if cfg.Global.LogDir == "" {
		cfg.Global.LogDir = "./logs"
	}
	if cfg.Global.WebPort <= 0 {
		cfg.Global.WebPort = 8080
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	GlobalConfig = cfg
	return nil
}

// SaveConfig persists the current GlobalConfig to disk
func SaveConfig() error {
	if ConfigPath == "" {
		return fmt.Errorf("config path not set")
	}

	data, err := yaml.Marshal(GlobalConfig)
	if err != nil {
		return err
	}

	// Atomic-ish write: write to .tmp then rename
	tmpPath := ConfigPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, ConfigPath)
}

// IsPathInTask checks if a given path (local for 'up', remote for 'down') is still governed by an active SyncTask
func (c *Config) IsPathInTask(accountID, direction, path string) bool {
	for _, task := range c.SyncTasks {
		if task.AccountID != accountID {
			continue
		}

		if direction == "up" {
			if !task.Up.Enable {
				continue
			}
			// Use filepath.Rel to check if 'path' is inside 'task.LocalPath'
			rel, err := filepath.Rel(strings.ToLower(filepath.Clean(task.LocalPath)), strings.ToLower(filepath.Clean(path)))
			if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
				return true
			}
		} else { // direction == "down"
			if !task.Down.Enable {
				continue
			}
			// Remote paths are forward-slash based
			remoteBase := strings.ToLower(task.RemotePath)
			target := strings.ToLower(path)
			if strings.HasPrefix(target, remoteBase) {
				return true
			}
		}
	}
	return false
}
