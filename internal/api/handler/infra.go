package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/models"
	"gorm.io/gorm"
)

type InfraHandler struct {
	dataDB *gorm.DB
	mainDB *gorm.DB
}

func NewInfraHandler(dataDB, mainDB *gorm.DB) *InfraHandler {
	return &InfraHandler{dataDB: dataDB, mainDB: mainDB}
}

// Health returns detailed health check.
func (h *InfraHandler) Health(c *gin.Context) {
	dataOK := h.dataDB.Raw("SELECT 1").Error == nil
	mainOK := h.mainDB.Raw("SELECT 1").Error == nil

	statusCode := http.StatusOK
	if !dataOK || !mainOK {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status": "ok",
		"databases": gin.H{
			"data": dataOK,
			"main": mainOK,
		},
	})
}

// Status returns aggregate infrastructure info.
func (h *InfraHandler) Status(c *gin.Context) {
	var sessionCount, msgCount int64
	h.dataDB.Model(&models.Session{}).Count(&sessionCount)
	h.dataDB.Model(&models.Message{}).Count(&msgCount)

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessionCount,
		"messages": msgCount,
		"database": gin.H{"type": "sqlite"},
	})
}

// Engines lists available WhatsApp engines.
func (h *InfraHandler) Engines(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{
			"id":      "whatsmeow",
			"name":    "WhatsMeow Engine (Go)",
			"enabled": true,
			"library": gin.H{"name": "whatsmeow", "version": "0.0.0"},
			"features": []string{"text", "image", "video", "audio", "document", "sticker",
				"location", "contact", "reply", "reaction", "group", "contacts"},
		},
	})
}

// CurrentEngine returns the active engine name.
func (h *InfraHandler) CurrentEngine(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"engine": "whatsmeow"})
}
