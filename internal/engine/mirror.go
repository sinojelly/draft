package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"syncghost/internal/drive"
	"syncghost/internal/logger"
	"syncghost/internal/status"
)

// MirrorTask defines a cloud-to-cloud synchronization job
type MirrorTask struct {
	Name       string
	SourceID   string
	TargetID   string
	RemotePath string // Root path on both sides (assuming same relative structure)
	TempRoot   string // Transit directory
}

// MirrorEngine orchestrates the data flow between two cloud accounts
type MirrorEngine struct {
	Task       MirrorTask
	Source     drive.CloudDrive
	Target     drive.CloudDrive
	transitMu  sync.Mutex
}

func NewMirrorEngine(task MirrorTask, source, target drive.CloudDrive) *MirrorEngine {
	if task.TempRoot == "" {
		task.TempRoot = filepath.Join(os.TempDir(), "syncghost_mirror", task.SourceID+"_to_"+task.TargetID)
	}
	return &MirrorEngine{
		Task:   task,
		Source: source,
		Target: target,
	}
}

// RunOnce performs a full mirror scan and sync
func (me *MirrorEngine) RunOnce() error {
	logger.LogInfo("[%s->%s] MIRROR: Starting full sync for %s", me.Task.SourceID, me.Task.TargetID, me.Task.RemotePath)
	status.AddActivity(fmt.Sprintf("Mirroring %s to %s...", me.Task.SourceID, me.Task.TargetID))
	
	err := os.MkdirAll(me.Task.TempRoot, 0755)
	if err != nil {
		return err
	}
	defer os.RemoveAll(me.Task.TempRoot)

	// 1. List Source
	items, err := me.Source.ListDir(me.Task.RemotePath)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.IsDir {
			// Recursive mirroring - for POC we just do one level or implement walk
			continue
		}

		// 2. Check Target
		targetSize, targetMD5, _, err := me.Target.GetFileInfo(item.Path)
		if err == nil && targetSize == item.Size && targetMD5 == item.MD5 {
			// Already in sync
			continue
		}

		// 3. Transit Sync
		logger.LogInfo("[%s->%s] MIRROR: Syncing %s", me.Task.SourceID, me.Task.TargetID, item.Path)
		localTransitPath := filepath.Join(me.Task.TempRoot, filepath.Base(item.Path))
		
		// Download from Source
		err = me.Source.Download(item.Path, localTransitPath, nil)
		if err != nil {
			logger.LogError("Mirror:Download", err)
			continue
		}

		// Upload to Target
		remoteDir := filepath.Dir(item.Path)
		_, err = me.Target.Upload(localTransitPath, remoteDir, "overwrite", nil)
		if err != nil {
			logger.LogError("Mirror:Upload", err)
		}

		// Cleanup
		os.Remove(localTransitPath)
	}

	status.AddActivity(fmt.Sprintf("Mirror Complete: %s", me.Task.Name))
	return nil
}
