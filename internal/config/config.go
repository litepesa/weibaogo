// ===============================
// internal/config/config.go - Updated Configuration
// ===============================

package config

import (
	"fmt"
	"os"
	"strings"
)

// R2Config holds Cloudflare R2 configuration
type R2Config struct {
	AccountID  string
	AccessKey  string
	SecretKey  string
	BucketName string
	PublicURL  string
}

// Config holds all application configuration
type Config struct {
	// Server configuration
	Environment string
	Port        string

	// Database configuration
	DatabaseURL string

	// Firebase configuration
	FirebaseProjectID   string
	FirebaseCredentials string // Path to service account JSON file

	// R2 Storage configuration
	R2Config R2Config

	// CORS configuration
	AllowedOrigins []string

	// Security
	JWTSecret string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Environment:         getEnv("GIN_MODE", "debug"),
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		FirebaseProjectID:   getEnv("FIREBASE_PROJECT_ID", ""),
		FirebaseCredentials: getEnv("FIREBASE_CREDENTIALS", ""),
		JWTSecret:           getEnv("JWT_SECRET", "your-secret-key"),
		R2Config: R2Config{
			AccountID:  getEnv("R2_ACCOUNT_ID", ""),
			AccessKey:  getEnv("R2_ACCESS_KEY", ""),
			SecretKey:  getEnv("R2_SECRET_KEY", ""),
			BucketName: getEnv("R2_BUCKET_NAME", "weibaomedia"),
		},
	}

	// Set public URL for R2
	if config.R2Config.AccountID != "" && config.R2Config.BucketName != "" {
		config.R2Config.PublicURL = fmt.Sprintf("https://%s.%s.r2.cloudflarestorage.com",
			config.R2Config.BucketName, config.R2Config.AccountID)
	}

	// Parse allowed origins
	originsStr := getEnv("ALLOWED_ORIGINS", "http://localhost:3000,https://yourdomain.com")
	config.AllowedOrigins = strings.Split(originsStr, ",")
	for i, origin := range config.AllowedOrigins {
		config.AllowedOrigins[i] = strings.TrimSpace(origin)
	}

	// Validate required configuration
	if config.DatabaseURL == "" {
		return nil, ErrMissingDatabaseURL
	}

	if config.R2Config.AccountID == "" || config.R2Config.AccessKey == "" || config.R2Config.SecretKey == "" {
		return nil, ErrMissingR2Config
	}

	if config.FirebaseProjectID == "" {
		return nil, ErrMissingFirebaseConfig
	}

	return config, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Configuration errors
var (
	ErrMissingDatabaseURL    = ConfigError{Message: "DATABASE_URL environment variable is required"}
	ErrMissingR2Config       = ConfigError{Message: "R2 configuration (R2_ACCOUNT_ID, R2_ACCESS_KEY, R2_SECRET_KEY) is required"}
	ErrMissingFirebaseConfig = ConfigError{Message: "FIREBASE_PROJECT_ID is required"}
)

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

func (e ConfigError) Error() string {
	return e.Message
}
