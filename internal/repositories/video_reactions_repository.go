// ===============================
// internal/repositories/video_reactions_repository.go - Video Reactions Repository
// ===============================

package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"weibaobe/internal/models"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type VideoReactionsRepository struct {
	db *sqlx.DB
}

func NewVideoReactionsRepository(db *sqlx.DB) *VideoReactionsRepository {
	return &VideoReactionsRepository{db: db}
}

// ===============================
// CHAT OPERATIONS
// ===============================

// GenerateChatID generates a unique chat ID for two users and a video
func (r *VideoReactionsRepository) GenerateChatID(user1ID, user2ID, videoID string) string {
	var chatID string
	err := r.db.QueryRow(
		"SELECT generate_video_reaction_chat_id($1, $2, $3::uuid)",
		user1ID, user2ID, videoID,
	).Scan(&chatID)

	if err != nil {
		// Fallback: generate manually if function fails
		sortedIDs := []string{user1ID, user2ID}
		if user1ID > user2ID {
			sortedIDs[0], sortedIDs[1] = user2ID, user1ID
		}
		return fmt.Sprintf("video_reaction_%s_%s_%s", videoID, sortedIDs[0], sortedIDs[1])
	}

	return chatID
}

// CreateChat creates a new video reaction chat
func (r *VideoReactionsRepository) CreateChat(ctx context.Context, chat *models.VideoReactionChat) error {
	query := `
		INSERT INTO video_reaction_chats (
			chat_id, participants, original_video_id, original_video_url,
			original_thumbnail_url, original_user_name, original_user_image,
			original_reaction, original_timestamp, last_message, last_message_type,
			last_message_sender, last_message_time, unread_counts, is_archived,
			is_pinned, is_muted, chat_wallpapers, font_sizes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)
		ON CONFLICT (chat_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, query,
		chat.ChatID, chat.Participants, chat.OriginalVideoID, chat.OriginalVideoURL,
		chat.OriginalThumbnailURL, chat.OriginalUserName, chat.OriginalUserImage,
		chat.OriginalReaction, chat.OriginalTimestamp, chat.LastMessage, chat.LastMessageType,
		chat.LastMessageSender, chat.LastMessageTime, chat.UnreadCounts, chat.IsArchived,
		chat.IsPinned, chat.IsMuted, chat.ChatWallpapers, chat.FontSizes, chat.CreatedAt, chat.UpdatedAt,
	)

	return err
}

// GetChatByID retrieves a chat by its ID
func (r *VideoReactionsRepository) GetChatByID(ctx context.Context, chatID string) (*models.VideoReactionChat, error) {
	var chat models.VideoReactionChat
	query := `
		SELECT chat_id, participants, original_video_id, original_video_url,
		       original_thumbnail_url, original_user_name, original_user_image,
		       original_reaction, original_timestamp, last_message, last_message_type,
		       last_message_sender, last_message_time, unread_counts, is_archived,
		       is_pinned, is_muted, chat_wallpapers, font_sizes, created_at, updated_at
		FROM video_reaction_chats
		WHERE chat_id = $1`

	err := r.db.GetContext(ctx, &chat, query, chatID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &chat, err
}

// GetUserChats retrieves all chats for a user
func (r *VideoReactionsRepository) GetUserChats(ctx context.Context, userID string, limit, offset int) ([]models.VideoReactionChat, error) {
	var chats []models.VideoReactionChat
	query := `
		SELECT chat_id, participants, original_video_id, original_video_url,
		       original_thumbnail_url, original_user_name, original_user_image,
		       original_reaction, original_timestamp, last_message, last_message_type,
		       last_message_sender, last_message_time, unread_counts, is_archived,
		       is_pinned, is_muted, chat_wallpapers, font_sizes, created_at, updated_at
		FROM video_reaction_chats
		WHERE $1 = ANY(participants)
		  AND NOT COALESCE((is_archived->>$1)::boolean, false)
		ORDER BY last_message_time DESC
		LIMIT $2 OFFSET $3`

	err := r.db.SelectContext(ctx, &chats, query, userID, limit, offset)
	return chats, err
}

// GetArchivedChats retrieves archived chats for a user
func (r *VideoReactionsRepository) GetArchivedChats(ctx context.Context, userID string, limit, offset int) ([]models.VideoReactionChat, error) {
	var chats []models.VideoReactionChat
	query := `
		SELECT chat_id, participants, original_video_id, original_video_url,
		       original_thumbnail_url, original_user_name, original_user_image,
		       original_reaction, original_timestamp, last_message, last_message_type,
		       last_message_sender, last_message_time, unread_counts, is_archived,
		       is_pinned, is_muted, chat_wallpapers, font_sizes, created_at, updated_at
		FROM video_reaction_chats
		WHERE $1 = ANY(participants)
		  AND COALESCE((is_archived->>$1)::boolean, false) = true
		ORDER BY last_message_time DESC
		LIMIT $2 OFFSET $3`

	err := r.db.SelectContext(ctx, &chats, query, userID, limit, offset)
	return chats, err
}

// MarkChatAsRead marks all messages in a chat as read for a user
func (r *VideoReactionsRepository) MarkChatAsRead(ctx context.Context, chatID, userID string) error {
	_, err := r.db.ExecContext(ctx, "SELECT mark_video_reaction_chat_as_read($1, $2)", chatID, userID)
	return err
}

// ToggleChatPin toggles the pin status of a chat for a user
func (r *VideoReactionsRepository) ToggleChatPin(ctx context.Context, chatID, userID string) error {
	query := `
		UPDATE video_reaction_chats
		SET is_pinned = jsonb_set(
			COALESCE(is_pinned, '{}'::jsonb),
			ARRAY[$2],
			to_jsonb(NOT COALESCE((is_pinned->>$2)::boolean, false))
		),
		updated_at = CURRENT_TIMESTAMP
		WHERE chat_id = $1`

	_, err := r.db.ExecContext(ctx, query, chatID, userID)
	return err
}

// ToggleChatArchive toggles the archive status of a chat for a user
func (r *VideoReactionsRepository) ToggleChatArchive(ctx context.Context, chatID, userID string) error {
	query := `
		UPDATE video_reaction_chats
		SET is_archived = jsonb_set(
			COALESCE(is_archived, '{}'::jsonb),
			ARRAY[$2],
			to_jsonb(NOT COALESCE((is_archived->>$2)::boolean, false))
		),
		updated_at = CURRENT_TIMESTAMP
		WHERE chat_id = $1`

	_, err := r.db.ExecContext(ctx, query, chatID, userID)
	return err
}

// ToggleChatMute toggles the mute status of a chat for a user
func (r *VideoReactionsRepository) ToggleChatMute(ctx context.Context, chatID, userID string) error {
	query := `
		UPDATE video_reaction_chats
		SET is_muted = jsonb_set(
			COALESCE(is_muted, '{}'::jsonb),
			ARRAY[$2],
			to_jsonb(NOT COALESCE((is_muted->>$2)::boolean, false))
		),
		updated_at = CURRENT_TIMESTAMP
		WHERE chat_id = $1`

	_, err := r.db.ExecContext(ctx, query, chatID, userID)
	return err
}

// UpdateChatSettings updates chat settings for a user
func (r *VideoReactionsRepository) UpdateChatSettings(ctx context.Context, chatID, userID string, wallpaper *string, fontSize *float64) error {
	query := `
		UPDATE video_reaction_chats
		SET chat_wallpapers = CASE 
			WHEN $3 IS NOT NULL THEN jsonb_set(COALESCE(chat_wallpapers, '{}'::jsonb), ARRAY[$2], to_jsonb($3))
			ELSE chat_wallpapers
		END,
		font_sizes = CASE 
			WHEN $4 IS NOT NULL THEN jsonb_set(COALESCE(font_sizes, '{}'::jsonb), ARRAY[$2], to_jsonb($4))
			ELSE font_sizes
		END,
		updated_at = CURRENT_TIMESTAMP
		WHERE chat_id = $1`

	_, err := r.db.ExecContext(ctx, query, chatID, userID, wallpaper, fontSize)
	return err
}

// DeleteChat deletes a chat (soft delete for one user or hard delete for all)
func (r *VideoReactionsRepository) DeleteChat(ctx context.Context, chatID, userID string, deleteForEveryone bool) error {
	if deleteForEveryone {
		// Hard delete - removes the entire chat
		query := `DELETE FROM video_reaction_chats WHERE chat_id = $1`
		_, err := r.db.ExecContext(ctx, query, chatID)
		return err
	}

	// Soft delete - just archive for this user
	return r.ToggleChatArchive(ctx, chatID, userID)
}

// ClearChatHistory clears all messages in a chat for a user
func (r *VideoReactionsRepository) ClearChatHistory(ctx context.Context, chatID, userID string) error {
	// In a real implementation, you might want to mark messages as deleted per user
	// For now, we'll just delete all messages (this affects both users)
	query := `DELETE FROM video_reaction_messages WHERE chat_id = $1`
	_, err := r.db.ExecContext(ctx, query, chatID)
	return err
}

// ===============================
// MESSAGE OPERATIONS
// ===============================

// CreateMessage creates a new message in a chat
func (r *VideoReactionsRepository) CreateMessage(ctx context.Context, message *models.VideoReactionMessage) error {
	query := `
		INSERT INTO video_reaction_messages (
			message_id, chat_id, sender_id, content, type, status, media_url,
			media_metadata, file_name, reply_to_message_id, reply_to_content,
			reply_to_sender, reactions, is_edited, edited_at, is_pinned,
			read_by, delivered_to, video_reaction_data, is_original_reaction, timestamp
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
		)`

	_, err := r.db.ExecContext(ctx, query,
		message.MessageID, message.ChatID, message.SenderID, message.Content, message.Type,
		message.Status, message.MediaURL, message.MediaMetadata, message.FileName,
		message.ReplyToMessageID, message.ReplyToContent, message.ReplyToSender,
		message.Reactions, message.IsEdited, message.EditedAt, message.IsPinned,
		message.ReadBy, message.DeliveredTo, message.VideoReactionData,
		message.IsOriginalReaction, message.Timestamp,
	)

	return err
}

// GetMessageByID retrieves a message by its ID
func (r *VideoReactionsRepository) GetMessageByID(ctx context.Context, messageID string) (*models.VideoReactionMessage, error) {
	var message models.VideoReactionMessage
	query := `
		SELECT message_id, chat_id, sender_id, content, type, status, media_url,
		       media_metadata, file_name, reply_to_message_id, reply_to_content,
		       reply_to_sender, reactions, is_edited, edited_at, is_pinned,
		       read_by, delivered_to, video_reaction_data, is_original_reaction, timestamp
		FROM video_reaction_messages
		WHERE message_id = $1`

	err := r.db.GetContext(ctx, &message, query, messageID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &message, err
}

// GetChatMessages retrieves messages from a chat
func (r *VideoReactionsRepository) GetChatMessages(ctx context.Context, chatID string, limit, offset int) ([]models.VideoReactionMessage, error) {
	var messages []models.VideoReactionMessage
	query := `
		SELECT message_id, chat_id, sender_id, content, type, status, media_url,
		       media_metadata, file_name, reply_to_message_id, reply_to_content,
		       reply_to_sender, reactions, is_edited, edited_at, is_pinned,
		       read_by, delivered_to, video_reaction_data, is_original_reaction, timestamp
		FROM video_reaction_messages
		WHERE chat_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3`

	err := r.db.SelectContext(ctx, &messages, query, chatID, limit, offset)
	return messages, err
}

// GetPinnedMessages retrieves all pinned messages in a chat
func (r *VideoReactionsRepository) GetPinnedMessages(ctx context.Context, chatID string) ([]models.VideoReactionMessage, error) {
	var messages []models.VideoReactionMessage
	query := `
		SELECT message_id, chat_id, sender_id, content, type, status, media_url,
		       media_metadata, file_name, reply_to_message_id, reply_to_content,
		       reply_to_sender, reactions, is_edited, edited_at, is_pinned,
		       read_by, delivered_to, video_reaction_data, is_original_reaction, timestamp
		FROM video_reaction_messages
		WHERE chat_id = $1 AND is_pinned = true
		ORDER BY timestamp DESC
		LIMIT 10`

	err := r.db.SelectContext(ctx, &messages, query, chatID)
	return messages, err
}

// UpdateMessageStatus updates the status of a message
func (r *VideoReactionsRepository) UpdateMessageStatus(ctx context.Context, messageID string, status models.MessageStatus) error {
	query := `UPDATE video_reaction_messages SET status = $1 WHERE message_id = $2`
	_, err := r.db.ExecContext(ctx, query, status, messageID)
	return err
}

// MarkMessageAsDelivered marks a message as delivered to a user
func (r *VideoReactionsRepository) MarkMessageAsDelivered(ctx context.Context, messageID, userID string) error {
	query := `
		UPDATE video_reaction_messages
		SET delivered_to = jsonb_set(
			COALESCE(delivered_to, '{}'::jsonb),
			ARRAY[$2],
			to_jsonb($3)
		),
		status = CASE 
			WHEN status = 'sent' THEN 'delivered'
			ELSE status
		END
		WHERE message_id = $1`

	_, err := r.db.ExecContext(ctx, query, messageID, userID, time.Now())
	return err
}

// MarkMessageAsRead marks a message as read by a user
func (r *VideoReactionsRepository) MarkMessageAsRead(ctx context.Context, messageID, userID string) error {
	query := `
		UPDATE video_reaction_messages
		SET read_by = jsonb_set(
			COALESCE(read_by, '{}'::jsonb),
			ARRAY[$2],
			to_jsonb($3)
		),
		status = 'read'
		WHERE message_id = $1`

	_, err := r.db.ExecContext(ctx, query, messageID, userID, time.Now())
	return err
}

// EditMessage updates the content of a message
func (r *VideoReactionsRepository) EditMessage(ctx context.Context, messageID, newContent string) error {
	query := `
		UPDATE video_reaction_messages
		SET content = $1, is_edited = true, edited_at = $2
		WHERE message_id = $3`

	_, err := r.db.ExecContext(ctx, query, newContent, time.Now(), messageID)
	return err
}

// DeleteMessage deletes a message (or marks as deleted)
func (r *VideoReactionsRepository) DeleteMessage(ctx context.Context, messageID string, deleteForEveryone bool) error {
	if deleteForEveryone {
		query := `DELETE FROM video_reaction_messages WHERE message_id = $1`
		_, err := r.db.ExecContext(ctx, query, messageID)
		return err
	}

	// Soft delete - update content to "Message deleted"
	query := `
		UPDATE video_reaction_messages
		SET content = 'Message deleted', is_edited = true, edited_at = $1
		WHERE message_id = $2`

	_, err := r.db.ExecContext(ctx, query, time.Now(), messageID)
	return err
}

// ToggleMessagePin toggles the pin status of a message
func (r *VideoReactionsRepository) ToggleMessagePin(ctx context.Context, messageID string) error {
	query := `
		UPDATE video_reaction_messages
		SET is_pinned = NOT is_pinned
		WHERE message_id = $1`

	_, err := r.db.ExecContext(ctx, query, messageID)
	return err
}

// AddMessageReaction adds a reaction to a message
func (r *VideoReactionsRepository) AddMessageReaction(ctx context.Context, messageID, userID, reaction string) error {
	query := `
		UPDATE video_reaction_messages
		SET reactions = jsonb_set(
			COALESCE(reactions, '{}'::jsonb),
			ARRAY[$2],
			to_jsonb($3)
		)
		WHERE message_id = $1`

	_, err := r.db.ExecContext(ctx, query, messageID, userID, reaction)
	return err
}

// RemoveMessageReaction removes a reaction from a message
func (r *VideoReactionsRepository) RemoveMessageReaction(ctx context.Context, messageID, userID string) error {
	query := `
		UPDATE video_reaction_messages
		SET reactions = reactions - $2
		WHERE message_id = $1`

	_, err := r.db.ExecContext(ctx, query, messageID, userID)
	return err
}

// SearchMessages searches for messages in a chat
func (r *VideoReactionsRepository) SearchMessages(ctx context.Context, chatID, searchQuery string, limit int) ([]models.VideoReactionMessage, error) {
	var messages []models.VideoReactionMessage
	query := `
		SELECT message_id, chat_id, sender_id, content, type, status, media_url,
		       media_metadata, file_name, reply_to_message_id, reply_to_content,
		       reply_to_sender, reactions, is_edited, edited_at, is_pinned,
		       read_by, delivered_to, video_reaction_data, is_original_reaction, msg_timestamp as timestamp
		FROM search_video_reaction_messages($1, $2, $3)`

	err := r.db.SelectContext(ctx, &messages, query, chatID, searchQuery, limit)
	return messages, err
}

// ===============================
// WEBSOCKET CONNECTION OPERATIONS
// ===============================

// CreateConnection creates a new WebSocket connection record
func (r *VideoReactionsRepository) CreateConnection(ctx context.Context, conn *models.WebSocketConnection) error {
	query := `
		INSERT INTO websocket_connections (
			connection_id, user_id, socket_id, connected_at, last_heartbeat,
			device_type, platform, app_version, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		conn.ConnectionID, conn.UserID, conn.SocketID, conn.ConnectedAt,
		conn.LastHeartbeat, conn.DeviceType, conn.Platform, conn.AppVersion, conn.IsActive,
	)

	return err
}

