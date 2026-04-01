package drive

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

var (
	ErrCursorInvalid = errors.New("incremental cursor is invalid or expired")
)

// DriveCapabilities defines the limits and features supported by a specific drive plugin
type DriveCapabilities struct {
	MaxFileSize    int64
	MaxConcurrency int
	SupportChunked bool
	MediaTypeOnly  bool     // If true, only allowed extensions are synced
	AllowedExts    []string // List of allowed extensions (e.g., .jpg, .mp4)
}

// CloudChange represents a generic change record from any cloud provider
type CloudChange struct {
	Path      string
	Action    string // create, delete (modify is treated as create/delete or update)
	FsID      string
	Size      int64
	MD5       string
	IsDir     bool
	ModTime   int64
}

// ProgressReporter is a callback for tracking data transfer progress
type ProgressReporter func(transferred int64, total int64)

// CloudDrive is the common interface that all cloud drive plugins must implement
type CloudDrive interface {
	GetCapabilities() DriveCapabilities
	
	// Upload Operations
	Upload(localPath string, remoteDir string, onConflict string, reporter ProgressReporter) (remoteFileID string, err error)
	
	// Directory Operations
	GetDirID(remotePath string) (dirID string, err error)

	// Metadata Operations
	GetFileInfo(remotePath string) (size int64, md5 string, fsID string, err error)
	CheckExistence(remotePath string) (bool, error)

	// Lifecycle Operations
	Delete(remotePath string) error
	DeleteDir(remotePath string) error

	// Sensing Operations (For Down-sync)
	GetIncrementalChanges(cursor string) (changes []CloudChange, nextCursor string, hasMore bool, err error)
	GetLatestCursor() (string, error)

	// Download Operations
	Download(remotePath string, localPath string, reporter ProgressReporter) error

	// Directory Traversal
	ListDir(remotePath string) ([]CloudChange, error)
}

// CalculateMD5 computes a simple hex MD5 of a local file
func CalculateMD5(localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := md5.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
