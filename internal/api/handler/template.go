package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/models"
	"gorm.io/gorm"
)

type TemplateHandler struct {
	db *gorm.DB
}

func NewTemplateHandler(db *gorm.DB) *TemplateHandler {
	return &TemplateHandler{db: db}
}

func (h *TemplateHandler) List(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var templates []models.Template
	if err := h.db.Where("session_id = ?", sessionID).Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, templates)
}

func (h *TemplateHandler) Get(c *gin.Context) {
	id := c.Param("id")
	var t models.Template
	if err := h.db.First(&t, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *TemplateHandler) Create(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var req struct {
		Name   string `json:"name" binding:"required"`
		Body   string `json:"body" binding:"required"`
		Params string `json:"params,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t := models.Template{
		SessionID: sessionID,
		Name:      req.Name,
		Body:      req.Body,
		Params:    &req.Params,
	}
	if err := h.db.Create(&t).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *TemplateHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var t models.Template
	if err := h.db.First(&t, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	var req struct {
		Name   *string `json:"name,omitempty"`
		Body   *string `json:"body,omitempty"`
		Params *string `json:"params,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Body != nil {
		updates["body"] = *req.Body
	}
	if req.Params != nil {
		updates["params"] = *req.Params
	}
	if len(updates) > 0 {
		h.db.Model(&t).Updates(updates)
	}
	h.db.First(&t, "id = ?", id)
	c.JSON(http.StatusOK, t)
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Delete(&models.Template{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Template deleted"})
}
