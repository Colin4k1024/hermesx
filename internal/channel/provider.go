package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/store"
)

const (
	PlatformFeishu = "feishu"
	PlatformWeixin = "weixin"
	PlatformWeCom  = "wecom"
)

type Principal struct {
	Platform         string
	AppKey           string
	ProviderUserID   string
	DisplayName      string
	RawAuthenticated bool
}

type Provider interface {
	Platform() string
	AuthCodeURL(app *store.ChannelApp, redirectURI, state string) (string, error)
	ExchangeCode(ctx context.Context, app *store.ChannelApp, code string) (*Principal, error)
	VerifyWebhook(ctx context.Context, app *store.ChannelApp, r *http.Request, body []byte) error
}

type ProviderRegistry struct {
	providers map[string]Provider
}

func NewProviderRegistry(resolver secrets.SecretResolver) *ProviderRegistry {
	return &ProviderRegistry{providers: map[string]Provider{
		PlatformFeishu: &feishuProvider{resolver: resolver, client: defaultHTTPClient()},
		PlatformWeixin: &weixinProvider{resolver: resolver, client: defaultHTTPClient()},
		PlatformWeCom:  &wecomProvider{resolver: resolver, client: defaultHTTPClient()},
	}}
}

func (r *ProviderRegistry) Register(p Provider) {
	if r.providers == nil {
		r.providers = make(map[string]Provider)
	}
	r.providers[p.Platform()] = p
}

func (r *ProviderRegistry) Get(platform string) (Provider, bool) {
	p, ok := r.providers[platform]
	return p, ok
}

func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

type feishuProvider struct {
	resolver secrets.SecretResolver
	client   *http.Client
}

func (p *feishuProvider) Platform() string { return PlatformFeishu }

func (p *feishuProvider) AuthCodeURL(app *store.ChannelApp, redirectURI, state string) (string, error) {
	q := url.Values{}
	q.Set("app_id", app.AppKey)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	return "https://open.feishu.cn/open-apis/authen/v1/authorize?" + q.Encode(), nil
}

func (p *feishuProvider) ExchangeCode(ctx context.Context, app *store.ChannelApp, code string) (*Principal, error) {
	secret, err := resolveAppSecret(ctx, p.resolver, app)
	if err != nil {
		return nil, err
	}
	appToken, err := p.feishuAppToken(ctx, app.AppKey, secret)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(map[string]string{"grant_type": "authorization_code", "code": code})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.feishu.cn/open-apis/authen/v1/access_token", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+appToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feishu code exchange failed: status %d", resp.StatusCode)
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			OpenID string `json:"open_id"`
			UserID string `json:"user_id"`
			Name   string `json:"name"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("feishu code exchange error: %s", out.Msg)
	}
	userID := out.Data.OpenID
	if userID == "" {
		userID = out.Data.UserID
	}
	if userID == "" {
		return nil, fmt.Errorf("feishu principal missing user id")
	}
	return &Principal{Platform: app.Platform, AppKey: app.AppKey, ProviderUserID: userID, DisplayName: out.Data.Name, RawAuthenticated: true}, nil
}

func (p *feishuProvider) VerifyWebhook(ctx context.Context, app *store.ChannelApp, r *http.Request, body []byte) error {
	if app.WebhookSecretRef == "" {
		return fmt.Errorf("feishu webhook secret_ref required")
	}
	secret, err := p.resolver.Resolve(ctx, app.WebhookSecretRef)
	if err != nil {
		return err
	}
	var payload struct {
		Header struct {
			Token string `json:"token"`
		} `json:"header"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if payload.Header.Token == "" || payload.Header.Token != secret {
		return fmt.Errorf("invalid feishu webhook token")
	}
	return nil
}

func (p *feishuProvider) feishuAppToken(ctx context.Context, appID, secret string) (string, error) {
	body, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": secret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Code           int    `json:"code"`
		AppAccessToken string `json:"app_access_token"`
		Msg            string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Code != 0 || out.AppAccessToken == "" {
		return "", fmt.Errorf("feishu app token error: %s", out.Msg)
	}
	return out.AppAccessToken, nil
}

type weixinProvider struct {
	resolver secrets.SecretResolver
	client   *http.Client
}

func (p *weixinProvider) Platform() string { return PlatformWeixin }

func (p *weixinProvider) AuthCodeURL(app *store.ChannelApp, redirectURI, state string) (string, error) {
	q := url.Values{}
	q.Set("appid", app.AppKey)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_base")
	q.Set("state", state)
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + q.Encode() + "#wechat_redirect", nil
}

