package handler

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/engine"
	"github.com/openwa/openwa-go/internal/services"
)

// messageResponse wraps a MessageResult into the format the dashboard expects.
func messageResponse(result engine.MessageResult) gin.H {
	return gin.H{
		"messageId": result.ID,
		"id":        result.ID,
		"timestamp": result.Timestamp,
	}
}

// sendError logs the error and returns a JSON error response to the client.
func sendError(c *gin.Context, status int, err error) {
	msg := err.Error()
	// Map known WhatsApp error codes to user-friendly messages
	if strings.Contains(msg, "463") {
		msg = "Pesan ditolak WhatsApp. Kemungkinan nomor tujuan tidak valid, memblokir Anda, atau terkena kebijakan WhatsApp."
	} else if strings.Contains(msg, "429") {
		msg = "Terlalu banyak permintaan. Tunggu beberapa saat lalu coba lagi."
	} else if strings.Contains(msg, "401") {
		msg = "Sesi tidak terautentikasi. Scan ulang QR code."
	}
	c.JSON(status, gin.H{"error": msg})
}

type MessageHandler struct {
	sessionService *services.SessionService
}

func NewMessageHandler(svc *services.SessionService) *MessageHandler {
	return &MessageHandler{sessionService: svc}
}

// --- Request types matching dashboard API exactly ---

type sendTextRequest struct {
	ChatID   string   `json:"chatId" binding:"required"`
	Text     string   `json:"text" binding:"required"`
	Mentions []string `json:"mentions,omitempty"`
}

type sendMediaRequest struct {
	ChatID   string `json:"chatId" binding:"required"`
	Mimetype string `json:"mimetype,omitempty"`
	Data     string `json:"base64,omitempty"`
	URL      string `json:"url,omitempty"`
	Filename string `json:"filename,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

type sendLocationReq struct {
	ChatID      string  `json:"chatId" binding:"required"`
	Latitude    float64 `json:"latitude" binding:"required"`
	Longitude   float64 `json:"longitude" binding:"required"`
	Description string  `json:"description,omitempty"`
	Address     string  `json:"address,omitempty"`
}

type sendContactReq struct {
	ChatID string `json:"chatId" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Number string `json:"number" binding:"required"`
}

type replyRequest struct {
	ChatID          string `json:"chatId" binding:"required"`
	QuotedMessageID string `json:"quotedMessageId" binding:"required"`
	Text            string `json:"text" binding:"required"`
}

type reactRequest struct {
	ChatID    string `json:"chatId" binding:"required"`
	MessageID string `json:"messageId" binding:"required"`
	Emoji     string `json:"emoji" binding:"required"`
}

type forwardRequest struct {
	ChatID    string `json:"chatId" binding:"required"`
	FromChat  string `json:"fromChat" binding:"required"`
	MessageID string `json:"messageId" binding:"required"`
}

type deleteMsgReq struct {
	ChatID      string `json:"chatId" binding:"required"`
	MessageID   string `json:"messageId" binding:"required"`
	ForEveryone bool   `json:"forEveryone,omitempty"`
}

func (h *MessageHandler) getEngine(c *gin.Context) (engine.IWhatsAppEngine, error) {
	sessionID := c.Param("sessionId")
	return h.sessionService.GetEngine(sessionID)
}

// POST /sessions/:sessionId/messages/send-text
func (h *MessageHandler) SendText(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req sendTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := eng.SendTextMessage(req.ChatID, req.Text, req.Mentions)
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"messageId": result.ID,
		"id":        result.ID,
		"timestamp": result.Timestamp,
	})
}

// POST /sessions/:sessionId/messages/send-image etc
func (h *MessageHandler) SendImage(c *gin.Context)   { h.sendMedia(c, "image") }
func (h *MessageHandler) SendVideo(c *gin.Context)   { h.sendMedia(c, "video") }
func (h *MessageHandler) SendAudio(c *gin.Context)   { h.sendMedia(c, "audio") }
func (h *MessageHandler) SendDocument(c *gin.Context) { h.sendMedia(c, "document") }
func (h *MessageHandler) SendSticker(c *gin.Context)  { h.sendMedia(c, "sticker") }

func (h *MessageHandler) sendMedia(c *gin.Context, mediaType string) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req sendMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	media := engine.MediaInput{
		Mimetype: req.Mimetype,
		Filename: req.Filename,
		Caption:  req.Caption,
	}
	if req.Data != "" {
		data, err := base64.StdEncoding.DecodeString(req.Data)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64"})
			return
		}
		media.Data = data
	} else if req.URL != "" {
		media.URL = req.URL
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "base64 or url required"})
		return
	}

	var result engine.MessageResult
	switch mediaType {
	case "image":
		result, err = eng.SendImageMessage(req.ChatID, media)
	case "video":
		result, err = eng.SendVideoMessage(req.ChatID, media)
	case "audio":
		result, err = eng.SendAudioMessage(req.ChatID, media)
	case "document":
		result, err = eng.SendDocumentMessage(req.ChatID, media)
	case "sticker":
		result, err = eng.SendStickerMessage(req.ChatID, media)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown media type"})
		return
	}
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, messageResponse(result))
}

// POST /sessions/:sessionId/messages/send-location
func (h *MessageHandler) SendLocation(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req sendLocationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := eng.SendLocationMessage(req.ChatID, engine.LocationInput{
		Latitude: req.Latitude, Longitude: req.Longitude,
		Description: req.Description, Address: req.Address,
	})
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, messageResponse(result))
}

// POST /sessions/:sessionId/messages/send-contact
func (h *MessageHandler) SendContact(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req sendContactReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := eng.SendContactMessage(req.ChatID, engine.ContactCard{Name: req.Name, Number: req.Number})
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, messageResponse(result))
}

