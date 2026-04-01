package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"syncghost/internal/auth/baidu"
	"syncghost/internal/config"
	"syncghost/internal/engine"
	"syncghost/internal/logger"
	"syncghost/internal/state"
)

// ssoSession 包装了 SSO 实例和它的销毁函数，防止无头浏览器变僵尸进程
type ssoSession struct {
	sso    *baidu.BaiduSSO
	cancel context.CancelFunc
}

// 【核心升级】：使用 sync.Map 隔离不同页面的 SSO 会话，存储完整的生命周期对象
var ssoStore sync.Map

// generateFallbackSign 用于在无法从 URL 提取 sign 时进行兜底
func generateFallbackSign() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func handleBaiduQR(w http.ResponseWriter, r *http.Request) {
	tpl := r.URL.Query().Get("tpl")
	if tpl == "" { tpl = "pp" } // Default to Yike Photo

	sso := baidu.NewBaiduSSO(tpl)
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	qrURL, err := sso.StartChromedpLoginFlow(bgCtx)
	if err != nil {
		cancel()
		http.Error(w, err.Error(), 500)
		return
	}

	u, _ := url.Parse(qrURL)
	sign := u.Query().Get("sign")
	if sign == "" {
		sign = generateFallbackSign()
	}

	ssoStore.Store(sign, &ssoSession{
		sso:    sso,
		cancel: cancel,
	})

	go func(sig string) {
		time.Sleep(5 * time.Minute)
		if val, ok := ssoStore.Load(sig); ok {
			val.(*ssoSession).cancel()
			ssoStore.Delete(sig)
		}
	}(sign)

	logger.LogInfo("Started Headless Chrome QR Flow for Baidu/Yike (tpl=%s): Sign=%s", tpl, sign)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"sign":   sign,
		"imgURL": qrURL,
	})
}

func handleBaiduPoll(w http.ResponseWriter, r *http.Request) {
	sign := r.URL.Query().Get("sign")
	accountID := r.URL.Query().Get("account_id")
	driveType := r.URL.Query().Get("type")
	if driveType == "" { driveType = "yike" }

	if sign == "" || accountID == "" {
		http.Error(w, "missing sign or account_id", 400)
		return
	}

	val, ok := ssoStore.Load(sign)
	if !ok {
		http.Error(w, "QR code session expired", 400)
		return
	}
	session := val.(*ssoSession)
	activeSSO := session.sso

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	res, err := activeSSO.GetFinalCookies(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Save divine credentials
	state.SaveAuthToken(accountID, driveType, state.AuthToken{
		AccessToken:  res["BDUSS"],
		RefreshToken: res["SToken"],
		Expiry:       time.Now().Add(365 * 24 * time.Hour).Unix(),
	})

	session.cancel()
	ssoStore.Delete(sign)

	logger.LogInfo("Successfully authenticated %s account %s via Headless Chrome", driveType, accountID)
	
	// Trigger Hot-Reload so the engine uses the new credentials immediately
	engine.TriggerReload()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// HandleListAccounts 提供给前端展示当前网盘授权状态
func HandleListAccounts(w http.ResponseWriter, r *http.Request) {
	type AccStatus struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	var res []AccStatus
	
	// 遍历配置文件中定义的账号
	for _, acc := range config.GlobalConfig.Accounts {
		statusStr := "未授权 (需绑定)"
		// 去数据库查一下是否有 Token
		if token, _ := state.GetAuthToken(acc.ID, acc.Type); token != nil {
			statusStr = "已绑定 ✅"
		}
		res = append(res, AccStatus{ID: acc.ID, Type: acc.Type, Status: statusStr})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
