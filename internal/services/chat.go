// ===============================
// internal/services/chat.go - Chat Service
// ===============================

package services

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"weibaobe/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ChatService struct {
	db *sqlx.DB
}

func NewChatService(db *sqlx.DB) *ChatService {
	return &ChatService{db: db}
}

// GenerateChatID generates a consistent chat ID from participants
func (s *ChatService) GenerateChatID(participants []string) string {
	// Sort participants to ensure consistent chat IDs
	sorted := make([]string, len(participants))
	copy(sorted, participants)
	sort.Strings(sorted)
	return strings.Join(sorted, "_")
}

// CreateOrGetChat creates a new chat or returns existing one
func (s *ChatService) CreateOrGetChat(participants []string) (*models.Chat, error) {
	chatID := s.GenerateChatID(participants)

	// Check if chat already exists
	var existingChat models.Chat
	err := s.db.Get(&existingChat, `
		SELECT chat_id, participants, last_message, last_message_type, 
		       last_message_sender, last_message_time, unread_counts,
		       is_archived, is_pinned, is_muted, chat_wallpapers, font_sizes,
		       created_at, updated_at
		FROM chats WHERE chat_id = $1`, chatID)

	if err == nil {
		// Chat exists, return it
		return &existingChat, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing chat: %w", err)
	}

	// Create new chat
	chat := &models.Chat{
		ChatID:            chatID,
		Participants:      participants,
		LastMessage:       "",
		LastMessageType:   models.MessageTypeText,
		LastMessageSender: "",
		LastMessageTime:   time.Now(),
		UnreadCounts:      make(models.IntMap),
		IsArchived:        make(models.BoolMap),
		IsPinned:          make(models.BoolMap),
		IsMuted:           make(models.BoolMap),
		ChatWallpapers:    make(models.StringMap),
		FontSizes:         make(models.FloatMap),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	query := `
		INSERT INTO chats (chat_id, participants, last_message, last_message_type,
		                   last_message_sender, last_message_time, unread_counts,
		                   is_archived, is_pinned, is_muted, chat_wallpapers,
		                   font_sizes, created_at, updated_at)
		VALUES (:chat_id, :participants, :last_message, :last_message_type,
		        :last_message_sender, :last_message_time, :unread_counts,
		        :is_archived, :is_pinned, :is_muted, :chat_wallpapers,
		        :font_sizes, :created_at, :updated_at)`

	_, err = s.db.NamedExec(query, chat)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	// Create chat participants entries
	for _, participantID := range participants {
		_, err = s.db.Exec(`
			INSERT INTO chat_participants (chat_id, user_id, joined_at)
			VALUES ($1, $2, $3)`,
			chatID, participantID, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to create chat participant: %w", err)
		}
	}

	return chat, nil
}

// GetChat gets a specific chat
func (s *ChatService) GetChat(chatID string) (*models.Chat, error) {
	var chat models.Chat
	err := s.db.Get(&chat, `
		SELECT chat_id, participants, last_message, last_message_type,
		       last_message_sender, last_message_time, unread_counts,
		       is_archived, is_pinned, is_muted, chat_wallpapers, font_sizes,
		       created_at, updated_at
		FROM chats WHERE chat_id = $1`, chatID)

	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	return &chat, nil
}

// GetUserChats gets chats for a user
func (s *ChatService) GetUserChats(userID string, limit, offset int) ([]models.Chat, error) {
	var chats []models.Chat
	err := s.db.Select(&chats, `
		SELECT chat_id, participants, last_message, last_message_type,
		       last_message_sender, last_message_time, unread_counts,
		       is_archived, is_pinned, is_muted, chat_wallpapers, font_sizes,
		       created_at, updated_at
		FROM chats 
		WHERE $1 = ANY(participants)
		ORDER BY last_message_time DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to get user chats: %w", err)
	}

	return chats, nil
}

// SendMessage sends a message to a chat
func (s *ChatService) SendMessage(message *models.Message) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert message
	query := `
		INSERT INTO messages (message_id, chat_id, sender_id, content, type, status,
		                     timestamp, media_url, media_metadata, reply_to_message_id,
		                     reply_to_content, reply_to_sender, reactions, is_edited,
		                     edited_at, is_pinned, read_by, delivered_to, created_at)
		VALUES (:message_id, :chat_id, :sender_id, :content, :type, :status,
		        :timestamp, :media_url, :media_metadata, :reply_to_message_id,
		        :reply_to_content, :reply_to_sender, :reactions, :is_edited,
		        :edited_at, :is_pinned, :read_by, :delivered_to, :created_at)`

	_, err = tx.NamedExec(query, message)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Update chat's last message info (will be handled by trigger, but we can also do it manually)
	_, err = tx.Exec(`
		UPDATE chats 
		SET last_message = $1, last_message_type = $2, last_message_sender = $3,
		    last_message_time = $4, updated_at = $5
		WHERE chat_id = $6`,
		message.Content, message.Type, message.SenderID,
		message.Timestamp, time.Now(), message.ChatID)
	if err != nil {
		return fmt.Errorf("failed to update chat last message: %w", err)
	}

	return tx.Commit()
}

// GetMessages gets messages from a chat
func (s *ChatService) GetMessages(chatID string, limit, offset int) ([]models.Message, error) {
	var messages []models.Message
	err := s.db.Select(&messages, `
		SELECT message_id, chat_id, sender_id, content, type, status,
		       timestamp, media_url, media_metadata, reply_to_message_id,
		       reply_to_content, reply_to_sender, reactions, is_edited,
		       edited_at, is_pinned, read_by, delivered_to, created_at
		FROM messages 
		WHERE chat_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3`,
		chatID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	return messages, nil
}

// UpdateMessage updates a message
func (s *ChatService) UpdateMessage(messageID, senderID, newContent string) error {
	result, err := s.db.Exec(`
		UPDATE messages 
		SET content = $1, is_edited = true, edited_at = $2
		WHERE message_id = $3 AND sender_id = $4`,
		newContent, time.Now(), messageID, senderID)

	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check update result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("message not found or permission denied")
	}

	return nil
}

// DeleteMessage deletes a message
func (s *ChatService) DeleteMessage(messageID, senderID string, deleteForEveryone bool) error {
	if deleteForEveryone {
		// Delete for everyone
		result, err := s.db.Exec(`
			DELETE FROM messages 
			WHERE message_id = $1 AND sender_id = $2`,
			messageID, senderID)

		if err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to check delete result: %w", err)
		}

		if rowsAffected == 0 {
			return fmt.Errorf("message not found or permission denied")
		}
	} else {
		// Just mark as deleted for sender (we could implement this with a deleted_for field)
		// For now, we'll just delete completely
		return s.DeleteMessage(messageID, senderID, true)
	}

	return nil
}

// MarkMessagesAsDelivered marks messages as delivered for a user
func (s *ChatService) MarkMessagesAsDelivered(chatID string, messageIDs []string, userID string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	// Build query for multiple message IDs
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs)+2)
	args[0] = userID
	args[1] = time.Now()

	for i, messageID := range messageIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args[i+2] = messageID
	}

	query := fmt.Sprintf(`
		UPDATE messages 
		SET delivered_to = COALESCE(delivered_to, '{}'::jsonb) || 
		    jsonb_build_object($1, $2)
		WHERE message_id IN (%s) AND sender_id != $1`,
		strings.Join(placeholders, ", "))

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark messages as delivered: %w", err)
	}

	return nil
}

// MarkChatAsRead marks a chat as read
func (s *ChatService) MarkChatAsRead(chatID, userID string) error {
	// Reset unread count for user
	_, err := s.db.Exec(`
		UPDATE chats 
		SET unread_counts = COALESCE(unread_counts, '{}'::jsonb) || 
		    jsonb_build_object($1, 0),
		    updated_at = $2
		WHERE chat_id = $3`,
		userID, time.Now(), chatID)

	if err != nil {
		return fmt.Errorf("failed to mark chat as read: %w", err)
	}

	// Mark all unread messages as read
	_, err = s.db.Exec(`
		UPDATE messages 
		SET read_by = COALESCE(read_by, '{}'::jsonb) || 
		    jsonb_build_object($1, $2)
		WHERE chat_id = $3 AND sender_id != $1 
		AND NOT (read_by ? $1)`,
		userID, time.Now(), chatID)

	if err != nil {
		return fmt.Errorf("failed to mark messages as read: %w", err)
	}

	return nil
}

// ToggleChatPin toggles chat pin status
func (s *ChatService) ToggleChatPin(chatID, userID string) error {
	_, err := s.db.Exec(`
		UPDATE chats 
		SET is_pinned = CASE 
			WHEN is_pinned ? $1 THEN 
				is_pinned - $1
			ELSE 
				COALESCE(is_pinned, '{}'::jsonb) || jsonb_build_object($1, true)
		END,
		updated_at = $2
		WHERE chat_id = $3`,
		userID, time.Now(), chatID)

	if err != nil {
		return fmt.Errorf("failed to toggle chat pin: %w", err)
	}

	return nil
}

// ToggleChatArchive toggles chat archive status
func (s *ChatService) ToggleChatArchive(chatID, userID string) error {
	_, err := s.db.Exec(`
		UPDATE chats 
		SET is_archived = CASE 
			WHEN is_archived ? $1 THEN 
				is_archived - $1
			ELSE 
				COALESCE(is_archived, '{}'::jsonb) || jsonb_build_object($1, true)
		END,
		updated_at = $2
		WHERE chat_id = $3`,
		userID, time.Now(), chatID)

	if err != nil {
		return fmt.Errorf("failed to toggle chat archive: %w", err)
	}

	return nil
}

// ToggleChatMute toggles chat mute status
func (s *ChatService) ToggleChatMute(chatID, userID string) error {
	_, err := s.db.Exec(`
		UPDATE chats 
		SET is_muted = CASE 
			WHEN is_muted ? $1 THEN 
				is_muted - $1
			ELSE 
				COALESCE(is_muted, '{}'::jsonb) || jsonb_build_object($1, true)
		END,
		updated_at = $2
		WHERE chat_id = $3`,
		userID, time.Now(), chatID)

	if err != nil {
		return fmt.Errorf("failed to toggle chat mute: %w", err)
	}

	return nil
}

// SetChatSettings sets chat settings
func (s *ChatService) SetChatSettings(chatID, userID string, wallpaperURL *string, fontSize *float64) error {
	updateParts := []string{"updated_at = $2"}
	args := []interface{}{userID, time.Now()}
	argIndex := 3

	if wallpaperURL != nil {
		updateParts = append(updateParts, fmt.Sprintf(`
			chat_wallpapers = COALESCE(chat_wallpapers, '{}'::jsonb) || 
			jsonb_build_object($1, $%d)`, argIndex))
		args = append(args, *wallpaperURL)
		argIndex++
	}

	if fontSize != nil {
		updateParts = append(updateParts, fmt.Sprintf(`
			font_sizes = COALESCE(font_sizes, '{}'::jsonb) || 
			jsonb_build_object($1, $%d)`, argIndex))
		args = append(args, *fontSize)
		argIndex++
	}

	if len(updateParts) == 1 {
		return nil // No settings to update
	}

	query := fmt.Sprintf(`
		UPDATE chats 
		SET %s
		WHERE chat_id = $%d`,
		strings.Join(updateParts, ", "), argIndex)
	args = append(args, chatID)

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update chat settings: %w", err)
	}

	return nil
}

// SendVideoReactionMessage sends a video reaction message
func (s *ChatService) SendVideoReactionMessage(chatID, senderID string, videoReaction models.VideoReactionMessage) error {
	metadata := map[string]interface{}{
		"isVideoReaction": true,
		"videoId":         videoReaction.VideoID,
		"videoUrl":        videoReaction.VideoURL,
		"thumbnailUrl":    videoReaction.ThumbnailURL,
		"channelName":     videoReaction.ChannelName,
		"channelImage":    videoReaction.ChannelImage,
	}

	message := &models.Message{
		MessageID:     uuid.New().String(),
		ChatID:        chatID,
		SenderID:      senderID,
		Content:       videoReaction.Reaction,
		Type:          models.MessageTypeText,
		Status:        models.MessageStatusSent,
		Timestamp:     time.Now(),
		MediaURL:      &videoReaction.VideoURL,
		MediaMetadata: metadata,
		Reactions:     make(models.StringMap),
		ReadBy:        make(models.TimeMap),
		DeliveredTo:   make(models.TimeMap),
		CreatedAt:     time.Now(),
	}

	return s.SendMessage(message)
}

// SendMomentReactionMessage sends a moment reaction message
func (s *ChatService) SendMomentReactionMessage(chatID, senderID string, momentReaction models.MomentReactionMessage) error {
	metadata := map[string]interface{}{
		"isMomentReaction": true,
		"momentId":         momentReaction.MomentID,
		"thumbnailUrl":     momentReaction.ThumbnailURL,
		"authorName":       momentReaction.AuthorName,
		"authorImage":      momentReaction.AuthorImage,
		"mediaType":        momentReaction.MediaType,
		"momentContent":    momentReaction.Content,
	}

	message := &models.Message{
		MessageID:     uuid.New().String(),
		ChatID:        chatID,
		SenderID:      senderID,
		Content:       momentReaction.Reaction,
		Type:          models.MessageTypeText,
		Status:        models.MessageStatusSent,
		Timestamp:     time.Now(),
		MediaURL:      &momentReaction.MediaURL,
		MediaMetadata: metadata,
		Reactions:     make(models.StringMap),
		ReadBy:        make(models.TimeMap),
		DeliveredTo:   make(models.TimeMap),
		CreatedAt:     time.Now(),
	}

	return s.SendMessage(message)
}

// AreUsersBlocked checks if two users have blocked each other
func (s *ChatService) AreUsersBlocked(userID1, userID2 string) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM blocked_contacts 
		WHERE (user_id = $1 AND blocked_user_id = $2) 
		   OR (user_id = $2 AND blocked_user_id = $1)`,
		userID1, userID2).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check block status: %w", err)
	}

	return count > 0, nil
}
