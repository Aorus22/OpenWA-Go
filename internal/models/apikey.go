package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ApiKeyRole string

const (
	ApiKeyRoleAdmin    ApiKeyRole = "admin"
	ApiKeyRoleOperator ApiKeyRole = "operator"
	ApiKeyRoleMonitor  ApiKeyRole = "monitor"
)

type ApiKey struct {
	ID              string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name            string     `gorm:"not null;type:varchar(255)" json:"name"`
	KeyHash         string     `gorm:"uniqueIndex;not null;type:varchar(255)" json:"-"`
	KeyPrefix       string     `gorm:"type:varchar(10)" json:"keyPrefix"`
	Role            ApiKeyRole `gorm:"not null;default:admin;type:varchar(20)" json:"role"`
	AllowedIPs      *string    `gorm:"type:text" json:"allowedIps,omitempty"`
	AllowedSessions *string    `gorm:"type:text" json:"allowedSessions,omitempty"`
	Enabled         bool       `gorm:"not null;default:true" json:"enabled"`
	LastUsedAt      *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (k *ApiKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	return nil
}