func (p *weixinProvider) ExchangeCode(ctx context.Context, app *store.ChannelApp, code string) (*Principal, error) {
	secret, err := resolveAppSecret(ctx, p.resolver, app)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("appid", app.AppKey)
	q.Set("secret", secret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	reqURL := "https://api.weixin.qq.com/sns/oauth2/access_token?" + q.Encode()
	resp, err := p.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, fmt.Errorf("weixin code exchange error: %s", out.ErrMsg)
	}
	if out.OpenID == "" {
		return nil, fmt.Errorf("weixin principal missing openid")
	}
	return &Principal{Platform: app.Platform, AppKey: app.AppKey, ProviderUserID: out.OpenID, RawAuthenticated: true}, nil
}

func (p *weixinProvider) VerifyWebhook(ctx context.Context, app *store.ChannelApp, r *http.Request, body []byte) error {
	if app.WebhookSecretRef == "" {
		return fmt.Errorf("weixin webhook secret_ref required")
	}
	token, err := p.resolver.Resolve(ctx, app.WebhookSecretRef)
	if err != nil {
		return err
	}
	signature := r.URL.Query().Get("signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	if !VerifyWeixinSignature(token, timestamp, nonce, signature) {
		return fmt.Errorf("invalid weixin webhook signature")
	}
	_ = body
	return nil
}

type wecomProvider struct {
	resolver secrets.SecretResolver
	client   *http.Client
}

func (p *wecomProvider) Platform() string { return PlatformWeCom }

func (p *wecomProvider) AuthCodeURL(app *store.ChannelApp, redirectURI, state string) (string, error) {
	corpID, agentID := splitWeComAppKey(app.AppKey)
	q := url.Values{}
	q.Set("appid", corpID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_base")
	q.Set("state", state)
	if agentID != "" {
		q.Set("agentid", agentID)
	}
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + q.Encode() + "#wechat_redirect", nil
}

func (p *wecomProvider) ExchangeCode(ctx context.Context, app *store.ChannelApp, code string) (*Principal, error) {
	secret, err := resolveAppSecret(ctx, p.resolver, app)
	if err != nil {
		return nil, err
	}
	corpID, _ := splitWeComAppKey(app.AppKey)
	accessToken, err := p.wecomAccessToken(ctx, corpID, secret)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("access_token", accessToken)
	q.Set("code", code)
	resp, err := p.client.Get("https://qyapi.weixin.qq.com/cgi-bin/user/getuserinfo?" + q.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		UserID  string `json:"UserId"`
		OpenID  string `json:"OpenId"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, fmt.Errorf("wecom code exchange error: %s", out.ErrMsg)
	}
	userID := out.UserID
	if userID == "" {
		userID = out.OpenID
	}
	if userID == "" {
		return nil, fmt.Errorf("wecom principal missing user id")
	}
	return &Principal{Platform: app.Platform, AppKey: app.AppKey, ProviderUserID: userID, RawAuthenticated: true}, nil
}

func (p *wecomProvider) VerifyWebhook(ctx context.Context, app *store.ChannelApp, r *http.Request, body []byte) error {
	if app.WebhookSecretRef == "" {
		return fmt.Errorf("wecom webhook secret_ref required")
	}
	token, err := p.resolver.Resolve(ctx, app.WebhookSecretRef)
	if err != nil {
		return err
	}
	var payload struct {
		XMLName xml.Name `xml:"xml"`
		Encrypt string   `xml:"Encrypt"`
	}
	_ = xml.Unmarshal(body, &payload)
	msgSig := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	if !VerifyWeComSignature(token, timestamp, nonce, payload.Encrypt, msgSig) {
		return fmt.Errorf("invalid wecom webhook signature")
	}
	return nil
}

func (p *wecomProvider) wecomAccessToken(ctx context.Context, corpID, secret string) (string, error) {
	q := url.Values{}
	q.Set("corpid", corpID)
	q.Set("corpsecret", secret)
	resp, err := p.client.Get("https://qyapi.weixin.qq.com/cgi-bin/gettoken?" + q.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", fmt.Errorf("wecom token error: %s", out.ErrMsg)
	}
	return out.AccessToken, nil
}

func resolveAppSecret(ctx context.Context, resolver secrets.SecretResolver, app *store.ChannelApp) (string, error) {
	if resolver == nil {
		return "", fmt.Errorf("secret resolver is not configured")
	}
	ref := app.OAuthSecretRef
	if ref == "" {
		ref = app.AppSecretRef
	}
	if ref == "" {
		return "", fmt.Errorf("channel app secret_ref is required")
	}
	return resolver.Resolve(ctx, ref)
}

func splitWeComAppKey(appKey string) (corpID, agentID string) {
	parts := strings.SplitN(appKey, ":", 2)
	corpID = parts[0]
	if len(parts) == 2 {
		agentID = parts[1]
	}
	return corpID, agentID
}

func readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
