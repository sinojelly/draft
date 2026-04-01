package yike

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
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"syncghost/internal/drive"
	"syncghost/internal/drive/baidu"
	"syncghost/internal/logger"
)

func (p *YikePlugin) waitBeforeRequest() {
	if p.limiter != nil {
		p.limiter.Wait(context.Background())
	}
}

// 【黑客科技】：智能 Cookie 解析器
func (p *YikePlugin) getCookieStr() string {
	if strings.Contains(p.BDUSS, "BAIDUID=") || strings.Contains(p.BDUSS, ";") {
		return p.BDUSS
	}
	baiduid := "3F4469FB74FD1E9856574FEE301CA99E:FG=1"
	return fmt.Sprintf("BAIDUID=%s; BDUSS=%s; BDUSS_BFESS=%s; STOKEN=%s;", baiduid, p.BDUSS, p.BDUSS, p.SToken)
}

// 【致敬 pybaiduphoto】：使用完整 Cookie 访问主页并正则提取 bdstoken
func (p *YikePlugin) getBdsToken(ctx context.Context) string {
	p.mu.RLock()
	token := p.albumCache["__bdstoken"]
	p.mu.RUnlock()

	if token != "" {
		return token
	}

	cookieStr := p.getCookieStr()

	reqHome, _ := http.NewRequestWithContext(ctx, "GET", "https://photo.baidu.com/photo/web/home", nil)
	reqHome.Header.Set("Cookie", cookieStr)
	reqHome.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	if respHome, err := p.httpClient.Do(reqHome); err == nil {
		bodyBytes, _ := io.ReadAll(respHome.Body)
		respHome.Body.Close()

		bodyStr := string(bodyBytes)
		if strings.Contains(bodyStr, "pass_login") || strings.Contains(bodyStr, "passport.baidu.com") {
			logger.LogDebug("[Yike API] ❌ Cookie 已失效或不完整，被重定向到了百度登录页！")
		} else if strings.Contains(bodyStr, "anti-bot") || strings.Contains(bodyStr, "验证码") || strings.Contains(bodyStr, "verify") {
			logger.LogDebug("[Yike API] ❌ 遭遇百度风控拦截，必须提供包含了 __yjs_duid 等参数的完整浏览器 Cookie！")
		}

		re := regexp.MustCompile(`(?i)(?:'|")?bdstoken(?:'|")?\s*[:=]\s*(?:'|")([a-f0-9]{32})(?:'|")`)
		matches := re.FindStringSubmatch(bodyStr)
		if len(matches) > 1 {
			token = matches[1]
			logger.LogDebug("[Yike API] 🎯 成功从相册主页正则提取到 bdstoken: %s", token)
		} else {
			idx := strings.Index(bodyStr, `"bdstoken":"`)
			if idx != -1 {
				start := idx + 12
				end := strings.Index(bodyStr[start:], `"`)
				if end != -1 {
					token = bodyStr[start : start+end]
					logger.LogDebug("[Yike API] 🎯 成功从相册主页硬匹配提取到 bdstoken: %s", token)
				}
			}
		}
	}

	if token != "" {
		p.mu.Lock()
		p.albumCache["__bdstoken"] = token
		p.mu.Unlock()
	} else {
		logger.LogDebug("[Yike API] ⚠️ 无法获取 bdstoken。")
	}

	return token
}

func (p *YikePlugin) newRequest(ctx context.Context, method, apiURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, apiURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Cookie", p.getCookieStr())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if strings.Contains(apiURL, "pan.baidu.com") || strings.Contains(apiURL, "pcs.baidu.com") {
		req.Header.Set("Referer", "https://pan.baidu.com/disk/home")
	} else {
		req.Header.Set("Origin", "https://photo.baidu.com")
		req.Header.Set("Referer", "https://photo.baidu.com/photo/web/home")
	}

	return req, nil
}

