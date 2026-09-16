package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/app"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/config"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/crypto"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/email"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/keys"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/logging"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
	"github.com/spf13/viper"
)

// version is set at build time with -ldflags "-X main.version=v1.0.0".
var version = "dev"

func main() {
	// JSON logs on stdout, one object per line
	logger := logging.New(os.Stdout, slog.LevelInfo)
	slog.SetDefault(logger)
	fatal := func(msg string, args ...any) {
		logger.Error(msg, args...)
		os.Exit(1)
	}

	// Load configuration
	if err := config.Load(); err != nil {
		fatal("Failed to load configuration", slog.Any("error", err))
	}

	// Load and validate the master key(s)
	keyring, err := crypto.LoadKeyringFromEnv()
	if err != nil {
		fatal("Refusing to start with an invalid master key", slog.Any("error", err))
	}
	logger.Info("Master keys loaded", slog.Int("current_version", keyring.CurrentVersion()), slog.Any("versions", keyring.Versions()))

	// Connect to PostgreSQL
	ctx := context.Background()
	dbConfig := config.Database()

	pool, err := storage.NewPostgresPool(ctx, dbConfig)
	if err != nil {
		fatal("Failed to connect to database", slog.Any("error", err))
	}
	defer pool.Close()

	logger.Info("Successfully connected to database")

	// Run database migrations
	migrationsPath := config.MigrationsPath()

	logger.Info("Running database migrations", slog.String("path", migrationsPath))
	if err := storage.RunMigrations(pool, migrationsPath); err != nil {
		fatal("Failed to run database migrations", slog.Any("error", err))
	}
	logger.Info("Database migrations completed successfully")

	if err := keys.CheckKeyring(ctx, pool, keyring); err != nil {
		fatal("Refusing to start: some vaults cannot be decrypted with the configured master keys", slog.Any("error", err))
	}

	// Create email service
	emailService := email.NewEmailServiceFromEnv(email.Options{
		PublicURL:   config.PublicURL(),
		Development: config.IsDevelopment(),
	}, logger)
	logger.Info("Environment", slog.String("app_env", config.Environment()), slog.String("public_url", config.PublicURL()))

	// Read JWT configuration
	jwtSecret := viper.GetString("jwt.secret")
	if err := crypto.ValidateJWTSecret(jwtSecret); err != nil {
		fatal("Refusing to start with an insecure JWT secret", slog.Any("error", err))
	}
	if strings.EqualFold(jwtSecret, os.Getenv("MASTER_KEK")) || strings.Contains(strings.ToLower(os.Getenv("MASTER_KEK_PREVIOUS")), strings.ToLower(jwtSecret)) {
		fatal("Refusing to start: APP_JWT_SECRET must differ from MASTER_KEK")
	}

	router, err := app.New(pool, app.Config{
		Keyring:         keyring,
		JWTSecret:       jwtSecret,
		AccessTokenTTL:  viper.GetDuration("jwt.access_token_ttl"),
		RefreshTokenTTL: viper.GetDuration("jwt.refresh_token_ttl"),
		Email:           emailService,
		Logger:          logger,

		RevealAutoHideSeconds: viper.GetInt("reveal_auto_hide_seconds"),
		Version:               config.Version(version),
		TrustedProxies:        trustedProxies(),
		ClientIPHeader:        viper.GetString("client_ip_header"),
		CORSAllowedOrigins:    splitList(viper.GetString("cors_allowed_origins")),
		EdgeToken:             viper.GetString("edge_token"),
		InsecureCookies:       config.IsDevelopment(),
	})
	if err != nil {
		fatal("Invalid configuration", slog.Any("error", err))
	}

	// Configure HTTP server
	port := config.ServerPort()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", slog.Int("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal("Failed to start server", slog.Any("error", err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fatal("Server forced to shutdown", slog.Any("error", err))
	}

	logger.Info("Server exited gracefully")
}

// trustedProxies reads APP_TRUSTED_PROXIES, a comma-separated list of CIDR ranges whose
// X-Forwarded-For header is trusted. Unset means loopback and private networks.
func trustedProxies() []string {
	return splitList(viper.GetString("trusted_proxies"))
}

// splitList splits a comma-separated setting, returning nil when it is empty.
func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
