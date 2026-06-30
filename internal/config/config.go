// Package config loads and provides typed configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Core
	Port    int    `env:"API_PORT" default:"2785"`
	LogLevel string `env:"LOG_LEVEL" default:"info"`
	LogFormat string `env:"LOG_FORMAT" default:"auto"`
	Domain   string `env:"DOMAIN" default:"localhost"`

	// Engine
	EngineType string `env:"ENGINE_TYPE" default:"whatsmeow"`

	// Database
	DatabaseType     string `env:"DATABASE_TYPE" default:"sqlite"`
	DatabaseName     string `env:"DATABASE_NAME" default:"openwa"`
	DatabaseHost     string `env:"DATABASE_HOST" default:"localhost"`
	DatabasePort     int    `env:"DATABASE_PORT" default:"5432"`
	DatabaseUser     string `env:"DATABASE_USERNAME" default:"openwa"`
	DatabasePassword string `env:"DATABASE_PASSWORD" default:""`
	DatabaseSsl      bool   `env:"DATABASE_SSL" default:"false"`

	// SQLite path
	SQLitePath string `env:"DATABASE_NAME" default:"./data/openwa.sqlite"`

	// Main DB (always SQLite for auth/audit)
	MainDBPath string `env:"MAIN_DATABASE_NAME" default:"./data/main.sqlite"`

	// Auth
	APIMasterKey  string `env:"API_MASTER_KEY" default:""`
	APIKeyPepper  string `env:"API_KEY_PEPPER" default:""`
	CORSOrigins   string `env:"CORS_ORIGINS" default:"*"`
	TrustedProxies string `env:"TRUSTED_PROXIES" default:""`

	// Session
	AutoStartSessions    bool `env:"AUTO_START_SESSIONS" default:"false"`
	MaxConcurrentSessions int  `env:"MAX_CONCURRENT_SESSIONS" default:"0"`

	// Webhook
	WebhookTimeout    int `env:"WEBHOOK_TIMEOUT" default:"10000"`
	WebhookMaxRetries int `env:"WEBHOOK_MAX_RETRIES" default:"3"`
	WebhookRetryDelay int `env:"WEBHOOK_RETRY_DELAY" default:"5000"`
	WebhookSSRFProtect bool `env:"WEBHOOK_SSRF_PROTECT" default:"true"`

	// Media
	MediaDownloadEnabled  bool   `env:"MEDIA_DOWNLOAD_ENABLED" default:"true"`
	MediaDownloadMaxBytes int64  `env:"MEDIA_DOWNLOAD_MAX_BYTES" default:"52428800"`
	MediaDownloadTimeoutMs int   `env:"MEDIA_DOWNLOAD_TIMEOUT_MS" default:"30000"`

	// Storage
	StorageType    string `env:"STORAGE_TYPE" default:"local"`
	StorageLocalPath string `env:"STORAGE_LOCAL_PATH" default:"./data/media"`
	S3Endpoint     string `env:"S3_ENDPOINT" default:""`
	S3Bucket       string `env:"S3_BUCKET" default:"openwa"`
	S3Region       string `env:"S3_REGION" default:"us-east-1"`
	S3AccessKey    string `env:"S3_ACCESS_KEY_ID" default:""`
	S3SecretKey    string `env:"S3_SECRET_ACCESS_KEY" default:""`

	// Redis
	RedisEnabled bool   `env:"REDIS_ENABLED" default:"false"`
	RedisHost    string `env:"REDIS_HOST" default:"localhost"`
	RedisPort    int    `env:"REDIS_PORT" default:"6379"`
	RedisPassword string `env:"REDIS_PASSWORD" default:""`

	// Rate limit
	RateLimitEnabled  bool `env:"RATE_LIMIT_ENABLED" default:"true"`
	RateLimitTTL      int  `env:"RATE_LIMIT_MEDIUM_TTL" default:"60000"`
	RateLimitMax      int  `env:"RATE_LIMIT_MEDIUM_LIMIT" default:"100"`

	// MCP
	MCPEnabled bool `env:"MCP_ENABLED" default:"false"`

	// Plugins
	PluginsEnabled bool   `env:"PLUGINS_ENABLED" default:"true"`
	PluginsDir     string `env:"PLUGINS_DIR" default:"./data/plugins"`

	// Baileys/whatsmeow auth dir
	AuthDir string `env:"BAILEYS_AUTH_DIR" default:"./data/whatsmeow"`

	// Whatsmeow connection
	SyncFullHistory bool `env:"SYNC_FULL_HISTORY" default:"false"`

	// Dashboard
	ServeDashboard bool `env:"SERVE_DASHBOARD" default:"true"`
}

func Load() *Config {
	cfg := &Config{}
	cfg.load()
	return cfg
}

