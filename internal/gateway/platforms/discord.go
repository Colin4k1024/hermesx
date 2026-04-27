package platforms

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/hermes-agent/hermes-agent-go/internal/gateway"
)

// DiscordAdapter implements PlatformAdapter for Discord.
type DiscordAdapter struct {
	BasePlatformAdapter
	session *discordgo.Session
	token   string
	botID   string
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewDiscordAdapter(token string) *DiscordAdapter {
	return &DiscordAdapter{
		BasePlatformAdapter: NewBasePlatformAdapter(gateway.PlatformDiscord),
		token:               token,
	}
}

func (d *DiscordAdapter) Connect(ctx context.Context) error {
	sess, err := discordgo.New("Bot " + d.token)
	if err != nil {
		return fmt.Errorf("discord connect: %w", err)
	}

	sess.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	sess.AddHandler(d.onMessageCreate)

	if err := sess.Open(); err != nil {
		return fmt.Errorf("discord open: %w", err)
	}

	d.session = sess
	d.botID = sess.State.User.ID
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.connected = true

	slog.Info("Discord connected", "bot", sess.State.User.Username)
	return nil
}

func (d *DiscordAdapter) Disconnect() error {
	d.connected = false
	if d.cancel != nil {
		d.cancel()
	}
	if d.session != nil {
		return d.session.Close()
	}
	return nil
}

func (d *DiscordAdapter) Send(ctx context.Context, chatID string, text string, metadata map[string]string) (*gateway.SendResult, error) {
	msg, err := d.session.ChannelMessageSend(chatID, text)
	if err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	return &gateway.SendResult{Success: true, MessageID: msg.ID}, nil
}

func (d *DiscordAdapter) SendTyping(ctx context.Context, chatID string) error {
	return d.session.ChannelTyping(chatID)
}

func (d *DiscordAdapter) SendImage(ctx context.Context, chatID string, imagePath string, caption string, metadata map[string]string) (*gateway.SendResult, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return &gateway.SendResult{Error: err.Error()}, err
	}
	defer f.Close()

	msg, err := d.session.ChannelFileSendWithMessage(chatID, caption, imagePath, f)
	if err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	return &gateway.SendResult{Success: true, MessageID: msg.ID}, nil
}

func (d *DiscordAdapter) SendVoice(ctx context.Context, chatID string, audioPath string, metadata map[string]string) (*gateway.SendResult, error) {
	return d.sendFile(chatID, audioPath)
}

func (d *DiscordAdapter) SendDocument(ctx context.Context, chatID string, filePath string, metadata map[string]string) (*gateway.SendResult, error) {
	return d.sendFile(chatID, filePath)
}

func (d *DiscordAdapter) sendFile(chatID, filePath string) (*gateway.SendResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return &gateway.SendResult{Error: err.Error()}, err
	}
	defer f.Close()

	msg, err := d.session.ChannelFileSend(chatID, filePath, f)
	if err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	return &gateway.SendResult{Success: true, MessageID: msg.ID}, nil
}

func (d *DiscordAdapter) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == d.botID {
		return
	}

	chatType := "dm"
	if m.GuildID != "" {
		chatType = "group"
		// Only respond if bot is mentioned or in DM
		mentioned := false
		for _, mention := range m.Mentions {
			if mention.ID == d.botID {
				mentioned = true
				break
			}
		}
		if !mentioned && !strings.Contains(m.Content, "<@"+d.botID+">") {
			return
		}
	}

	threadID := ""
	if m.Thread != nil {
		threadID = m.Thread.ID
	}

	event := &gateway.MessageEvent{
		Text:        m.Content,
		MessageType: gateway.MessageTypeText,
		Source: gateway.SessionSource{
			Platform: gateway.PlatformDiscord,
			ChatID:   m.ChannelID,
			ChatType: chatType,
			UserID:   m.Author.ID,
			UserName: m.Author.Username,
			ThreadID: threadID,
		},
	}

	d.EmitMessage(event)
}