// GetActiveConnections retrieves all active connections for a user
func (r *VideoReactionsRepository) GetActiveConnections(ctx context.Context, userID string) ([]models.WebSocketConnection, error) {
	var connections []models.WebSocketConnection
	query := `
		SELECT connection_id, user_id, socket_id, connected_at, last_heartbeat,
		       device_type, platform, app_version, is_active
		FROM websocket_connections
		WHERE user_id = $1 AND is_active = true`

	err := r.db.SelectContext(ctx, &connections, query, userID)
	return connections, err
}

// UpdateHeartbeat updates the last heartbeat time for a connection
func (r *VideoReactionsRepository) UpdateHeartbeat(ctx context.Context, socketID string) error {
	_, err := r.db.ExecContext(ctx, "SELECT update_websocket_heartbeat($1)", socketID)
	return err
}

// DeactivateConnection marks a connection as inactive
func (r *VideoReactionsRepository) DeactivateConnection(ctx context.Context, socketID string) error {
	query := `UPDATE websocket_connections SET is_active = false WHERE socket_id = $1`
	_, err := r.db.ExecContext(ctx, query, socketID)
	return err
}

// CleanupStaleConnections removes stale connections
func (r *VideoReactionsRepository) CleanupStaleConnections(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "SELECT cleanup_stale_websocket_connections()")
	return err
}

