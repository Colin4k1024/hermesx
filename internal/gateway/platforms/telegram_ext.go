package platforms

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hermes-agent/hermes-agent-go/internal/gateway"
)

// --- Inline Keyboard ---

// InlineButton represents a button in an inline keyboard.
type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// SendWithInlineKeyboard sends a message with an inline keyboard attached.
func (t *TelegramAdapter) SendWithInlineKeyboard(ctx context.Context, chatID string, text string, rows [][]InlineButton) (*gateway.SendResult, error) {
	if t.bot == nil {
		return nil, fmt.Errorf("not connected")
	}

	id, err := parseChatID(chatID)
	if err != nil {
		return &gateway.SendResult{Success: false, Error: err.Error()}, nil
	}

	// Build keyboard markup.
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, row := range rows {
		var kbRow []tgbotapi.InlineKeyboardButton
		for _, btn := range row {
			if btn.URL != "" {
				kbRow = append(kbRow, tgbotapi.NewInlineKeyboardButtonURL(btn.Text, btn.URL))
			} else {
				kbRow = append(kbRow, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.CallbackData))
			}
		}
		keyboard = append(keyboard, kbRow)
	}

	msg := tgbotapi.NewMessage(id, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	sent, err := t.bot.Send(msg)
	if err != nil {
		return &gateway.SendResult{Success: false, Error: err.Error(), Retryable: true}, nil
	}

	return &gateway.SendResult{
		Success:   true,
		MessageID: fmt.Sprintf("%d", sent.MessageID),
	}, nil
}

// EditInlineKeyboard updates the keyboard on an existing message.
func (t *TelegramAdapter) EditInlineKeyboard(ctx context.Context, chatID string, messageID int, rows [][]InlineButton) error {
	if t.bot == nil {
		return fmt.Errorf("not connected")
	}

	id, err := parseChatID(chatID)
	if err != nil {
		return err
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, row := range rows {
		var kbRow []tgbotapi.InlineKeyboardButton
		for _, btn := range row {
			if btn.URL != "" {
				kbRow = append(kbRow, tgbotapi.NewInlineKeyboardButtonURL(btn.Text, btn.URL))
			} else {
				kbRow = append(kbRow, tgbotapi.NewInlineKeyboardButtonData(btn.Text, btn.CallbackData))
			}
		}
		keyboard = append(keyboard, kbRow)
	}

	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	edit := tgbotapi.NewEditMessageReplyMarkup(id, messageID, markup)
	_, err = t.bot.Send(edit)
	return err
}

// AnswerCallbackQuery acknowledges a callback query from an inline button press.
func (t *TelegramAdapter) AnswerCallbackQuery(queryID string, text string, showAlert bool) error {
	if t.bot == nil {
		return fmt.Errorf("not connected")
	}
	callback := tgbotapi.NewCallback(queryID, text)
	callback.ShowAlert = showAlert
	_, err := t.bot.Request(callback)
	return err
}

// --- Media Groups ---

// MediaGroupItem represents one item in a media group (album).
type MediaGroupItem struct {
	Type    string // "photo" or "document"
	Path    string
	Caption string
}

// SendMediaGroup sends multiple photos/documents as an album.
func (t *TelegramAdapter) SendMediaGroup(ctx context.Context, chatID string, items []MediaGroupItem) (*gateway.SendResult, error) {
	if t.bot == nil {
		return nil, fmt.Errorf("not connected")
	}
	if len(items) < 2 || len(items) > 10 {
		return nil, fmt.Errorf("media group requires 2-10 items, got %d", len(items))
	}

	id, err := parseChatID(chatID)
	if err != nil {
		return &gateway.SendResult{Success: false, Error: err.Error()}, nil
	}

	var media []interface{}
	for _, item := range items {
		switch item.Type {
		case "photo":
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(item.Path))
			photo.Caption = item.Caption
			media = append(media, photo)
		case "document":
			doc := tgbotapi.NewInputMediaDocument(tgbotapi.FilePath(item.Path))
			doc.Caption = item.Caption
			media = append(media, doc)
		default:
			return nil, fmt.Errorf("unsupported media type: %s", item.Type)
		}
	}

	group := tgbotapi.NewMediaGroup(id, media)
	msgs, err := t.bot.SendMediaGroup(group)
	if err != nil {
		return &gateway.SendResult{Success: false, Error: err.Error(), Retryable: true}, nil
	}

	var lastID int
	if len(msgs) > 0 {
		lastID = msgs[len(msgs)-1].MessageID
	}

	return &gateway.SendResult{
		Success:   true,
		MessageID: fmt.Sprintf("%d", lastID),
	}, nil
}

