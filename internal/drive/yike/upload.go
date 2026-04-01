package yike

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"syncghost/internal/drive"
	"syncghost/internal/drive/baidu"
	"syncghost/internal/logger"
)

func (p *YikePlugin) Upload(localPath string, remoteDir string, onConflict string, reporter drive.ProgressReporter) (string, error) {
	hashes, err := baidu.CalculateHashes(localPath)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// 1. Determine physical and logical paths
	// Physical: /youa/web/test6/image.jpg (actual location on Baidu)
	// Logical: /test6/image.jpg (used for album naming)
	physicalRemotePath := path.Join("/youa/web", remoteDir, filepath.Base(localPath))
	logicalRemotePath := path.Join("/", remoteDir, filepath.Base(localPath))

	// 2. Upload lifecycle
	res, err := p.ApiPrecreate(ctx, localPath, physicalRemotePath, hashes)
	if err != nil {
		return "", fmt.Errorf("precreate failed: %w", err)
	}

	fsID := ""
	returnType := extractYikeString(res["return_type"])
	if returnType == "2" || returnType == "3" {
		fsID = extractYikeString(extractIDFromMap(res, "fs_id"))
	}

	if fsID == "" {
		uploadID := extractYikeString(res["uploadid"])
		if uploadID == "" {
			return "", fmt.Errorf("no uploadid returned from precreate")
		}

		err = p.ApiUploadSlices(ctx, localPath, physicalRemotePath, uploadID, hashes.BlockList, reporter)
		if err != nil {
			return "", fmt.Errorf("upload slices failed: %w", err)
		}

		commitRes, err := p.ApiCommitFile(ctx, localPath, physicalRemotePath, uploadID, hashes, hashes.BlockList)
		if err != nil {
			return "", fmt.Errorf("commit file failed: %w", err)
		}
		fsID = extractYikeString(extractIDFromMap(commitRes, "fs_id"))
	} else {
		logger.LogInfo("Yike: Rapid upload success for %s", localPath)
	}

	// 3. Album binding
	albumID, tid, err := p.GetAlbumIDByPath(localPath, logicalRemotePath)
	if err != nil {
		logger.LogError(fmt.Sprintf("Yike: Failed to resolve album for %s", logicalRemotePath), err)
		return fsID, nil
	}

	if albumID != "" {
		bindRes, err := p.ApiAddFileToAlbum(ctx, albumID, tid, fsID)
		if err != nil {
			logger.LogError(fmt.Sprintf("Yike: Failed to bind file %s to album %s", fsID, albumID), err)
		} else {
			errno := extractYikeString(bindRes["errno"])
			if errno == "0" {
				logger.LogInfo("Yike: Successfully bound file to album %s", albumID)
			} else {
				logger.LogDebug("Yike: Bind operation returned errno %s", errno)
			}
		}
	}

	return fsID, nil
}

// Api stubs moved to api.go

func (p *YikePlugin) calculateAlbumName(remotePath string) string {
	dir := path.Dir(remotePath)
	dir = strings.TrimPrefix(dir, "/")

	parts := strings.Split(dir, "/")
	var filtered []string
	for _, part := range parts {
		lowPart := strings.ToLower(part)
		if part == "" || part == "." || lowPart == "apps" || lowPart == "syncghost" || lowPart == "youa" || lowPart == "web" || lowPart == "syncghostautotest" || lowPart == "yikesmoke" || lowPart == "yike" {
			continue
		}
		filtered = append(filtered, part)
	}

	if len(filtered) == 0 {
		return "SyncGhost_Default"
	}

	return strings.Join(filtered, "_")
}

