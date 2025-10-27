// ===============================
// internal/handlers/video_reactions_handler.go - Video Reactions HTTP Handler
// ===============================

package handlers

import (
	"net/http"
	"strconv"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
)

type VideoReactionsHandler struct {
	service *services.VideoReactionsService
}

func NewVideoReactionsHandler(service *services.VideoReactionsService) *VideoReactionsHandler {
	return &VideoReactionsHandler{
		service: service,
	}
}

// ===============================
// CHAT ENDPOINTS
// ===============================

// CreateVideoReactionChat creates a new chat from a video reaction
// POST /api/v1/video-reactions/chats
func (h *VideoReactionsHandler) CreateVideoReactionChat(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var request models.CreateVideoReactionChatRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	chat, err := h.service.CreateVideoReactionChat(
		c.Request.Context(),
		userID,
		request.VideoOwnerID,
		&request.VideoReaction,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Chat created successfully",
		"chat":    chat,
	})
}

// GetUserChats retrieves all chats for the authenticated user
// GET /api/v1/video-reactions/chats
func (h *VideoReactionsHandler) GetUserChats(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	chats, err := h.service.GetUserChats(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chats", "details": err.Error()})
		return
	}

	// Calculate unread count
	unreadCount := 0
	for _, chat := range chats {
		unreadCount += chat.GetUnreadCount(userID)
	}

	c.JSON(http.StatusOK, gin.H{
		"chats":       chats,
		"total":       len(chats),
		"hasMore":     len(chats) == limit,
		"unreadCount": unreadCount,
	})
}

// GetChatByID retrieves a specific chat by ID
// GET /api/v1/video-reactions/chats/:chatId
func (h *VideoReactionsHandler) GetChatByID(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	chat, err := h.service.GetChatByID(c.Request.Context(), chatID, userID)
	if err != nil {
		if err.Error() == "chat not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Chat not found"})
		} else if err.Error() == "access denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chat", "details": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, chat)
}

// MarkChatAsRead marks all messages in a chat as read
// POST /api/v1/video-reactions/chats/:chatId/read
func (h *VideoReactionsHandler) MarkChatAsRead(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	err := h.service.MarkChatAsRead(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark chat as read", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat marked as read"})
}

// ToggleChatPin toggles the pin status of a chat
// POST /api/v1/video-reactions/chats/:chatId/pin
func (h *VideoReactionsHandler) ToggleChatPin(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	err := h.service.ToggleChatPin(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle pin", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat pin toggled"})
}

// ToggleChatArchive toggles the archive status of a chat
// POST /api/v1/video-reactions/chats/:chatId/archive
func (h *VideoReactionsHandler) ToggleChatArchive(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	err := h.service.ToggleChatArchive(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle archive", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat archive toggled"})
}

// ToggleChatMute toggles the mute status of a chat
// POST /api/v1/video-reactions/chats/:chatId/mute
func (h *VideoReactionsHandler) ToggleChatMute(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	err := h.service.ToggleChatMute(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle mute", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat mute toggled"})
}

// UpdateChatSettings updates chat settings (wallpaper, font size)
// PUT /api/v1/video-reactions/chats/:chatId/settings
func (h *VideoReactionsHandler) UpdateChatSettings(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	var request models.ChatSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	err := h.service.UpdateChatSettings(c.Request.Context(), chatID, userID, request.Wallpaper, request.FontSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat settings updated"})
}

// DeleteChat deletes a chat
// DELETE /api/v1/video-reactions/chats/:chatId
func (h *VideoReactionsHandler) DeleteChat(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	deleteForEveryone := c.Query("deleteForEveryone") == "true"

	err := h.service.DeleteChat(c.Request.Context(), chatID, userID, deleteForEveryone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete chat", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat deleted successfully"})
}

// ClearChatHistory clears all messages in a chat
// POST /api/v1/video-reactions/chats/:chatId/clear
func (h *VideoReactionsHandler) ClearChatHistory(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	err := h.service.ClearChatHistory(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear history", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat history cleared"})
}

// ===============================
// MESSAGE ENDPOINTS
// ===============================

// SendMessage sends a new message in a chat
// POST /api/v1/video-reactions/chats/:chatId/messages
func (h *VideoReactionsHandler) SendMessage(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	var request models.SendMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Set default type if not provided
	if request.Type == "" {
		request.Type = models.MessageTypeText
	}

	message, err := h.service.SendMessage(c.Request.Context(), chatID, userID, &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Message sent successfully",
		"data":    message,
	})
}

