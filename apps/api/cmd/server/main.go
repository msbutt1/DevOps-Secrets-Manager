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
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// version is set at build time with -ldflags "-X main.version=v1.0.0".
var version = "dev"

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load configuration
	if err := config.Load(); err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Load and validate Master KEK
	masterKEK, err := crypto.LoadKEKFromEnv()
	if err != nil {
		logger.Fatal("Refusing to start with an invalid master key", zap.Error(err))
	}
	logger.Info("Master KEK loaded successfully")

	// Connect to PostgreSQL
	ctx := context.Background()
	dbConfig := config.Database()

	pool, err := storage.NewPostgresPool(ctx, dbConfig)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	logger.Info("Successfully connected to database")

	// Run database migrations
	migrationsPath := config.MigrationsPath()

	logger.Info("Running database migrations", zap.String("path", migrationsPath))
	if err := storage.RunMigrations(pool, migrationsPath); err != nil {
		logger.Fatal("Failed to run database migrations", zap.Error(err))
	}
	logger.Info("Database migrations completed successfully")

	// Create slog logger for components that need it
	slogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create email service
	emailService := email.NewEmailServiceFromEnv(email.Options{
		PublicURL:   config.PublicURL(),
		Development: config.IsDevelopment(),
	}, slogger)
	logger.Info("Environment", zap.String("app_env", config.Environment()), zap.String("public_url", config.PublicURL()))

	// Read JWT configuration
	jwtSecret := viper.GetString("jwt.secret")
	if err := crypto.ValidateJWTSecret(jwtSecret); err != nil {
		logger.Fatal("Refusing to start with an insecure JWT secret", zap.Error(err))
	}
	if strings.EqualFold(jwtSecret, os.Getenv("MASTER_KEK")) {
		logger.Fatal("Refusing to start: APP_JWT_SECRET must differ from MASTER_KEK")
	}

	router, err := app.New(pool, app.Config{
		MasterKEK:       masterKEK,
		JWTSecret:       jwtSecret,
		AccessTokenTTL:  viper.GetDuration("jwt.access_token_ttl"),
		RefreshTokenTTL: viper.GetDuration("jwt.refresh_token_ttl"),
		Email:           emailService,
		Logger:          logger,
		SLogger:         slogger,

		RevealAutoHideSeconds: viper.GetInt("reveal_auto_hide_seconds"),
		Version:               version,
		TrustedProxies:        trustedProxies(),
	})
	if err != nil {
		logger.Fatal("Invalid configuration", zap.Error(err))
	}

	// Configure HTTP server
	port := viper.GetInt("server.port")
	if port == 0 {
		port = 8080
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", zap.Int("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
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
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited gracefully")
}

// trustedProxies reads APP_TRUSTED_PROXIES, a comma-separated list of CIDR ranges whose
// X-Forwarded-For header is trusted. Unset means loopback and private networks.
func trustedProxies() []string {
	raw := viper.GetString("trusted_proxies")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}
