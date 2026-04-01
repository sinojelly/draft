package baidu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"syncghost/internal/drive"

	"golang.org/x/time/rate"
)

type BaiduPlugin struct {
	mu      sync.RWMutex
	bduss   string
	stoken  string
	limiter *rate.Limiter
}

func (b *BaiduPlugin) waitBeforeRequest() {
	if b.limiter != nil {
		b.limiter.Wait(context.Background())
	}
}

func NewBaiduPlugin(bduss, stoken string) *BaiduPlugin {
	return &BaiduPlugin{
		bduss:   bduss,
		stoken:  stoken,
		limiter: rate.NewLimiter(rate.Limit(5), 10),
	}
}

// 核心封装：自动注入高权限 Cookie 和 PC 客户端 UA
func (b *BaiduPlugin) doReq(ctx context.Context, method, apiURL string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, apiURL, body)
	if err != nil {
		return nil, err
	}

	cookie := fmt.Sprintf("BDUSS=%s", b.bduss)
	if b.stoken != "" {
		cookie += fmt.Sprintf("; SToken=%s", b.stoken)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "netdisk;11.12.3;PC;PC-Windows;10.0.19042;WindowsDevice")

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

func (b *BaiduPlugin) DeleteDir(remotePath string) error {
	return b.Delete(remotePath)
}

func (b *BaiduPlugin) Delete(remotePath string) error {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 【修改】：补齐 app_id
	apiLine := "https://pcs.baidu.com/rest/2.0/pcs/file?method=delete&app_id=250528"

	form := url.Values{}
	form.Add("path", remotePath)

	resp, err := b.doReq(ctx, "POST", apiLine, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	json.Unmarshal(body, &res)

	if errno, ok := res["errno"].(float64); ok && errno != 0 {
		return fmt.Errorf("baidu pcs delete errno: %v (data: %s)", errno, string(body))
	}
	return nil
}

func (b *BaiduPlugin) precreateNormal(remotePath string, hashes *FileHashes, onConflict string) (string, []string, error) {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 【修改】：补齐 app_id
	apiLine := "https://pcs.baidu.com/rest/2.0/pcs/file?method=precreate&app_id=250528"

	blockListJSON, _ := json.Marshal(hashes.BlockList)
	form := url.Values{}
	form.Add("path", remotePath)
	form.Add("size", fmt.Sprintf("%d", hashes.Size))
	form.Add("isdir", "0")

	baiduRType := "3"
	if onConflict == "rename" {
		baiduRType = "2"
	}
	form.Add("rtype", baiduRType)
	form.Add("block_list", string(blockListJSON))

	resp, err := b.doReq(ctx, "POST", apiLine, strings.NewReader(form.Encode()))
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Errno    int    `json:"errno"`
		UploadID string `json:"uploadid"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", nil, fmt.Errorf("precreate parse error: %v", err)
	}

	if res.Errno != 0 {
		return "", nil, fmt.Errorf("precreate failed with errno: %d", res.Errno)
	}
	return res.UploadID, hashes.BlockList, nil
}

func (b *BaiduPlugin) GetFileInfo(remotePath string) (int64, string, string, error) {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	apiURL := fmt.Sprintf("https://pcs.baidu.com/rest/2.0/pcs/file?method=meta&app_id=250528&path=%s", url.QueryEscape(remotePath))

	resp, err := b.doReq(ctx, "GET", apiURL, nil)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Errno int `json:"errno"`
		List  []struct {
			FsID  int64  `json:"fs_id"`
			Size  int64  `json:"size"`
			MD5   string `json:"md5"`
			IsDir int    `json:"isdir"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return 0, "", "", fmt.Errorf("pcs meta parse error: %v", err)
	}

	if res.Errno != 0 || len(res.List) == 0 {
		return 0, "", "", fmt.Errorf("file not found on cloud (errno: %d)", res.Errno)
	}

	item := res.List[0]
	if item.IsDir == 1 {
		return 0, "", fmt.Sprintf("%d", item.FsID), fmt.Errorf("path is a directory")
	}

	return item.Size, item.MD5, fmt.Sprintf("%d", item.FsID), nil
}

func (b *BaiduPlugin) CheckExistence(remotePath string) (bool, error) {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	apiURL := fmt.Sprintf("https://pcs.baidu.com/rest/2.0/pcs/file?method=meta&app_id=250528&path=%s", url.QueryEscape(remotePath))

	resp, err := b.doReq(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Errno int           `json:"errno"`
		List  []interface{} `json:"list"`
	}
	json.Unmarshal(body, &res)
	return res.Errno == 0 && len(res.List) > 0, nil
}

type DiffEntry struct {
	Path           string `json:"path"`
	FsID           int64  `json:"fs_id"`
	ServerFilename string `json:"server_filename"`
	Size           int64  `json:"size"`
	MD5            string `json:"md5"`
	IsDir          int    `json:"isdir"`
	IsDelete       int    `json:"isdelete"`
	ServerMtime    int64  `json:"server_mtime"`
}

type DiffResult struct {
	Errno   int         `json:"errno"`
	Cursor  string      `json:"cursor"`
	HasMore bool        `json:"has_more"`
	Entries []DiffEntry `json:"entries"`
}

