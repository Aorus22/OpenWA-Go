package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	SessionID *string   `gorm:"type:varchar(36)" json:"sessionId,omitempty"`
	Action    string    `gorm:"not null;type:varchar(255)" json:"action"`
	Resource  string    `gorm:"type:varchar(255)" json:"resource"`
	Details   *string   `gorm:"type:text" json:"details,omitempty"`
	IP        string    `gorm:"type:varchar(45)" json:"ip,omitempty"`
	ApiKeyID  *string   `gorm:"type:varchar(36)" json:"apiKeyId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
