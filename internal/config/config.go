package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	AppMode  string
	Port     string
	Database DatabaseConfig
	JWT      JWTConfig
	Cookie   CookieConfig
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// JWTConfig holds JWT configuration (for Phase 2)
type JWTConfig struct {
	Secret           string
	RefreshSecret    string
	AccessTokenMins  int
	RefreshTokenDays int
}

// CookieConfig holds cookie configuration (for Phase 2)
type CookieConfig struct {
	Secure   bool
	SameSite string
	Domain   string
}

// Global config instance
var AppConfig *Config

// Load reads configuration from .env file and environment variables
func Load() (*Config, error) {
	// Load .env file (ignore error if file doesn't exist in production)
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Get APP_MODE (default to "dev") - trim spaces for Windows compatibility
	appMode := strings.TrimSpace(getEnv("APP_MODE", "dev"))
	if appMode != "dev" && appMode != "prod" {
		return nil, fmt.Errorf("invalid APP_MODE: '%s' (must be 'dev' or 'prod')", appMode)
	}

	// Build config based on APP_MODE
	config := &Config{
		AppMode:  appMode,
		Port:     getEnv("PORT", "3000"),
		Database: loadDatabaseConfig(appMode),
		JWT:      loadJWTConfig(appMode),
		Cookie:   loadCookieConfig(appMode),
	}

	// Set global config
	AppConfig = config

	log.Printf("✅ Configuration loaded successfully [MODE: %s]", appMode)
	return config, nil
}

// loadDatabaseConfig loads database config based on mode
func loadDatabaseConfig(mode string) DatabaseConfig {
	prefix := "DEV_"
	if mode == "prod" {
		prefix = "PROD_"
	}

	return DatabaseConfig{
		Host:     getEnv(prefix+"DB_HOST", "localhost"),
		Port:     getEnv(prefix+"DB_PORT", "3306"),
		User:     getEnv(prefix+"DB_USER", "root"),
		Password: getEnv(prefix+"DB_PASS", ""),
		DBName:   getEnv(prefix+"DB_NAME", "spsccoop_webt2app"),
	}
}

// loadJWTConfig loads JWT config based on mode
func loadJWTConfig(mode string) JWTConfig {
	prefix := "DEV_"
	if mode == "prod" {
		prefix = "PROD_"
	}

	accessMins, _ := strconv.Atoi(getEnv("ACCESS_TOKEN_MINUTES", "15"))
	refreshDays, _ := strconv.Atoi(getEnv("REFRESH_TOKEN_DAYS", "7"))

	return JWTConfig{
		Secret:           getEnv(prefix+"JWT_SECRET", "default_secret"),
		RefreshSecret:    getEnv(prefix+"JWT_REFRESH_SECRET", "default_refresh_secret"),
		AccessTokenMins:  accessMins,
		RefreshTokenDays: refreshDays,
	}
}

// loadCookieConfig loads cookie config based on mode
func loadCookieConfig(mode string) CookieConfig {
	prefix := "DEV_"
	if mode == "prod" {
		prefix = "PROD_"
	}

	secure, _ := strconv.ParseBool(getEnv(prefix+"COOKIE_SECURE", "false"))

	return CookieConfig{
		Secure:   secure,
		SameSite: getEnv("COOKIE_SAMESITE", "lax"),
		Domain:   getEnv("COOKIE_DOMAIN", ""),
	}
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// IsDev returns true if running in development mode
func (c *Config) IsDev() bool {
	return c.AppMode == "dev"
}

// IsProd returns true if running in production mode
func (c *Config) IsProd() bool {
	return c.AppMode == "prod"
}

// GetAllowedOrigins returns allowed origins for CORS
func (c *Config) GetAllowedOrigins() string {
	origins := getEnv("ALLOWED_ORIGINS", "")
	if origins == "" {
		if c.IsDev() {
			return "*"
		}
		// Default production origins
		return "https://loaneasy.spsc.or.th"
	}
	return origins
}
