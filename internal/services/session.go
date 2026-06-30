package services

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/openwa/openwa-go/internal/config"
	"github.com/openwa/openwa-go/internal/engine"
	"github.com/openwa/openwa-go/internal/models"
	"gorm.io/gorm"
)

// SessionService manages WhatsApp engine sessions.
type SessionService struct {
	db      *gorm.DB
	config  *config.Config
	factory *EngineFactory

	mu                    sync.RWMutex
	engines               map[string]engine.IWhatsAppEngine
	reconnectStates       map[string]*reconnectState
	stoppingSessions      map[string]bool
	initializingSessions  map[string]bool
	sessionErrors         map[string]string

	webhookService *WebhookService
	messageService *MessageService
}

func (s *SessionService) SetWebhookService(ws *WebhookService) {
	s.webhookService = ws
}

func (s *SessionService) SetMessageService(ms *MessageService) {
	s.messageService = ms
}

type reconnectState struct {
	attempts    int
	maxAttempts int
	baseDelay   time.Duration
}

// EngineFactory creates engine instances.
type EngineFactory struct {
	cfg *config.Config
}

func NewEngineFactory(cfg *config.Config) *EngineFactory {
	return &EngineFactory{cfg: cfg}
}

func (f *EngineFactory) Create(sessionID, proxyURL, proxyType string) engine.IWhatsAppEngine {
	return engine.NewWhatsmeowAdapter(engine.WhatsmeowConfig{
		SessionID:       sessionID,
		AuthDir:         f.cfg.AuthDir,
		SyncFullHistory: f.cfg.SyncFullHistory,
		LogLevel:        f.cfg.LogLevel,
		ProxyURL:        proxyURL,
	})
}

func NewSessionService(db *gorm.DB, cfg *config.Config, factory *EngineFactory) *SessionService {
	return &SessionService{
		db:               db,
		config:           cfg,
		factory:          factory,
		engines:          make(map[string]engine.IWhatsAppEngine),
		reconnectStates:  make(map[string]*reconnectState),
		stoppingSessions: make(map[string]bool),
		initializingSessions: make(map[string]bool),
		sessionErrors:    make(map[string]string),
	}
}

// Create creates a new session record.
func (s *SessionService) Create(name string, configData map[string]interface{}, proxyURL, proxyType string) (*models.Session, error) {
	var cfgStr *string
	if configData != nil {
		b, _ := json.Marshal(configData)
		str := string(b)
		cfgStr = &str
	}

	session := &models.Session{
		Name:      name,
		Status:    models.SessionStatusCreated,
		Config:    cfgStr,
		ProxyURL:  &proxyURL,
		ProxyType: &proxyType,
	}

	if err := s.db.Create(session).Error; err != nil {
		return nil, err
	}

	return session, nil
}

// FindAll returns all sessions.
func (s *SessionService) FindAll(limit, offset int) ([]models.Session, error) {
	var sessions []models.Session
	query := s.db.Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&sessions).Error; err != nil {
		return nil, err
	}
	// Attach transient last error
	for i := range sessions {
		if sessions[i].Status == models.SessionStatusFailed {
			s.mu.RLock()
			errStr := s.sessionErrors[sessions[i].ID]
			s.mu.RUnlock()
			sessions[i].LastError = &errStr
		}
	}
	return sessions, nil
}

// FindOne returns a single session by ID.
func (s *SessionService) FindOne(id string) (*models.Session, error) {
	var session models.Session
	if err := s.db.First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if session.Status == models.SessionStatusFailed {
		s.mu.RLock()
		errStr := s.sessionErrors[session.ID]
		s.mu.RUnlock()
		session.LastError = &errStr
	}
	return &session, nil
}