// --- Network Auto-Reconnect ---

// reconnectConfig holds reconnection parameters.
type reconnectConfig struct {
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

var defaultReconnectConfig = reconnectConfig{
	maxRetries:     10,
	initialBackoff: 2 * time.Second,
	maxBackoff:     5 * time.Minute,
}

// ConnectWithReconnect establishes a connection and automatically reconnects
// on polling errors with exponential backoff.
func (t *TelegramAdapter) ConnectWithReconnect(ctx context.Context) error {
	return t.connectWithReconnect(ctx, defaultReconnectConfig)
}

func (t *TelegramAdapter) connectWithReconnect(ctx context.Context, cfg reconnectConfig) error {
	if t.token == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}

	bot, err := tgbotapi.NewBotAPI(t.token)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	t.bot = bot
	t.connected = true
	slog.Info("Telegram bot connected", "username", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		backoff := cfg.initialBackoff
		failures := 0

		for {
			select {
			case <-ctx.Done():
				bot.StopReceivingUpdates()
				return
			default:
			}

			updates := bot.GetUpdatesChan(u)

			for {
				select {
				case <-ctx.Done():
					bot.StopReceivingUpdates()
					return
				case update, ok := <-updates:
					if !ok {
						// Channel closed — polling error, need reconnect.
						goto reconnect
					}
					// Reset backoff on successful message.
					backoff = cfg.initialBackoff
					failures = 0

					if update.Message != nil {
						t.handleUpdate(update)
					}
					if update.CallbackQuery != nil {
						t.handleCallbackQuery(update)
					}
				}
			}

		reconnect:
			failures++
			if failures > cfg.maxRetries {
				slog.Error("telegram reconnect failed, max retries exceeded",
					"failures", failures)
				t.connected = false
				return
			}

			slog.Warn("telegram polling error, reconnecting",
				"backoff", backoff.Round(time.Second),
				"attempt", failures,
			)

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			// Exponential backoff with cap.
			backoff *= 2
			if backoff > cfg.maxBackoff {
				backoff = cfg.maxBackoff
			}
		}
	}()

	// Store cleanup function.
	t.reconnectWg = &wg

	return nil
}

// handleCallbackQuery processes inline keyboard button presses.
func (t *TelegramAdapter) handleCallbackQuery(update tgbotapi.Update) {
	cb := update.CallbackQuery
	if cb == nil || cb.Message == nil {
		return
	}

	source := gateway.SessionSource{
		Platform: gateway.PlatformTelegram,
		ChatID:   fmt.Sprintf("%d", cb.Message.Chat.ID),
		ChatName: cb.Message.Chat.Title,
		ChatType: "dm",
		UserID:   fmt.Sprintf("%d", cb.From.ID),
		UserName: effectiveName(cb.From),
	}

	if cb.Message.Chat.IsGroup() || cb.Message.Chat.IsSuperGroup() {
		source.ChatType = "group"
	}

	event := &gateway.MessageEvent{
		Text:        cb.Data,
		MessageType: gateway.MessageTypeCommand,
		Source:      source,
		RawMessage:  update,
	}

	t.EmitMessage(event)
}
