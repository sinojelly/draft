package baidu

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"syncghost/internal/logger"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type BaiduSSO struct {
	tpl string // 产品线 ID

	pollMu     sync.Mutex
	isDone     bool
	resCookies map[string]string
	resErr     error
}

func NewBaiduSSO(tpl string) *BaiduSSO {
	if tpl == "" {
		tpl = "pp" // 默认登录一刻相册
	}
	return &BaiduSSO{
		tpl: tpl,
	}
}

type QRCodeInfo struct {
	SignUrl string `json:"sign_url"`
	Sign    string `json:"sign"`
}

// GetQRCode 兼容旧接口，防止其他地方调用报错
func (s *BaiduSSO) GetQRCode() (*QRCodeInfo, error) {
	return nil, fmt.Errorf("this method is deprecated, please use StartChromedpLoginFlow")
}

// PollScanStatus 兼容旧接口
func (s *BaiduSSO) PollScanStatus(ctx context.Context, sign string) (map[string]string, error) {
	return nil, fmt.Errorf("this method is deprecated, please use GetFinalCookies")
}

// StartChromedpLoginFlow 启动 Headless Chrome 登录流程，并返回二维码链接
func (s *BaiduSSO) StartChromedpLoginFlow(ctx context.Context) (string, error) {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
	)

	// 【核心修复1】：脱离 HTTP 请求上下文！
	// 创建一个独立的后台 Context，生命周期为 3 分钟。哪怕前端拿完二维码断开了，浏览器依然存活等待扫码
	bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

	allocCtx, _ := chromedp.NewExecAllocator(bgCtx, opts...)
	taskCtx, _ := chromedp.NewContext(allocCtx)

	var targetHome string
	if s.tpl == "pp" {
		targetHome = "https://photo.baidu.com/photo/web/home"
	} else {
		// 【核心修复2】：百度网盘现在的落地页经常变化，统一定向到 pan 主域名
		targetHome = "https://pan.baidu.com/disk/main"
	}
	loginURL := fmt.Sprintf("https://passport.baidu.com/v2/?login&tpl=%s&u=%s", s.tpl, url.QueryEscape(targetHome))
	logger.LogInfo("[BaiduSSO] Starting login flow for tpl=%s, target=%s", s.tpl, targetHome)

	qrCodeUrlChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		// 【核心修复3】：无论登录成功还是 3 分钟超时，最终必须销毁浏览器释放内存
		defer cancel()

		var qrUrl string
		var ok bool

		err := chromedp.Run(taskCtx,
			network.Enable(),
			chromedp.Navigate(loginURL),
			// 【优化】：用更精确的选择器，防止抓到“下载APP”的二维码
			chromedp.WaitVisible(`img[src*="/v2/api/qrcode"]`, chromedp.ByQuery),
			chromedp.AttributeValue(`img[src*="/v2/api/qrcode"]`, "src", &qrUrl, &ok, chromedp.ByQuery),
			chromedp.ActionFunc(func(taskCtx context.Context) error {
				if !ok || qrUrl == "" {
					return fmt.Errorf("failed to extract QR code src")
				}
				if strings.HasPrefix(qrUrl, "//") {
					qrUrl = "https:" + qrUrl
				} else if strings.HasPrefix(qrUrl, "/") {
					qrUrl = "https://passport.baidu.com" + qrUrl
				}
				logger.LogDebug("[Chromedp] 成功捕获二维码链接: %s", qrUrl)
				qrCodeUrlChan <- qrUrl
				return nil
			}),
			chromedp.ActionFunc(func(taskCtx context.Context) error {
				for {
					select {
					case <-taskCtx.Done(): // 监听后台 3 分钟超时的 Context
						return taskCtx.Err()
					default:
						var currentURL string
						if err := chromedp.Evaluate(`window.location.href`, &currentURL).Do(taskCtx); err != nil {
							time.Sleep(1 * time.Second)
							continue
						}

						// 【核心修复4】：放宽判定！只要当前域名不是 passport，并且进入了 pan 或 photo 就算成功
						if !strings.Contains(currentURL, "passport.baidu.com") &&
							(strings.Contains(currentURL, "photo.baidu.com") || strings.Contains(currentURL, "pan.baidu.com")) {

							logger.LogDebug("[Chromedp] 检测到登录成功重定向！当前URL: %s", currentURL)

							cookies, err := network.GetCookies().Do(taskCtx)
							if err != nil {
								return err
							}

							s.pollMu.Lock()
							s.resCookies = make(map[string]string)

							var fullCookie strings.Builder
							for _, c := range cookies {
								fullCookie.WriteString(fmt.Sprintf("%s=%s; ", c.Name, c.Value))
								if c.Name == "SToken" {
									s.resCookies["SToken"] = c.Value
								}
							}

							s.resCookies["BDUSS"] = fullCookie.String()
							s.isDone = true
							s.pollMu.Unlock()

							logger.LogDebug("[Chromedp] ✅ 成功捕获全量浏览器指纹，风控护盾已挂载！")
							return nil
						}
						time.Sleep(1 * time.Second)
					}
				}
			}),
		)

		if err != nil && taskCtx.Err() == nil {
			errChan <- err
		}
	}()

	// 主线程只等拿二维码，拿到就立刻返回给前端
	select {
	case <-ctx.Done(): // 如果前端请求断开了（比如用户关了页面），取消后台任务
		cancel()
		return "", ctx.Err()
	case err := <-errChan:
		return "", fmt.Errorf("chromedp task failed: %v", err)
	case qrUrl := <-qrCodeUrlChan:
		return qrUrl, nil
	}
}

func (s *BaiduSSO) GetFinalCookies(ctx context.Context) (map[string]string, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			s.pollMu.Lock()
			if s.isDone {
				defer s.pollMu.Unlock()
				return s.resCookies, s.resErr
			}
			s.pollMu.Unlock()
			time.Sleep(1 * time.Second)
		}
	}
}
