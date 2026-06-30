package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/models"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db     *gorm.DB
	pepper string
}

func NewAuthHandler(db *gorm.DB, pepper string) *AuthHandler {
	return &AuthHandler{db: db, pepper: pepper}
}

type createApiKeyRequest struct {
	Name            string          `json:"name" binding:"required"`
	Role            models.ApiKeyRole `json:"role" binding:"required"`
	AllowedIPs      []string        `json:"allowedIps,omitempty"`
	AllowedSessions []string        `json:"allowedSessions,omitempty"`
	ExpiresAt       *time.Time      `json:"expiresAt,omitempty"`
}

type updateApiKeyRequest struct {
	Name            *string         `json:"name,omitempty"`
	Role            *models.ApiKeyRole `json:"role,omitempty"`
	AllowedIPs      []string        `json:"allowedIps,omitempty"`
	AllowedSessions []string        `json:"allowedSessions,omitempty"`
	ExpiresAt       *time.Time      `json:"expiresAt,omitempty"`
}

func (h *AuthHandler) Create(c *gin.Context) {
	var req createApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate raw key
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate key"})
		return
	}
	rawKey := "openwa-" + hex.EncodeToString(rawBytes)[:40]

	// Hash
	hash := hashKey(rawKey, h.pepper)
	prefix := rawKey[:12]

	allowedIPsJSON := ""
	if len(req.AllowedIPs) > 0 {
		b, _ := json.Marshal(req.AllowedIPs)
		allowedIPsJSON = string(b)
	}
	allowedSessionsJSON := ""
	if len(req.AllowedSessions) > 0 {
		b, _ := json.Marshal(req.AllowedSessions)
		allowedSessionsJSON = string(b)
	}

	key := models.ApiKey{
		Name:            req.Name,
		Role:            req.Role,
		KeyHash:         hash,
		KeyPrefix:       prefix,
		Enabled:         true,
		AllowedIPs:      &allowedIPsJSON,
		AllowedSessions: &allowedSessionsJSON,
		ExpiresAt:       req.ExpiresAt,
	}

	if err := h.db.Create(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        key.ID,
		"name":      key.Name,
		"keyPrefix": key.KeyPrefix,
		"rawKey":    rawKey,
		"role":      key.Role,
		"isActive":  key.Enabled,
		"createdAt": key.CreatedAt,
	})
}

func (h *AuthHandler) List(c *gin.Context) {
	var keys []models.ApiKey
	if err := h.db.Order("created_at DESC").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Mask sensitive fields
	type keyResponse struct {
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		KeyPrefix string          `json:"keyPrefix"`
		Role      models.ApiKeyRole `json:"role"`
		Enabled   bool            `json:"isActive"`
		CreatedAt time.Time       `json:"createdAt"`
		LastUsed  *time.Time      `json:"lastUsedAt,omitempty"`
		ExpiresAt *time.Time      `json:"expiresAt,omitempty"`
	}

	result := make([]keyResponse, len(keys))
	for i, k := range keys {
		result[i] = keyResponse{
			ID:        k.ID,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			Role:      k.Role,
			Enabled:   k.Enabled,
			CreatedAt: k.CreatedAt,
			LastUsed:  k.LastUsedAt,
			ExpiresAt: k.ExpiresAt,
		}
	}

	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) Update(c *gin.Context) {
	id := c.Param("keyId")
	var key models.ApiKey
	if err := h.db.First(&key, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	}

	var req updateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.AllowedIPs != nil {
		b, _ := json.Marshal(req.AllowedIPs)
		updates["allowed_ips"] = string(b)
	}
	if req.AllowedSessions != nil {
		b, _ := json.Marshal(req.AllowedSessions)
		updates["allowed_sessions"] = string(b)
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = req.ExpiresAt
	}

	if err := h.db.Model(&key).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Key updated"})
}

func (h *AuthHandler) Delete(c *gin.Context) {
	id := c.Param("keyId")
	if err := h.db.Delete(&models.ApiKey{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Key deleted"})
}

func (h *AuthHandler) Toggle(c *gin.Context) {
	id := c.Param("keyId")
	var key models.ApiKey
	if err := h.db.First(&key, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	}

	newEnabled := !key.Enabled
	if err := h.db.Model(&key).Update("enabled", newEnabled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"isActive": newEnabled})
}

func hashKey(key, pepper string) string {
	input := key + pepper
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}