// ===============================
// TYPING INDICATOR OPERATIONS
// ===============================

// SetTypingIndicator sets or updates a typing indicator
func (r *VideoReactionsRepository) SetTypingIndicator(ctx context.Context, chatID, userID string, isTyping bool) error {
	if !isTyping {
		// Remove typing indicator
		query := `DELETE FROM typing_indicators WHERE chat_id = $1 AND user_id = $2`
		_, err := r.db.ExecContext(ctx, query, chatID, userID)
		return err
	}

	// Insert or update typing indicator
	query := `
		INSERT INTO typing_indicators (id, chat_id, user_id, is_typing, started_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (chat_id, user_id) 
		DO UPDATE SET is_typing = $4, started_at = $5, expires_at = $6`

	id := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(10 * time.Second)

	_, err := r.db.ExecContext(ctx, query, id, chatID, userID, isTyping, now, expiresAt)
	return err
}

// GetTypingUsers retrieves users currently typing in a chat
func (r *VideoReactionsRepository) GetTypingUsers(ctx context.Context, chatID string) ([]string, error) {
	var userIDs []string
	query := `
		SELECT user_id 
		FROM typing_indicators 
		WHERE chat_id = $1 AND expires_at > CURRENT_TIMESTAMP`

	err := r.db.SelectContext(ctx, &userIDs, query, chatID)
	return userIDs, err
}

