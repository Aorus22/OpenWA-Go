package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/models"
	"gorm.io/gorm"
)

// AuthMiddleware provides API key authentication.
type AuthMiddleware struct {
	db      *gorm.DB
	pepper  string
	masterKey string
}

func NewAuthMiddleware(db *gorm.DB, pepper, masterKey string) *AuthMiddleware {
	return &AuthMiddleware{
		db:        db,
		pepper:    pepper,
		masterKey: masterKey,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			return
		}

	// Check master key
	if m.masterKey != "" && subtle.ConstantTimeCompare([]byte(apiKey), []byte(m.masterKey)) == 1 {
			c.Set("apiKeyID", "master")
			c.Set("apiKeyRole", models.ApiKeyRoleAdmin)
			c.Set("apiKeyName", "Master Key")
			c.Next()
			return
		}

		// Hash and lookup
		hash := m.hashKey(apiKey)
		var key models.ApiKey
		if err := m.db.Where("key_hash = ? AND enabled = ?", hash, true).First(&key).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			return
		}

		// Check expiry
		if key.ExpiresAt != nil && key.ExpiresAt.Before(timeNow()) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key expired"})
			return
		}

		// Check IP whitelist
		if key.AllowedIPs != nil && *key.AllowedIPs != "" {
			clientIP := c.ClientIP()
			if !m.isIPAllowed(clientIP, *key.AllowedIPs) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "IP not allowed"})
				return
			}
		}

		// Update last used
		m.db.Model(&key).Update("last_used_at", timeNow())

		c.Set("apiKeyID", key.ID)
		c.Set("apiKeyRole", key.Role)
		c.Set("apiKeyName", key.Name)
		c.Next()
	}
}

// RequireRole restricts access to specific roles.
func (m *AuthMiddleware) RequireRole(roles ...models.ApiKeyRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleStr, exists := c.Get("apiKeyRole")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		role := models.ApiKeyRole(roleStr.(string))
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
	}
}

func (m *AuthMiddleware) hashKey(key string) string {
	if m.pepper != "" {
		mac := hmac.New(sha256.New, []byte(m.pepper))
		mac.Write([]byte(key))
		return hex.EncodeToString(mac.Sum(nil))
	}
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func (m *AuthMiddleware) isIPAllowed(clientIP, allowedIPsStr string) bool {
	client := net.ParseIP(clientIP)
	if client == nil {
		return false
	}

	parts := strings.Split(allowedIPsStr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, "/") {
			_, cidr, err := net.ParseCIDR(p)
			if err == nil && cidr.Contains(client) {
				return true
			}
		} else if p == clientIP {
			return true
		}
	}
	return false
}

func timeNow() time.Time {
	return time.Now()
}
