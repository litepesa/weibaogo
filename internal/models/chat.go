// ===============================
// internal/models/chat.go - Chat and Message Models
// ===============================

package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringMap represents a map[string]string that can be stored in PostgreSQL as JSONB
type StringMap map[string]string

func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *StringMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(StringMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into StringMap", value)
	}
	return json.Unmarshal(bytes, m)
}

// BoolMap represents a map[string]bool that can be stored in PostgreSQL as JSONB
type BoolMap map[string]bool

func (m BoolMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *BoolMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(BoolMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into BoolMap", value)
	}
	return json.Unmarshal(bytes, m)
}

// FloatMap represents a map[string]float64 that can be stored in PostgreSQL as JSONB
type FloatMap map[string]float64

func (m FloatMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *FloatMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(FloatMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into FloatMap", value)
	}
	return json.Unmarshal(bytes, m)
}

// TimeMap represents a map[string]time.Time that can be stored in PostgreSQL as JSONB
type TimeMap map[string]time.Time

func (m TimeMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	// Convert times to RFC3339 strings for JSON storage
	stringMap := make(map[string]string)
	for k, v := range m {
		stringMap[k] = v.Format(time.RFC3339)
	}
	return json.Marshal(stringMap)
}

func (m *TimeMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(TimeMap)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into TimeMap", value)
	}

	var stringMap map[string]string
	if err := json.Unmarshal(bytes, &stringMap); err != nil {
		return err
	}

	*m = make(TimeMap)
	for k, v := range stringMap {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			(*m)[k] = t
		}
	}
	return nil
}