func (p *YikePlugin) doRequest(req *http.Request) (*http.Response, error) {
	logger.LogDebug("[Yike API] %s %s", req.Method, req.URL.String())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	logger.LogDebug("[Yike API] 响应 Status Code: %d", resp.StatusCode)

	// 先把 Body 全读出来，以备后续检查和解析
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	// 把读取出来的字节重新放回 Body
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// logger.LogDebug("[Yike API] 响应 Body: %s", string(bodyBytes))

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		bodyStr := strings.TrimSpace(string(bodyBytes))

		// 【终极排雷】：百度的分片上传接口就算成功，也会返回 text/html。
		// 如果内容不是以 JSON 括号开头，我们才认定它是真正的 WAF 拦截页。
		if !strings.HasPrefix(bodyStr, "{") && !strings.HasPrefix(bodyStr, "[") {
			snippet := bodyStr
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			logger.LogDebug("[Yike API] ❌ WAF 拦截 (真实的 HTML): %s", snippet)
			return nil, ErrAntiBotTriggered
		}
	}

	return resp, nil
}

func (p *YikePlugin) ApiListAlbums() ([]map[string]interface{}, error) {
	p.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	token := p.getBdsToken(ctx)
	apiURL := "https://photo.baidu.com/youai/album/v1/list?limit=100&clienttype=70"
	if token != "" {
		apiURL += "&bdstoken=" + token
	}

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Errno int                      `json:"errno"`
		List  []map[string]interface{} `json:"list"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}

	if res.Errno == -6 {
		return nil, fmt.Errorf("ERR_AUTH_INVALID: Session expired or BDUSS invalid (errno: -6)")
	}
	if res.Errno != 0 {
		return nil, fmt.Errorf("list albums error: %d", res.Errno)
	}

	return res.List, nil
}

// --- Atomic API Layer (Atom Operations) ---

func (p *YikePlugin) ApiPrecreate(ctx context.Context, localPath string, remotePath string, hashes *baidu.FileHashes) (map[string]interface{}, error) {
	token := p.getBdsToken(ctx)
	if token == "" {
		return nil, fmt.Errorf("missing bdstoken")
	}

	apiURL := "https://photo.baidu.com/youai/file/v1/precreate?clienttype=70&bdstoken=" + token

	stat, _ := os.Stat(localPath)
	fileTime := fmt.Sprintf("%d", stat.ModTime().Unix())

	blockListJSON, _ := json.Marshal(hashes.BlockList)
	form := url.Values{}
	form.Add("path", remotePath)
	form.Add("size", fmt.Sprintf("%d", hashes.Size))
	form.Add("isdir", "0")
	form.Add("block_list", string(blockListJSON))
	form.Add("content-md5", hashes.MD5)
	form.Add("slice-md5", hashes.SliceMD5)
	form.Add("autoinit", "1")
	form.Add("rtype", "1")
	form.Add("ctype", "11")
	form.Add("local_ctime", fileTime)
	form.Add("local_mtime", fileTime)

	req, err := p.newRequest(ctx, "POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}

func (p *YikePlugin) ApiUploadSlices(ctx context.Context, localPath string, remotePath, uploadID string, blockList []string, reporter drive.ProgressReporter) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, _ := file.Stat()
	totalSize := stat.Size()

	for i := range blockList {
		p.waitBeforeRequest()

		apiURL := fmt.Sprintf("https://c.pcs.baidu.com/rest/2.0/pcs/superfile2?method=upload&app_id=16051585&clienttype=70&type=tmpfile&path=%s&uploadid=%s&partseq=%d",
			url.QueryEscape(remotePath), uploadID, i)

		offset := int64(i) * int64(baidu.BlockSize)
		chunkSize := int64(baidu.BlockSize)
		if offset+chunkSize > totalSize {
			chunkSize = totalSize - offset
		}

		section := io.NewSectionReader(file, offset, chunkSize)

		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		fw, err := mw.CreateFormFile("file", filepath.Base(localPath))
		if err != nil {
			return err
		}
		if _, err := io.Copy(fw, section); err != nil {
			return err
		}
		mw.Close()

		req, err := p.newRequest(ctx, "POST", apiURL, &b)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.ContentLength = int64(b.Len())

		resp, err := p.doRequest(req)
		if err != nil {
			return err
		}
		resp.Body.Close()

		if reporter != nil {
			reporter(offset+chunkSize, totalSize)
		}
	}
	return nil
}

func (p *YikePlugin) ApiCommitFile(ctx context.Context, localPath string, remotePath, uploadID string, hashes *baidu.FileHashes, blockList []string) (map[string]interface{}, error) {
	token := p.getBdsToken(ctx)
	apiURL := "https://photo.baidu.com/youai/file/v1/create?clienttype=70&bdstoken=" + token
	blockListJSON, _ := json.Marshal(blockList)

	stat, _ := os.Stat(localPath)
	fileTime := fmt.Sprintf("%d", stat.ModTime().Unix())

	form := url.Values{}
	form.Add("path", remotePath)
	form.Add("size", fmt.Sprintf("%d", hashes.Size))
	form.Add("isdir", "0")
	form.Add("uploadid", uploadID)
	form.Add("block_list", string(blockListJSON))
	form.Add("rtype", "1")
	form.Add("ctype", "11")
	form.Add("content-md5", hashes.MD5)
	form.Add("slice-md5", hashes.SliceMD5)
	form.Add("local_ctime", fileTime)
	form.Add("local_mtime", fileTime)

	req, err := p.newRequest(ctx, "POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}

func (p *YikePlugin) ApiCreateAlbum(ctx context.Context, name string) (map[string]interface{}, error) {
	token := p.getBdsToken(ctx)
	q := url.Values{}
	q.Add("clienttype", "70")
	if token != "" {
		q.Add("bdstoken", token)
	}
	q.Add("title", name)
	q.Add("source", "0")

	// 18-digit unique TID
	timestampMs := time.Now().UnixNano() / 1e6
	random3 := time.Now().UnixNano() % 1000
	q.Add("tid", fmt.Sprintf("%d%03d", timestampMs, random3))

	apiURL := "https://photo.baidu.com/youai/album/v1/create?" + q.Encode()

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}

func (p *YikePlugin) ApiAddFileToAlbum(ctx context.Context, albumID, tid, fsID string) (map[string]interface{}, error) {
	token := p.getBdsToken(ctx)
	q := url.Values{}
	q.Add("clienttype", "70")
	if token != "" {
		q.Add("bdstoken", token)
	}
	q.Add("album_id", albumID)
	if tid != "" {
		q.Add("tid", tid)
	}

	// Double encoded as seen in official logs to prevent bracket mangling
	apiURL := "https://photo.baidu.com/youai/album/v1/addfile?" + q.Encode()
	apiURL += fmt.Sprintf("&list=[%%7B%%22fsid%%22:%s%%7D]", fsID)

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://photo.baidu.com/photo/web/album/"+albumID)

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}

	return res, nil
}

func (p *YikePlugin) ApiDeleteFile(ctx context.Context, remotePath string) error {
	p.waitBeforeRequest()
	physPath := remotePath
	if !strings.HasPrefix(physPath, "/youa/web") {
		physPath = path.Join("/youa/web", remotePath)
	}

	token := p.getBdsToken(ctx)
	// 【核心修复1】：必须硬编码带上 app_id=16051585！这样才能切入一刻相册所在的平行宇宙！
	apiURL := "https://pan.baidu.com/rest/2.0/xpan/file?method=filemanager&opera=delete&app_id=16051585&bdstoken=" + token

	form := url.Values{}
	form.Add("async", "0") // 【核心修复2】：强制改为同步模式(0)，确保立刻落盘并返回真实结果
	form.Add("filelist", fmt.Sprintf(`["%s"]`, physPath))

	req, err := p.newRequest(ctx, "POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return err
	}

	// 【新增日志】：彻底告别盲猜，把网盘物理删除的底层真实结果打出来
	logger.LogDebug("[Yike API] xpan delete response for %s: %v", physPath, res)

	if errno, ok := res["errno"].(json.Number); ok {
		e, _ := errno.Int64()
		// errno 为 12 表示文件本来就不存在，直接放行
		if e != 0 && e != 12 {
			return fmt.Errorf("yike xpan delete error: %d (path: %s)", e, physPath)
		}
	} else if errnoFloat, ok := res["errno"].(float64); ok {
		if int64(errnoFloat) != 0 && int64(errnoFloat) != 12 {
			return fmt.Errorf("yike xpan delete error: %v (path: %s)", errnoFloat, physPath)
		}
	}

	return nil
}

// ApiDeleteAlbum removes a virtual album by its album_id
func (p *YikePlugin) ApiDeleteAlbum(ctx context.Context, albumID string, tid string) (map[string]interface{}, error) {
	p.waitBeforeRequest()
	token := p.getBdsToken(ctx)
	q := url.Values{}
	q.Add("clienttype", "70")
	if token != "" {
		q.Add("bdstoken", token)
	}
	q.Add("album_id", albumID)

	// 【核心修复】：如果有真实的 tid，必须用真实的！否则才退级使用伪造时间戳
	if tid != "" {
		q.Add("tid", tid)
	} else {
		timestampMs := time.Now().UnixNano() / 1e6
		random3 := time.Now().UnixNano() % 1000
		q.Add("tid", fmt.Sprintf("%d%03d", timestampMs, random3))
	}

	apiURL := "https://photo.baidu.com/youai/album/v1/delete?" + q.Encode()

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}

	logger.LogDebug("[Yike API] delete album response for %s: %v", albumID, res)

	return res, nil
}

func (p *YikePlugin) GetFileInfo(remotePath string) (int64, string, string, error) {
	p.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	physPath := remotePath
	if !strings.HasPrefix(physPath, "/youa/web") {
		physPath = path.Join("/youa/web", remotePath)
	}

	// PCS Web API: meta
	apiURL := fmt.Sprintf("https://pcs.baidu.com/rest/2.0/pcs/file?method=meta&path=%s", url.QueryEscape(physPath))

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return 0, "", "", err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	var res struct {
		Errno int `json:"errno"`
		List  []struct {
			FsID  json.Number `json:"fs_id"`
			Size  int64       `json:"size"`
			MD5   string      `json:"md5"`
			IsDir int         `json:"isdir"`
		} `json:"list"`
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&res); err != nil {
		return 0, "", "", err
	}

	if res.Errno != 0 || len(res.List) == 0 {
		return 0, "", "", fmt.Errorf("file not found on yike pcs (errno: %d, path: %s)", res.Errno, physPath)
	}

	item := res.List[0]
	return item.Size, item.MD5, item.FsID.String(), nil
}

// ApiGetFsID 根据网盘路径查询真实的 fs_id
func (p *YikePlugin) ApiGetFsID(ctx context.Context, remotePath string) (string, error) {
	p.waitBeforeRequest()
	token := p.getBdsToken(ctx)
	physPath := remotePath
	if !strings.HasPrefix(physPath, "/youa/web") {
		physPath = path.Join("/youa/web", remotePath)
	}

	dir := path.Dir(physPath)
	name := path.Base(physPath)

	q := url.Values{}
	q.Add("method", "list")
	q.Add("app_id", "16051585")
	q.Add("dir", dir)
	if token != "" {
		q.Add("bdstoken", token)
	}

	apiURL := "https://pan.baidu.com/rest/2.0/xpan/file?" + q.Encode()

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Errno int `json:"errno"`
		List  []struct {
			ServerFilename string `json:"server_filename"`
			FsID           int64  `json:"fs_id"`
		} `json:"list"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	for _, item := range res.List {
		if item.ServerFilename == name {
			return fmt.Sprintf("%d", item.FsID), nil
		}
	}
	return "", fmt.Errorf("file not found in directory listing")
}