// CleanupExpiredTypingIndicators removes expired typing indicators
func (r *VideoReactionsRepository) CleanupExpiredTypingIndicators(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "SELECT cleanup_expired_typing_indicators()")
	return err
}

// ===============================
// STATISTICS
// ===============================

// GetChatStats retrieves statistics for a chat
func (r *VideoReactionsRepository) GetChatStats(ctx context.Context, chatID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get message count
	var messageCount int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM video_reaction_messages WHERE chat_id = $1",
		chatID).Scan(&messageCount)
	if err != nil {
		return nil, err
	}
	stats["messageCount"] = messageCount

	// Get media message count
	var mediaCount int
	err = r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM video_reaction_messages WHERE chat_id = $1 AND type IN ('image', 'video', 'file')",
		chatID).Scan(&mediaCount)
	if err != nil {
		return nil, err
	}
	stats["mediaCount"] = mediaCount

	return stats, nil
}

// GetUserChatStats retrieves chat statistics for a user
func (r *VideoReactionsRepository) GetUserChatStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get total chats
	var totalChats int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM video_reaction_chats WHERE $1 = ANY(participants)",
		userID).Scan(&totalChats)
	if err != nil {
		return nil, err
	}
	stats["totalChats"] = totalChats

	// Get unread count
	var unreadCount int
	err = r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM((unread_counts->>$1)::int), 0) FROM video_reaction_chats WHERE $1 = ANY(participants)",
		userID).Scan(&unreadCount)
	if err != nil {
		return nil, err
	}
	stats["unreadCount"] = unreadCount

	return stats, nil
}
