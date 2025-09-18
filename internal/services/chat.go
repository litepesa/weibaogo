// ===============================
// internal/services/chat.go - FIXED Chat Service with Proper JSONB Handling
// ===============================

package services

import (
	"database/sql"
	//"encoding/json"
	"fmt"
	"log"
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

// CreateOrGetChat creates a new chat or returns existing one - FIXED with proper error handling
func (s *ChatService) CreateOrGetChat(participants []string) (*models.Chat, error) {
	log.Printf("Creating/getting chat for participants: %v", participants)

	chatID := s.GenerateChatID(participants)
	log.Printf("Generated chat ID: %s", chatID)

	// First try to get existing chat
	var existingChat models.Chat
	err := s.db.Get(&existingChat, `
		SELECT chat_id, participants, last_message, last_message_type, 
		       last_message_sender, last_message_time, unread_counts,
		       is_archived, is_pinned, is_muted, chat_wallpapers, font_sizes,
		       created_at, updated_at
		FROM chats WHERE chat_id = $1`, chatID)

	if err == nil {
		log.Printf("Chat already exists: %s", chatID)
		return &existingChat, nil
	}

	if err != sql.ErrNoRows {
		log.Printf("Error checking existing chat: %v", err)
		return nil, fmt.Errorf("failed to check existing chat: %w", err)
	}

	log.Printf("Creating new chat: %s", chatID)

	// Initialize empty maps with proper types
	unreadCounts := make(models.IntMap)
	isArchived := make(models.BoolMap)
	isPinned := make(models.BoolMap)
	isMuted := make(models.BoolMap)
	chatWallpapers := make(models.StringMap)
	fontSizes := make(models.FloatMap)

	// Initialize with default values for each participant
	for _, participantID := range participants {
		unreadCounts[participantID] = 0
		isArchived[participantID] = false
		isPinned[participantID] = false
		isMuted[participantID] = false
		chatWallpapers[participantID] = ""
		fontSizes[participantID] = 16.0
	}

	// Create new chat
	chat := &models.Chat{
		ChatID:            chatID,
		Participants:      participants,
		LastMessage:       "",
		LastMessageType:   models.MessageTypeText,
		LastMessageSender: "",
		LastMessageTime:   time.Now(),
		UnreadCounts:      unreadCounts,
		IsArchived:        isArchived,
		IsPinned:          isPinned,
		IsMuted:           isMuted,
		ChatWallpapers:    chatWallpapers,
		FontSizes:         fontSizes,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Start transaction
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert chat
	query := `
		INSERT INTO chats (chat_id, participants, last_message, last_message_type,
		                   last_message_sender, last_message_time, unread_counts,
		                   is_archived, is_pinned, is_muted, chat_wallpapers,
		                   font_sizes, created_at, updated_at)
		VALUES (:chat_id, :participants, :last_message, :last_message_type,
		        :last_message_sender, :last_message_time, :unread_counts,
		        :is_archived, :is_pinned, :is_muted, :chat_wallpapers,
		        :font_sizes, :created_at, :updated_at)
		ON CONFLICT (chat_id) DO NOTHING`

	_, err = tx.NamedExec(query, chat)
	if err != nil {
		log.Printf("Failed to create chat: %v", err)
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	// Create chat participants entries
	for _, participantID := range participants {
		var exists bool
		err = tx.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM chat_participants 
				WHERE chat_id = $1 AND user_id = $2
			)`, chatID, participantID).Scan(&exists)

		if err != nil {
			log.Printf("Failed to check chat participant existence: %v", err)
			return nil, fmt.Errorf("failed to check chat participant: %w", err)
		}

		if !exists {
			_, err = tx.Exec(`
				INSERT INTO chat_participants (chat_id, user_id, joined_at)
				VALUES ($1, $2, $3)`,
				chatID, participantID, time.Now())
			if err != nil {
				log.Printf("Failed to create chat participant: %v", err)
				return nil, fmt.Errorf("failed to create chat participant: %w", err)
			}
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit chat creation: %w", err)
	}

	log.Printf("Successfully created chat: %s", chatID)
	return chat, nil
}

// GetChat gets a specific chat
func (s *ChatService) GetChat(chatID string) (*models.Chat, error) {
	log.Printf("Getting chat: %s", chatID)

	var chat models.Chat
	err := s.db.Get(&chat, `
		SELECT chat_id, participants, last_message, last_message_type,
		       last_message_sender, last_message_time, unread_counts,
		       is_archived, is_pinned, is_muted, chat_wallpapers, font_sizes,
		       created_at, updated_at
		FROM chats WHERE chat_id = $1`, chatID)

	if err != nil {
		log.Printf("Failed to get chat %s: %v", chatID, err)
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	return &chat, nil
}

// GetUserChats gets chats for a user
func (s *ChatService) GetUserChats(userID string, limit, offset int) ([]models.Chat, error) {
	log.Printf("Getting chats for user: %s (limit: %d, offset: %d)", userID, limit, offset)

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
		log.Printf("Failed to get user chats for %s: %v", userID, err)
		return nil, fmt.Errorf("failed to get user chats: %w", err)
	}

	log.Printf("Found %d chats for user %s", len(chats), userID)
	return chats, nil
}

// SendMessage sends a message to a chat - FIXED with proper JSONB handling
func (s *ChatService) SendMessage(message *models.Message) error {
	log.Printf("Sending message to chat %s from user %s", message.ChatID, message.SenderID)

	// Validate required fields
	if message.ChatID == "" {
		return fmt.Errorf("chat ID is required")
	}
	if message.SenderID == "" {
		return fmt.Errorf("sender ID is required")
	}
	if message.Content == "" && message.MediaURL == nil {
		return fmt.Errorf("message content or media URL is required")
	}

	// Initialize maps if nil
	if message.Reactions == nil {
		message.Reactions = make(models.StringMap)
	}
	if message.ReadBy == nil {
		message.ReadBy = make(models.TimeMap)
	}
	if message.DeliveredTo == nil {
		message.DeliveredTo = make(models.TimeMap)
	}
	if message.MediaMetadata == nil {
		message.MediaMetadata = make(models.JSONMap)
	}

	tx, err := s.db.Beginx()
	if err != nil {
		log.Printf("Failed to start transaction for message: %v", err)
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			log.Printf("Rolling back transaction due to error: %v", err)
			tx.Rollback()
		}
	}()

	// Check if chat exists, if not try to create it
	var chatExists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM chats WHERE chat_id = $1)", message.ChatID).Scan(&chatExists)
	if err != nil {
		log.Printf("Failed to check if chat exists: %v", err)
		return fmt.Errorf("failed to check chat existence: %w", err)
	}

	if !chatExists {
		log.Printf("Chat does not exist: %s - attempting to create it", message.ChatID)

		participants := strings.Split(message.ChatID, "_")
		if len(participants) == 2 {
			unreadCounts := make(models.IntMap)
			isArchived := make(models.BoolMap)
			isPinned := make(models.BoolMap)
			isMuted := make(models.BoolMap)
			chatWallpapers := make(models.StringMap)
			fontSizes := make(models.FloatMap)

			for _, participantID := range participants {
				unreadCounts[participantID] = 0
				isArchived[participantID] = false
				isPinned[participantID] = false
				isMuted[participantID] = false
				chatWallpapers[participantID] = ""
				fontSizes[participantID] = 16.0
			}

			chat := &models.Chat{
				ChatID:            message.ChatID,
				Participants:      participants,
				LastMessage:       "",
				LastMessageType:   models.MessageTypeText,
				LastMessageSender: "",
				LastMessageTime:   time.Now(),
				UnreadCounts:      unreadCounts,
				IsArchived:        isArchived,
				IsPinned:          isPinned,
				IsMuted:           isMuted,
				ChatWallpapers:    chatWallpapers,
				FontSizes:         fontSizes,
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
				        :font_sizes, :created_at, :updated_at)
				ON CONFLICT (chat_id) DO NOTHING`

			_, err = tx.NamedExec(query, chat)
			if err != nil {
				log.Printf("Failed to auto-create chat: %v", err)
				return fmt.Errorf("failed to auto-create chat: %w", err)
			}

			for _, participantID := range participants {
				_, err = tx.Exec(`
					INSERT INTO chat_participants (chat_id, user_id, joined_at)
					VALUES ($1, $2, $3)
					ON CONFLICT (chat_id, user_id) DO NOTHING`,
					message.ChatID, participantID, time.Now())
				if err != nil {
					log.Printf("Failed to create chat participant: %v", err)
					return fmt.Errorf("failed to create chat participant: %w", err)
				}
			}

			log.Printf("Auto-created chat: %s", message.ChatID)
		} else {
			log.Printf("Invalid chatID format for auto-creation: %s", message.ChatID)
			return fmt.Errorf("chat not found: %s", message.ChatID)
		}
	}

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
		log.Printf("Failed to insert message: %v", err)
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Get display content for chat last message
	displayContent := message.GetDisplayContent()
	if len(displayContent) > 100 {
		displayContent = displayContent[:100] + "..."
	}

	// Update chat's last message info
	_, err = tx.Exec(`
		UPDATE chats 
		SET last_message = $1, last_message_type = $2, last_message_sender = $3,
		    last_message_time = $4, updated_at = $5
		WHERE chat_id = $6`,
		displayContent, message.Type, message.SenderID,
		message.Timestamp, time.Now(), message.ChatID)
	if err != nil {
		log.Printf("Failed to update chat last message: %v", err)
		return fmt.Errorf("failed to update chat last message: %w", err)
	}

	// FIXED: Increment unread count for other participants - Simplified approach
	// Get the chat to access participants
	var chat models.Chat
	err = tx.Get(&chat, "SELECT participants FROM chats WHERE chat_id = $1", message.ChatID)
	if err == nil {
		for _, participant := range chat.Participants {
			if participant != message.SenderID {
				// Update unread count for each other participant
				_, err = tx.Exec(`
                UPDATE chats 
                SET unread_counts = COALESCE(unread_counts, '{}'::jsonb) || 
                    jsonb_build_object($1::text, COALESCE((unread_counts->>$1::text)::int, 0) + 1)
                WHERE chat_id = $2`,
					participant, message.ChatID)
				if err != nil {
					log.Printf("Failed to update unread count for participant %s: %v", participant, err)
					// Don't fail the whole operation for this
				}
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("Failed to commit message transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully sent message %s to chat %s", message.MessageID, message.ChatID)
	return nil
}

// GetMessages gets messages from a chat
func (s *ChatService) GetMessages(chatID string, limit, offset int) ([]models.Message, error) {
	log.Printf("Getting messages for chat %s (limit: %d, offset: %d)", chatID, limit, offset)

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
		log.Printf("Failed to get messages for chat %s: %v", chatID, err)
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	log.Printf("Found %d messages for chat %s", len(messages), chatID)
	return messages, nil
}

// UpdateMessage updates a message
func (s *ChatService) UpdateMessage(messageID, senderID, newContent string) error {
	log.Printf("Updating message %s by user %s", messageID, senderID)

	if newContent == "" {
		return fmt.Errorf("message content cannot be empty")
	}

	result, err := s.db.Exec(`
		UPDATE messages 
		SET content = $1, is_edited = true, edited_at = $2
		WHERE message_id = $3 AND sender_id = $4`,
		newContent, time.Now(), messageID, senderID)

	if err != nil {
		log.Printf("Failed to update message %s: %v", messageID, err)
		return fmt.Errorf("failed to update message: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check update result: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("Message %s not found or permission denied for user %s", messageID, senderID)
		return fmt.Errorf("message not found or permission denied")
	}

	log.Printf("Successfully updated message %s", messageID)
	return nil
}

// DeleteMessage deletes a message
func (s *ChatService) DeleteMessage(messageID, senderID string, deleteForEveryone bool) error {
	log.Printf("Deleting message %s by user %s (deleteForEveryone: %v)", messageID, senderID, deleteForEveryone)

	if deleteForEveryone {
		result, err := s.db.Exec(`
			DELETE FROM messages 
			WHERE message_id = $1 AND sender_id = $2`,
			messageID, senderID)

		if err != nil {
			log.Printf("Failed to delete message %s: %v", messageID, err)
			return fmt.Errorf("failed to delete message: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to check delete result: %w", err)
		}

		if rowsAffected == 0 {
			log.Printf("Message %s not found or permission denied for user %s", messageID, senderID)
			return fmt.Errorf("message not found or permission denied")
		}
	} else {
		return s.DeleteMessage(messageID, senderID, true)
	}

	log.Printf("Successfully deleted message %s", messageID)
	return nil
}

// MarkMessagesAsDelivered marks messages as delivered for a user
func (s *ChatService) MarkMessagesAsDelivered(chatID string, messageIDs []string, userID string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	log.Printf("Marking %d messages as delivered for user %s in chat %s", len(messageIDs), userID, chatID)

	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs)+2)
	args[0] = userID
	args[1] = time.Now().Format(time.RFC3339)

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
		log.Printf("Failed to mark messages as delivered: %v", err)
		return fmt.Errorf("failed to mark messages as delivered: %w", err)
	}

	log.Printf("Successfully marked messages as delivered for user %s", userID)
	return nil
}

// MarkChatAsRead marks a chat as read
func (s *ChatService) MarkChatAsRead(chatID, userID string) error {
	log.Printf("Marking chat %s as read for user %s", chatID, userID)

	tx, err := s.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Reset unread count for user
	_, err = tx.Exec(`
		UPDATE chats 
		SET unread_counts = unread_counts || jsonb_build_object($1, 0),
		    updated_at = $2
		WHERE chat_id = $3`,
		userID, time.Now(), chatID)

	if err != nil {
		log.Printf("Failed to reset unread count: %v", err)
		return fmt.Errorf("failed to mark chat as read: %w", err)
	}

	// Mark all unread messages as read
	_, err = tx.Exec(`
		UPDATE messages 
		SET read_by = COALESCE(read_by, '{}'::jsonb) || 
		    jsonb_build_object($1, $2)
		WHERE chat_id = $3 AND sender_id != $1 
		AND NOT (read_by ? $1)`,
		userID, time.Now().Format(time.RFC3339), chatID)

	if err != nil {
		log.Printf("Failed to mark messages as read: %v", err)
		return fmt.Errorf("failed to mark messages as read: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully marked chat %s as read for user %s", chatID, userID)
	return nil
}

// ToggleChatPin toggles chat pin status - FIXED JSONB handling
func (s *ChatService) ToggleChatPin(chatID, userID string) error {
	log.Printf("Toggling pin status for chat %s and user %s", chatID, userID)

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
		log.Printf("Failed to toggle chat pin: %v", err)
		return fmt.Errorf("failed to toggle chat pin: %w", err)
	}

	log.Printf("Successfully toggled pin status for chat %s", chatID)
	return nil
}

// ToggleChatArchive toggles chat archive status - FIXED JSONB handling
func (s *ChatService) ToggleChatArchive(chatID, userID string) error {
	log.Printf("Toggling archive status for chat %s and user %s", chatID, userID)

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
		log.Printf("Failed to toggle chat archive: %v", err)
		return fmt.Errorf("failed to toggle chat archive: %w", err)
	}

	log.Printf("Successfully toggled archive status for chat %s", chatID)
	return nil
}

// ToggleChatMute toggles chat mute status - FIXED JSONB handling
func (s *ChatService) ToggleChatMute(chatID, userID string) error {
	log.Printf("Toggling mute status for chat %s and user %s", chatID, userID)

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
		log.Printf("Failed to toggle chat mute: %v", err)
		return fmt.Errorf("failed to toggle chat mute: %w", err)
	}

	log.Printf("Successfully toggled mute status for chat %s", chatID)
	return nil
}

