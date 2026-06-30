package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/services"
)

type SessionHandler struct {
	sessionService *services.SessionService
}

func NewSessionHandler(svc *services.SessionService) *SessionHandler {
	return &SessionHandler{sessionService: svc}
}

type createSessionRequest struct {
	Name      string                 `json:"name" binding:"required"`
	Config    map[string]interface{} `json:"config,omitempty"`
	ProxyURL  string                 `json:"proxyUrl,omitempty"`
	ProxyType string                 `json:"proxyType,omitempty"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.sessionService.Create(req.Name, req.Config, req.ProxyURL, req.ProxyType)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

func (h *SessionHandler) FindAll(c *gin.Context) {
	limit := 0
	offset := 0
	// Parse limit/offset from query params
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := parseInt(o); err == nil {
			offset = parsed
		}
	}

	sessions, err := h.sessionService.FindAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

func (h *SessionHandler) FindOne(c *gin.Context) {
	id := 	c.Param("sessionId")
	session, err := h.sessionService.FindOne(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Attach QR code if engine is running
	if eng, err := h.sessionService.GetEngine(id); err == nil {
		if qr := eng.GetQRCode(); qr != nil {
			c.JSON(http.StatusOK, gin.H{
				"session": session,
				"qrCode":  *qr,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"session": session})
}

func (h *SessionHandler) Delete(c *gin.Context) {
	id := 	c.Param("sessionId")
	if err := h.sessionService.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

func (h *SessionHandler) Start(c *gin.Context) {
	id := 	c.Param("sessionId")
	session, err := h.sessionService.Start(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session": session})
}

func (h *SessionHandler) Stop(c *gin.Context) {
	id := 	c.Param("sessionId")
	session, err := h.sessionService.Stop(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"session": session})
}

func (h *SessionHandler) GetQR(c *gin.Context) {
	id := 	c.Param("sessionId")
	eng, err := h.sessionService.GetEngine(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}

	qr := eng.GetQRCode()
	if qr == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "QR code not available"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"qrCode": *qr})
}

type pairingCodeRequest struct {
	PhoneNumber string `json:"phoneNumber" binding:"required"`
}

func (h *SessionHandler) RequestPairingCode(c *gin.Context) {
	id := 	c.Param("sessionId")
	var req pairingCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	eng, err := h.sessionService.GetEngine(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}

	code, err := eng.RequestPairingCode(req.PhoneNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pairingCode": code})
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
