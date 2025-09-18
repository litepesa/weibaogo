// ===============================
// internal/handlers/chat.go - FIXED Chat Handler with Comprehensive Error Handling and Logging
// ===============================

package handlers

import (
	"log"
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
	log.Printf("CreateOrGetChat called by user from IP: %s", c.ClientIP())

	var request models.CreateChatRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("JSON binding error in CreateOrGetChat: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	currentUserID := c.GetString("userID")
	if currentUserID == "" {
		log.Printf("CreateOrGetChat: User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	log.Printf("CreateOrGetChat: Current user: %s, Participants: %v", currentUserID, request.Participants)

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
		log.Printf("Added current user to participants: %v", request.Participants)
	}

	// For now, only support 2-person chats
	if len(request.Participants) != 2 {
		log.Printf("Invalid participant count: %d", len(request.Participants))
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
		log.Printf("Error checking block status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check block status"})
		return
	}
	if blocked {
		log.Printf("Users are blocked: %s and %s", currentUserID, otherUserID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot create chat with blocked user"})
		return
	}

	chat, err := h.chatService.CreateOrGetChat(request.Participants)
	if err != nil {
		log.Printf("Error creating/getting chat: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat", "details": err.Error()})
		return
	}

	log.Printf("Successfully created/got chat: %s", chat.ChatID)
	c.JSON(http.StatusOK, gin.H{"chat": chat, "chatId": chat.ChatID})
}

// GetChats gets user's chats
func (h *ChatHandler) GetChats(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		log.Printf("GetChats: User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	log.Printf("GetChats called for user: %s", userID)

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
		log.Printf("Error getting user chats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chats", "details": err.Error()})
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

	log.Printf("Returning %d chats for user %s", len(enhancedChats), userID)
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

	log.Printf("GetChat: %s for user %s", chatID, userID)

	chat, err := h.chatService.GetChat(chatID)
	if err != nil {
		log.Printf("Error getting chat: %v", err)
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
		log.Printf("User %s is not a participant in chat %s", userID, chatID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat": chat})
}

// SendMessage sends a message in a chat
func (h *ChatHandler) SendMessage(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		log.Printf("SendMessage: Chat ID missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	log.Printf("SendMessage called for chat: %s", chatID)

	var request models.SendMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("SendMessage JSON binding error: %v", err)
		log.Printf("Request body that failed to bind: %+v", c.Request.Body)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	log.Printf("SendMessage request: Type=%s, Content length=%d", request.Type, len(request.Content))

	senderID := c.GetString("userID")
	if senderID == "" {
		log.Printf("SendMessage: User not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	log.Printf("SendMessage: Sender=%s, Chat=%s", senderID, chatID)

	// Validate message type
	if request.Type == "" {
		request.Type = models.MessageTypeText
		log.Printf("SendMessage: Type not specified, defaulting to text")
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
		log.Printf("SendMessage: Invalid message type: %s", request.Type)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message type"})
		return
	}

	// Verify chat exists and user is participant
	chat, err := h.chatService.GetChat(chatID)
	if err != nil {
		log.Printf("SendMessage: Chat not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Chat not found"})
		return
	}

	// Check if user is participant
	isParticipant := false
	for _, participant := range chat.Participants {
		if participant == senderID {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		log.Printf("SendMessage: User %s not participant in chat %s", senderID, chatID)
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not a participant in this chat"})
		return
	}

	// Generate message ID if not provided
	messageID := uuid.New().String()
	log.Printf("SendMessage: Generated message ID: %s", messageID)

	// Create message with initialized maps
	message := &models.Message{
		MessageID:        messageID,
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

	// Set media URL if provided
	if request.MediaURL != "" {
		message.MediaURL = &request.MediaURL
		log.Printf("SendMessage: Media URL set: %s", request.MediaURL)
	}

	// Set reply information if provided
	if request.ReplyToMessageID != "" {
		message.ReplyToMessageID = &request.ReplyToMessageID
		log.Printf("SendMessage: Reply to message: %s", request.ReplyToMessageID)
		// TODO: Get reply message content and sender from database
	}

	// Initialize metadata if nil
	if message.MediaMetadata == nil {
		message.MediaMetadata = make(map[string]interface{})
	}

	log.Printf("SendMessage: Calling service to send message")
	err = h.chatService.SendMessage(message)
	if err != nil {
		log.Printf("SendMessage: Service error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message", "details": err.Error()})
		return
	}

	log.Printf("SendMessage: Message sent successfully: %s", messageID)
	c.JSON(http.StatusCreated, gin.H{
		"message":   message,
		"messageId": messageID,
		"success":   true,
	})
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

	log.Printf("GetMessages: Chat=%s, User=%s", chatID, userID)

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
		log.Printf("GetMessages: Chat not found: %v", err)
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
		log.Printf("GetMessages: User %s not participant", userID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	messages, err := h.chatService.GetMessages(chatID, limit, offset)
	if err != nil {
		log.Printf("GetMessages: Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get messages", "details": err.Error()})
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

	log.Printf("GetMessages: Returning %d messages", len(messages))
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

	log.Printf("UpdateMessage: Chat=%s, Message=%s", chatID, messageID)

	var request models.UpdateMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("UpdateMessage JSON error: %v", err)
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
		log.Printf("UpdateMessage error: %v", err)
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

	log.Printf("UpdateMessage: Success")
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

	log.Printf("DeleteMessage: Chat=%s, Message=%s", chatID, messageID)

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	deleteForEveryone := c.Query("deleteForEveryone") == "true"
	log.Printf("DeleteMessage: deleteForEveryone=%v", deleteForEveryone)

	err := h.chatService.DeleteMessage(messageID, userID, deleteForEveryone)
	if err != nil {
		log.Printf("DeleteMessage error: %v", err)
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

	log.Printf("DeleteMessage: Success")
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

	log.Printf("MarkChatAsRead: Chat=%s, User=%s", chatID, userID)

	err := h.chatService.MarkChatAsRead(chatID, userID)
	if err != nil {
		log.Printf("MarkChatAsRead error: %v", err)
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

	log.Printf("TogglePinChat: Chat=%s, User=%s", chatID, userID)

	err := h.chatService.ToggleChatPin(chatID, userID)
	if err != nil {
		log.Printf("TogglePinChat error: %v", err)
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

	log.Printf("ToggleArchiveChat: Chat=%s, User=%s", chatID, userID)

	err := h.chatService.ToggleChatArchive(chatID, userID)
	if err != nil {
		log.Printf("ToggleArchiveChat error: %v", err)
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

	log.Printf("ToggleMuteChat: Chat=%s, User=%s", chatID, userID)

	err := h.chatService.ToggleChatMute(chatID, userID)
	if err != nil {
		log.Printf("ToggleMuteChat error: %v", err)
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
		log.Printf("SetChatSettings JSON error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	log.Printf("SetChatSettings: Chat=%s, User=%s", chatID, userID)

	err := h.chatService.SetChatSettings(chatID, userID, request.WallpaperURL, request.FontSize)
	if err != nil {
		log.Printf("SetChatSettings error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update chat settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Chat settings updated"})
}

// SendVideoReaction sends a video reaction message
func (h *ChatHandler) SendVideoReaction(c *gin.Context) {
	chatID := c.Param("chatId")
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chat ID required"})
		return
	}

	var request models.VideoReactionMessage
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("SendVideoReaction JSON error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	senderID := c.GetString("userID")
	if senderID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	log.Printf("SendVideoReaction: Chat=%s, Sender=%s, VideoID=%s", chatID, senderID, request.VideoID)

	err := h.chatService.SendVideoReactionMessage(chatID, senderID, request)
	if err != nil {
		log.Printf("SendVideoReaction error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send video reaction", "details": err.Error()})
		return
	}

	log.Printf("SendVideoReaction: Success")
	c.JSON(http.StatusCreated, gin.H{"message": "Video reaction sent successfully", "success": true})
}
