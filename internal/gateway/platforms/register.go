package platforms

import "github.com/Colin4k1024/hermesx/internal/gateway"

func init() {
	r := gateway.GlobalRegistry()

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformTelegram,
		DisplayName: "Telegram",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			token := cfg.Token
			if token == "" {
				return nil, ErrMissingToken
			}
			return NewTelegramAdapter(token), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     true,
			SupportsDocuments: true,
			SupportsStickers:  true,
			SupportsThreads:   true,
			SupportsReactions: true,
			SupportsEdits:     true,
			MaxMessageLength:  4096,
			MaxImages:         10,
		},
		EnvVars: []string{"TELEGRAM_BOT_TOKEN"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformDiscord,
		DisplayName: "Discord",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			token := cfg.Token
			if token == "" {
				return nil, ErrMissingToken
			}
			return NewDiscordAdapter(token), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     false,
			SupportsDocuments: true,
			SupportsStickers:  false,
			SupportsThreads:   true,
			SupportsReactions: true,
			SupportsEdits:     true,
			MaxMessageLength:  2000,
			MaxImages:         10,
		},
		EnvVars: []string{"DISCORD_BOT_TOKEN"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformSlack,
		DisplayName: "Slack",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			token := cfg.Token
			if token == "" {
				return nil, ErrMissingToken
			}
			appToken := cfg.Settings["app_token"]
			return NewSlackAdapter(token, appToken), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     false,
			SupportsDocuments: true,
			SupportsStickers:  false,
			SupportsThreads:   true,
			SupportsReactions: true,
			SupportsEdits:     true,
			MaxMessageLength:  4000,
			MaxImages:         10,
		},
		EnvVars: []string{"SLACK_BOT_TOKEN", "SLACK_APP_TOKEN"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformDMWork,
		DisplayName: "DMWork",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			token := cfg.Token
			if token == "" {
				return nil, ErrMissingToken
			}
			apiURL := cfg.Settings["api_url"]
			if apiURL == "" {
				apiURL = "https://api.botgate.dmwork.cn"
			}
			return NewDMWorkAdapter(apiURL, token), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     true,
			SupportsDocuments: true,
			SupportsStickers:  false,
			SupportsThreads:   false,
			SupportsReactions: false,
			SupportsEdits:     false,
			MaxMessageLength:  4096,
			MaxImages:         9,
		},
		EnvVars: []string{"DMWORK_BOT_TOKEN"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformDingTalk,
		DisplayName: "DingTalk",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			token := cfg.Token
			if token == "" {
				return nil, ErrMissingToken
			}
			secret := cfg.Settings["secret"]
			port := parsePort(cfg.Settings["webhook_port"])
			return NewDingTalkAdapter(token, secret, port), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     false,
			SupportsDocuments: true,
			SupportsStickers:  false,
			SupportsThreads:   false,
			SupportsReactions: false,
			SupportsEdits:     false,
			MaxMessageLength:  20000,
			MaxImages:         0,
		},
		EnvVars: []string{"DINGTALK_TOKEN"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformFeishu,
		DisplayName: "Feishu (Lark)",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			appID := cfg.Settings["app_id"]
			appSecret := cfg.Settings["app_secret"]
			if appID == "" || appSecret == "" {
				return nil, ErrMissingToken
			}
			port := parsePort(cfg.Settings["webhook_port"])
			return NewFeishuAdapter(appID, appSecret, port), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     false,
			SupportsDocuments: true,
			SupportsStickers:  false,
			SupportsThreads:   true,
			SupportsReactions: true,
			SupportsEdits:     false,
			MaxMessageLength:  4096,
			MaxImages:         0,
		},
		EnvVars: []string{"FEISHU_APP_ID", "FEISHU_APP_SECRET"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformWeCom,
		DisplayName: "WeCom (企业微信)",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			corpID := cfg.Settings["corp_id"]
			agentID := cfg.Settings["agent_id"]
			secret := cfg.Settings["secret"]
			token := cfg.Settings["token"]
			aesKey := cfg.Settings["aes_key"]
			if corpID == "" || secret == "" {
				return nil, ErrMissingToken
			}
			port := parsePort(cfg.Settings["webhook_port"])
			return NewWeComAdapter(corpID, agentID, secret, token, aesKey, port), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     true,
			SupportsDocuments: true,
			SupportsStickers:  false,
			SupportsThreads:   false,
			SupportsReactions: false,
			SupportsEdits:     false,
			MaxMessageLength:  2048,
			MaxImages:         0,
		},
		EnvVars: []string{"WECOM_CORP_ID", "WECOM_SECRET"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformWeixin,
		DisplayName: "WeChat (微信公众号)",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			appID := cfg.Settings["app_id"]
			appSecret := cfg.Settings["app_secret"]
			token := cfg.Token
			if appID == "" || token == "" {
				return nil, ErrMissingToken
			}
			port := parsePort(cfg.Settings["webhook_port"])
			return NewWeixinAdapter(appID, appSecret, token, port), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     true,
			SupportsDocuments: false,
			SupportsStickers:  false,
			SupportsThreads:   false,
			SupportsReactions: false,
			SupportsEdits:     false,
			MaxMessageLength:  2048,
			MaxImages:         1,
		},
		EnvVars: []string{"WEIXIN_APP_ID", "WEIXIN_TOKEN"},
	})

	r.Register(&gateway.PlatformRegistration{
		Platform:    gateway.PlatformAPI,
		DisplayName: "HTTP API Server",
		Factory: func(cfg *gateway.PlatformConfig) (gateway.PlatformAdapter, error) {
			port := 8080
			if portStr, ok := cfg.Settings["port"]; ok {
				if p := parsePort(portStr); p > 0 {
					port = p
				}
			}
			apiKey := cfg.Settings["api_key"]
			if apiKey == "" {
				apiKey = cfg.Token
			}
			return NewAPIServerAdapter(port, apiKey), nil
		},
		Capabilities: gateway.PlatformCapabilities{
			SupportsImages:    true,
			SupportsVoice:     true,
			SupportsDocuments: true,
			SupportsStickers:  false,
			SupportsThreads:   true,
			SupportsReactions: false,
			SupportsEdits:     false,
			MaxMessageLength:  0, // unlimited
			MaxImages:         0,
		},
		EnvVars: []string{"HERMES_API_PORT"},
	})
}

func parsePort(s string) int {
	var port int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return port
}