func (b *BaiduPlugin) GetDiff(cursor string) (*DiffResult, error) {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	apiLine := fmt.Sprintf("https://pan.baidu.com/rest/2.0/xpan/file?method=diff&cursor=%s", url.QueryEscape(cursor))

	resp, err := b.doReq(ctx, "GET", apiLine, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res DiffResult
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("diff parse error")
	}

	if res.Errno != 0 {
		if res.Errno == 31023 || res.Errno == 31296 {
			return nil, drive.ErrCursorInvalid
		}
		return nil, fmt.Errorf("baidu diff api errno: %d", res.Errno)
	}

	return &res, nil
}

func (b *BaiduPlugin) GetLatestCursor() (string, error) {
	b.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	apiLine := "https://pan.baidu.com/rest/2.0/xpan/file?method=diff&start=0&limit=100"

	resp, err := b.doReq(ctx, "GET", apiLine, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	json.Unmarshal(body, &res)

	errno := 0
	if e, ok := res["errno"].(float64); ok {
		errno = int(e)
	}

	if errno != 0 && errno != 31023 && errno != 31296 {
		return "", fmt.Errorf("baidu diff api errno: %d", errno)
	}

	cursorRaw := res["cursor"]
	switch v := cursorRaw.(type) {
	case string:
		return v, nil
	case float64:
		return fmt.Sprintf("%.0f", v), nil
	default:
		return "", nil
	}
}

func (b *BaiduPlugin) GetIncrementalChanges(cursor string) ([]drive.CloudChange, string, bool, error) {
	res, err := b.GetDiff(cursor)
	if err != nil {
		return nil, "", false, err
	}

	changes := make([]drive.CloudChange, 0, len(res.Entries))
	for _, entry := range res.Entries {
		action := "create"
		if entry.IsDelete == 1 {
			action = "delete"
		}

		changes = append(changes, drive.CloudChange{
			Path:    entry.Path,
			Action:  action,
			FsID:    fmt.Sprintf("%v", entry.FsID),
			Size:    entry.Size,
			MD5:     entry.MD5,
			IsDir:   entry.IsDir == 1,
			ModTime: entry.ServerMtime,
		})
	}

	return changes, res.Cursor, res.HasMore, nil
}

// 【彻底重构下载链路】：无需取 dlink，直接带着 Cookie 去下
func (b *BaiduPlugin) Download(remotePath string, localPath string, reporter drive.ProgressReporter) error {
	size, cloudMD5, _, err := b.GetFileInfo(remotePath)
	if err != nil {
		return fmt.Errorf("pre-download metadata fetch failed: %v", err)
	}

	tmpPath := localPath + ".syncghost.downloading"
	os.MkdirAll(filepath.Dir(localPath), 0755)

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	// 原生 PCS 下载 API，带着高权限 Cookie 直连
	downloadURL := "https://d.pcs.baidu.com/rest/2.0/pcs/file?method=download&app_id=250528&path=" + url.QueryEscape(remotePath)
	req, _ := http.NewRequest("GET", downloadURL, nil)

	cookie := fmt.Sprintf("BDUSS=%s", b.bduss)
	if b.stoken != "" {
		cookie += fmt.Sprintf("; SToken=%s", b.stoken)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "netdisk;11.12.3;PC;PC-Windows;10.0.19042;WindowsDevice")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	if reporter != nil {
		pw := &progressWriter{total: size, reporter: reporter}
		io.Copy(io.MultiWriter(out, pw), resp.Body)
	} else {
		io.Copy(out, resp.Body)
	}
	out.Close()

	localMD5, err := CalculateMD5(tmpPath)
	if err == nil && localMD5 != cloudMD5 {
		isObfuscated := false
		for _, c := range cloudMD5 {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				isObfuscated = true
				break
			}
		}
		if !isObfuscated {
			return fmt.Errorf("integrity check failed: local %s != cloud %s", localMD5, cloudMD5)
		}
	}

	return os.Rename(tmpPath, localPath)
}

func (b *BaiduPlugin) ListDir(remotePath string) ([]drive.CloudChange, error) {
	var allChanges []drive.CloudChange
	start := 0
	limit := 1000

	for {
		b.waitBeforeRequest()
		apiURL := fmt.Sprintf("https://pcs.baidu.com/rest/2.0/pcs/file?method=list&app_id=250528&dir=%s&start=%d&limit=%d",
			url.QueryEscape(remotePath), start, limit)

		resp, err := b.doReq(context.Background(), "GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var res struct {
			Errno int `json:"errno"`
			List  []struct {
				FsID        int64  `json:"fs_id"`
				Path        string `json:"path"`
				Size        int64  `json:"size"`
				MD5         string `json:"md5"`
				IsDir       int    `json:"isdir"`
				ServerMtime int64  `json:"server_mtime"`
			} `json:"list"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, err
		}

		if res.Errno != 0 {
			return nil, fmt.Errorf("baidu pcs list error: %d", res.Errno)
		}

		for _, item := range res.List {
			allChanges = append(allChanges, drive.CloudChange{
				Path:    item.Path,
				IsDir:   item.IsDir == 1,
				Size:    item.Size,
				MD5:     item.MD5,
				FsID:    fmt.Sprintf("%d", item.FsID),
				ModTime: item.ServerMtime,
				Action:  "create",
			})
		}

		if len(res.List) < limit {
			break
		}
		start += len(res.List)
	}

	return allChanges, nil
}

type progressWriter struct {
	total       int64
	transferred int64
	reporter    drive.ProgressReporter
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.transferred += int64(n)
	if pw.reporter != nil {
		pw.reporter(pw.transferred, pw.total)
	}
	return n, nil
}