// SetChatSettings sets chat settings
func (s *ChatService) SetChatSettings(chatID, userID string, wallpaperURL *string, fontSize *float64) error {
	log.Printf("Setting chat settings for chat %s and user %s", chatID, userID)

	if wallpaperURL != nil {
		_, err := s.db.Exec(`
			UPDATE chats 
			SET chat_wallpapers = COALESCE(chat_wallpapers, '{}'::jsonb) || 
			    jsonb_build_object($1, $2),
			    updated_at = $3
			WHERE chat_id = $4`,
			userID, *wallpaperURL, time.Now(), chatID)
		if err != nil {
			log.Printf("Failed to update wallpaper: %v", err)
			return fmt.Errorf("failed to update wallpaper: %w", err)
		}
	}

	if fontSize != nil {
		_, err := s.db.Exec(`
			UPDATE chats 
			SET font_sizes = COALESCE(font_sizes, '{}'::jsonb) || 
			    jsonb_build_object($1, $2),
			    updated_at = $3
			WHERE chat_id = $4`,
			userID, *fontSize, time.Now(), chatID)
		if err != nil {
			log.Printf("Failed to update font size: %v", err)
			return fmt.Errorf("failed to update font size: %w", err)
		}
	}

	log.Printf("Successfully updated chat settings for chat %s", chatID)
	return nil
}

