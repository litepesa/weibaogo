// ===============================
// internal/models/video_reaction.go - Video Reactions Models
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// ===============================
// VIDEO REACTION MODEL
// ===============================

type VideoReaction struct {
	VideoID      string    `json:"videoId" db:"video_id"`
	VideoURL     string    `json:"videoUrl" db:"video_url"`
	ThumbnailURL string    `json:"thumbnailUrl" db:"thumbnail_url"`
	UserName     string    `json:"userName" db:"user_name"`
	UserImage    string    `json:"userImage" db:"user_image"`
	Reaction     *string   `json:"reaction" db:"reaction"`
	Timestamp    time.Time `json:"timestamp" db:"timestamp"`
}

// ===============================
// VIDEO REACTION CHAT MODEL
// ===============================

type VideoReactionChat struct {
	ChatID               string      `json:"chatId" db:"chat_id"`
	Participants         StringSlice `json:"participants" db:"participants"`
	OriginalVideoID      string      `json:"originalVideoId" db:"original_video_id"`
	OriginalVideoURL     string      `json:"originalVideoUrl" db:"original_video_url"`
	OriginalThumbnailURL string      `json:"originalThumbnailUrl" db:"original_thumbnail_url"`
	OriginalUserName     string      `json:"originalUserName" db:"original_user_name"`
	OriginalUserImage    string      `json:"originalUserImage" db:"original_user_image"`
	OriginalReaction     *string     `json:"originalReaction" db:"original_reaction"`
	OriginalTimestamp    time.Time   `json:"originalTimestamp" db:"original_timestamp"`
	LastMessage          string      `json:"lastMessage" db:"last_message"`
	LastMessageType      string      `json:"lastMessageType" db:"last_message_type"`
	LastMessageSender    string      `json:"lastMessageSender" db:"last_message_sender"`
	LastMessageTime      time.Time   `json:"lastMessageTime" db:"last_message_time"`
	UnreadCounts         IntMap      `json:"unreadCounts" db:"unread_counts"`
	IsArchived           BoolMap     `json:"isArchived" db:"is_archived"`
	IsPinned             BoolMap     `json:"isPinned" db:"is_pinned"`
	IsMuted              BoolMap     `json:"isMuted" db:"is_muted"`
	ChatWallpapers       StringMap   `json:"chatWallpapers" db:"chat_wallpapers"`
	FontSizes            Float64Map  `json:"fontSizes" db:"font_sizes"`
	CreatedAt            time.Time   `json:"createdAt" db:"created_at"`
	UpdatedAt            time.Time   `json:"updatedAt" db:"updated_at"`
}

// Helper methods
func (c *VideoReactionChat) GetOtherParticipant(currentUserID string) string {
	for _, participant := range c.Participants {
		if participant != currentUserID {
			return participant
		}
	}
	return ""
}

func (c *VideoReactionChat) GetUnreadCount(userID string) int {
	if count, ok := c.UnreadCounts[userID]; ok {
		return count
	}
	return 0
}

func (c *VideoReactionChat) IsPinnedForUser(userID string) bool {
	if pinned, ok := c.IsPinned[userID]; ok {
		return pinned
	}
	return false
}

func (c *VideoReactionChat) IsArchivedForUser(userID string) bool {
	if archived, ok := c.IsArchived[userID]; ok {
		return archived
	}
	return false
}

func (c *VideoReactionChat) IsMutedForUser(userID string) bool {
	if muted, ok := c.IsMuted[userID]; ok {
		return muted
	}
	return false
}

// ===============================
// VIDEO REACTION MESSAGE MODEL
// ===============================

