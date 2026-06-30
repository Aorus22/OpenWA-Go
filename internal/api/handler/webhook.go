package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/services"
)

type WebhookHandler struct {
	webhookService *services.WebhookService
}

func NewWebhookHandler(svc *services.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: svc}
}

type createWebhookRequest struct {
	URL     string `json:"url" binding:"required"`
	Events  string `json:"events,omitempty"`
	Secret  string `json:"secret,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
	Filters string `json:"filters,omitempty"`
}

type updateWebhookRequest struct {
	URL     string `json:"url,omitempty"`
	Events  string `json:"events,omitempty"`
	Secret  string `json:"secret,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
	Filters string `json:"filters,omitempty"`
}

func (h *WebhookHandler) List(c *gin.Context) {
	sessionID := c.Param("sessionId")
	hooks, err := h.webhookService.GetWebhooks(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, hooks)
}

func (h *WebhookHandler) Create(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req createWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wh, err := h.webhookService.CreateWebhook(sessionID, req.URL, req.Events, req.Secret, req.Enabled, req.Filters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, wh)
}

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

	wh, err := h.webhookService.UpdateWebhook(id, req.URL, req.Events, req.Secret, enabled, req.Filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, wh)
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	id := c.Param("webhookId")
	if err := h.webhookService.DeleteWebhook(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Webhook deleted"})
}