// FindByName returns a session by name.
func (s *SessionService) FindByName(name string) (*models.Session, error) {
	var session models.Session
	if err := s.db.First(&session, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// Start initializes the WhatsApp engine for a session.
func (s *SessionService) Start(id string) (*models.Session, error) {
	session, err := s.FindOne(id)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if _, exists := s.engines[id]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("session is already started")
	}
	if s.initializingSessions[id] {
		s.mu.Unlock()
		return nil, fmt.Errorf("session is already starting")
	}

	// Check max concurrent sessions
	if s.config.MaxConcurrentSessions > 0 {
		activeCount := len(s.engines)
		if s.initializingSessions[id] {
			activeCount++ // Count ourselves
		}
		if activeCount >= s.config.MaxConcurrentSessions {
			s.mu.Unlock()
			return nil, fmt.Errorf("maximum concurrent sessions reached (%d)", s.config.MaxConcurrentSessions)
		}
	}

	s.initializingSessions[id] = true
	s.stoppingSessions[id] = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.initializingSessions, id)
		s.mu.Unlock()
	}()

	// Setup reconnect config
	reconnectCfg := parseReconnectConfig(session.Config)
	s.mu.Lock()
	s.reconnectStates[id] = &reconnectState{
		maxAttempts: reconnectCfg.maxAttempts,
		baseDelay:   reconnectCfg.baseDelay,
	}
	s.mu.Unlock()

	// Determine proxy
	proxyURL := ""
	proxyType := ""
	if session.ProxyURL != nil {
		proxyURL = *session.ProxyURL
	}
	if session.ProxyType != nil {
		proxyType = *session.ProxyType
	}

	// Create engine
	eng := s.factory.Create(session.Name, proxyURL, proxyType)

	// Initialize engine
	err = s.initializeEngine(id, session, eng)
	if err != nil {
		return nil, err
	}

	return s.FindOne(id)
}

// Stop disconnects the engine but preserves session data.
func (s *SessionService) Stop(id string) (*models.Session, error) {
	if _, err := s.FindOne(id); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.stoppingSessions[id] = true
	s.cancelReconnectLocked(id)
	eng := s.engines[id]
	s.mu.Unlock()

	if eng != nil {
		_ = eng.Disconnect()
		s.mu.Lock()
		delete(s.engines, id)
		s.mu.Unlock()
	}

	s.updateStatus(id, models.SessionStatusDisconnected)
	return s.FindOne(id)
}

// Delete removes a session and stops its engine.
func (s *SessionService) Delete(id string) error {
	if _, err := s.FindOne(id); err != nil {
		return err
	}

	s.mu.Lock()
	s.stoppingSessions[id] = true
	s.cancelReconnectLocked(id)
	eng := s.engines[id]
	s.mu.Unlock()

	if eng != nil {
		_ = eng.Destroy()
		s.mu.Lock()
		delete(s.engines, id)
		s.mu.Unlock()
	}

	// Delete from DB
	return s.db.Delete(&models.Session{}, "id = ?", id).Error
}

// GetEngine returns the engine instance for a session.
func (s *SessionService) GetEngine(id string) (engine.IWhatsAppEngine, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	eng, exists := s.engines[id]
	if !exists {
		return nil, fmt.Errorf("session not started")
	}
	return eng, nil
}

// initializeEngine sets up callbacks and starts the engine.
func (s *SessionService) initializeEngine(id string, session *models.Session, eng engine.IWhatsAppEngine) error {
	s.mu.Lock()
	s.engines[id] = eng
	delete(s.sessionErrors, id)
	s.mu.Unlock()

	s.updateStatus(id, models.SessionStatusInitializing)

	err := eng.Initialize(engine.EngineEventCallbacks{
		OnQRCode: func(qr string) {
			s.updateStatus(id, models.SessionStatusQRReady)
		},
		OnReady: func(phone, pushName string) {
			s.mu.Lock()
			if state := s.reconnectStates[id]; state != nil {
				state.attempts = 0
			}
			delete(s.sessionErrors, id)
			s.mu.Unlock()

			s.db.Model(&models.Session{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":         models.SessionStatusReady,
				"phone":          phone,
				"push_name":      pushName,
				"connected_at":   time.Now(),
				"last_active_at": time.Now(),
			})
		},
		OnMessage: func(msg engine.IncomingMessage) {
			if msg.IsStatusBroadcast {
				return
			}
			// Persist message
			if s.messageService != nil {
				s.messageService.PersistIncoming(id, msg)
			}
			// Dispatch webhook
			if s.webhookService != nil {
				s.webhookService.Dispatch(id, "message.received", msg)
			}
		},
		OnMessageCreate: func(msg engine.IncomingMessage) {
			if !msg.FromMe || msg.IsStatusBroadcast {
				return
			}
			// Persist outgoing message
			if s.messageService != nil {
				s.messageService.PersistOutgoing(id, msg.ChatID, msg.To, msg.Body, string(msg.Type), msg.ID, msg.Timestamp)
			}
			// Dispatch webhook
			if s.webhookService != nil {
				s.webhookService.Dispatch(id, "message.sent", msg)
			}
		},
		OnMessageAck: func(messageID string, status engine.DeliveryStatus) {
			// Update message status in DB
			msgStatus := deliveryStatusToMessageStatus(status)
			if msgStatus != "" {
				s.db.Model(&models.Message{}).
					Where("session_id = ? AND wa_message_id = ?", id, messageID).
					Update("status", msgStatus)
			}
		},
		OnMessageRevoked: func(msg engine.RevokedMessage) {
			s.db.Model(&models.Message{}).
				Where("session_id = ? AND wa_message_id = ?", id, msg.ID).
				Updates(map[string]interface{}{
					"body": "",
					"type": "revoked",
				})
		},
		OnMessageReaction: func(event engine.ReactionEvent) {
			// Handle reaction
		},
		OnDisconnected: func(reason string) {
			s.updateStatus(id, models.SessionStatusDisconnected)
			s.scheduleReconnect(id, session)
		},
		OnStateChanged: func(state engine.EngineStatus) {
			statusMap := map[engine.EngineStatus]models.SessionStatus{
				engine.StatusDisconnected:   models.SessionStatusDisconnected,
				engine.StatusInitializing:   models.SessionStatusInitializing,
				engine.StatusQRReady:        models.SessionStatusQRReady,
				engine.StatusAuthenticating: models.SessionStatusAuthenticating,
				engine.StatusReady:          models.SessionStatusReady,
				engine.StatusFailed:         models.SessionStatusFailed,
			}
			if st, ok := statusMap[state]; ok {
				s.updateStatus(id, st)
			}
		},
		OnError: func(reason string) {
			s.mu.Lock()
			s.sessionErrors[id] = reason
			s.cancelReconnectLocked(id)
			s.mu.Unlock()
			s.updateStatus(id, models.SessionStatusFailed)
		},
	})

	return err
}

