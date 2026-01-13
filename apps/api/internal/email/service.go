package email

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"text/template"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	SendVerificationEmail(ctx context.Context, to, name, token string) error
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

type emailService struct {
	config *SMTPConfig
	logger *slog.Logger
	devMode bool
}

// NewEmailService creates a new email service
func NewEmailService(config *SMTPConfig, logger *slog.Logger) EmailService {
	// Check if SMTP is configured
	devMode := config.Host == "" || config.Port == ""

	return &emailService{
		config:  config,
		logger:  logger,
		devMode: devMode,
	}
}

// NewEmailServiceFromEnv creates an email service from environment variables
func NewEmailServiceFromEnv(logger *slog.Logger) EmailService {
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

	return NewEmailService(config, logger)
}

// SendVerificationEmail sends an email verification link to the user
func (s *emailService) SendVerificationEmail(ctx context.Context, to, name, token string) error {
	subject := "Verify your email address"

	// Create email body from template
	body, err := s.renderVerificationTemplate(name, token)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	// In dev mode, just log the email
	if s.devMode {
		s.logger.Info("Email sending (DEV MODE - logging only)",
			slog.String("to", to),
			slog.String("subject", subject),
			slog.String("verification_token", token),
		)
		s.logger.Info("Email body", slog.String("body", body))
		return nil
	}

	// Send actual email via SMTP
	return s.sendSMTP(to, subject, body)
}

// renderVerificationTemplate renders the email verification template
func (s *emailService) renderVerificationTemplate(name, token string) (string, error) {
	tmpl := `Hello {{.Name}},

Thank you for registering with DevOps Secrets Manager!

Please verify your email address by clicking the link below:

http://localhost:3000/verify-email?token={{.Token}}

This link will expire in 24 hours.

If you did not create an account, please ignore this email.

Best regards,
DevOps Secrets Manager Team
`

	t, err := template.New("verification").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	data := struct {
		Name  string
		Token string
	}{
		Name:  name,
		Token: token,
	}

	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// sendSMTP sends an email via SMTP
func (s *emailService) sendSMTP(to, subject, body string) error {
	// Build email message
	msg := s.buildMessage(s.config.From, to, subject, body)

	// Setup authentication
	auth := smtp.PlainAuth("", s.config.User, s.config.Password, s.config.Host)

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
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)
}