// GetChatMessages retrieves messages from a chat
// GET /api/v1/video-reactions/chats/:chatId/messages
func (h *VideoReactionsHandler) GetChatMessages(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	response, err := h.service.GetChatMessages(c.Request.Context(), chatID, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// EditMessage edits a message
// PUT /api/v1/video-reactions/messages/:messageId
func (h *VideoReactionsHandler) EditMessage(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
		return
	}

	var request models.UpdateMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	err := h.service.EditMessage(c.Request.Context(), messageID, userID, request.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to edit message", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message edited successfully"})
}

// DeleteMessage deletes a message
// DELETE /api/v1/video-reactions/messages/:messageId
func (h *VideoReactionsHandler) DeleteMessage(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
		return
	}

	deleteForEveryone := c.Query("deleteForEveryone") == "true"

	err := h.service.DeleteMessage(c.Request.Context(), messageID, userID, deleteForEveryone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted successfully"})
}

// ToggleMessagePin toggles the pin status of a message
// POST /api/v1/video-reactions/messages/:messageId/pin
func (h *VideoReactionsHandler) ToggleMessagePin(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
		return
	}

	err := h.service.ToggleMessagePin(c.Request.Context(), messageID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle pin", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message pin toggled"})
}

// AddMessageReaction adds a reaction to a message
// POST /api/v1/video-reactions/messages/:messageId/reactions
func (h *VideoReactionsHandler) AddMessageReaction(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
		return
	}

	var request models.MessageReactionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	err := h.service.AddMessageReaction(c.Request.Context(), messageID, userID, request.Reaction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reaction", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reaction added"})
}

// RemoveMessageReaction removes a reaction from a message
// DELETE /api/v1/video-reactions/messages/:messageId/reactions
func (h *VideoReactionsHandler) RemoveMessageReaction(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
		return
	}

	err := h.service.RemoveMessageReaction(c.Request.Context(), messageID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove reaction", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reaction removed"})
}

// MarkMessageAsDelivered marks a message as delivered
// POST /api/v1/video-reactions/messages/:messageId/delivered
func (h *VideoReactionsHandler) MarkMessageAsDelivered(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
		return
	}

	err := h.service.MarkMessageAsDelivered(c.Request.Context(), messageID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark as delivered", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message marked as delivered"})
}

// MarkMessageAsRead marks a message as read
// POST /api/v1/video-reactions/messages/:messageId/read
func (h *VideoReactionsHandler) MarkMessageAsRead(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	messageID := c.Param("messageId")
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID required"})
		return
	}

	err := h.service.MarkMessageAsRead(c.Request.Context(), messageID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark as read", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message marked as read"})
}

// SearchMessages searches for messages in a chat
// GET /api/v1/video-reactions/chats/:chatId/messages/search
func (h *VideoReactionsHandler) SearchMessages(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	messages, err := h.service.SearchMessages(c.Request.Context(), chatID, userID, query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search messages", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"total":    len(messages),
		"query":    query,
	})
}

// ===============================
// TYPING INDICATORS
// ===============================

// SetTypingIndicator sets typing status
// POST /api/v1/video-reactions/chats/:chatId/typing
func (h *VideoReactionsHandler) SetTypingIndicator(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	var request struct {
		IsTyping bool `json:"isTyping"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	err := h.service.SetTypingIndicator(c.Request.Context(), chatID, userID, request.IsTyping)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set typing indicator", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Typing indicator updated"})
}

// GetTypingUsers gets users currently typing
// GET /api/v1/video-reactions/chats/:chatId/typing
func (h *VideoReactionsHandler) GetTypingUsers(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	users, err := h.service.GetTypingUsers(c.Request.Context(), chatID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get typing users", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"typingUsers": users,
		"count":       len(users),
	})
}

// ===============================
// STATISTICS
// ===============================

// GetChatStats retrieves chat statistics
// GET /api/v1/video-reactions/chats/:chatId/stats
func (h *VideoReactionsHandler) GetChatStats(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	stats, err := h.service.GetChatStats(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetUserChatStats retrieves user's chat statistics
// GET /api/v1/video-reactions/stats
func (h *VideoReactionsHandler) GetUserChatStats(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	stats, err := h.service.GetUserChatStats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
