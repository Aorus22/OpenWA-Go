package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Template struct {
	ID        string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	SessionID string    `gorm:"index;not null;type:varchar(36)" json:"sessionId"`
	Name      string    `gorm:"not null;type:varchar(255)" json:"name"`
	Body      string    `gorm:"type:text" json:"body"`
	Params    *string   `gorm:"type:text" json:"params,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (t *Template) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}
