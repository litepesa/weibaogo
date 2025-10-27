// ===============================
// internal/services/video_reactions_service.go - Video Reactions Service
// ===============================

package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/repositories"

	"github.com/google/uuid"
)

type VideoReactionsService struct {
	repo         *repositories.VideoReactionsRepository
	userService  *UserService
	videoService *VideoService
}

func NewVideoReactionsService(
	repo *repositories.VideoReactionsRepository,
	userService *UserService,
	videoService *VideoService,
) *VideoReactionsService {
	return &VideoReactionsService{
		repo:         repo,
		userService:  userService,
		videoService: videoService,
	}
}

// ===============================
// CHAT OPERATIONS
// ===============================

// CreateVideoReactionChat creates a new chat from a video reaction
func (s *VideoReactionsService) CreateVideoReactionChat(
	ctx context.Context,
	currentUserID string,
	videoOwnerID string,
	videoReaction *models.VideoReaction,
) (*models.VideoReactionChat, error) {
	// Validate users exist
	if currentUserID == videoOwnerID {
		return nil, errors.New("cannot create chat with yourself")
	}

	// Check if video exists
	video, err := s.videoService.GetVideoOptimized(ctx, videoReaction.VideoID)
	if err != nil {
		return nil, fmt.Errorf("video not found: %w", err)
	}

	// Generate chat ID
	chatID := s.repo.GenerateChatID(currentUserID, videoOwnerID, videoReaction.VideoID)

	// Check if chat already exists
	existingChat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if existingChat != nil {
		return existingChat, nil // Return existing chat
	}

	// Get video owner info
	videoOwnerName, videoOwnerImage, _, err := s.userService.GetUserBasicInfo(ctx, videoOwnerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get video owner info: %w", err)
	}

	// Create chat object
	now := time.Now()
	chat := &models.VideoReactionChat{
		ChatID:               chatID,
		Participants:         models.StringSlice{currentUserID, videoOwnerID},
		OriginalVideoID:      video.ID,
		OriginalVideoURL:     video.VideoURL,
		OriginalThumbnailURL: video.ThumbnailURL,
		OriginalUserName:     videoOwnerName,
		OriginalUserImage:    videoOwnerImage,
		OriginalReaction:     videoReaction.Reaction,
		OriginalTimestamp:    videoReaction.Timestamp,
		LastMessage:          "",
		LastMessageType:      "text",
		LastMessageSender:    currentUserID,
		LastMessageTime:      now,
		UnreadCounts:         models.IntMap{},
		IsArchived:           models.BoolMap{},
		IsPinned:             models.BoolMap{},
		IsMuted:              models.BoolMap{},
		ChatWallpapers:       models.StringMap{},
		FontSizes:            models.Float64Map{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	// Create chat in database
	err = s.repo.CreateChat(ctx, chat)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	// Create initial video reaction message
	initialMessage := &models.VideoReactionMessage{
		MessageID:          uuid.New().String(),
		ChatID:             chatID,
		SenderID:           currentUserID,
		Content:            s.getVideoReactionDisplayContent(videoReaction),
		Type:               models.MessageTypeText,
		Status:             models.MessageStatusSent,
		VideoReactionData:  videoReaction,
		IsOriginalReaction: true,
		Timestamp:          now,
		Reactions:          models.StringMap{},
		ReadBy:             models.TimeMap{},
		DeliveredTo:        models.TimeMap{},
		MediaMetadata:      models.InterfaceMap{},
	}

	err = s.repo.CreateMessage(ctx, initialMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial message: %w", err)
	}

	return chat, nil
}

// GetUserChats retrieves all chats for a user with enhanced data
func (s *VideoReactionsService) GetUserChats(ctx context.Context, userID string, limit, offset int) ([]models.VideoReactionChatResponse, error) {
	chats, err := s.repo.GetUserChats(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	responses := make([]models.VideoReactionChatResponse, len(chats))
	for i, chat := range chats {
		responses[i] = s.enrichChatResponse(ctx, &chat, userID)
	}

	return responses, nil
}

// GetChatByID retrieves a chat by ID with enhanced data
func (s *VideoReactionsService) GetChatByID(ctx context.Context, chatID, userID string) (*models.VideoReactionChatResponse, error) {
	chat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return nil, errors.New("chat not found")
	}

	// Verify user is participant
	if !s.isParticipant(chat, userID) {
		return nil, errors.New("access denied")
	}

	response := s.enrichChatResponse(ctx, chat, userID)
	return &response, nil
}

// MarkChatAsRead marks all messages as read
func (s *VideoReactionsService) MarkChatAsRead(ctx context.Context, chatID, userID string) error {
	return s.repo.MarkChatAsRead(ctx, chatID, userID)
}

// ToggleChatPin toggles pin status
func (s *VideoReactionsService) ToggleChatPin(ctx context.Context, chatID, userID string) error {
	return s.repo.ToggleChatPin(ctx, chatID, userID)
}

// ToggleChatArchive toggles archive status
func (s *VideoReactionsService) ToggleChatArchive(ctx context.Context, chatID, userID string) error {
	return s.repo.ToggleChatArchive(ctx, chatID, userID)
}

// ToggleChatMute toggles mute status
func (s *VideoReactionsService) ToggleChatMute(ctx context.Context, chatID, userID string) error {
	return s.repo.ToggleChatMute(ctx, chatID, userID)
}

// UpdateChatSettings updates chat settings
func (s *VideoReactionsService) UpdateChatSettings(ctx context.Context, chatID, userID string, wallpaper *string, fontSize *float64) error {
	return s.repo.UpdateChatSettings(ctx, chatID, userID, wallpaper, fontSize)
}

// DeleteChat deletes a chat
func (s *VideoReactionsService) DeleteChat(ctx context.Context, chatID, userID string, deleteForEveryone bool) error {
	// Get chat to verify ownership
	chat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return err
	}
	if chat == nil {
		return errors.New("chat not found")
	}

	// Only allow delete for everyone if user is a participant
	if deleteForEveryone && !s.isParticipant(chat, userID) {
		return errors.New("access denied")
	}

	return s.repo.DeleteChat(ctx, chatID, userID, deleteForEveryone)
}

// ClearChatHistory clears all messages
func (s *VideoReactionsService) ClearChatHistory(ctx context.Context, chatID, userID string) error {
	// Verify access
	chat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return err
	}
	if chat == nil {
		return errors.New("chat not found")
	}
	if !s.isParticipant(chat, userID) {
		return errors.New("access denied")
	}

	return s.repo.ClearChatHistory(ctx, chatID, userID)
}

