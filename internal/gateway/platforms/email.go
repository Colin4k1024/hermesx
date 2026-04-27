package platforms

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"

	"github.com/hermes-agent/hermes-agent-go/internal/gateway"
)

const PlatformEmail gateway.Platform = "email"

// EmailAdapter implements PlatformAdapter for email (SMTP send, IMAP receive stub).
type EmailAdapter struct {
	BasePlatformAdapter
	smtpHost string
	smtpPort string
	imapHost string
	username string
	password string
	fromAddr string
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewEmailAdapter(smtpHost, smtpPort, imapHost, username, password, fromAddr string) *EmailAdapter {
	return &EmailAdapter{
		BasePlatformAdapter: NewBasePlatformAdapter(PlatformEmail),
		smtpHost:            smtpHost,
		smtpPort:            smtpPort,
		imapHost:            imapHost,
		username:            username,
		password:            password,
		fromAddr:            fromAddr,
	}
}

func NewEmailAdapterFromEnv() *EmailAdapter {
	return NewEmailAdapter(
		os.Getenv("EMAIL_SMTP_HOST"),
		os.Getenv("EMAIL_SMTP_PORT"),
		os.Getenv("EMAIL_IMAP_HOST"),
		os.Getenv("EMAIL_USERNAME"),
		os.Getenv("EMAIL_PASSWORD"),
		os.Getenv("EMAIL_FROM"),
	)
}

func (e *EmailAdapter) Connect(ctx context.Context) error {
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.connected = true
	slog.Info("Email adapter connected", "smtp", e.smtpHost, "from", e.fromAddr)
	return nil
}

func (e *EmailAdapter) Disconnect() error {
	e.connected = false
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}

func (e *EmailAdapter) Send(_ context.Context, chatID string, text string, metadata map[string]string) (*gateway.SendResult, error) {
	subject := "Hermes Agent Response"
	if s, ok := metadata["subject"]; ok {
		subject = s
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", e.fromAddr, chatID, subject, text)

	auth := smtp.PlainAuth("", e.username, e.password, e.smtpHost)
	addr := e.smtpHost + ":" + e.smtpPort

	if err := smtp.SendMail(addr, auth, e.fromAddr, []string{chatID}, []byte(msg)); err != nil {
		return &gateway.SendResult{Error: err.Error(), Retryable: true}, err
	}
	return &gateway.SendResult{Success: true}, nil
}

func (e *EmailAdapter) SendTyping(_ context.Context, _ string) error { return nil }
func (e *EmailAdapter) SendImage(ctx context.Context, chatID string, _ string, caption string, md map[string]string) (*gateway.SendResult, error) {
	return e.Send(ctx, chatID, caption+" [attachment pending]", md)
}
func (e *EmailAdapter) SendVoice(ctx context.Context, chatID string, _ string, md map[string]string) (*gateway.SendResult, error) {
	return e.Send(ctx, chatID, "[voice attachment pending]", md)
}
func (e *EmailAdapter) SendDocument(ctx context.Context, chatID string, _ string, md map[string]string) (*gateway.SendResult, error) {
	return e.Send(ctx, chatID, "[document attachment pending]", md)
}
