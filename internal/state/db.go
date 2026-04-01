package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

var (
	db *bbolt.DB
)

// FileState represents the synced state of a local file
type FileState struct {
	FileID   string `json:"FileID"`
	MD5      string `json:"MD5"`
	IsDir    bool   `json:"IsDir"`
	Size     int64  `json:"Size"`
	SyncTime int64  `json:"SyncTime"`
}

// FailureRecord stores details about a failed sync for later retry
type FailureRecord struct {
	AccountID  string `json:"AccountID"`
	DriveType  string `json:"DriveType"`
	LocalPath  string `json:"LocalPath"`
	RemotePath string `json:"RemotePath"`
	ErrorMsg   string `json:"ErrorMsg"`
	Direction  string `json:"Direction"` // "up" or "down"
	FileID     string `json:"FileID"`    // New: for down-sync resume
	MD5        string `json:"MD5"`       // New: for integrity check
	Size       int64  `json:"Size"`      // New: for progress tracking
	Timestamp  int64  `json:"Timestamp"`
}

type AuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Expiry       int64  `json:"expiry"`
}

const (
	StateBucket        = "File_State"
	AccountBucket      = "Accounts"
	TaskMetaBucket     = "Task_Meta"
	TaskErrorsBucket   = "Task_Errors"
	CloudCursorsBucket = "Cloud_Cursors"
)

// InitDB initializes the bbolt database in the working directory
func InitDB(dbPath string) error {
	var err error
	db, err = bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}

	return db.Update(func(tx *bbolt.Tx) error {
		_, _ = tx.CreateBucketIfNotExists([]byte(StateBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(AccountBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(TaskMetaBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(TaskErrorsBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(CloudCursorsBucket))
		return nil
	})
}

// ... existing functions ...

// SaveCloudCursor persists the diff cursor for a specific account
func SaveCloudCursor(accountID, cursor string) error {
	return db.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(CloudCursorsBucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(accountID), []byte(cursor))
	})
}

// GetCloudCursor retrieves the latest saved cursor for an account
func GetCloudCursor(accountID string) string {
	var cursor string
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(CloudCursorsBucket))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(accountID))
		if v != nil {
			cursor = string(v)
		}
		return nil
	})
	return cursor
}

// CloseDB closes the instance
func CloseDB() {
	if db != nil {
		db.Close()
	}
}

// SaveFileState persists file info for a specific driver and account
func SaveFileState(accountID, driveType, localPath string, state FileState) error {
	bucketName := fmt.Sprintf("SyncState_%s_%s", accountID, driveType)

	return db.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}

		encoded, err := json.Marshal(state)
		if err != nil {
			return err
		}

		return b.Put([]byte(localPath), encoded)
	})
}

// GetFileState retrieves file info, returns nil, nil if not found
func GetFileState(accountID, driveType, localPath string) (*FileState, error) {
	bucketName := fmt.Sprintf("SyncState_%s_%s", accountID, driveType)
	var fs FileState

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil // Bucket not created yet, so file doesn't exist
		}
		data := b.Get([]byte(localPath))
		if data == nil {
			return nil // Key not found
		}

		return json.Unmarshal(data, &fs)
	})

	if err != nil {
		return nil, err
	}

	// If empty struct returned but bucket check passed
	// Directories might only have IsDir=true
	if fs.FileID == "" && fs.MD5 == "" && !fs.IsDir {
		return nil, nil
	}

	return &fs, nil
}

// DeleteFileState removes the key when a local file is deleted
func DeleteFileState(accountID, driveType, localPath string) error {
	bucketName := fmt.Sprintf("SyncState_%s_%s", accountID, driveType)

	return db.Batch(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(localPath))
	})
}

// GetFileStatesForAccount returns all tracked files and their states for a specific account and drive type
func GetFileStatesForAccount(accountID, driveType string) (map[string]FileState, error) {
	bucketName := fmt.Sprintf("SyncState_%s_%s", accountID, driveType)
	states := make(map[string]FileState)

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var s FileState
			if err := json.Unmarshal(v, &s); err == nil {
				states[string(k)] = s
			}
			return nil
		})
	})
	return states, err
}