// Chat represents a chat conversation
type Chat struct {
	ChatID            string      `json:"chatId" db:"chat_id"`
	Participants      StringSlice `json:"participants" db:"participants"`
	LastMessage       string      `json:"lastMessage" db:"last_message"`
	LastMessageType   string      `json:"lastMessageType" db:"last_message_type"`
	LastMessageSender string      `json:"lastMessageSender" db:"last_message_sender"`
	LastMessageTime   time.Time   `json:"lastMessageTime" db:"last_message_time"`
	UnreadCounts      IntMap      `json:"unreadCounts" db:"unread_counts"`
	IsArchived        BoolMap     `json:"isArchived" db:"is_archived"`
	IsPinned          BoolMap     `json:"isPinned" db:"is_pinned"`
	IsMuted           BoolMap     `json:"isMuted" db:"is_muted"`
	ChatWallpapers    StringMap   `json:"chatWallpapers" db:"chat_wallpapers"`
	FontSizes         FloatMap    `json:"fontSizes" db:"font_sizes"`
	CreatedAt         time.Time   `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time   `json:"updatedAt" db:"updated_at"`
}

// Message represents a chat message
type Message struct {
	MessageID        string                 `json:"messageId" db:"message_id"`
	ChatID           string                 `json:"chatId" db:"chat_id"`
	SenderID         string                 `json:"senderId" db:"sender_id"`
	Content          string                 `json:"content" db:"content"`
	Type             string                 `json:"type" db:"type"`
	Status           string                 `json:"status" db:"status"`
	Timestamp        time.Time              `json:"timestamp" db:"timestamp"`
	MediaURL         *string                `json:"mediaUrl" db:"media_url"`
	MediaMetadata    map[string]interface{} `json:"mediaMetadata" db:"media_metadata"`
	ReplyToMessageID *string                `json:"replyToMessageId" db:"reply_to_message_id"`
	ReplyToContent   *string                `json:"replyToContent" db:"reply_to_content"`
	ReplyToSender    *string                `json:"replyToSender" db:"reply_to_sender"`
	Reactions        StringMap              `json:"reactions" db:"reactions"`
	IsEdited         bool                   `json:"isEdited" db:"is_edited"`
	EditedAt         *time.Time             `json:"editedAt" db:"edited_at"`
	IsPinned         bool                   `json:"isPinned" db:"is_pinned"`
	ReadBy           TimeMap                `json:"readBy" db:"read_by"`
	DeliveredTo      TimeMap                `json:"deliveredTo" db:"delivered_to"`
	CreatedAt        time.Time              `json:"createdAt" db:"created_at"`
}

// MessageReaction represents a reaction to a message
type MessageReaction struct {
	ID        string    `json:"id" db:"id"`
	MessageID string    `json:"messageId" db:"message_id"`
	UserID    string    `json:"userId" db:"user_id"`
	Emoji     string    `json:"emoji" db:"emoji"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// ChatParticipant represents a participant in a chat
type ChatParticipant struct {
	ID       string     `json:"id" db:"id"`
	ChatID   string     `json:"chatId" db:"chat_id"`
	UserID   string     `json:"userId" db:"user_id"`
	JoinedAt time.Time  `json:"joinedAt" db:"joined_at"`
	LeftAt   *time.Time `json:"leftAt" db:"left_at"`
}

// Helper methods for Chat
func (c *Chat) GetOtherParticipant(currentUserID string) string {
	for _, participant := range c.Participants {
		if participant != currentUserID {
			return participant
		}
	}
	return ""
}

func (c *Chat) GetUnreadCount(userID string) int {
	if c.UnreadCounts == nil {
		return 0
	}
	return c.UnreadCounts[userID]
}

func (c *Chat) IsPinnedForUser(userID string) bool {
	if c.IsPinned == nil {
		return false
	}
	return c.IsPinned[userID]
}

func (c *Chat) IsArchivedForUser(userID string) bool {
	if c.IsArchived == nil {
		return false
	}
	return c.IsArchived[userID]
}

func (c *Chat) IsMutedForUser(userID string) bool {
	if c.IsMuted == nil {
		return false
	}
	return c.IsMuted[userID]
}

func (c *Chat) GetWallpaperForUser(userID string) string {
	if c.ChatWallpapers == nil {
		return ""
	}
	return c.ChatWallpapers[userID]
}

func (c *Chat) GetFontSizeForUser(userID string) float64 {
	if c.FontSizes == nil {
		return 16.0 // default font size
	}
	if size, exists := c.FontSizes[userID]; exists {
		return size
	}
	return 16.0
}

// Helper methods for Message
func (m *Message) IsReadBy(userID string) bool {
	if m.ReadBy == nil {
		return false
	}
	_, exists := m.ReadBy[userID]
	return exists
}

func (m *Message) IsDeliveredTo(userID string) bool {
	if m.DeliveredTo == nil {
		return false
	}
	_, exists := m.DeliveredTo[userID]
	return exists
}

func (m *Message) GetReaction(userID string) string {
	if m.Reactions == nil {
		return ""
	}
	return m.Reactions[userID]
}

func (m *Message) HasReactions() bool {
	return m.Reactions != nil && len(m.Reactions) > 0
}

func (m *Message) IsReply() bool {
	return m.ReplyToMessageID != nil
}

func (m *Message) HasMedia() bool {
	return m.MediaURL != nil && *m.MediaURL != ""
}

func (m *Message) GetDisplayContent() string {
	switch m.Type {
	case "text":
		return m.Content
	case "image":
		if m.Content != "" {
			return m.Content
		}
		return "📷 Photo"
	case "video":
		if m.Content != "" {
			return m.Content
		}
		return "📹 Video"
	case "file":
		fileName := "Document"
		if m.MediaMetadata != nil {
			if fn, ok := m.MediaMetadata["fileName"].(string); ok {
				fileName = fn
			}
		}
		return "📎 " + fileName
	case "audio":
		return "🎤 Voice message"
	case "location":
		return "📍 Location"
	case "contact":
		return "👤 Contact"
	default:
		return m.Content
	}
}

// Request/Response models
type CreateChatRequest struct {
	Participants []string `json:"participants" binding:"required,min=2"`
}

type SendMessageRequest struct {
	Content          string                 `json:"content" binding:"required"`
	Type             string                 `json:"type,omitempty"`
	MediaURL         string                 `json:"mediaUrl,omitempty"`
	MediaMetadata    map[string]interface{} `json:"mediaMetadata,omitempty"`
	ReplyToMessageID string                 `json:"replyToMessageId,omitempty"`
}

type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type MessageStatusUpdate struct {
	Status string `json:"status" binding:"required"`
}

type ChatSettingsRequest struct {
	WallpaperURL *string  `json:"wallpaperUrl"`
	FontSize     *float64 `json:"fontSize"`
}

type VideoReactionMessage struct {
	VideoID      string `json:"videoId" binding:"required"`
	VideoURL     string `json:"videoUrl" binding:"required"`
	ThumbnailURL string `json:"thumbnailUrl" binding:"required"`
	ChannelName  string `json:"channelName" binding:"required"`
	ChannelImage string `json:"channelImage" binding:"required"`
	Reaction     string `json:"reaction,omitempty"`
}

type MomentReactionMessage struct {
	MomentID     string `json:"momentId" binding:"required"`
	MediaURL     string `json:"mediaUrl" binding:"required"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	AuthorName   string `json:"authorName" binding:"required"`
	AuthorImage  string `json:"authorImage" binding:"required"`
	Content      string `json:"content" binding:"required"`
	Reaction     string `json:"reaction" binding:"required"`
	MediaType    string `json:"mediaType" binding:"required"`
}

// Response models
type ChatResponse struct {
	Chat
	ContactName  string `json:"contactName,omitempty"`
	ContactImage string `json:"contactImage,omitempty"`
	ContactPhone string `json:"contactPhone,omitempty"`
	IsOnline     bool   `json:"isOnline"`
}

type MessageResponse struct {
	Message
	SenderName  string `json:"senderName,omitempty"`
	SenderImage string `json:"senderImage,omitempty"`
}

type ChatListResponse struct {
	Chats   []ChatResponse `json:"chats"`
	HasMore bool           `json:"hasMore"`
	Total   int            `json:"total"`
}

type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	HasMore  bool              `json:"hasMore"`
	Total    int               `json:"total"`
}

// Constants
const (
	MessageTypeText     = "text"
	MessageTypeImage    = "image"
	MessageTypeVideo    = "video"
	MessageTypeFile     = "file"
	MessageTypeAudio    = "audio"
	MessageTypeLocation = "location"
	MessageTypeContact  = "contact"

	MessageStatusSending   = "sending"
	MessageStatusSent      = "sent"
	MessageStatusDelivered = "delivered"
	MessageStatusRead      = "read"
	MessageStatusFailed    = "failed"
)
