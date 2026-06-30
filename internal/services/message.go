package services

import (
	"time"

	"github.com/openwa/openwa-go/internal/engine"
	"github.com/openwa/openwa-go/internal/models"
	"gorm.io/gorm"
)

// MessageService manages message persistence and retrieval.
type MessageService struct {
	db *gorm.DB
}

func NewMessageService(db *gorm.DB) *MessageService {
	return &MessageService{db: db}
}

// PersistIncoming saves an incoming message to the database.
func (s *MessageService) PersistIncoming(sessionID string, msg engine.IncomingMessage) (*models.Message, error) {
	metadata := models.JSONMap{}
	if msg.Media != nil {
		metadata["media"] = msg.Media
	}
	if msg.QuotedMessage != nil {
		metadata["quotedMessage"] = msg.QuotedMessage
	}
	if msg.Call != nil {
		metadata["call"] = msg.Call
	}

	message := &models.Message{
		SessionID:   sessionID,
		WaMessageID: &msg.ID,
		ChatID:      msg.ChatID,
		From:        msg.From,
		To:          msg.To,
		Body:        msg.Body,
		Type:        string(msg.Type),
		Direction:   models.MessageDirectionIncoming,
		Timestamp:   msg.Timestamp,
		Status:      models.MessageStatusSent,
		Metadata:    &metadata,
	}

	if msg.Timestamp > 0 {
		message.CreatedAt = time.Unix(msg.Timestamp, 0)
	}

	// Use insert with OnConflict ignore for dedup
	err := s.db.Where("session_id = ? AND wa_message_id = ?", sessionID, msg.ID).
		FirstOrCreate(message).Error
	if err != nil {
		return nil, err
	}

	return message, nil
}

// PersistOutgoing saves an outgoing message to the database.
func (s *MessageService) PersistOutgoing(sessionID, chatID, to, body, msgType, waMsgID string, timestamp int64) (*models.Message, error) {
	message := &models.Message{
		SessionID:   sessionID,
		WaMessageID: &waMsgID,
		ChatID:      chatID,
		From:        "", // will be filled by engine callback
		To:          to,
		Body:        body,
		Type:        msgType,
		Direction:   models.MessageDirectionOutgoing,
		Timestamp:   timestamp,
		Status:      models.MessageStatusSent,
	}

	if err := s.db.Create(message).Error; err != nil {
		return nil, err
	}

	return message, nil
}

// GetMessages retrieves messages with filtering.
func (s *MessageService) GetMessages(sessionID string, opts GetMessagesOptions) ([]models.Message, error) {
	query := s.db.Where("session_id = ?", sessionID)

	if opts.ChatID != "" {
		query = query.Where("chat_id = ?", opts.ChatID)
	}
	if opts.From != "" {
		query = query.Where("(from = ? OR to = ?)", opts.From, opts.From)
	}
	if opts.Limit <= 0 || opts.Limit > 1000 {
		opts.Limit = 100
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}

	var messages []models.Message
	if err := query.Order("created_at DESC").Limit(opts.Limit).Offset(opts.Offset).Find(&messages).Error; err != nil {
		return nil, err
	}

	return messages, nil
}

// UpdateMessageStatus updates delivery status of a message.
func (s *MessageService) UpdateMessageStatus(sessionID, waMsgID string, status models.MessageStatus) error {
	return s.db.Model(&models.Message{}).
		Where("session_id = ? AND wa_message_id = ?", sessionID, waMsgID).
		Update("status", status).Error
}

// GetMessageByWaID finds a message by WhatsApp message ID.
func (s *MessageService) GetMessageByWaID(sessionID, waMsgID string) (*models.Message, error) {
	var msg models.Message
	if err := s.db.Where("session_id = ? AND wa_message_id = ?", sessionID, waMsgID).First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetChatMessages retrieves messages for a specific chat.
func (s *MessageService) GetChatMessages(sessionID, chatID string, limit, offset int) ([]models.Message, error) {
	return s.GetMessages(sessionID, GetMessagesOptions{
		ChatID: chatID,
		Limit:  limit,
		Offset: offset,
	})
}

// GetMessagesOptions defines filtering options for GetMessages.
type GetMessagesOptions struct {
	ChatID string
	From   string
	Limit  int
	Offset int
}

// PersistHistoryMessages saves a batch of history messages (pre-connection).
func (s *MessageService) PersistHistoryMessages(sessionID string, messages []engine.IncomingMessage) error {
	for _, msg := range messages {
		if msg.IsStatusBroadcast || msg.ID == "" || msg.ChatID == "" {
			continue
		}

		metadata := models.JSONMap{}
		if msg.Media != nil {
			metadata["media"] = msg.Media
		}

		message := &models.Message{
			SessionID:   sessionID,
			WaMessageID: &msg.ID,
			ChatID:      msg.ChatID,
			From:        msg.From,
			To:          msg.To,
			Body:        msg.Body,
			Type:        string(msg.Type),
			Direction:   models.MessageDirectionOutgoing,
			Timestamp:   msg.Timestamp,
			Status:      models.MessageStatusSent,
			Metadata:    &metadata,
		}
		if msg.FromMe {
			message.Direction = models.MessageDirectionOutgoing
		} else {
			message.Direction = models.MessageDirectionIncoming
		}
		if msg.Timestamp > 0 {
			message.CreatedAt = time.Unix(msg.Timestamp, 0)
		}

		// Ignore duplicate
		s.db.Where("session_id = ? AND wa_message_id = ?", sessionID, msg.ID).
			FirstOrCreate(message)
	}
	return nil
}