func SaveAuthToken(accountID, driveType string, token AuthToken) error {
	bucketName := fmt.Sprintf("Auth_Tokens_%s", driveType)
	return db.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(token)
		if err != nil {
			return err
		}
		return b.Put([]byte(accountID), encoded)
	})
}

func GetAuthToken(accountID, driveType string) (*AuthToken, error) {
	bucketName := fmt.Sprintf("Auth_Tokens_%s", driveType)
	var token AuthToken
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		data := b.Get([]byte(accountID))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &token)
	})
	if err != nil || token.AccessToken == "" {
		return nil, err
	}
	return &token, nil
}
// MarkInitialSyncDone records that the first-run full scan has been completed for a task
func MarkInitialSyncDone(accountID, localPath string) error {
	bucketName := TaskMetaBucket
	key := fmt.Sprintf("%s|%s", accountID, localPath)
	return db.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), []byte("true"))
	})
}

// IsInitialSyncDone checks if the first-run full scan for this task has already been completed
func IsInitialSyncDone(accountID string, localPath string) bool {
	bucketName := TaskMetaBucket
	key := fmt.Sprintf("%s|%s", accountID, localPath)
	done := false
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v != nil && string(v) == "true" {
			done = true
		}
		return nil
	})
	return done
}


// MarkInitialDownSyncDone records that the cloud-to-local initial scan has been completed
func MarkInitialDownSyncDone(accountID, remotePath string) error {
	bucketName := TaskMetaBucket
	key := fmt.Sprintf("down|%s|%s", accountID, remotePath)
	return db.Batch(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), []byte("true"))
	})
}

// IsInitialDownSyncDone checks if the cloud-to-local initial scan has been completed
func IsInitialDownSyncDone(accountID string, remotePath string) bool {
	bucketName := TaskMetaBucket
	key := fmt.Sprintf("down|%s|%s", accountID, remotePath)
	done := false
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v != nil && string(v) == "true" {
			done = true
		}
		return nil
	})
	return done
}

// RecordFailure marks a file as failed for persistent retry
func RecordFailure(accountID, driveType, localPath, remotePath, errorMsg, direction string, fileID string, md5 string, size int64) {
	_ = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(TaskErrorsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", TaskErrorsBucket)
		}
		
		record := FailureRecord{
			AccountID:  accountID,
			DriveType:  driveType,
			LocalPath:  localPath,
			RemotePath: remotePath,
			ErrorMsg:   errorMsg,
			Direction:  direction,
			FileID:     fileID,
			MD5:        md5,
			Size:       size,
			Timestamp:  time.Now().Unix(),
		}
		encoded, _ := json.Marshal(record)
		
		key := accountID + ":" + localPath
		return b.Put([]byte(key), encoded)
	})
}


// ClearFailure removes a file from the failure tracking list
func ClearFailure(accountID string, localPath string) {
	_ = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(TaskErrorsBucket))
		if b == nil {
			return nil
		}
		key := accountID + ":" + localPath
		return b.Delete([]byte(key))
	})
}

// GetPendingFailures returns all failed paths for a specific account (OLD - for migration or simpler cases)
func GetPendingFailures(accountID string) []string {
	var failures []string
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(TaskErrorsBucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(accountID + ":")
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			failures = append(failures, string(k[len(prefix):]))
		}
		return nil
	})
	return failures
}

// GetPendingFailureDetails returns full records of failed syncs for a specific account
func GetPendingFailureDetails(accountID string) []FailureRecord {
	var failures []FailureRecord
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(TaskErrorsBucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(accountID + ":")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var rec FailureRecord
			if err := json.Unmarshal(v, &rec); err == nil {
				failures = append(failures, rec)
			}
		}
		return nil
	})
	return failures
}

// GetAllFailureDetails returns all failed sync records across all accounts
func GetAllFailureDetails() []FailureRecord {
	var failures []FailureRecord
	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(TaskErrorsBucket))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var rec FailureRecord
			if err := json.Unmarshal(v, &rec); err == nil {
				failures = append(failures, rec)
			}
			return nil
		})
	})
	return failures
}
