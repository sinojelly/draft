package baidu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"syncghost/internal/drive"
	"syncghost/internal/logger"
)

func (b *BaiduPlugin) GetCapabilities() drive.DriveCapabilities {
	return drive.DriveCapabilities{
		MaxFileSize:    20 * 1024 * 1024 * 1024,
		MaxConcurrency: 5,
		SupportChunked: true,
	}
}

func (b *BaiduPlugin) GetDirID(remotePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 【修改】：使用统一请求，移除 access_token 拼接
	apiLine := fmt.Sprintf("https://pan.baidu.com/rest/2.0/xpan/file?method=meta&path=%s", url.QueryEscape(remotePath))

	resp, err := b.doReq(ctx, "GET", apiLine, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	json.Unmarshal(body, &res)

	if errCode, ok := res["errno"].(float64); ok && errCode != 0 {
		return "", fmt.Errorf("baidu api errno: %v (data: %s)", errCode, string(body))
	}

	list, ok := res["list"].([]interface{})
	if !ok || len(list) == 0 {
		return "", fmt.Errorf("baidu api reported success but file entry is missing in the list (data: %s)", string(body))
	}

	return remotePath, nil
}

func (b *BaiduPlugin) Upload(localPath string, remoteDir string, onConflict string, reporter drive.ProgressReporter) (string, error) {
	return b.uploadWithDepth(localPath, remoteDir, 0, onConflict, reporter)
}

func (b *BaiduPlugin) uploadWithDepth(localPath string, remoteDir string, depth int, onConflict string, reporter drive.ProgressReporter) (string, error) {
	return b.uploadWithDepthInhibit(localPath, remoteDir, depth, false, onConflict, reporter)
}

func (b *BaiduPlugin) uploadWithDepthInhibit(localPath string, remoteDir string, depth int, inhibitRapid bool, onConflict string, reporter drive.ProgressReporter) (string, error) {
	if depth > 3 {
		return "", fmt.Errorf("recursion limit exceeded for %s: persistent cloud conflict", localPath)
	}

	filename := filepath.Base(localPath)
	remotePath := filepath.ToSlash(filepath.Join(remoteDir, filename))

	remoteSize, remoteMD5, remoteFsID, vErr := b.GetFileInfo(remotePath)

	if vErr == nil && onConflict == "skip" {
		logger.LogInfo("Skip Policy Hit: %s already exists on cloud. Skipping redundant hashing and upload.", localPath)
		return remoteFsID, nil
	}

	if vErr != nil && strings.Contains(vErr.Error(), "path is a directory") {
		logger.LogInfo("CRITICAL CONFLICT: %s is occupied by a directory on cloud. Safety guard triggered: Aborting sync.", localPath)
		return "", fmt.Errorf("cloud path is a directory (Safety Block)")
	}

	hashes, err := CalculateHashes(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hashes for %s: %v", localPath, err)
	}

	if vErr == nil {
		localMD5 := strings.ToLower(hashes.MD5)
		cloudMD5 := strings.ToLower(remoteMD5)

		isObfuscated := false
		for _, c := range cloudMD5 {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				isObfuscated = true
				break
			}
		}

		if remoteSize == hashes.Size {
			if cloudMD5 == localMD5 {
				logger.LogInfo("Metadata Match: %s already exists on cloud with identical Hash and Size. Skipping upload.", localPath)
				return remoteFsID, nil
			}
			if isObfuscated {
				logger.LogInfo("Smart Skip (Obfuscated Cloud MD5): %s size matches (%d) and cloud hash is obfuscated (%s). Assuming identical content.",
					localPath, remoteSize, cloudMD5)
				return remoteFsID, nil
			}
		}

		if onConflict == "skip" {
			return "", fmt.Errorf("skipping upload due to 'skip' conflict policy")
		}
	}

	var uploadID string
	var blockList []string
	var rtype int
	var rapidFsID string

	if inhibitRapid {
		logger.LogInfo("Forcing normal upload for %s (Rapid Upload inhibited)", localPath)
		uploadID, blockList, err = b.precreateNormal(remotePath, hashes, onConflict)
		rtype = 0
	} else {
		uploadID, blockList, rtype, rapidFsID, err = b.precreate(remotePath, hashes, onConflict)
	}

	if err != nil {
		return "", fmt.Errorf("precreate stage failed for %s: %v", localPath, err)
	}

	if rtype == 2 {
		logger.LogInfo("Rapid upload hit and auto-created (rtype=2) for %s", localPath)
		if rapidFsID != "" {
			return rapidFsID, nil
		}
		return "RAPID_MATCH", nil
	}

	if uploadID == "" {
		return "", fmt.Errorf("precreate did not return an uploadid for %s", localPath)
	}

	err = b.uploadSlices(localPath, remotePath, uploadID, blockList, reporter)
	if err != nil {
		return "", fmt.Errorf("upload slices failed for %s (id: %s): %v", localPath, uploadID, err)
	}

	createdFsID, err := b.create(remotePath, uploadID, hashes, blockList, onConflict)
	if err != nil {
		return "", fmt.Errorf("final create merge failed for %s: %v", localPath, err)
	}

	return createdFsID, nil
}