type VideoReactionMessage struct {
	MessageID          string                 `json:"messageId" db:"message_id"`
	ChatID             string                 `json:"chatId" db:"chat_id"`
	SenderID           string                 `json:"senderId" db:"sender_id"`
	Content            string                 `json:"content" db:"content"`
	Type               MessageType            `json:"type" db:"type"`
	Status             MessageStatus          `json:"status" db:"status"`
	MediaURL           *string                `json:"mediaUrl" db:"media_url"`
	MediaMetadata      map[string]interface{} `json:"mediaMetadata" db:"media_metadata"`
	FileName           *string                `json:"fileName" db:"file_name"`
	ReplyToMessageID   *string                `json:"replyToMessageId" db:"reply_to_message_id"`
	ReplyToContent     *string                `json:"replyToContent" db:"reply_to_content"`
	ReplyToSender      *string                `json:"replyToSender" db:"reply_to_sender"`
	Reactions          map[string]string      `json:"reactions" db:"reactions"`
	IsEdited           bool                   `json:"isEdited" db:"is_edited"`
	EditedAt           *time.Time             `json:"editedAt" db:"edited_at"`
	IsPinned           bool                   `json:"isPinned" db:"is_pinned"`
	ReadBy             map[string]time.Time   `json:"readBy" db:"read_by"`
	DeliveredTo        map[string]time.Time   `json:"deliveredTo" db:"delivered_to"`
	VideoReactionData  *VideoReaction         `json:"videoReactionData" db:"video_reaction_data"`
	IsOriginalReaction bool                   `json:"isOriginalReaction" db:"is_original_reaction"`
	Timestamp          time.Time              `json:"timestamp" db:"timestamp"`
}

// Message types
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeImage    MessageType = "image"
	MessageTypeVideo    MessageType = "video"
	MessageTypeFile     MessageType = "file"
	MessageTypeAudio    MessageType = "audio"
	MessageTypeLocation MessageType = "location"
	MessageTypeContact  MessageType = "contact"
)

// Message status
type MessageStatus string

const (
	MessageStatusSending   MessageStatus = "sending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

// Helper methods
func (m *VideoReactionMessage) IsReadBy(userID string) bool {
	_, ok := m.ReadBy[userID]
	return ok
}

func (m *VideoReactionMessage) IsDeliveredTo(userID string) bool {
	_, ok := m.DeliveredTo[userID]
	return ok
}

func (m *VideoReactionMessage) GetDisplayContent() string {
	if m.IsOriginalReaction && m.VideoReactionData != nil {
		if m.VideoReactionData.Reaction != nil && *m.VideoReactionData.Reaction != "" {
			return *m.VideoReactionData.Reaction
		}
		return "Shared a video"
	}

	switch m.Type {
	case MessageTypeText:
		return m.Content
	case MessageTypeImage:
		if m.Content != "" {
			return m.Content
		}
		return "📷 Photo"
	case MessageTypeVideo:
		if m.Content != "" {
			return m.Content
		}
		return "📹 Video"
	case MessageTypeFile:
		if m.FileName != nil && *m.FileName != "" {
			return "📎 " + *m.FileName
		}
		return "📎 " + m.Content
	case MessageTypeAudio:
		return "🎤 Voice message"
	case MessageTypeLocation:
		return "📍 Location"
	case MessageTypeContact:
		return "👤 Contact"
	default:
		return m.Content
	}
}

// ===============================
// WEBSOCKET CONNECTION MODEL
// ===============================

type WebSocketConnection struct {
	ConnectionID  string    `json:"connectionId" db:"connection_id"`
	UserID        string    `json:"userId" db:"user_id"`
	SocketID      string    `json:"socketId" db:"socket_id"`
	ConnectedAt   time.Time `json:"connectedAt" db:"connected_at"`
	LastHeartbeat time.Time `json:"lastHeartbeat" db:"last_heartbeat"`
	DeviceType    *string   `json:"deviceType" db:"device_type"`
	Platform      *string   `json:"platform" db:"platform"`
	AppVersion    *string   `json:"appVersion" db:"app_version"`
	IsActive      bool      `json:"isActive" db:"is_active"`
}

// ===============================
// TYPING INDICATOR MODEL
// ===============================

type TypingIndicator struct {
	ID        string    `json:"id" db:"id"`
	ChatID    string    `json:"chatId" db:"chat_id"`
	UserID    string    `json:"userId" db:"user_id"`
	IsTyping  bool      `json:"isTyping" db:"is_typing"`
	StartedAt time.Time `json:"startedAt" db:"started_at"`
	ExpiresAt time.Time `json:"expiresAt" db:"expires_at"`
}

// ===============================
// REQUEST/RESPONSE MODELS
// ===============================

type CreateVideoReactionChatRequest struct {
	VideoOwnerID  string        `json:"videoOwnerId" binding:"required"`
	VideoReaction VideoReaction `json:"videoReaction" binding:"required"`
}

type SendMessageRequest struct {
	Content          string                 `json:"content" binding:"required"`
	Type             MessageType            `json:"type"`
	MediaURL         *string                `json:"mediaUrl"`
	MediaMetadata    map[string]interface{} `json:"mediaMetadata"`
	FileName         *string                `json:"fileName"`
	ReplyToMessageID *string                `json:"replyToMessageId"`
}

type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type MessageReactionRequest struct {
	Reaction string `json:"reaction" binding:"required"`
}

type ToggleChatSettingRequest struct {
	Value bool `json:"value"`
}

type SearchMessagesRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}