func (c *Config) load() {
	c.Port = envInt("API_PORT", 2785)
	c.LogLevel = envStr("LOG_LEVEL", "info")
	c.LogFormat = envStr("LOG_FORMAT", "auto")
	c.Domain = envStr("DOMAIN", "localhost")
	c.EngineType = envStr("ENGINE_TYPE", "whatsmeow")
	c.DatabaseType = envStr("DATABASE_TYPE", "sqlite")
	c.DatabaseName = envStr("DATABASE_NAME", "openwa")
	c.DatabaseHost = envStr("DATABASE_HOST", "localhost")
	c.DatabasePort = envInt("DATABASE_PORT", 5432)
	c.DatabaseUser = envStr("DATABASE_USERNAME", "openwa")
	c.DatabasePassword = envStr("DATABASE_PASSWORD", "")
	c.DatabaseSsl = envBool("DATABASE_SSL", false)
	c.SQLitePath = envStr("DATABASE_NAME", "./data/openwa.sqlite")
	c.MainDBPath = envStr("MAIN_DATABASE_NAME", "./data/main.sqlite")
	c.APIMasterKey = envStr("API_MASTER_KEY", "")
	c.APIKeyPepper = envStr("API_KEY_PEPPER", "")
	c.CORSOrigins = envStr("CORS_ORIGINS", "*")
	c.TrustedProxies = envStr("TRUSTED_PROXIES", "")
	c.AutoStartSessions = envBool("AUTO_START_SESSIONS", false)
	c.MaxConcurrentSessions = envInt("MAX_CONCURRENT_SESSIONS", 0)
	c.WebhookTimeout = envInt("WEBHOOK_TIMEOUT", 10000)
	c.WebhookMaxRetries = envInt("WEBHOOK_MAX_RETRIES", 3)
	c.WebhookRetryDelay = envInt("WEBHOOK_RETRY_DELAY", 5000)
	c.WebhookSSRFProtect = envBool("WEBHOOK_SSRF_PROTECT", true)
	c.MediaDownloadEnabled = envBool("MEDIA_DOWNLOAD_ENABLED", true)
	c.MediaDownloadMaxBytes = int64(envInt("MEDIA_DOWNLOAD_MAX_BYTES", 52428800))
	c.MediaDownloadTimeoutMs = envInt("MEDIA_DOWNLOAD_TIMEOUT_MS", 30000)
	c.StorageType = envStr("STORAGE_TYPE", "local")
	c.StorageLocalPath = envStr("STORAGE_LOCAL_PATH", "./data/media")
	c.S3Endpoint = envStr("S3_ENDPOINT", "")
	c.S3Bucket = envStr("S3_BUCKET", "openwa")
	c.S3Region = envStr("S3_REGION", "us-east-1")
	c.S3AccessKey = envStr("S3_ACCESS_KEY_ID", "")
	c.S3SecretKey = envStr("S3_SECRET_ACCESS_KEY", "")
	c.RedisEnabled = envBool("REDIS_ENABLED", false)
	c.RedisHost = envStr("REDIS_HOST", "localhost")
	c.RedisPort = envInt("REDIS_PORT", 6379)
	c.RedisPassword = envStr("REDIS_PASSWORD", "")
	c.RateLimitEnabled = envBool("RATE_LIMIT_ENABLED", true)
	c.RateLimitTTL = envInt("RATE_LIMIT_MEDIUM_TTL", 60000)
	c.RateLimitMax = envInt("RATE_LIMIT_MEDIUM_LIMIT", 100)
	c.MCPEnabled = envBool("MCP_ENABLED", false)
	c.PluginsEnabled = envBool("PLUGINS_ENABLED", true)
	c.PluginsDir = envStr("PLUGINS_DIR", "./data/plugins")
	c.AuthDir = envStr("BAILEYS_AUTH_DIR", "./data/whatsmeow")
	c.SyncFullHistory = envBool("SYNC_FULL_HISTORY", false)
	c.ServeDashboard = envBool("SERVE_DASHBOARD", true)
}

// CORSOriginList returns the parsed list of allowed CORS origins.
func (c *Config) CORSOriginList() []string {
	if c.CORSOrigins == "*" || c.CORSOrigins == "" {
		return []string{"*"}
	}
	parts := strings.Split(c.CORSOrigins, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// TrustedProxyList returns the parsed list of trusted proxy CIDRs.
func (c *Config) TrustedProxyList() []string {
	if c.TrustedProxies == "" {
		return nil
	}
	parts := strings.Split(c.TrustedProxies, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// DBDSN returns the database connection string.
func (c *Config) DBDSN() string {
	if c.DatabaseType == "sqlite" {
		return c.SQLitePath
	}
	sslMode := "disable"
	if c.DatabaseSsl {
		sslMode = "require"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DatabaseHost, c.DatabasePort, c.DatabaseUser, c.DatabasePassword, c.DatabaseName, sslMode)
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		switch strings.ToLower(v) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return def
}

// Duration returns a time.Duration for millisecond config values.
func (c *Config) Duration(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