// ===============================
// MESSAGE OPERATIONS
// ===============================

// SendMessage sends a new message in a chat
func (s *VideoReactionsService) SendMessage(
	ctx context.Context,
	chatID string,
	senderID string,
	request *models.SendMessageRequest,
) (*models.VideoReactionMessage, error) {
	// Verify chat exists and user has access
	chat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return nil, errors.New("chat not found")
	}
	if !s.isParticipant(chat, senderID) {
		return nil, errors.New("access denied")
	}

	// Create message
	message := &models.VideoReactionMessage{
		MessageID:        uuid.New().String(),
		ChatID:           chatID,
		SenderID:         senderID,
		Content:          request.Content,
		Type:             request.Type,
		Status:           models.MessageStatusSent,
		MediaURL:         request.MediaURL,
		MediaMetadata:    request.MediaMetadata,
		FileName:         request.FileName,
		ReplyToMessageID: request.ReplyToMessageID,
		Timestamp:        time.Now(),
		Reactions:        models.StringMap{},
		ReadBy:           models.TimeMap{senderID: time.Now()},
		DeliveredTo:      models.TimeMap{},
	}

	// If replying, get reply context
	if request.ReplyToMessageID != nil {
		replyToMsg, err := s.repo.GetMessageByID(ctx, *request.ReplyToMessageID)
		if err == nil && replyToMsg != nil {
			message.ReplyToContent = &replyToMsg.Content
			message.ReplyToSender = &replyToMsg.SenderID
		}
	}

	// Save message
	err = s.repo.CreateMessage(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return message, nil
}

