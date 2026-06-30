package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/openwa/openwa-go/internal/engine"
	"github.com/openwa/openwa-go/internal/services"
)

type GroupHandler struct {
	sessionService *services.SessionService
}

func NewGroupHandler(svc *services.SessionService) *GroupHandler {
	return &GroupHandler{sessionService: svc}
}

type createGroupRequest struct {
	Name         string   `json:"name" binding:"required"`
	Participants []string `json:"participants" binding:"required"`
}

type participantsRequest struct {
	Participants []string `json:"participants" binding:"required"`
}

type subjectRequest struct {
	Subject string `json:"subject" binding:"required"`
}

type descriptionRequest struct {
	Description string `json:"description" binding:"required"`
}

func (h *GroupHandler) List(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groups, err := eng.GetGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, groups)
}

func (h *GroupHandler) GetInfo(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	info, err := eng.GetGroupInfo(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if info == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	c.JSON(http.StatusOK, info)
}

func (h *GroupHandler) Create(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group, err := eng.CreateGroup(req.Name, req.Participants)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, group)
}

func (h *GroupHandler) AddParticipants(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req participantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	if err := eng.AddParticipants(groupID, req.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Participants added"})
}

func (h *GroupHandler) RemoveParticipants(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req participantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	if err := eng.RemoveParticipants(groupID, req.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Participants removed"})
}

func (h *GroupHandler) PromoteParticipants(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req participantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	if err := eng.PromoteParticipants(groupID, req.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Participants promoted"})
}

func (h *GroupHandler) DemoteParticipants(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req participantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	if err := eng.DemoteParticipants(groupID, req.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Participants demoted"})
}

func (h *GroupHandler) Leave(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	if err := eng.LeaveGroup(groupID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Left group"})
}

func (h *GroupHandler) SetSubject(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req subjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	if err := eng.SetGroupSubject(groupID, req.Subject); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subject updated"})
}

func (h *GroupHandler) SetDescription(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req descriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	if err := eng.SetGroupDescription(groupID, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Description updated"})
}

func (h *GroupHandler) GetInviteCode(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	code, err := eng.GetGroupInviteCode(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"inviteCode": code})
}

func (h *GroupHandler) RevokeInviteCode(c *gin.Context) {
	eng, err := h.getGroupEngine(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	groupID := c.Param("groupId")
	code, err := eng.RevokeGroupInviteCode(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"inviteCode": code})
}

func (h *GroupHandler) getGroupEngine(c *gin.Context) (engine.IWhatsAppEngine, error) {
	sessionID := c.Param("sessionId")
	return h.sessionService.GetEngine(sessionID)
}