func (b *BaiduPlugin) precreate(remotePath string, hashes *FileHashes, onConflict string) (uploadID string, blockList []string, rtype int, rapidFsID string, err error) {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 【修改】：使用统一请求
	apiLine := "https://pan.baidu.com/rest/2.0/xpan/file?method=precreate"

	blocks := hashes.BlockList
	blockListJSON, _ := json.Marshal(blocks)

	form := url.Values{}
	form.Add("path", remotePath)
	form.Add("size", fmt.Sprintf("%d", hashes.Size))
	form.Add("isdir", "0")
	form.Add("autoinit", "1")

	baiduRType := "3"
	if onConflict == "rename" {
		baiduRType = "2"
	}
	form.Add("rtype", baiduRType)

	form.Add("block_list", string(blockListJSON))
	form.Add("content-md5", hashes.MD5)
	form.Add("slice-md5", hashes.SliceMD5)

	resp, err := b.doReq(ctx, "POST", apiLine, strings.NewReader(form.Encode()))
	if err != nil {
		return "", nil, 0, "", err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &res)

	if errCode, ok := res["errno"].(float64); ok && errCode != 0 {
		return "", nil, 0, "", fmt.Errorf("baidu precreate api errno %v", errCode)
	}

	if rtInfo, ok := res["return_type"].(float64); ok {
		rtype = int(rtInfo)
	}

	if infoMap, ok := res["info"].(map[string]interface{}); ok {
		if fsIdFloat, ok := infoMap["fs_id"].(float64); ok {
			rapidFsID = fmt.Sprintf("%v", int64(fsIdFloat))
		}
	}

	var upID string
	if id, ok := res["uploadid"].(string); ok {
		upID = id
	}

	return upID, hashes.BlockList, rtype, rapidFsID, nil
}

func (b *BaiduPlugin) uploadSlices(localPath string, remotePath string, uploadID string, blockList []string, reporter drive.ProgressReporter) error {
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	totalSize := fileInfo.Size()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	for i := range blockList {
		b.waitBeforeRequest()
		// 【修改】：补齐 app_id=250528，移除 access_token
		apiLine := fmt.Sprintf("https://d.pcs.baidu.com/rest/2.0/pcs/superfile2?method=upload&app_id=250528&type=tmpfile&path=%s&uploadid=%s&partseq=%d",
			url.QueryEscape(remotePath), uploadID, i)

		bodyBuf := new(bytes.Buffer)
		w := multipart.NewWriter(bodyBuf)
		fw, _ := w.CreateFormFile("file", filepath.Base(localPath))
		f, err := os.Open(localPath)
		if err != nil {
			return err
		}

		offset := int64(i) * int64(BlockSize)
		chunkSize := int64(BlockSize)
		if offset+chunkSize > totalSize {
			chunkSize = totalSize - offset
		}

		sectionReader := io.NewSectionReader(f, offset, chunkSize)
		io.Copy(fw, sectionReader)
		f.Close()
		w.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", apiLine, bodyBuf)
		if err != nil {
			return err
		}

		// 【核心】：手工注入 Cookie，因为这里用的自定义长连接 client
		cookie := fmt.Sprintf("BDUSS=%s", b.bduss)
		if b.stoken != "" {
			cookie += fmt.Sprintf("; SToken=%s", b.stoken)
		}
		req.Header.Set("Cookie", cookie)
		req.Header.Set("User-Agent", "netdisk;11.12.3;PC;PC-Windows;10.0.19042;WindowsDevice")

		req.ContentLength = int64(bodyBuf.Len())
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Del("Transfer-Encoding")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		var res map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &res)
		resp.Body.Close()

		if errCode, ok := res["error_code"].(float64); ok && errCode != 0 {
			return fmt.Errorf("baidu upload error: %v", res["error_msg"])
		}

		if reporter != nil {
			transferred := int64(i+1) * int64(BlockSize)
			if transferred > totalSize {
				transferred = totalSize
			}
			reporter(transferred, totalSize)
		}
	}
	return nil
}

func (b *BaiduPlugin) create(remotePath string, uploadID string, hashes *FileHashes, blockList []string, onConflict string) (string, error) {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 【修改】：使用统一请求，移除 access_token 拼接
	apiLine := "https://pan.baidu.com/rest/2.0/xpan/file?method=create"
	blockListJSON, _ := json.Marshal(blockList)

	form := url.Values{}
	form.Add("path", remotePath)
	form.Add("size", fmt.Sprintf("%d", hashes.Size))
	form.Add("isdir", "0")

	baiduRType := "3"
	if onConflict == "rename" {
		baiduRType = "2"
	}
	form.Add("rtype", baiduRType)
	form.Add("uploadid", uploadID)
	form.Add("block_list", string(blockListJSON))

	resp, err := b.doReq(ctx, "POST", apiLine, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &res)

	if errCode, ok := res["errno"].(float64); ok && errCode != 0 {
		return "", fmt.Errorf("baidu create api errno %v (data: %s)", errCode, string(body))
	}
	if fsIdFloat, ok := res["fs_id"].(float64); ok {
		return fmt.Sprintf("%v", int64(fsIdFloat)), nil
	}
	return "", nil
}