// GetChatMessages retrieves messages from a chat
func (s *VideoReactionsService) GetChatMessages(
	ctx context.Context,
	chatID string,
	userID string,
	limit, offset int,
) (*models.MessagesListResponse, error) {
	// Verify access
	chat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return nil, errors.New("chat not found")
	}
	if !s.isParticipant(chat, userID) {
		return nil, errors.New("access denied")
	}

	// Get messages
	messages, err := s.repo.GetChatMessages(ctx, chatID, limit, offset)
	if err != nil {
		return nil, err
	}

	// Get pinned messages
	pinnedMessages, err := s.repo.GetPinnedMessages(ctx, chatID)
	if err != nil {
		return nil, err
	}

	// Enrich messages with user data
	enrichedMessages := make([]models.VideoReactionMessageResponse, len(messages))
	for i, msg := range messages {
		enrichedMessages[i] = s.enrichMessageResponse(ctx, &msg)
	}

	enrichedPinned := make([]models.VideoReactionMessageResponse, len(pinnedMessages))
	for i, msg := range pinnedMessages {
		enrichedPinned[i] = s.enrichMessageResponse(ctx, &msg)
	}

	return &models.MessagesListResponse{
		Messages:       enrichedMessages,
		Total:          len(enrichedMessages),
		HasMore:        len(messages) == limit,
		PinnedMessages: enrichedPinned,
	}, nil
}

// EditMessage edits a message
func (s *VideoReactionsService) EditMessage(ctx context.Context, messageID, userID, newContent string) error {
	// Get message
	message, err := s.repo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if message == nil {
		return errors.New("message not found")
	}

	// Verify sender
	if message.SenderID != userID {
		return errors.New("access denied")
	}

	// Verify message type (only text messages can be edited)
	if message.Type != models.MessageTypeText {
		return errors.New("only text messages can be edited")
	}

	return s.repo.EditMessage(ctx, messageID, newContent)
}

// DeleteMessage deletes a message
func (s *VideoReactionsService) DeleteMessage(ctx context.Context, messageID, userID string, deleteForEveryone bool) error {
	// Get message
	message, err := s.repo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if message == nil {
		return errors.New("message not found")
	}

	// Verify sender
	if message.SenderID != userID {
		return errors.New("access denied")
	}

	return s.repo.DeleteMessage(ctx, messageID, deleteForEveryone)
}

// ToggleMessagePin toggles message pin status
func (s *VideoReactionsService) ToggleMessagePin(ctx context.Context, messageID, userID string) error {
	// Get message
	message, err := s.repo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if message == nil {
		return errors.New("message not found")
	}

	// Verify access
	chat, err := s.repo.GetChatByID(ctx, message.ChatID)
	if err != nil {
		return err
	}
	if !s.isParticipant(chat, userID) {
		return errors.New("access denied")
	}

	// Check pinned message limit
	if !message.IsPinned {
		pinnedMessages, err := s.repo.GetPinnedMessages(ctx, message.ChatID)
		if err != nil {
			return err
		}
		if len(pinnedMessages) >= 10 {
			return errors.New("maximum 10 messages can be pinned")
		}
	}

	return s.repo.ToggleMessagePin(ctx, messageID)
}

// AddMessageReaction adds a reaction to a message
func (s *VideoReactionsService) AddMessageReaction(ctx context.Context, messageID, userID, reaction string) error {
	// Get message
	message, err := s.repo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if message == nil {
		return errors.New("message not found")
	}

	// Verify access
	chat, err := s.repo.GetChatByID(ctx, message.ChatID)
	if err != nil {
		return err
	}
	if !s.isParticipant(chat, userID) {
		return errors.New("access denied")
	}

	return s.repo.AddMessageReaction(ctx, messageID, userID, reaction)
}

// RemoveMessageReaction removes a reaction from a message
func (s *VideoReactionsService) RemoveMessageReaction(ctx context.Context, messageID, userID string) error {
	// Get message
	message, err := s.repo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if message == nil {
		return errors.New("message not found")
	}

	// Verify access
	chat, err := s.repo.GetChatByID(ctx, message.ChatID)
	if err != nil {
		return err
	}
	if !s.isParticipant(chat, userID) {
		return errors.New("access denied")
	}

	return s.repo.RemoveMessageReaction(ctx, messageID, userID)
}

// MarkMessageAsDelivered marks a message as delivered
func (s *VideoReactionsService) MarkMessageAsDelivered(ctx context.Context, messageID, userID string) error {
	return s.repo.MarkMessageAsDelivered(ctx, messageID, userID)
}

// MarkMessageAsRead marks a message as read
func (s *VideoReactionsService) MarkMessageAsRead(ctx context.Context, messageID, userID string) error {
	return s.repo.MarkMessageAsRead(ctx, messageID, userID)
}

