// Package config loads API configuration from an optional .env file, an optional
// config.yaml and APP_-prefixed environment variables.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
	"github.com/spf13/viper"
)

// Load reads .env (if present) into the process environment and configures viper.
func Load() error {
	// .env is optional; only report errors other than "file not found"
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: error loading .env file: %v\n", err)
	}

	// Set default values
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.migrations_path", "migrations")

	// Bind environment variables
	viper.SetEnvPrefix("APP")
	// Map nested keys to env vars: database.host -> APP_DATABASE_HOST (as docker-compose sets them)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

// Database returns the PostgreSQL connection settings.
func Database() storage.PostgresConfig {
	return storage.PostgresConfig{
		URL:      os.Getenv("DATABASE_URL"), // For Neon, Railway, Fly.io, etc.
		Host:     viper.GetString("database.host"),
		Port:     viper.GetInt("database.port"),
		User:     viper.GetString("database.user"),
		Password: viper.GetString("database.password"),
		Database: viper.GetString("database.name"),
		SSLMode:  viper.GetString("database.sslmode"),
	}
}

// MigrationsPath returns the directory containing the SQL migrations.
func MigrationsPath() string {
	return viper.GetString("database.migrations_path")
}
