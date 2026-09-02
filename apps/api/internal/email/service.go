package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"text/template"
	"time"
)

// ErrNotConfigured is returned when SMTP is not configured outside development.
var ErrNotConfigured = errors.New("email delivery is not configured (set SMTP_HOST and SMTP_PORT)")

// EmailService defines the interface for sending emails
type EmailService interface {
	SendVerificationEmail(ctx context.Context, to, name, token string) error
	SendInviteEmail(ctx context.Context, invite Invite) error
	SendPasswordResetEmail(ctx context.Context, to, name, token string, expiresAt time.Time) error
}

// Invite describes an organization invitation email.
type Invite struct {
	To               string
	OrganizationName string
	InviterName      string
	Role             string
	Token            string
	ExpiresAt        time.Time
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

// Options controls how links in emails are built and what happens without SMTP.
type Options struct {
	// PublicURL is the web app's base URL, used to build links (e.g. http://localhost:5173).
	PublicURL string
	// Development enables the fallback that logs links instead of sending mail when SMTP
	// is not configured. It must be off in production, where the links are credentials.
	Development bool
}

type emailService struct {
	config  *SMTPConfig
	options Options
	logger  *slog.Logger
}

// NewEmailService creates a new email service
func NewEmailService(config *SMTPConfig, options Options, logger *slog.Logger) EmailService {
	options.PublicURL = strings.TrimRight(options.PublicURL, "/")
	return &emailService{
		config:  config,
		options: options,
		logger:  logger,
	}
}

// NewEmailServiceFromEnv creates an email service from SMTP_* environment variables
func NewEmailServiceFromEnv(options Options, logger *slog.Logger) EmailService {
	config := &SMTPConfig{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}

	// Default from address if not configured
	if config.From == "" {
		config.From = "noreply@devops-secrets.local"
	}

	return NewEmailService(config, options, logger)
}

// Configured reports whether SMTP delivery is set up.
func (c *SMTPConfig) Configured() bool {
	return c.Host != "" && c.Port != ""
}

// SendVerificationEmail sends an email verification link to the user
func (s *emailService) SendVerificationEmail(ctx context.Context, to, name, token string) error {
	link := s.link("/verify-email", token)
	body, err := render(verificationTemplate, map[string]string{"Name": name, "Link": link})
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}
	return s.deliver(to, "Verify your email address", body, link)
}

// SendPasswordResetEmail sends a link to choose a new password
func (s *emailService) SendPasswordResetEmail(ctx context.Context, to, name, token string, expiresAt time.Time) error {
	link := s.link("/reset-password", token)
	body, err := render(passwordResetTemplate, map[string]string{
		"Name":    name,
		"Link":    link,
		"Expires": expiresAt.UTC().Format("2 January 2006 15:04 MST"),
	})
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}
	return s.deliver(to, "Reset your password", body, link)
}

// SendInviteEmail sends an invitation to join an organization
func (s *emailService) SendInviteEmail(ctx context.Context, invite Invite) error {
	link := s.link("/invite", invite.Token)
	body, err := render(inviteTemplate, map[string]string{
		"Inviter":      invite.InviterName,
		"Organization": invite.OrganizationName,
		"Role":         invite.Role,
		"Link":         link,
		"Expires":      invite.ExpiresAt.UTC().Format("2 January 2006 15:04 MST"),
	})
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}
	return s.deliver(invite.To, invite.InviterName+" invited you to "+invite.OrganizationName, body, link)
}

// link builds an absolute web app URL carrying a token query parameter.
func (s *emailService) link(path, token string) string {
	return s.options.PublicURL + path + "?token=" + url.QueryEscape(token)
}

// deliver sends the message over SMTP, or in development without SMTP logs the link.
func (s *emailService) deliver(to, subject, body, link string) error {
	if s.config.Configured() {
		return s.sendSMTP(to, subject, body)
	}

	if !s.options.Development {
		return ErrNotConfigured
	}

	// Development fallback: the link is a one-time credential, so this only runs with APP_ENV=development.
	s.logger.Warn("SMTP is not configured; email not sent. Development fallback: open the link below",
		slog.String("to", to),
		slog.String("subject", subject),
		slog.String("link", link),
	)
	return nil
}

const verificationTemplate = `Hello {{.Name}},

Thank you for registering with DevOps Secrets Manager!

Please verify your email address by opening the link below:

{{.Link}}

This link will expire in 24 hours.

If you did not create an account, please ignore this email.

Best regards,
DevOps Secrets Manager Team
`

const passwordResetTemplate = `Hello {{.Name}},

Someone asked to reset the password for your DevOps Secrets Manager account. If it was you,
open the link below to choose a new password:

{{.Link}}

The link can be used once and expires on {{.Expires}}. Resetting your password signs out all
of your sessions.

If you did not ask for this, you can ignore this email; your password stays the same.

DevOps Secrets Manager
`

const inviteTemplate = `Hello,

{{.Inviter}} has invited you to join {{.Organization}} on DevOps Secrets Manager as {{.Role}}.

Open the link below to accept. If you do not have an account yet, you can create one there
with this email address.

{{.Link}}

The invitation expires on {{.Expires}} and can only be used once.

If you were not expecting this, you can ignore this email.

DevOps Secrets Manager
`

func render(tmpl string, data map[string]string) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// sendSMTP sends an email via SMTP
func (s *emailService) sendSMTP(to, subject, body string) error {
	// Build email message
	msg := s.buildMessage(s.config.From, to, subject, body)

	// Only authenticate when credentials are provided (local relays often need none)
	var auth smtp.Auth
	if s.config.User != "" {
		auth = smtp.PlainAuth("", s.config.User, s.config.Password, s.config.Host)
	}

	// Send email
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	if err := smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Email sent successfully",
		slog.String("to", to),
		slog.String("subject", subject),
	)

	return nil
}

// buildMessage constructs the email message
func (s *emailService) buildMessage(from, to, subject, body string) string {
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		headerValue(from), headerValue(to), headerValue(subject), body)
}

// headerValue removes line breaks so user-controlled text (names in subjects) cannot add headers.
func headerValue(v string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(v)
}
