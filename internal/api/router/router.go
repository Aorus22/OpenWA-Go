package router

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/api/handler"
	"github.com/openwa/openwa-go/internal/api/middleware"
)

func Setup(
	sessionHandler *handler.SessionHandler,
	messageHandler *handler.MessageHandler,
	groupHandler *handler.GroupHandler,
	webhookHandler *handler.WebhookHandler,
	authHandler *handler.AuthHandler,
	infraHandler *handler.InfraHandler,
	auditHandler *handler.AuditHandler,
	templateHandler *handler.TemplateHandler,
	auth *middleware.AuthMiddleware,
	rateLimiter *middleware.RateLimiter,
	serveDashboard bool,
	dashboardDir string,
) *gin.Engine {
	r := gin.Default()
	r.Use(gin.Recovery())

	// Public (no auth)
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/api/health/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	r.GET("/api/infra/health", infraHandler.Health)

	// Protected API
	api := r.Group("/api")
	api.Use(auth.RequireAuth())
	if rateLimiter != nil {
		api.Use(rateLimiter.Limit())
	}

	// ── Auth ──
	api.POST("/auth/validate", func(c *gin.Context) {
		roleStr, _ := c.Get("apiKeyRole")
		c.JSON(200, gin.H{"valid": true, "role": roleStr})
	})
	ak := api.Group("/auth/api-keys")
	ak.GET("", authHandler.List)
	ak.GET("/:keyId", authHandler.List) // fallback to list 
	ak.POST("", authHandler.Create)
	ak.PUT("/:keyId", authHandler.Update)
	ak.DELETE("/:keyId", authHandler.Delete)
	ak.POST("/:keyId/revoke", authHandler.Toggle)

	// ── Audit ──
	api.GET("/audit", auditHandler.List)

	// ── Infrastructure ──
	api.GET("/infra/status", infraHandler.Status)
	api.GET("/infra/engines", infraHandler.Engines)
	api.GET("/infra/engines/current", infraHandler.CurrentEngine)
	api.GET("/infra/config", infraHandler.Status)    // simplified
	api.PUT("/infra/config", infraHandler.Status)     // simplified

	// ── Settings ──
	api.GET("/settings", infraHandler.Status)
	api.PUT("/settings", infraHandler.Status)

	// ── Stats ──
	api.GET("/stats/overview", infraHandler.Status)
	api.GET("/stats/messages", infraHandler.Status)

	// ── Plugins (stub) ──
	api.GET("/plugins", func(c *gin.Context) { c.JSON(200, []gin.H{}) })
	api.GET("/plugins/catalog", func(c *gin.Context) { c.JSON(200, []gin.H{}) })

	// ── Sessions ──
	ss := api.Group("/sessions")
	ss.GET("", sessionHandler.FindAll)
	ss.POST("", sessionHandler.Create)
	ss.GET("/:sessionId", sessionHandler.FindOne)
	ss.DELETE("/:sessionId", sessionHandler.Delete)
	ss.POST("/:sessionId/start", sessionHandler.Start)
	ss.POST("/:sessionId/stop", sessionHandler.Stop)
	ss.POST("/:sessionId/force-kill", messageHandler.ForceKill)
	ss.GET("/:sessionId/qr", sessionHandler.GetQR)
	ss.POST("/:sessionId/pairing-code", sessionHandler.RequestPairingCode)
	ss.GET("/:sessionId/chats", messageHandler.GetChats)
	ss.POST("/:sessionId/chats/read", messageHandler.MarkRead)
	ss.POST("/:sessionId/chats/unread", messageHandler.MarkUnread)
	ss.POST("/:sessionId/chats/delete", messageHandler.DeleteChat)
	ss.POST("/:sessionId/chats/typing", messageHandler.SendTyping)
	ss.GET("/:sessionId/contacts", messageHandler.GetContacts)
	ss.GET("/:sessionId/contacts/check/:number", messageHandler.CheckNumber)
	ss.GET("/:sessionId/contacts/:contactId", messageHandler.GetContactByID)
	ss.GET("/:sessionId/contacts/:contactId/phone", messageHandler.ResolvePhone)
	ss.GET("/:sessionId/contacts/:contactId/profile-picture", messageHandler.GetProfilePic)
	ss.POST("/:sessionId/contacts/:contactId/block", messageHandler.BlockContact)
	ss.DELETE("/:sessionId/contacts/:contactId/block", messageHandler.UnblockContact)
	ss.GET("/:sessionId/groups", groupHandler.List)
	ss.GET("/:sessionId/groups/:groupId", groupHandler.GetInfo)
	ss.POST("/:sessionId/groups", groupHandler.Create)
	ss.POST("/:sessionId/groups/:groupId/participants", groupHandler.AddParticipants)
	ss.DELETE("/:sessionId/groups/:groupId/participants", groupHandler.RemoveParticipants)
	ss.POST("/:sessionId/groups/:groupId/participants/promote", groupHandler.PromoteParticipants)
	ss.POST("/:sessionId/groups/:groupId/participants/demote", groupHandler.DemoteParticipants)
	ss.POST("/:sessionId/groups/:groupId/leave", groupHandler.Leave)
	ss.PUT("/:sessionId/groups/:groupId/subject", groupHandler.SetSubject)
	ss.PUT("/:sessionId/groups/:groupId/description", groupHandler.SetDescription)
	ss.GET("/:sessionId/groups/:groupId/invite-code", groupHandler.GetInviteCode)
	ss.POST("/:sessionId/groups/:groupId/invite-code/revoke", groupHandler.RevokeInviteCode)

	// ── Messages ──
	msgs := ss.Group("/:sessionId/messages")
	msgs.POST("/send-text", messageHandler.SendText)
	msgs.POST("/send-image", messageHandler.SendImage)
	msgs.POST("/send-video", messageHandler.SendVideo)
	msgs.POST("/send-audio", messageHandler.SendAudio)
	msgs.POST("/send-document", messageHandler.SendDocument)
	msgs.POST("/send-location", messageHandler.SendLocation)
	msgs.POST("/send-contact", messageHandler.SendContact)
	msgs.POST("/send-sticker", messageHandler.SendSticker)
	msgs.POST("/reply", messageHandler.Reply)
	msgs.POST("/forward", messageHandler.Forward)
	msgs.POST("/react", messageHandler.React)
	msgs.POST("/delete", messageHandler.Delete)
	msgs.GET("", messageHandler.GetMessages)
	msgs.GET("/:chatId/history", messageHandler.GetChatHistory)

	// ── Webhooks ──
	wh := ss.Group("/:sessionId/webhooks")
	wh.GET("", webhookHandler.List)
	wh.POST("", webhookHandler.Create)
	wh.PUT("/:webhookId", webhookHandler.Update)
	wh.DELETE("/:webhookId", webhookHandler.Delete)

	// ── Templates ──
	tpl := ss.Group("/:sessionId/templates")
	tpl.GET("", templateHandler.List)
	tpl.GET("/:id", templateHandler.Get)
	tpl.POST("", templateHandler.Create)
	tpl.PUT("/:id", templateHandler.Update)
	tpl.DELETE("/:id", templateHandler.Delete)

	// ── Dashboard SPA ──
	if serveDashboard && dashboardDir != "" {
		r.Use(func(c *gin.Context) {
			if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
				fullPath := dashboardDir + c.Request.URL.Path
				if _, err := os.Stat(fullPath); err == nil {
					c.File(fullPath)
					c.Abort()
					return
				}
				c.File(dashboardDir + "/index.html")
				c.Abort()
				return
			}
			c.Next()
		})
	}

	return r
}
