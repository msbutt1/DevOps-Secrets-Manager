package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/audit"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/auth"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/crypto"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/email"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/environments"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/storage"
	httphandler "github.com/razlafan/devops-secret-manager/apps/api/internal/http"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/policy"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/secrets"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/tokens"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/users"
	"github.com/razlafan/devops-secret-manager/apps/api/internal/vaults"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	// Load .env file if it exists (before logger so env vars are available)
	if err := godotenv.Load(); err != nil {
		// .env file is optional, only log if it's not "file not found"
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: error loading .env file: %v\n", err)
		}
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Load configuration
	if err := loadConfig(); err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Load and validate Master KEK
	masterKEK, err := crypto.LoadKEKFromEnv()
	if err != nil {
		logger.Fatal("Failed to load Master KEK", zap.Error(err))
	}
	logger.Info("Master KEK loaded successfully")

	// Connect to PostgreSQL
	ctx := context.Background()
	dbConfig := storage.PostgresConfig{
		URL:      os.Getenv("DATABASE_URL"), // For Neon, Railway, Fly.io, etc.
		Host:     viper.GetString("database.host"),
		Port:     viper.GetInt("database.port"),
		User:     viper.GetString("database.user"),
		Password: viper.GetString("database.password"),
		Database: viper.GetString("database.name"),
		SSLMode:  viper.GetString("database.sslmode"),
	}

	pool, err := storage.NewPostgresPool(ctx, dbConfig)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	logger.Info("Successfully connected to database")

	// Run database migrations
	migrationsPath := viper.GetString("database.migrations_path")
		if migrationsPath == "" {
			migrationsPath = "../../migrations"
	}


	logger.Info("Running database migrations", zap.String("path", migrationsPath))
	if err := storage.RunMigrations(pool, migrationsPath); err != nil {
		logger.Fatal("Failed to run database migrations", zap.Error(err))
	}
	logger.Info("Database migrations completed successfully")

	// Create slog logger for components that need it
	slogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create repositories
	userRepo := users.NewPostgresRepository(pool)
	refreshTokenRepo := tokens.NewPostgresRepository(pool)
	verificationTokenRepo := email.NewVerificationTokenRepository(pool)
	vaultRepo := vaults.NewPostgresRepository(pool)
	environmentRepo := environments.NewPostgresRepository(pool)
	secretRepo := secrets.NewPostgresRepository(pool)
	auditRepo := audit.NewPostgresRepository(pool)

	// Create email service
	emailService := email.NewEmailServiceFromEnv(slogger)

	// Read JWT configuration
	jwtSecret := viper.GetString("jwt.secret")
	if jwtSecret == "" {
		logger.Fatal("JWT secret is required")
	}

	accessTokenTTL := viper.GetDuration("jwt.access_token_ttl")
	if accessTokenTTL == 0 {
		accessTokenTTL = 15 * time.Minute
	}

	refreshTokenTTL := viper.GetDuration("jwt.refresh_token_ttl")
	if refreshTokenTTL == 0 {
		refreshTokenTTL = 7 * 24 * time.Hour // 7 days
	}

	// Create auth service
	authService := auth.NewAuthService(
		userRepo,
		refreshTokenRepo,
		verificationTokenRepo,
		emailService,
		pool,
		jwtSecret,
		accessTokenTTL,
		refreshTokenTTL,
	)

	// Create vault service
	vaultService := vaults.NewVaultService(vaultRepo, masterKEK)

	// Create environment service
	environmentService := environments.NewEnvironmentService(environmentRepo)

	// Create audit service
	auditService := audit.NewAuditService(auditRepo, slogger)

	// Create secret service
	secretService := secrets.NewSecretService(secretRepo, environmentRepo, vaultRepo, auditService, masterKEK)

	// Create policy service
	policyService := policy.NewPolicyService(pool)

	// Create handlers
	authHandlers := httphandler.NewAuthHandlers(authService, pool, logger)
	vaultHandlers := httphandler.NewVaultHandlers(vaultService, policyService, pool, logger)
	environmentHandlers := httphandler.NewEnvironmentHandlers(environmentService, policyService, pool, logger)
	secretHandlers := httphandler.NewSecretHandlers(secretService, environmentService, policyService, pool, logger)
	auditHandlers := httphandler.NewAuditHandlers(auditService, policyService, pool, slogger)
	memberHandlers := httphandler.NewMemberHandlers(vaultService, policyService, pool, logger)

	// Create HTTP router
	router := httphandler.NewRouter(authHandlers, vaultHandlers, environmentHandlers, secretHandlers, auditHandlers, memberHandlers, jwtSecret)

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

// loadConfig loads configuration from environment variables and optional config file
func loadConfig() error {
	// Set default values
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.sslmode", "disable")

	// Bind environment variables
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	// Optional: load from config file if it exists
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		// Config file is optional, only return error if it's not "file not found"
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	return nil
}