// SearchMessages searches for messages in a chat
func (s *VideoReactionsService) SearchMessages(ctx context.Context, chatID, userID, query string, limit int) ([]models.VideoReactionMessageResponse, error) {
	// Verify access
	chat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return nil, errors.New("chat not found")
	}
	if !s.isParticipant(chat, userID) {
		return nil, errors.New("access denied")
	}

	// Search messages
	messages, err := s.repo.SearchMessages(ctx, chatID, query, limit)
	if err != nil {
		return nil, err
	}

	// Enrich with user data
	enrichedMessages := make([]models.VideoReactionMessageResponse, len(messages))
	for i, msg := range messages {
		enrichedMessages[i] = s.enrichMessageResponse(ctx, &msg)
	}

	return enrichedMessages, nil
}

// ===============================
// TYPING INDICATORS
// ===============================

// SetTypingIndicator sets typing status
func (s *VideoReactionsService) SetTypingIndicator(ctx context.Context, chatID, userID string, isTyping bool) error {
	return s.repo.SetTypingIndicator(ctx, chatID, userID, isTyping)
}

// GetTypingUsers gets users currently typing
func (s *VideoReactionsService) GetTypingUsers(ctx context.Context, chatID string) ([]string, error) {
	return s.repo.GetTypingUsers(ctx, chatID)
}

// ===============================
// STATISTICS
// ===============================

// GetChatStats retrieves chat statistics
func (s *VideoReactionsService) GetChatStats(ctx context.Context, chatID, userID string) (map[string]interface{}, error) {
	// Verify access
	chat, err := s.repo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return nil, errors.New("chat not found")
	}
	if !s.isParticipant(chat, userID) {
		return nil, errors.New("access denied")
	}

	return s.repo.GetChatStats(ctx, chatID)
}

// GetUserChatStats retrieves user's chat statistics
func (s *VideoReactionsService) GetUserChatStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	return s.repo.GetUserChatStats(ctx, userID)
}

// ===============================
// HELPER METHODS
// ===============================

// isParticipant checks if user is a participant in chat
func (s *VideoReactionsService) isParticipant(chat *models.VideoReactionChat, userID string) bool {
	for _, participant := range chat.Participants {
		if participant == userID {
			return true
		}
	}
	return false
}

// enrichChatResponse adds user data to chat response
func (s *VideoReactionsService) enrichChatResponse(ctx context.Context, chat *models.VideoReactionChat, currentUserID string) models.VideoReactionChatResponse {
	response := models.VideoReactionChatResponse{
		VideoReactionChat: *chat,
	}

	// Get other participant info
	otherParticipantID := chat.GetOtherParticipant(currentUserID)
	if otherParticipantID != "" {
		name, image, _, err := s.userService.GetUserBasicInfo(ctx, otherParticipantID)
		if err == nil {
			response.OtherParticipantName = name
			response.OtherParticipantImage = image
		}
	}

	return response
}

// enrichMessageResponse adds user data to message response
func (s *VideoReactionsService) enrichMessageResponse(ctx context.Context, message *models.VideoReactionMessage) models.VideoReactionMessageResponse {
	response := models.VideoReactionMessageResponse{
		VideoReactionMessage: *message,
	}

	// Get sender info
	name, image, _, err := s.userService.GetUserBasicInfo(ctx, message.SenderID)
	if err == nil {
		response.SenderName = name
		response.SenderImage = image
	}

	return response
}

// getVideoReactionDisplayContent generates display content for video reaction
func (s *VideoReactionsService) getVideoReactionDisplayContent(reaction *models.VideoReaction) string {
	if reaction.Reaction != nil && *reaction.Reaction != "" {
		return *reaction.Reaction
	}
	return "Shared a video"
}

// ===============================
// CLEANUP OPERATIONS
// ===============================

// CleanupExpiredTypingIndicators cleans up expired typing indicators
func (s *VideoReactionsService) CleanupExpiredTypingIndicators(ctx context.Context) error {
	return s.repo.CleanupExpiredTypingIndicators(ctx)
}

// CleanupStaleConnections cleans up stale WebSocket connections
func (s *VideoReactionsService) CleanupStaleConnections(ctx context.Context) error {
	return s.repo.CleanupStaleConnections(ctx)
}
