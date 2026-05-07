package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/gateway"
)

// WeixinAdapter implements PlatformAdapter for WeChat/Weixin.
type WeixinAdapter struct {
	BasePlatformAdapter
	appID       string
	appSecret   string
	token       string // callback verification token
	accessToken string
	webhookPort int
	server      *http.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewWeixinAdapter(appID, appSecret, token string, webhookPort int) *WeixinAdapter {
	if webhookPort == 0 {
		webhookPort = 8077
	}
	return &WeixinAdapter{
		BasePlatformAdapter: NewBasePlatformAdapter(gateway.PlatformWeixin),
		appID:               appID,
		appSecret:           appSecret,
		token:               token,
		webhookPort:         webhookPort,
	}
}

func (w *WeixinAdapter) Connect(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)

	if err := w.refreshAccessToken(); err != nil {
		slog.Warn("Weixin token refresh failed, will retry", "error", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/weixin/callback", w.handleCallback)

	w.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", w.webhookPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	w.connected = true
	slog.Info("Weixin adapter starting", "port", w.webhookPort)

	go func() {
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Weixin server error", "error", err)
		}
	}()

	return nil
}

func (w *WeixinAdapter) Disconnect() error {
	w.connected = false
	if w.cancel != nil {
		w.cancel()
	}
	if w.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return w.server.Shutdown(ctx)
	}
	return nil
}

func (w *WeixinAdapter) Send(ctx context.Context, chatID string, text string, metadata map[string]string) (*gateway.SendResult, error) {
	payload := map[string]any{
		"touser":  chatID,
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	}
	return w.postAPI(ctx, "/cgi-bin/message/custom/send", payload)
}

func (w *WeixinAdapter) SendTyping(_ context.Context, _ string) error { return nil }

func (w *WeixinAdapter) SendImage(ctx context.Context, chatID string, _ string, caption string, metadata map[string]string) (*gateway.SendResult, error) {
	return w.Send(ctx, chatID, caption+" [image]", metadata)
}

func (w *WeixinAdapter) SendVoice(ctx context.Context, chatID string, _ string, metadata map[string]string) (*gateway.SendResult, error) {
	return w.Send(ctx, chatID, "[voice]", metadata)
}

func (w *WeixinAdapter) SendDocument(ctx context.Context, chatID string, _ string, metadata map[string]string) (*gateway.SendResult, error) {
	return w.Send(ctx, chatID, "[file]", metadata)
}

func (w *WeixinAdapter) postAPI(ctx context.Context, path string, payload map[string]any) (*gateway.SendResult, error) {
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.weixin.qq.com%s?access_token=%s", path, w.accessToken)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return &gateway.SendResult{Error: err.Error()}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	defer resp.Body.Close()

	return &gateway.SendResult{Success: true}, nil
}

func (w *WeixinAdapter) refreshAccessToken() error {
	url := fmt.Sprintf("https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s", w.appID, w.appSecret)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("weixin token error: %s", result.ErrMsg)
	}
	w.accessToken = result.AccessToken
	return nil
}

type weixinXMLMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
}

func (w *WeixinAdapter) handleCallback(rw http.ResponseWriter, r *http.Request) {
	// URL verification (GET)
	if r.Method == http.MethodGet {
		echostr := r.URL.Query().Get("echostr")
		fmt.Fprint(rw, echostr)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(rw, "read error", http.StatusBadRequest)
		return
	}

	var msg weixinXMLMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		http.Error(rw, "invalid xml", http.StatusBadRequest)
		return
	}

	if msg.MsgType != "text" {
		rw.WriteHeader(http.StatusOK)
		fmt.Fprint(rw, "success")
		return
	}

	event := &gateway.MessageEvent{
		Text:        msg.Content,
		MessageType: gateway.MessageTypeText,
		Source: gateway.SessionSource{
			Platform: gateway.PlatformWeixin,
			ChatID:   msg.FromUserName,
			ChatType: "dm",
			UserID:   msg.FromUserName,
		},
	}

	w.EmitMessage(event)
	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, "success")
}