// ApiListAlbumFiles 查询相册中所有的照片 fs_id
func (p *YikePlugin) ApiListAlbumFiles(ctx context.Context, albumID, tid string) ([]string, error) {
	p.waitBeforeRequest()
	token := p.getBdsToken(ctx)
	q := url.Values{}
	q.Add("clienttype", "70")
	if token != "" {
		q.Add("bdstoken", token)
	}
	q.Add("album_id", albumID)
	if tid != "" {
		q.Add("tid", tid)
	}
	q.Add("limit", "1000") // 取最大值

	apiURL := "https://photo.baidu.com/youai/album/v1/listfile?" + q.Encode()

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Errno int `json:"errno"`
		List  []struct {
			FsID interface{} `json:"fsid"`
		} `json:"list"`
	}
	json.NewDecoder(resp.Body).Decode(&res)

	var fsIDs []string
	for _, item := range res.List {
		idStr := extractYikeString(item.FsID)
		if idStr != "" && idStr != "0" {
			fsIDs = append(fsIDs, idStr)
		}
	}
	return fsIDs, nil
}

// ApiRemoveFilesFromAlbum 批量从相册解绑照片
func (p *YikePlugin) ApiRemoveFilesFromAlbum(ctx context.Context, albumID, tid string, fsIDs []string) error {
	if len(fsIDs) == 0 {
		return nil
	}
	p.waitBeforeRequest()
	token := p.getBdsToken(ctx)
	q := url.Values{}
	q.Add("clienttype", "70")
	if token != "" {
		q.Add("bdstoken", token)
	}
	q.Add("album_id", albumID)
	if tid != "" {
		q.Add("tid", tid)
	}

	// 拼装批量解绑的 JSON list
	var listItems []string
	for _, id := range fsIDs {
		listItems = append(listItems, fmt.Sprintf(`{"fsid":%s}`, id))
	}

	apiURL := "https://photo.baidu.com/youai/album/v1/deletefile?" + q.Encode()
	// 【核心】：手工拼接 list，防止方括号和花括号被错误转码
	apiURL += "&list=[" + url.QueryEscape(strings.Join(listItems, ",")) + "]"

	// 为了兼容一刻相册死板的 WAF，将 url.QueryEscape 生成的 %2C(逗号) 等恢复
	apiURL = strings.ReplaceAll(apiURL, "%2C", ",")
	apiURL = strings.ReplaceAll(apiURL, "%7B", "{")
	apiURL = strings.ReplaceAll(apiURL, "%7D", "}")
	apiURL = strings.ReplaceAll(apiURL, "%22", "\"")

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return err
	}
	resp, err := p.doRequest(req)
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func (p *YikePlugin) CheckExistence(remotePath string) (bool, error) {
	return false, nil
}

