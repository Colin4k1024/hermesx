package platforms

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/gateway"
)

// FeishuAdapter implements PlatformAdapter for Feishu/Lark.
type FeishuAdapter struct {
	BasePlatformAdapter
	appID       string
	appSecret   string
	tenantToken string
	webhookPort int
	server      *http.Server
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewFeishuAdapter(appID, appSecret string, webhookPort int) *FeishuAdapter {
	if webhookPort == 0 {
		webhookPort = 8076
	}
	return &FeishuAdapter{
		BasePlatformAdapter: NewBasePlatformAdapter(gateway.PlatformFeishu),
		appID:               appID,
		appSecret:           appSecret,
		webhookPort:         webhookPort,
	}
}

func (f *FeishuAdapter) Connect(ctx context.Context) error {
	f.ctx, f.cancel = context.WithCancel(ctx)

	if err := f.refreshTenantToken(); err != nil {
		return fmt.Errorf("feishu token: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/feishu/event", f.handleEvent)

	f.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", f.webhookPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	f.connected = true
	slog.Info("Feishu adapter starting", "port", f.webhookPort)

	go func() {
		if err := f.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Feishu server error", "error", err)
		}
	}()

	return nil
}

func (f *FeishuAdapter) Disconnect() error {
	f.connected = false
	if f.cancel != nil {
		f.cancel()
	}
	if f.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return f.server.Shutdown(ctx)
	}
	return nil
}

func (f *FeishuAdapter) Send(ctx context.Context, chatID string, text string, metadata map[string]string) (*gateway.SendResult, error) {
	payload := map[string]any{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, text),
	}
	return f.postAPI(ctx, "/open-apis/im/v1/messages?receive_id_type=chat_id", payload)
}

func (f *FeishuAdapter) SendTyping(_ context.Context, _ string) error { return nil }

func (f *FeishuAdapter) SendImage(ctx context.Context, chatID string, imagePath string, caption string, metadata map[string]string) (*gateway.SendResult, error) {
	return f.Send(ctx, chatID, caption+" [image]", metadata)
}

func (f *FeishuAdapter) SendVoice(ctx context.Context, chatID string, audioPath string, metadata map[string]string) (*gateway.SendResult, error) {
	return f.Send(ctx, chatID, "[voice]", metadata)
}

func (f *FeishuAdapter) SendDocument(ctx context.Context, chatID string, filePath string, metadata map[string]string) (*gateway.SendResult, error) {
	return f.Send(ctx, chatID, "[file]", metadata)
}

func (f *FeishuAdapter) postAPI(ctx context.Context, path string, payload map[string]any) (*gateway.SendResult, error) {
	body, _ := json.Marshal(payload)
	url := "https://open.feishu.cn" + path

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return &gateway.SendResult{Error: err.Error()}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.tenantToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return &gateway.SendResult{Error: string(b)}, fmt.Errorf("feishu API error %d", resp.StatusCode)
	}

	return &gateway.SendResult{Success: true}, nil
}

func (f *FeishuAdapter) refreshTenantToken() error {
	payload := map[string]string{
		"app_id":     f.appID,
		"app_secret": f.appSecret,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("feishu token error code %d", result.Code)
	}

	f.tenantToken = result.TenantAccessToken
	return nil
}

type feishuEventPayload struct {
	Challenge string `json:"challenge"` // URL verification
	Header    struct {
		EventType string `json:"event_type"`
		Token     string `json:"token"`
	} `json:"header"`
	Event struct {
		Message struct {
			ChatID    string `json:"chat_id"`
			Content   string `json:"content"` // JSON string
			MessageID string `json:"message_id"`
		} `json:"message"`
		Sender struct {
			SenderID struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
		} `json:"sender"`
	} `json:"event"`
}

func (f *FeishuAdapter) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var payload feishuEventPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// URL verification challenge
	if payload.Challenge != "" {
		json.NewEncoder(w).Encode(map[string]string{"challenge": payload.Challenge})
		return
	}

	if payload.Header.EventType != "im.message.receive_v1" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse message content
	var content struct {
		Text string `json:"text"`
	}
	json.Unmarshal([]byte(payload.Event.Message.Content), &content)

	event := &gateway.MessageEvent{
		Text:        content.Text,
		MessageType: gateway.MessageTypeText,
		Source: gateway.SessionSource{
			Platform: gateway.PlatformFeishu,
			ChatID:   payload.Event.Message.ChatID,
			ChatType: "group",
			UserID:   payload.Event.Sender.SenderID.OpenID,
		},
	}

	f.EmitMessage(event)
	w.WriteHeader(http.StatusOK)
}

// feishu uses sha256 for verification (unused but kept for completeness)
var _ = sha256.New
