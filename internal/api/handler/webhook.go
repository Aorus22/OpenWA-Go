package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/models"
	"github.com/openwa/openwa-go/internal/services"
)

type WebhookHandler struct {
	webhookService *services.WebhookService
}

func NewWebhookHandler(svc *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: svc}
}

type createWebhookRequest struct {
	URL      string   `json:"url" binding:"required"`
	Events   []string `json:"events,omitempty"`
	Secret   string   `json:"secret,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty"`
	Filters  string   `json:"filters,omitempty"`
	ChatIDs  []string `json:"chatIds,omitempty"`
}

type updateWebhookRequest struct {
	URL     string   `json:"url,omitempty"`
	Events  []string `json:"events,omitempty"`
	Secret  string   `json:"secret,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
	Filters string   `json:"filters,omitempty"`
}

// GET /api/webhooks
func (h *WebhookHandler) ListAll(c *gin.Context) {
	hooks, err := h.webhookService.ListAllWebhooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, len(hooks))
	for i, wh := range hooks {
		result[i] = webhookToMap(&wh)
	}
	c.JSON(http.StatusOK, result)
}

// GET /api/sessions/:sessionId/webhooks
func (h *WebhookHandler) List(c *gin.Context) {
	sessionID := c.Param("sessionId")
	hooks, err := h.webhookService.GetWebhooks(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	result := make([]gin.H, len(hooks))
	for i, wh := range hooks {
		result[i] = webhookToMap(&wh)
	}
	c.JSON(http.StatusOK, result)
}

// POST /api/sessions/:sessionId/webhooks
func (h *WebhookHandler) Create(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Convert events array to comma-separated
	eventsStr := ""
	if len(req.Events) > 0 {
		for _, e := range req.Events {
			if eventsStr != "" {
				eventsStr += ","
			}
			eventsStr += e
		}
	}

	wh, err := h.webhookService.CreateWebhook(sessionID, req.URL, eventsStr, req.Secret, enabled, req.Filters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, webhookToMap(wh))
}

// PUT /api/sessions/:sessionId/webhooks/:webhookId
func (h *WebhookHandler) Update(c *gin.Context) {
	id := c.Param("webhookId")
	var req updateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	eventsStr := ""
	if len(req.Events) > 0 {
		for _, e := range req.Events {
			if eventsStr != "" {
				eventsStr += ","
			}
			eventsStr += e
		}
	}

	wh, err := h.webhookService.UpdateWebhook(id, req.URL, eventsStr, req.Secret, enabled, req.Filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, webhookToMap(wh))
}

// DELETE /api/sessions/:sessionId/webhooks/:webhookId
func (h *WebhookHandler) Delete(c *gin.Context) {
	id := c.Param("webhookId")
	if err := h.webhookService.DeleteWebhook(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Webhook deleted"})
}

func webhookToMap(wh *models.Webhook) gin.H {
	// Parse events string to array
	events := []string{}
	if wh.Events != "" {
		// Split comma-separated
		current := ""
		for _, c := range wh.Events {
			if c == ',' {
				if current != "" {
					events = append(events, current)
				}
				current = ""
			} else {
				current += string(c)
			}
		}
		if current != "" {
			events = append(events, current)
		}
	}

	m := gin.H{
		"id":        wh.ID,
		"sessionId": wh.SessionID,
		"url":       wh.URL,
		"events":    events,
		"active":    wh.Enabled,
		"version":   wh.Version,
		"createdAt": wh.CreatedAt,
		"updatedAt": wh.UpdatedAt,
	}
	if wh.Secret != nil {
		m["secret"] = *wh.Secret
	}
	return m
}