// SendVideoReactionMessage sends a video reaction message
func (s *ChatService) SendVideoReactionMessage(chatID, senderID string, videoReaction models.VideoReactionMessage) error {
	log.Printf("Sending video reaction message to chat %s from user %s", chatID, senderID)

	metadata := models.JSONMap{
		"isVideoReaction": true,
		"videoId":         videoReaction.VideoID,
		"videoUrl":        videoReaction.VideoURL,
		"thumbnailUrl":    videoReaction.ThumbnailURL,
		"userName":        videoReaction.UserName,
		"userImage":       videoReaction.UserImage,
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

	err := s.SendMessage(message)
	if err != nil {
		log.Printf("Failed to send video reaction message: %v", err)
		return fmt.Errorf("failed to send video reaction: %w", err)
	}

	log.Printf("Successfully sent video reaction message %s", message.MessageID)
	return nil
}

// AreUsersBlocked checks if two users have blocked each other
func (s *ChatService) AreUsersBlocked(userID1, userID2 string) (bool, error) {
	log.Printf("Checking if users %s and %s are blocked", userID1, userID2)

	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM blocked_contacts 
		WHERE (user_id = $1 AND blocked_user_id = $2) 
		   OR (user_id = $2 AND blocked_user_id = $1)`,
		userID1, userID2,
	).Scan(&count)

	if err != nil {
		log.Printf("Failed to check block status: %v", err)
		return false, fmt.Errorf("failed to check block status: %w", err)
	}

	blocked := count > 0
	log.Printf("Users %s and %s blocked status: %v", userID1, userID2, blocked)
	return blocked, nil
}

// Additional helper methods remain unchanged...
// (GetChatParticipants, UpdateChatLastMessageTime, GetUnreadMessagesCount, etc.)
// For brevity, keeping only the critical fixes above
