// ===============================
// internal/handlers/chat.go - Updated Chat Handler (User-Based, No Moments)
// ===============================

package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ChatHandler struct {
	chatService *services.ChatService
	userService *services.UserService
}

func NewChatHandler(chatService *services.ChatService, userService *services.UserService) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		userService: userService,
	}
}

// CreateOrGetChat creates a new chat or returns existing one
func (h *ChatHandler) CreateOrGetChat(c *gin.Context) {
	var request models.CreateChatRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	currentUserID := c.GetString("userID")
	if currentUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Ensure current user is in participants
	found := false
	for _, participant := range request.Participants {
		if participant == currentUserID {
			found = true
			break
		}
	}
	if !found {
		request.Participants = append(request.Participants, currentUserID)
	}

	// For now, only support 2-person chats
	if len(request.Participants) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only 2-person chats are supported"})
		return
	}

	// Check if users are blocked
	otherUserID := ""
	for _, participant := range request.Participants {
		if participant != currentUserID {
			otherUserID = participant
			break
		}
	}

	blocked, err := h.chatService.AreUsersBlocked(currentUserID, otherUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check block status"})
		return
	}
	if blocked {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot create chat with blocked user"})
		return
	}

	chat, err := h.chatService.CreateOrGetChat(request.Participants)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat": chat})
}

// GetChats gets user's chats
func (h *ChatHandler) GetChats(c *gin.Context) {
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

	chats, err := h.chatService.GetUserChats(userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chats"})
		return
	}

	// Enhance chats with contact information
	enhancedChats := make([]models.ChatResponse, len(chats))
	for i, chat := range chats {
		otherUserID := chat.GetOtherParticipant(userID)

		enhancedChats[i] = models.ChatResponse{
			Chat:     chat,
			IsOnline: false, // TODO: Implement online status
		}

		// Get other user's info
		if otherUser, err := h.userService.GetUser(c, otherUserID); err == nil {
			enhancedChats[i].ContactName = otherUser.Name
			enhancedChats[i].ContactImage = otherUser.ProfileImage
			enhancedChats[i].ContactPhone = otherUser.PhoneNumber
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"chats":   enhancedChats,
		"total":   len(enhancedChats),
		"hasMore": len(chats) == limit,
	})
}

// GetChat gets a specific chat
func (h *ChatHandler) GetChat(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	chat, err := h.chatService.GetChat(chatID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chat not found"})
		return
	}

	// Check if user is participant
	isParticipant := false
	for _, participant := range chat.Participants {
		if participant == userID {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat": chat})
}

// SendMessage sends a message in a chat
func (h *ChatHandler) SendMessage(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	var request models.SendMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	senderID := c.GetString("userID")
	if senderID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Validate message type
	if request.Type == "" {
		request.Type = models.MessageTypeText
	}

	validTypes := []string{
		models.MessageTypeText,
		models.MessageTypeImage,
		models.MessageTypeVideo,
		models.MessageTypeFile,
		models.MessageTypeAudio,
		models.MessageTypeLocation,
		models.MessageTypeContact,
	}
	isValidType := false
	for _, validType := range validTypes {
		if request.Type == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message type"})
		return
	}

	// Create message
	message := &models.Message{
		MessageID:        uuid.New().String(),
		ChatID:           chatID,
		SenderID:         senderID,
		Content:          request.Content,
		Type:             request.Type,
		Status:           models.MessageStatusSent,
		Timestamp:        time.Now(),
		MediaMetadata:    request.MediaMetadata,
		ReplyToMessageID: nil,
		ReplyToContent:   nil,
		ReplyToSender:    nil,
		Reactions:        make(models.StringMap),
		IsEdited:         false,
		IsPinned:         false,
		ReadBy:           make(models.TimeMap),
		DeliveredTo:      make(models.TimeMap),
		CreatedAt:        time.Now(),
	}

	if request.MediaURL != "" {
		message.MediaURL = &request.MediaURL
	}

	if request.ReplyToMessageID != "" {
		message.ReplyToMessageID = &request.ReplyToMessageID
		// TODO: Get reply message content and sender
	}

	err := h.chatService.SendMessage(message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": message})
}

// GetMessages gets messages from a chat
func (h *ChatHandler) GetMessages(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

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

	// Verify user is participant
	chat, err := h.chatService.GetChat(chatID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chat not found"})
		return
	}

	isParticipant := false
	for _, participant := range chat.Participants {
		if participant == userID {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	messages, err := h.chatService.GetMessages(chatID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get messages"})
		return
	}

	// Mark messages as delivered for current user
	messageIDs := make([]string, len(messages))
	for i, message := range messages {
		messageIDs[i] = message.MessageID
	}
	if len(messageIDs) > 0 {
		h.chatService.MarkMessagesAsDelivered(chatID, messageIDs, userID)
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"total":    len(messages),
		"hasMore":  len(messages) == limit,
	})
}

// UpdateMessage updates a message
func (h *ChatHandler) UpdateMessage(c *gin.Context) {
	chatID := c.Param("chatId")
	messageID := c.Param("messageId")

	if chatID == "" || messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID and Message ID required"})
		return
	}

	var request models.UpdateMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.chatService.UpdateMessage(messageID, userID, request.Content)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
			return
		}
		if strings.Contains(err.Error(), "permission") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message updated successfully"})
}

// DeleteMessage deletes a message
func (h *ChatHandler) DeleteMessage(c *gin.Context) {
	chatID := c.Param("chatId")
	messageID := c.Param("messageId")

	if chatID == "" || messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID and Message ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	deleteForEveryone := c.Query("deleteForEveryone") == "true"

	err := h.chatService.DeleteMessage(messageID, userID, deleteForEveryone)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
			return
		}
		if strings.Contains(err.Error(), "permission") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted successfully"})
}

// MarkChatAsRead marks a chat as read
func (h *ChatHandler) MarkChatAsRead(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.chatService.MarkChatAsRead(chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark chat as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat marked as read"})
}

// TogglePinChat toggles chat pin status
func (h *ChatHandler) TogglePinChat(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.chatService.ToggleChatPin(chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle chat pin"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat pin status updated"})
}

// ToggleArchiveChat toggles chat archive status
func (h *ChatHandler) ToggleArchiveChat(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.chatService.ToggleChatArchive(chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle chat archive"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat archive status updated"})
}

// ToggleMuteChat toggles chat mute status
func (h *ChatHandler) ToggleMuteChat(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.chatService.ToggleChatMute(chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to toggle chat mute"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat mute status updated"})
}

// SetChatSettings sets chat settings (wallpaper, font size)
func (h *ChatHandler) SetChatSettings(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	var request models.ChatSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.chatService.SetChatSettings(chatID, userID, request.WallpaperURL, request.FontSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update chat settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat settings updated"})
}

// SendVideoReaction sends a video reaction message (UPDATED: user-based, no channels)
func (h *ChatHandler) SendVideoReaction(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	var request models.VideoReactionMessage
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	senderID := c.GetString("userID")
	if senderID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	err := h.chatService.SendVideoReactionMessage(chatID, senderID, request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send video reaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Video reaction sent successfully"})
}
