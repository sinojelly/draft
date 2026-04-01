package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	
	// Set global state for test
	ConfigPath = configPath
	GlobalConfig = &Config{
		Global: GlobalOptions{
			LogLevel: "DEBUG",
			WebPort:  9999,
		},
		Accounts: []AccountConfig{
			{ID: "test-acc", Type: "baidu"},
		},
	}
	
	err := SaveConfig()
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	
	// Verify file exists and is not empty
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Config file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Config file is empty")
	}
	
	// Verify reloadable
	err = LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig of saved file failed: %v", err)
	}
	
	if GlobalConfig.Global.WebPort != 9999 {
		t.Errorf("Expected WebPort 9999, got %d", GlobalConfig.Global.WebPort)
	}
}

func TestSaveConfigNoPath(t *testing.T) {
	ConfigPath = ""
	err := SaveConfig()
	if err == nil {
		t.Error("SaveConfig should fail when ConfigPath is empty")
	}
}