// POST /sessions/:sessionId/messages/reply
func (h *MessageHandler) Reply(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req replyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := eng.ReplyToMessage(req.ChatID, req.QuotedMessageID, req.Text)
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, messageResponse(result))
}

// POST /sessions/:sessionId/messages/react
func (h *MessageHandler) React(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req reactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := eng.ReactToMessage(req.ChatID, req.MessageID, req.Emoji); err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Reaction sent"})
}

// POST /sessions/:sessionId/messages/forward
func (h *MessageHandler) Forward(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req forwardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := eng.ForwardMessage(req.FromChat, req.ChatID, req.MessageID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, messageResponse(result))
}

// POST /sessions/:sessionId/messages/delete
func (h *MessageHandler) Delete(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req deleteMsgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := eng.DeleteMessage(req.ChatID, req.MessageID, req.ForEveryone); err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Message deleted"})
}

// GET /sessions/:sessionId/messages?chatId=...&limit=...
func (h *MessageHandler) GetMessages(c *gin.Context) {
	chatID := c.Query("chatId")
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 {
			limit = n
		}
	}

	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chatId query param required"})
		return
	}

	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	history, err := eng.GetChatHistory(chatID, limit, false)
	if err != nil {
		// Return empty list if engine doesn't support history
		c.JSON(http.StatusOK, gin.H{"messages": []engine.IncomingMessage{}, "total": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": history, "total": len(history)})
}

// GET /sessions/:sessionId/messages/:chatId/history
func (h *MessageHandler) GetChatHistory(c *gin.Context) {
	chatID := c.Param("chatId")
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 {
			limit = n
		}
	}
	includeMedia := c.Query("includeMedia") == "true"

	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	msgs, err := eng.GetChatHistory(chatID, limit, includeMedia)
	if err != nil {
		c.JSON(http.StatusOK, []engine.IncomingMessage{})
		return
	}
	if msgs == nil {
		msgs = []engine.IncomingMessage{}
	}
	c.JSON(http.StatusOK, msgs)
}

// GET /sessions/:sessionId/chats
func (h *MessageHandler) GetChats(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	chats, err := eng.GetChats()
	if err != nil {
		// Return empty if not supported
		c.JSON(http.StatusOK, []engine.ChatSummary{})
		return
	}
	c.JSON(http.StatusOK, chats)
}

// POST /sessions/:sessionId/chats/read
func (h *MessageHandler) MarkRead(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req struct {
		ChatID string `json:"chatId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ok, err := eng.SendSeen(req.ChatID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": ok})
}

// POST /sessions/:sessionId/chats/unread
func (h *MessageHandler) MarkUnread(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req struct {
		ChatID string `json:"chatId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ok, err := eng.MarkUnread(req.ChatID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": ok})
}

// POST /sessions/:sessionId/chats/delete
func (h *MessageHandler) DeleteChat(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req struct {
		ChatID string `json:"chatId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ok, err := eng.DeleteChat(req.ChatID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": ok})
}

// POST /sessions/:sessionId/chats/typing
func (h *MessageHandler) SendTyping(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	var req struct {
		ChatID string `json:"chatId" binding:"required"`
		State  string `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	state := engine.ChatStateTyping
	if req.State == "recording" {
		state = engine.ChatStateRecording
	} else if req.State == "paused" {
		state = engine.ChatStatePaused
	}
	if err := eng.SendChatState(req.ChatID, state); err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// POST /sessions/:sessionId/force-kill
func (h *MessageHandler) ForceKill(c *gin.Context) {
	sessionID := c.Param("sessionId")
	eng, err := h.sessionService.GetEngine(sessionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	if err := eng.ForceDestroy(); err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	// Clean up in session service
	h.sessionService.Stop(sessionID)
	c.JSON(http.StatusOK, gin.H{"message": "Session force-killed"})
}

// --- Contact endpoints ---

func (h *MessageHandler) GetContacts(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	contacts, err := eng.GetContacts()
	if err != nil {
		c.JSON(http.StatusOK, []engine.Contact{})
		return
	}
	c.JSON(http.StatusOK, contacts)
}

func (h *MessageHandler) GetContactByID(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	contact, err := eng.GetContactByID(c.Param("contactId"))
	if err != nil || contact == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
		return
	}
	c.JSON(http.StatusOK, contact)
}

// GET /sessions/:sessionId/contacts/check/:number
func (h *MessageHandler) CheckNumber(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not started"})
		return
	}
	number := c.Param("number")
	exists, err := eng.CheckNumberExists(number)
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	result := gin.H{"number": number, "exists": exists}
	if exists {
		if id, _ := eng.GetNumberID(number); id != nil {
			result["whatsappId"] = *id
		}
	}
	c.JSON(http.StatusOK, result)
}

func (h *MessageHandler) ResolvePhone(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	phone, err := eng.ResolveContactPhone(c.Param("contactId"))
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"phone": phone})
}

func (h *MessageHandler) GetProfilePic(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pic, err := eng.GetProfilePicture(c.Param("contactId"))
	if err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": pic})
}

// POST /sessions/:sessionId/contacts/:contactId/block
func (h *MessageHandler) BlockContact(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := eng.BlockContact(c.Param("contactId")); err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Contact blocked"})
}

// DELETE /sessions/:sessionId/contacts/:contactId/block (unblock)
func (h *MessageHandler) UnblockContact(c *gin.Context) {
	eng, err := h.getEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := eng.UnblockContact(c.Param("contactId")); err != nil {
		sendError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Contact unblocked"})
}
