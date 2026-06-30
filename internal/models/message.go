package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageDirection string

const (
	MessageDirectionIncoming MessageDirection = "incoming"
	MessageDirectionOutgoing MessageDirection = "outgoing"
)

type MessageStatus string

const (
	MessageStatusPending   MessageStatus = "pending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

type JSONMap map[string]interface{}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSONMap - type is %T", value)
	}
	return json.Unmarshal(bytes, m)
}

type Message struct {
	ID          string           `gorm:"primaryKey;type:varchar(36)" json:"id"`
	SessionID   string           `gorm:"uniqueIndex:idx_session_msg;not null;type:varchar(36)" json:"sessionId"`
	WaMessageID *string          `gorm:"uniqueIndex:idx_session_msg;type:varchar(255)" json:"waMessageId,omitempty"`
	ChatID      string           `gorm:"not null;type:varchar(255)" json:"chatId"`
	From        string           `gorm:"not null;type:varchar(255)" json:"from"`
	To          string           `gorm:"not null;type:varchar(255)" json:"to"`
	Body        string           `gorm:"type:text" json:"body"`
	Type        string           `gorm:"type:varchar(50)" json:"type"`
	Direction   MessageDirection `gorm:"not null;type:varchar(10)" json:"direction"`
	Timestamp   int64            `gorm:"not null" json:"timestamp"`
	Status      MessageStatus    `gorm:"not null;default:sent;type:varchar(10)" json:"status"`
	Metadata    *JSONMap         `gorm:"type:text" json:"metadata,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
