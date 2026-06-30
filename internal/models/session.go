package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionStatus string

const (
	SessionStatusCreated        SessionStatus = "created"
	SessionStatusDisconnected   SessionStatus = "disconnected"
	SessionStatusInitializing   SessionStatus = "initializing"
	SessionStatusQRReady        SessionStatus = "qr_ready"
	SessionStatusAuthenticating SessionStatus = "authenticating"
	SessionStatusReady          SessionStatus = "ready"
	SessionStatusFailed         SessionStatus = "failed"
)

type Session struct {
	ID           string        `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name         string        `gorm:"uniqueIndex;not null;type:varchar(255)" json:"name"`
	Phone        *string       `gorm:"type:varchar(20)" json:"phone,omitempty"`
	PushName     *string       `gorm:"type:varchar(255)" json:"pushName,omitempty"`
	Status       SessionStatus `gorm:"not null;default:created;type:varchar(20)" json:"status"`
	Config       *string       `gorm:"type:text" json:"config,omitempty"`
	ProxyURL     *string       `gorm:"type:varchar(512)" json:"proxyUrl,omitempty"`
	ProxyType    *string       `gorm:"type:varchar(10)" json:"proxyType,omitempty"`
	LastError    *string       `gorm:"-" json:"lastError,omitempty"`
	ConnectedAt  *time.Time    `json:"connectedAt,omitempty"`
	LastActiveAt *time.Time    `json:"lastActiveAt,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}
