package email

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func newTestService(t *testing.T, development bool) (EmailService, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	svc := NewEmailService(&SMTPConfig{From: "noreply@example.test"}, Options{
		PublicURL:   "http://localhost:5173/",
		Development: development,
	}, logger)
	return svc, &logs
}

func TestSendVerificationEmailDevelopmentFallbackLogsLink(t *testing.T) {
	svc, logs := newTestService(t, true)

	if err := svc.SendVerificationEmail(context.Background(), "dev@example.test", "Dev", "tok en+/"); err != nil {
		t.Fatalf("expected development fallback to succeed, got %v", err)
	}

	out := logs.String()
	if !strings.Contains(out, `"link":"http://localhost:5173/verify-email?token=tok+en%2B%2F"`) {
		t.Fatalf("expected escaped verification link in logs, got %s", out)
	}
	if !strings.Contains(out, `"to":"dev@example.test"`) {
		t.Fatalf("expected recipient in logs, got %s", out)
	}
}

func TestSendVerificationEmailWithoutSMTPFailsOutsideDevelopment(t *testing.T) {
	svc, logs := newTestService(t, false)

	err := svc.SendVerificationEmail(context.Background(), "prod@example.test", "Prod", "secret-token")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
	if strings.Contains(logs.String(), "secret-token") {
		t.Fatalf("token must never be logged outside development, got %s", logs.String())
	}
}
