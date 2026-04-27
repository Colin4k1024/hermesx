package platforms

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/hermes-agent/hermes-agent-go/internal/gateway"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// SlackAdapter implements PlatformAdapter for Slack using Socket Mode.
type SlackAdapter struct {
	BasePlatformAdapter
	api       *slack.Client
	socket    *socketmode.Client
	botToken  string
	appToken  string
	botUserID string
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewSlackAdapter(botToken, appToken string) *SlackAdapter {
	if appToken == "" {
		appToken = os.Getenv("SLACK_APP_TOKEN")
	}
	return &SlackAdapter{
		BasePlatformAdapter: NewBasePlatformAdapter(gateway.PlatformSlack),
		botToken:            botToken,
		appToken:            appToken,
	}
}

func (s *SlackAdapter) Connect(ctx context.Context) error {
	s.api = slack.New(s.botToken, slack.OptionAppLevelToken(s.appToken))

	authResp, err := s.api.AuthTest()
	if err != nil {
		return fmt.Errorf("slack auth: %w", err)
	}
	s.botUserID = authResp.UserID

	s.socket = socketmode.New(s.api)
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.connected = true

	slog.Info("Slack connected", "bot", authResp.User, "team", authResp.Team)

	go s.handleEvents()
	go func() {
		if err := s.socket.RunContext(s.ctx); err != nil && s.connected {
			slog.Error("Slack socket mode error", "error", err)
		}
	}()

	return nil
}

func (s *SlackAdapter) Disconnect() error {
	s.connected = false
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *SlackAdapter) Send(ctx context.Context, chatID string, text string, metadata map[string]string) (*gateway.SendResult, error) {
	opts := []slack.MsgOption{slack.MsgOptionText(text, false)}

	if threadTS, ok := metadata["thread_id"]; ok && threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	_, ts, err := s.api.PostMessage(chatID, opts...)
	if err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	return &gateway.SendResult{Success: true, MessageID: ts}, nil
}

func (s *SlackAdapter) SendTyping(ctx context.Context, chatID string) error {
	return nil // Slack doesn't have a typing indicator API for bots
}

func (s *SlackAdapter) SendImage(ctx context.Context, chatID string, imagePath string, caption string, metadata map[string]string) (*gateway.SendResult, error) {
	return s.uploadFile(chatID, imagePath, caption, metadata)
}

func (s *SlackAdapter) SendVoice(ctx context.Context, chatID string, audioPath string, metadata map[string]string) (*gateway.SendResult, error) {
	return s.uploadFile(chatID, audioPath, "", metadata)
}

func (s *SlackAdapter) SendDocument(ctx context.Context, chatID string, filePath string, metadata map[string]string) (*gateway.SendResult, error) {
	return s.uploadFile(chatID, filePath, "", metadata)
}

func (s *SlackAdapter) uploadFile(chatID, filePath, comment string, metadata map[string]string) (*gateway.SendResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return &gateway.SendResult{Error: err.Error()}, err
	}
	defer f.Close()

	params := slack.UploadFileV2Parameters{
		Channel:        chatID,
		Reader:         f,
		Filename:       filePath,
		InitialComment: comment,
	}

	if threadTS, ok := metadata["thread_id"]; ok && threadTS != "" {
		params.ThreadTimestamp = threadTS
	}

	summary, err := s.api.UploadFileV2(params)
	if err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	return &gateway.SendResult{Success: true, MessageID: summary.ID}, nil
}

func (s *SlackAdapter) handleEvents() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case evt := <-s.socket.Events:
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				s.socket.Ack(*evt.Request)

				if eventsAPIEvent.Type == slackevents.CallbackEvent {
					switch innerEvent := eventsAPIEvent.InnerEvent.Data.(type) {
					case *slackevents.MessageEvent:
						s.handleMessage(innerEvent)
					}
				}
			}
		}
	}
}

func (s *SlackAdapter) handleMessage(msg *slackevents.MessageEvent) {
	if msg.User == s.botUserID || msg.User == "" {
		return
	}
	if msg.SubType != "" {
		return
	}

	chatType := "dm"
	if strings.HasPrefix(msg.Channel, "C") || strings.HasPrefix(msg.Channel, "G") {
		chatType = "group"
	}

	event := &gateway.MessageEvent{
		Text:        msg.Text,
		MessageType: gateway.MessageTypeText,
		Source: gateway.SessionSource{
			Platform: gateway.PlatformSlack,
			ChatID:   msg.Channel,
			ChatType: chatType,
			UserID:   msg.User,
			ThreadID: msg.ThreadTimeStamp,
		},
	}

	s.EmitMessage(event)
}
