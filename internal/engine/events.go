package engine

type OSFileEvent struct {
	LocalPath string
	Action    string // create, modify, delete
	IsDir     bool
	Size      int64
	ModTime   int64
}

type CloudFileEvent struct {
	RemotePath string
	Action     string // create, modify, delete
	FsID       string
	Size       int64
	MD5        string
	IsDir      bool
	ModTime    int64
	AccountID  string
}

type UpTaskEvent struct {
	OSFileEvent
	AccountID    string
	DriveType    string
	LocalRoot    string // Root of the sync task (for .syncghost/trash)
	RemotePath   string
	IsDir         bool
	OnConflict    string // overwrite, rename, skip
	SyncDeletion  bool
	Force         bool // If true, bypass local state DB checks
}

type DownTaskEvent struct {
	CloudFileEvent
	LocalPath    string
	LocalRoot    string // Root of the sync task (for .syncghost/trash)
	DriveType    string
	OnConflict   string // overwrite, rename
	SyncDeletion bool
}