func (s *SessionService) updateStatus(id string, status models.SessionStatus) {
	s.db.Model(&models.Session{}).Where("id = ?", id).Update("status", status)
}

func (s *SessionService) scheduleReconnect(id string, session *models.Session) {
	s.mu.Lock()
	state, exists := s.reconnectStates[id]
	if !exists {
		s.mu.Unlock()
		return
	}

	if state.attempts >= state.maxAttempts {
		s.mu.Unlock()
		s.sessionErrors[id] = fmt.Sprintf("Reconnection failed after %d attempts — restart the session.", state.attempts)
		s.updateStatus(id, models.SessionStatusFailed)
		return
	}

	delay := time.Duration(float64(state.baseDelay) * math.Pow(2, float64(state.attempts)))
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	delay = delay + jitter
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	state.attempts++
	s.mu.Unlock()

	time.AfterFunc(delay, func() {
		s.mu.Lock()
		if s.stoppingSessions[id] {
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		// Teardown old engine
		s.mu.Lock()
		oldEng := s.engines[id]
		s.mu.Unlock()
		if oldEng != nil {
			_ = oldEng.Destroy()
		}
		s.mu.Lock()
		delete(s.engines, id)
		s.mu.Unlock()

		// Reinitialize
		proxyURL := ""
		proxyType := ""
		if session.ProxyURL != nil {
			proxyURL = *session.ProxyURL
		}
		if session.ProxyType != nil {
			proxyType = *session.ProxyType
		}
		eng := s.factory.Create(session.Name, proxyURL, proxyType)
		if err := s.initializeEngine(id, session, eng); err != nil {
			s.scheduleReconnect(id, session)
		}
	})
}

func (s *SessionService) cancelReconnectLocked(id string) {
	delete(s.reconnectStates, id)
}

// reconnectConfig holds parsed reconnect settings.
type reconnectConfig struct {
	maxAttempts int
	baseDelay   time.Duration
}

func parseReconnectConfig(configStr *string) reconnectConfig {
	cfg := reconnectConfig{
		maxAttempts: 5,
		baseDelay:   5 * time.Second,
	}
	if configStr == nil || *configStr == "" {
		return cfg
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*configStr), &data); err != nil {
		return cfg
	}

	if v, ok := data["maxReconnectAttempts"].(float64); ok {
		cfg.maxAttempts = clampInt(int(v), 0, 20)
	}
	if v, ok := data["reconnectBaseDelay"].(float64); ok {
		cfg.baseDelay = time.Duration(clampInt(int(v), 1000, 300000)) * time.Millisecond
	}

	return cfg
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func deliveryStatusToMessageStatus(ds engine.DeliveryStatus) models.MessageStatus {
	switch ds {
	case engine.DeliverySent:
		return models.MessageStatusSent
	case engine.DeliveryDelivered:
		return models.MessageStatusDelivered
	case engine.DeliveryRead:
		return models.MessageStatusRead
	case engine.DeliveryFailed:
		return models.MessageStatusFailed
	default:
		return ""
	}
}
