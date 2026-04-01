package engine

import (
	"path/filepath"
	"testing"
)

func TestFilterSgDownload(t *testing.T) {
	filter := &SyncFilter{patterns: []string{"node_modules", "*.log"}}

	tests := []struct {
		relPath string
		ignored bool
	}{
		{filepath.FromSlash("data.txt"), false},
		{filepath.FromSlash("node_modules/foo.js"), true},
		{filepath.FromSlash("error.log"), true},
		{filepath.FromSlash(".syncghost/db"), true},                // Hardcoded ignore
		{filepath.FromSlash("movie.mp4.sgdownload"), true},         // Phase 76: Hardcoded ignore
		{filepath.FromSlash("path/to/backup.zip.sgdownload"), true},
		{filepath.FromSlash("normal_file.sgdownload.txt"), false},
	}

	for _, tt := range tests {
		got := filter.ShouldIgnore(tt.relPath)
		if got != tt.ignored {
			t.Errorf("ShouldIgnore(%s) = %v; want %v", tt.relPath, got, tt.ignored)
		}
	}
}
