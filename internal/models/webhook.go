package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Webhook struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	SessionID string    `gorm:"index;not null;type:varchar(36)" json:"sessionId"`
	URL       string    `gorm:"not null;type:varchar(1024)" json:"url"`
	Events    string    `gorm:"type:text" json:"events"`
	Secret    *string   `gorm:"type:varchar(255)" json:"secret,omitempty"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	Filters   *string   `gorm:"type:text" json:"filters,omitempty"`
	Version   string    `gorm:"type:varchar(10);default:'v2'" json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return nil
}
