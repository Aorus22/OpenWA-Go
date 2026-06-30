package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/openwa/openwa-go/internal/api/handler"
	"github.com/openwa/openwa-go/internal/api/middleware"
	"github.com/openwa/openwa-go/internal/api/router"
	"github.com/openwa/openwa-go/internal/config"
	"github.com/openwa/openwa-go/internal/models"
	"github.com/openwa/openwa-go/internal/services"
	"github.com/openwa/openwa-go/internal/storage"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	cfg := config.Load()

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialize databases
	dataDB, err := initDataDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize data database: %v", err)
	}

	mainDB, err := initMainDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize main database: %v", err)
	}

	// Auto migrate
	if err := dataDB.AutoMigrate(
		&models.Session{},
		&models.Message{},
		&models.Webhook{},
		&models.Template{},
	); err != nil {
		log.Fatalf("Failed to migrate data database: %v", err)
	}

	if err := mainDB.AutoMigrate(
		&models.ApiKey{},
		&models.AuditLog{},
		&models.Settings{},
	); err != nil {
		log.Fatalf("Failed to migrate main database: %v", err)
	}

	// Create default admin key if none exists
	ensureAdminKey(mainDB, cfg)

	// Initialize services
	engineFactory := services.NewEngineFactory(cfg)
	messageService := services.NewMessageService(dataDB)
	webhookService := services.NewWebhookService(
		dataDB,
		cfg.WebhookTimeout,
		cfg.WebhookMaxRetries,
		cfg.WebhookRetryDelay,
		cfg.WebhookSSRFProtect,
		cfg.TrustedProxyList(),
	)
	sessionService := services.NewSessionService(dataDB, cfg, engineFactory)

	// Reset all active session statuses to disconnected on startup
	dataDB.Model(&models.Session{}).
		Where("status IN ('ready', 'initializing', 'qr_ready', 'authenticating')").
		Update("status", models.SessionStatusDisconnected)

	// Initialize storage
	var mediaStorage storage.MediaStorage
	if cfg.StorageType == "s3" && cfg.S3Endpoint != "" {
		s3store, err := storage.NewS3Storage(
			cfg.S3Endpoint, cfg.S3Bucket, cfg.S3Region,
			cfg.S3AccessKey, cfg.S3SecretKey, "media/",
		)
		if err != nil {
			log.Printf("Warning: Failed to initialize S3 storage: %v, using local", err)
			mediaStorage = storage.NewLocalStorage(cfg.StorageLocalPath)
		} else {
			mediaStorage = s3store
		}
	} else {
		mediaStorage = storage.NewLocalStorage(cfg.StorageLocalPath)
	}
	_ = mediaStorage // Will be used for media downloads/uploads

	// Initialize handlers
	sessionHandler := handler.NewSessionHandler(sessionService)
	messageHandler := handler.NewMessageHandler(sessionService)
	groupHandler := handler.NewGroupHandler(sessionService)
	webhookHandler := handler.NewWebhookHandler(webhookService)
	authHandler := handler.NewAuthHandler(mainDB, cfg.APIKeyPepper)
	infraHandler := handler.NewInfraHandler(dataDB, mainDB)
	auditHandler := handler.NewAuditHandler(mainDB)
	templateHandler := handler.NewTemplateHandler(dataDB)

	// Initialize middleware
	auth := middleware.NewAuthMiddleware(mainDB, cfg.APIKeyPepper, cfg.APIMasterKey)
	var rateLimiter *middleware.RateLimiter
	if cfg.RateLimitEnabled {
		rateLimiter = middleware.NewRateLimiter(
			time.Duration(cfg.RateLimitTTL)*time.Millisecond,
			cfg.RateLimitMax,
		)
	}

	// Dashboard directory (check both Docker path and local dev path)
	dashboardDir := "/app/dashboard/dist"
	if _, err := os.Stat(dashboardDir + "/index.html"); os.IsNotExist(err) {
		// Fallback to local dev path
		dashboardDir = "./dashboard/dist"
		if _, err := os.Stat(dashboardDir + "/index.html"); os.IsNotExist(err) {
			dashboardDir = ""
		}
	}
	if _, err := os.Stat(dashboardDir + "/index.html"); os.IsNotExist(err) {
		dashboardDir = ""
	}

	// Wire webhook + message service into session service
	sessionService.SetWebhookService(webhookService)
	sessionService.SetMessageService(messageService)

	// Setup router
	r := router.Setup(
		sessionHandler,
		messageHandler,
		groupHandler,
		webhookHandler,
		authHandler,
		infraHandler,
		auditHandler,
		templateHandler,
		auth,
		rateLimiter,
		cfg.ServeDashboard && dashboardDir != "",
		dashboardDir,
	)

	// Auto-start sessions if configured
	if cfg.AutoStartSessions {
		sessions, err := sessionService.FindAll(0, 0)
		if err == nil {
			for _, s := range sessions {
				if s.Phone != nil && s.Status == models.SessionStatusDisconnected {
					go func(id string) {
						if _, err := sessionService.Start(id); err != nil {
							log.Printf("Auto-start failed for session %s: %v", id, err)
						}
					}(s.ID)
					time.Sleep(2 * time.Second)
				}
			}
		}
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("OpenWA-Go starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDataDB(cfg *config.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	if cfg.DatabaseType == "postgres" {
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseUser, cfg.DatabasePassword, cfg.DatabaseName)
		return gorm.Open(postgres.Open(dsn), gormCfg)
	}

	// SQLite
	return gorm.Open(sqlite.Open(cfg.SQLitePath), gormCfg)
}

func initMainDB(cfg *config.Config) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(cfg.MainDBPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

func ensureAdminKey(db *gorm.DB, cfg *config.Config) {
	var count int64
	db.Model(&models.ApiKey{}).Count(&count)
	if count > 0 {
		return
	}

	if cfg.APIMasterKey != "" {
		hash := sha256Hash(cfg.APIMasterKey, cfg.APIKeyPepper)
		key := models.ApiKey{
			Name:    "Default Admin Key",
			Role:    models.ApiKeyRoleAdmin,
			KeyHash: hash,
			Enabled: true,
		}
		if err := db.Create(&key).Error; err != nil {
			log.Printf("Warning: Failed to create default admin key: %v", err)
		} else {
			log.Printf("Created default admin API key")
		}
	}
}

func sha256Hash(key, pepper string) string {
	h := sha256.Sum256([]byte(key + pepper))
	return hex.EncodeToString(h[:])
}
