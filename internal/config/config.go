// ===============================
// internal/config/config.go - Updated Configuration
// ===============================

package config

import (
	"fmt"
	"os"
	"strings"
)

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// ConnectionString generates a PostgreSQL connection string from the database config
func (db DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		db.Host, db.Port, db.User, db.Password, db.Name, db.SSLMode)
}

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
	Database DatabaseConfig

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
		FirebaseProjectID:   getEnv("FIREBASE_PROJECT_ID", ""),
		FirebaseCredentials: getEnv("FIREBASE_CREDENTIALS", ""),
		JWTSecret:           getEnv("JWT_SECRET", "your-secret-key"),
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", ""),
			Port:     getEnv("DB_PORT", "25060"),
			User:     getEnv("DB_USER", "doadmin"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "defaultdb"),
			SSLMode:  getEnv("DB_SSLMODE", "require"),
		},
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
	if config.Database.Host == "" || config.Database.User == "" ||
		config.Database.Password == "" || config.Database.Name == "" {
		return nil, ErrMissingDatabaseConfig
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
	ErrMissingDatabaseConfig = ConfigError{Message: "Database configuration (DB_HOST, DB_USER, DB_PASSWORD, DB_NAME) is required"}
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