// ApiListDir uses XPAN to list files in a directory (for cleanup/admin use)
func (p *YikePlugin) ApiListDir(remotePath string) ([]drive.CloudChange, error) {
	p.waitBeforeRequest()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	token := p.getBdsToken(ctx)
	physPath := remotePath
	if !strings.HasPrefix(physPath, "/youa/web") {
		physPath = path.Join("/youa/web", remotePath)
	}

	q := url.Values{}
	q.Add("method", "list")
	q.Add("app_id", "16051585")
	q.Add("dir", physPath)
	if token != "" {
		q.Add("bdstoken", token)
	}

	apiURL := "https://pan.baidu.com/rest/2.0/xpan/file?" + q.Encode()

	req, err := p.newRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Errno int `json:"errno"`
		List  []struct {
			ServerFilename string      `json:"server_filename"`
			Path           string      `json:"path"`
			FsID           json.Number `json:"fs_id"`
			Size           int64       `json:"size"`
			MD5            string      `json:"md5"`
			IsDir          int         `json:"isdir"`
			ServerMtime    int64       `json:"server_mtime"`
		} `json:"list"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.Errno != 0 {
		return nil, fmt.Errorf("yike xpan list error: %d (path: %s)", res.Errno, physPath)
	}

	var changes []drive.CloudChange
	for _, item := range res.List {
		// Calculate the logical path (stripping /youa/web)
		logicalPath := strings.TrimPrefix(item.Path, "/youa/web")
		
		changes = append(changes, drive.CloudChange{
			Path:    logicalPath,
			Action:  "create", // For generic listing
			FsID:    item.FsID.String(),
			Size:    item.Size,
			MD5:     item.MD5,
			IsDir:   item.IsDir == 1,
			ModTime: item.ServerMtime,
		})
	}

	return changes, nil
}
