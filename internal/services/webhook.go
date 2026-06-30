package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/openwa/openwa-go/internal/models"
	"gorm.io/gorm"
)

// WebhookService manages webhook endpoints and dispatches events.
type WebhookService struct {
	db         *gorm.DB
	httpClient *http.Client
	mu         sync.RWMutex

	ssrfProtect bool
	allowedHosts []string
	timeout     time.Duration
	maxRetries  int
	retryDelay  time.Duration
}

func NewWebhookService(db *gorm.DB, timeout, maxRetries, retryDelay int, ssrfProtect bool, allowedHosts []string) *WebhookService {
	return &WebhookService{
		db: db,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
			Transport: &http.Transport{
				MaxIdleConns:    20,
				IdleConnTimeout: 30 * time.Second,
			},
		},
		ssrfProtect: ssrfProtect,
		allowedHosts: allowedHosts,
		timeout:     time.Duration(timeout) * time.Millisecond,
		maxRetries:  maxRetries,
		retryDelay:  time.Duration(retryDelay) * time.Millisecond,
	}
}

// CreateWebhook creates a new webhook endpoint.
func (s *WebhookService) CreateWebhook(sessionID, url, events, secret string, enabled bool, filters string) (*models.Webhook, error) {
	if s.ssrfProtect {
		if err := s.validateURL(url); err != nil {
			return nil, err
		}
	}

	wh := &models.Webhook{
		SessionID: sessionID,
		URL:       url,
		Events:    events,
		Secret:    &secret,
		Enabled:   enabled,
		Filters:   &filters,
		Version:   "v2",
	}

	if err := s.db.Create(wh).Error; err != nil {
		return nil, err
	}

	return wh, nil
}

// GetWebhooks returns all webhooks for a session.
func (s *WebhookService) GetWebhooks(sessionID string) ([]models.Webhook, error) {
	var hooks []models.Webhook
	if err := s.db.Where("session_id = ?", sessionID).Find(&hooks).Error; err != nil {
		return nil, err
	}
	return hooks, nil
}

// ListAllWebhooks returns all webhooks across all sessions.
func (s *WebhookService) ListAllWebhooks() ([]models.Webhook, error) {
	var hooks []models.Webhook
	if err := s.db.Find(&hooks).Error; err != nil {
		return nil, err
	}
	return hooks, nil
}

// UpdateWebhook updates a webhook endpoint.
func (s *WebhookService) UpdateWebhook(id, url, events, secret string, enabled bool, filters string) (*models.Webhook, error) {
	if s.ssrfProtect && url != "" {
		if err := s.validateURL(url); err != nil {
			return nil, err
		}
	}

	updates := map[string]interface{}{}
	if url != "" {
		updates["url"] = url
	}
	if events != "" {
		updates["events"] = events
	}
	if secret != "" {
		updates["secret"] = secret
	}
	updates["enabled"] = enabled
	if filters != "" {
		updates["filters"] = filters
	}

	if err := s.db.Model(&models.Webhook{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}

	var wh models.Webhook
	if err := s.db.First(&wh, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &wh, nil
}

// DeleteWebhook deletes a webhook endpoint.
func (s *WebhookService) DeleteWebhook(id string) error {
	return s.db.Delete(&models.Webhook{}, "id = ?", id).Error
}

// Dispatch sends an event to all matching webhooks.
func (s *WebhookService) Dispatch(sessionID, event string, data interface{}) {
	hooks, err := s.GetWebhooks(sessionID)
	if err != nil || len(hooks) == 0 {
		return
	}

	payload := map[string]interface{}{
		"event":       event,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"sessionId":   sessionID,
		"data":        data,
	}

	for _, wh := range hooks {
		if !wh.Enabled {
			continue
		}

		// Filter by event type
		if wh.Events != "" && !s.matchesEvent(wh.Events, event) {
			continue
		}

		// Evaluate filters
		if wh.Filters != nil && *wh.Filters != "" {
			if !s.evaluateFilters(*wh.Filters, data) {
				continue
			}
		}

		// Dispatch in background
		go s.deliver(wh, payload)
	}
}

func (s *WebhookService) deliver(wh models.Webhook, payload map[string]interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	// Sign payload if secret is set
	var signature string
	if wh.Secret != nil && *wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(*wh.Secret))
		mac.Write(body)
		signature = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(s.retryDelay)
		}

		req, err := http.NewRequest("POST", wh.URL, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "OpenWA-Go/1.0")
		if signature != "" {
			req.Header.Set("X-Webhook-Signature", signature)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return // Success
		}
	}
}

func (s *WebhookService) matchesEvent(eventFilter, event string) bool {
	events := strings.Split(eventFilter, ",")
	for _, e := range events {
		if strings.TrimSpace(e) == event || strings.TrimSpace(e) == "*" {
			return true
		}
		// Support wildcard: message.* matches message.received, message.sent, etc.
		if strings.HasSuffix(strings.TrimSpace(e), ".*") {
			prefix := strings.TrimSuffix(strings.TrimSpace(e), ".*")
			if strings.HasPrefix(event, prefix) {
				return true
			}
		}
	}
	return false
}

func (s *WebhookService) evaluateFilters(filters string, data interface{}) bool {
	// Simple filter evaluation - can be extended
	return true
}

func (s *WebhookService) validateURL(rawURL string) error {
	// Basic URL validation
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("invalid webhook URL scheme")
	}

	// Check against allowed hosts
	if len(s.allowedHosts) > 0 {
		for _, h := range s.allowedHosts {
			if strings.Contains(rawURL, h) {
				return nil
			}
		}
	}

	// Block private/reserved IPs
	host := extractHost(rawURL)
	ip := net.ParseIP(host)
	if ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("webhook URL points to private IP (SSRF protection)")
		}
	} else {
		// Resolve hostname and check
		ips, err := net.LookupIP(host)
		if err == nil {
			for _, ip := range ips {
				if isPrivateIP(ip) {
					return fmt.Errorf("webhook URL resolves to private IP (SSRF protection)")
				}
			}
		}
	}

	return nil
}

func extractHost(rawURL string) string {
	// Remove scheme
	url := rawURL
	if idx := strings.Index(url, "://"); idx >= 0 {
		url = url[idx+3:]
	}
	// Remove path/port
	if idx := strings.IndexAny(url, "/:"); idx >= 0 {
		url = url[:idx]
	}
	return url
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}
	return false
}