type ChatSettingsRequest struct {
	Wallpaper *string  `json:"wallpaper"`
	FontSize  *float64 `json:"fontSize"`
}

// ===============================
// RESPONSE MODELS
// ===============================

type VideoReactionChatResponse struct {
	VideoReactionChat
	OtherParticipantName  string `json:"otherParticipantName"`
	OtherParticipantImage string `json:"otherParticipantImage"`
}

type VideoReactionMessageResponse struct {
	VideoReactionMessage
	SenderName  string `json:"senderName"`
	SenderImage string `json:"senderImage"`
}

type ChatsListResponse struct {
	Chats       []VideoReactionChatResponse `json:"chats"`
	Total       int                         `json:"total"`
	HasMore     bool                        `json:"hasMore"`
	UnreadCount int                         `json:"unreadCount"`
}

type MessagesListResponse struct {
	Messages       []VideoReactionMessageResponse `json:"messages"`
	Total          int                            `json:"total"`
	HasMore        bool                           `json:"hasMore"`
	PinnedMessages []VideoReactionMessageResponse `json:"pinnedMessages"`
}

// ===============================
// CUSTOM TYPES FOR JSONB FIELDS
// ===============================

type IntMap map[string]int
type BoolMap map[string]bool
type StringMap map[string]string
type Float64Map map[string]float64
type TimeMap map[string]time.Time
type InterfaceMap map[string]interface{}

// ===============================
// JSONB SCANNING HELPERS
// ===============================

// IntMap (map[string]int)
func (m *IntMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(IntMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan IntMap")
	}
	return json.Unmarshal(bytes, m)
}

func (m IntMap) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(IntMap{})
	}
	return json.Marshal(m)
}

// BoolMap (map[string]bool)
func (m *BoolMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(BoolMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan BoolMap")
	}
	return json.Unmarshal(bytes, m)
}

func (m BoolMap) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(BoolMap{})
	}
	return json.Marshal(m)
}

// StringMap (map[string]string)
func (m *StringMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(StringMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan StringMap")
	}
	return json.Unmarshal(bytes, m)
}

func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(StringMap{})
	}
	return json.Marshal(m)
}

// Float64Map (map[string]float64)
func (m *Float64Map) Scan(value interface{}) error {
	if value == nil {
		*m = make(Float64Map)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan Float64Map")
	}
	return json.Unmarshal(bytes, m)
}

func (m Float64Map) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(Float64Map{})
	}
	return json.Marshal(m)
}

// TimeMap (map[string]time.Time)
func (m *TimeMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(TimeMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan TimeMap")
	}

	// Parse as map[string]string first, then convert to time.Time
	var temp map[string]string
	if err := json.Unmarshal(bytes, &temp); err != nil {
		return err
	}

	*m = make(TimeMap)
	for k, v := range temp {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return err
		}
		(*m)[k] = t
	}
	return nil
}

func (m TimeMap) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(map[string]string{})
	}

	// Convert time.Time to string for JSON
	temp := make(map[string]string)
	for k, v := range m {
		temp[k] = v.Format(time.RFC3339)
	}
	return json.Marshal(temp)
}

// InterfaceMap (map[string]interface{})
func (m *InterfaceMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(InterfaceMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan InterfaceMap")
	}
	return json.Unmarshal(bytes, m)
}

func (m InterfaceMap) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(InterfaceMap{})
	}
	return json.Marshal(m)
}

// VideoReaction JSONB scanning
func (v *VideoReaction) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan VideoReaction")
	}
	return json.Unmarshal(bytes, v)
}

func (v VideoReaction) Value() (driver.Value, error) {
	return json.Marshal(v)
}
