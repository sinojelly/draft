package engine

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"syncghost/internal/logger"
)

// SyncFilter handles file exclusion based on .syncghostignore patterns
type SyncFilter struct {
	patterns []string
}

// NewSyncFilter loads patterns from a file or remains empty
func NewSyncFilter(rootPath string) *SyncFilter {
	ignorePath := filepath.Join(rootPath, ".syncghostignore")
	filter := &SyncFilter{patterns: []string{}}

	f, err := os.Open(ignorePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.LogError("Filter:Load", err)
		}
		return filter
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		filter.patterns = append(filter.patterns, line)
	}

	logger.LogInfo("Loaded %d ignore patterns from %s", len(filter.patterns), ignorePath)
	return filter
}

// ShouldIgnore returns true if the relative path matches any ignore pattern
func (sf *SyncFilter) ShouldIgnore(relPath string) bool {
	if relPath == "" || relPath == "." {
		return false
	}

	// Always ignore the internal .syncghost directory and atomic download temporary files
	if strings.Contains(relPath, ".syncghost") || strings.HasSuffix(relPath, ".sgdownload") {
		return true
	}

	for _, p := range sf.patterns {
		// 1. Literal match or suffix match (e.g., "node_modules")
		if relPath == p || strings.HasPrefix(relPath, p+string(os.PathSeparator)) || strings.Contains(relPath, string(os.PathSeparator)+p) {
			return true
		}

		// 2. Glob match on filename (e.g., "*.log")
		base := filepath.Base(relPath)
		match, _ := filepath.Match(p, base)
		if match {
			return true
		}

		// 3. Glob match on full relative path (e.g., "dist/*.js")
		matchRel, _ := filepath.Match(p, relPath)
		if matchRel {
			return true
		}
	}
	return false
}