func (p *YikePlugin) GetAlbumIDByPath(localPath string, remotePath string) (string, string, error) {
	albumName := p.calculateAlbumName(remotePath)
	if albumName == "" {
		return "", "", nil
	}

	// 1. Check cache
	p.mu.RLock()
	if cached, ok := p.albumCache[albumName]; ok {
		p.mu.RUnlock()
		parts := strings.Split(cached, "|")
		if len(parts) >= 2 && parts[0] != "" && parts[0] != "0" {
			return parts[0], parts[1], nil
		}
	} else {
		p.mu.RUnlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 2. Fetch real IDs from server
	albums, err := p.ApiListAlbums()
	if err == nil {
		for _, alb := range albums {
			if title, ok := alb["title"].(string); ok && title == albumName {
				albumID := extractYikeString(alb["album_id"])
				tid := extractYikeString(alb["tid"])

				if albumID != "" && albumID != "0" && tid != "" {
					p.mu.Lock()
					p.albumCache[albumName] = albumID + "|" + tid
					p.mu.Unlock()
					return albumID, tid, nil
				}
			}
		}
	}

	// 3. Create if missing
	createRes, err := p.ApiCreateAlbum(ctx, albumName)
	if err != nil {
		// 【核心修复】：如果返回 50100 (通常是重名/已存在)，再次尝试刷新列表获取 ID
		errno := extractYikeString(createRes["errno"])
		if errno == "50100" {
			logger.LogDebug("Yike: Album '%s' creation returned 50100 (likely exists). Retrying list...", albumName)
			time.Sleep(3 * time.Second)
			albums, _ := p.ApiListAlbums()
			for _, alb := range albums {
				if title, ok := alb["title"].(string); ok && title == albumName {
					albumID := extractYikeString(alb["album_id"])
					tid := extractYikeString(alb["tid"])
					if albumID != "" && tid != "" {
						p.mu.Lock()
						p.albumCache[albumName] = albumID + "|" + tid
						p.mu.Unlock()
						return albumID, tid, nil
					}
				}
			}
		}
		return "", "", fmt.Errorf("create album failed: %w (res: %v)", err, createRes)
	}

	albumID := extractYikeString(extractIDFromMap(createRes, "album_id"))
	if albumID == "" || albumID == "0" {
		// FALLBACK: Maybe it's nested differently or the key name is slightly different
		if val, ok := createRes["album_id"]; ok {
			albumID = extractYikeString(val)
		}
	}

	if albumID != "" && albumID != "0" {
		// Consistency wait
		time.Sleep(2 * time.Second)

		// Re-fetch to get real tid (critical for binding)
		albums, _ := p.ApiListAlbums()
		for _, alb := range albums {
			if title, ok := alb["title"].(string); ok && title == albumName {
				tid := extractYikeString(alb["tid"])
				if tid != "" {
					p.mu.Lock()
					p.albumCache[albumName] = albumID + "|" + tid
					p.mu.Unlock()
					return albumID, tid, nil
				}
			}
		}

		// If still no TID, we return what we have (though bind might fail)
		p.mu.Lock()
		p.albumCache[albumName] = albumID + "|"
		p.mu.Unlock()
		return albumID, "", nil
	}

	return "", "", fmt.Errorf("failed to create/resolve album_id for %s (res: %v)", albumName, createRes)
}

func (p *YikePlugin) DeleteDir(remotePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Album Cleanup
	albumName := p.calculateAlbumName(path.Join(remotePath, "dummy.txt"))
	if albumName != "" && albumName != "SyncGhost_Default" {
		albums, err := p.ApiListAlbums()
		if err == nil {
			for _, alb := range albums {
				if title, ok := alb["title"].(string); ok && title == albumName {
					albumID := extractYikeString(alb["album_id"])
					tid := extractYikeString(alb["tid"])

					if albumID != "" {
						// 【新增终极必杀：删相册前，先把里面所有的残留照片全部解绑清空！】
						fsIDs, err := p.ApiListAlbumFiles(ctx, albumID, tid)
						if err == nil && len(fsIDs) > 0 {
							logger.LogInfo("Yike: Found %d residual photos in album %s, unbinding them all...", len(fsIDs), albumName)
							p.ApiRemoveFilesFromAlbum(ctx, albumID, tid, fsIDs)
						}

						// 此时相册已被彻底抽干，调用删除必将返回 0 (成功)
						res, err := p.ApiDeleteAlbum(ctx, albumID, tid)
						errno := extractYikeString(res["errno"])
						if err == nil && errno == "0" {
							logger.LogInfo("Yike: Successfully deleted album %s for directory %s", albumName, remotePath)
							p.mu.Lock()
							delete(p.albumCache, albumName)
							p.mu.Unlock()
						} else if errno == "50801" {
							logger.LogInfo("Yike: Album %s is STILL not completely empty (errno 50801), skip deleting.", albumName)
						} else {
							logger.LogInfo("Yike: Could not strictly delete album %s. Res: %v", albumName, res)
						}
					}
					break
				}
			}
		}
	}

	// 2. Physical PCS directory delete
	return p.ApiDeleteFile(ctx, remotePath)
}

func (p *YikePlugin) Delete(remotePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// 【新增核心逻辑：删除单文件前，先从相册逻辑解绑】
	albumName := p.calculateAlbumName(remotePath)
	if albumName != "" && albumName != "SyncGhost_Default" {
		// 1. 抓取物理 fs_id (必须在物理删除前抓取)
		fsID, err := p.ApiGetFsID(ctx, remotePath)
		if err == nil && fsID != "" {
			// 2. 获取相册真钥匙 TID
			albumID, tid, _ := p.GetAlbumIDByPath("", remotePath)
			if albumID != "" && tid != "" {
				// 3. 彻底解绑幽灵引用！
				err = p.ApiRemoveFilesFromAlbum(ctx, albumID, tid, []string{fsID})
				if err == nil {
					logger.LogInfo("Yike: Successfully unbound file %s from album %s", fsID, albumName)
				}
			}
		}
	}

	// 解绑完成后，安心进行物理网盘空间删除
	return p.ApiDeleteFile(ctx, remotePath)
}
