// internal/handlers/websocket_chat_handler.go
// Real-time WebSocket handler for chat system
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"weibaobe/internal/models"
	"weibaobe/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket connection manager
type ConnectionManager struct {
	connections map[string]*websocket.Conn // userID -> connection
	userChats   map[string][]string        // userID -> chatIDs
	chatUsers   map[string][]string        // chatID -> userIDs
	mutex       sync.RWMutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*websocket.Conn),
		userChats:   make(map[string][]string),
		chatUsers:   make(map[string][]string),
	}
}

type WebSocketChatHandler struct {
	chatService *services.ChatService
	userService *services.UserService
	connManager *ConnectionManager
	upgrader    websocket.Upgrader
}

func NewWebSocketChatHandler(chatService *services.ChatService, userService *services.UserService) *WebSocketChatHandler {
	return &WebSocketChatHandler{
		chatService: chatService,
		userService: userService,
		connManager: NewConnectionManager(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins in development - restrict in production
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// WebSocket message types
type WSMessageType string

const (
	WSMessageTypeAuth            WSMessageType = "auth"
	WSMessageTypeJoinChats       WSMessageType = "join_chats"
	WSMessageTypeLeaveChats      WSMessageType = "leave_chats"
	WSMessageTypeSendMessage     WSMessageType = "send_message"
	WSMessageTypeMessageReceived WSMessageType = "message_received"
	WSMessageTypeMessageSent     WSMessageType = "message_sent"
	WSMessageTypeMessageFailed   WSMessageType = "message_failed"
	WSMessageTypeTypingStart     WSMessageType = "typing_start"
	WSMessageTypeTypingStop      WSMessageType = "typing_stop"
	WSMessageTypeUserOnline      WSMessageType = "user_online"
	WSMessageTypeUserOffline     WSMessageType = "user_offline"
	WSMessageTypeChatUpdated     WSMessageType = "chat_updated"
	WSMessageTypeError           WSMessageType = "error"
	WSMessageTypePing            WSMessageType = "ping"
	WSMessageTypePong            WSMessageType = "pong"
)

// WebSocket message structure
type WSMessage struct {
	Type      WSMessageType   `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	ChatID    string          `json:"chatId,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	RequestID string          `json:"requestId,omitempty"`
}

// Specific message data structures
type WSAuthData struct {
	UserID string `json:"userId"`
	Token  string `json:"token"`
}

type WSJoinChatsData struct {
	ChatIDs []string `json:"chatIds"`
}

type WSSendMessageData struct {
	Message *models.Message `json:"message"`
}

type WSTypingData struct {
	ChatID   string `json:"chatId"`
	UserID   string `json:"userId"`
	IsTyping bool   `json:"isTyping"`
}

type WSUserStatusData struct {
	UserID   string `json:"userId"`
	IsOnline bool   `json:"isOnline"`
}

type WSErrorData struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Handle WebSocket connection
func (h *WebSocketChatHandler) HandleWebSocket(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}
	defer conn.Close()

	// Set connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Handle connection
	h.handleConnection(conn)
}

func (h *WebSocketChatHandler) handleConnection(conn *websocket.Conn) {
	var userID string
	var authenticated bool

	// Set up ping handler
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Handle incoming messages
	for {
		select {
		case <-ticker.C:
			// Send ping
			if err := h.sendMessage(conn, WSMessage{
				Type:      WSMessageTypePing,
				Timestamp: time.Now(),
			}); err != nil {
				log.Printf("Failed to send ping: %v", err)
				return
			}

		default:
			// Read message
			var wsMsg WSMessage
			if err := conn.ReadJSON(&wsMsg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				break
			}

			// Reset read deadline
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Handle message based on type
			switch wsMsg.Type {
			case WSMessageTypeAuth:
				userID, authenticated = h.handleAuth(conn, wsMsg)
				if authenticated {
					h.connManager.addConnection(userID, conn)
					h.broadcastUserStatus(userID, true)
				}

			case WSMessageTypeJoinChats:
				if !authenticated {
					h.sendError(conn, "Not authenticated", "AUTH_REQUIRED")
					continue
				}
				h.handleJoinChats(conn, userID, wsMsg)

			case WSMessageTypeLeaveChats:
				if !authenticated {
					h.sendError(conn, "Not authenticated", "AUTH_REQUIRED")
					continue
				}
				h.handleLeaveChats(userID, wsMsg)

			case WSMessageTypeSendMessage:
				if !authenticated {
					h.sendError(conn, "Not authenticated", "AUTH_REQUIRED")
					continue
				}
				h.handleSendMessage(userID, wsMsg)

			case WSMessageTypeTypingStart, WSMessageTypeTypingStop:
				if !authenticated {
					h.sendError(conn, "Not authenticated", "AUTH_REQUIRED")
					continue
				}
				h.handleTyping(userID, wsMsg)

			case WSMessageTypePong:
				// Pong received, connection is alive

			default:
				h.sendError(conn, "Unknown message type", "INVALID_MESSAGE_TYPE")
			}
		}
	}

	// Cleanup on disconnect
	if authenticated && userID != "" {
		h.connManager.removeConnection(userID)
		h.broadcastUserStatus(userID, false)
	}
}

func (h *WebSocketChatHandler) handleAuth(conn *websocket.Conn, wsMsg WSMessage) (string, bool) {
	var authData WSAuthData
	if err := json.Unmarshal(wsMsg.Data, &authData); err != nil {
		h.sendError(conn, "Invalid auth data", "INVALID_AUTH_DATA")
		return "", false
	}

	// TODO: Verify token with Firebase or your auth service
	// For now, just check if userID is provided
	if authData.UserID == "" {
		h.sendError(conn, "User ID required", "USER_ID_REQUIRED")
		return "", false
	}

	log.Printf("User %s authenticated via WebSocket", authData.UserID)
	return authData.UserID, true
}

func (h *WebSocketChatHandler) handleJoinChats(conn *websocket.Conn, userID string, wsMsg WSMessage) {
	var joinData WSJoinChatsData
	if err := json.Unmarshal(wsMsg.Data, &joinData); err != nil {
		h.sendError(conn, "Invalid join data", "INVALID_JOIN_DATA")
		return
	}

	h.connManager.joinChats(userID, joinData.ChatIDs)
	log.Printf("User %s joined chats: %v", userID, joinData.ChatIDs)
}

func (h *WebSocketChatHandler) handleLeaveChats(userID string, wsMsg WSMessage) {
	var leaveData WSJoinChatsData // Same structure
	if err := json.Unmarshal(wsMsg.Data, &leaveData); err != nil {
		return
	}

	h.connManager.leaveChats(userID, leaveData.ChatIDs)
	log.Printf("User %s left chats: %v", userID, leaveData.ChatIDs)
}

func (h *WebSocketChatHandler) handleSendMessage(userID string, wsMsg WSMessage) {
	var msgData WSSendMessageData
	if err := json.Unmarshal(wsMsg.Data, &msgData); err != nil {
		h.sendErrorToUser(userID, "Invalid message data", "INVALID_MESSAGE_DATA")
		return
	}

	message := msgData.Message
	message.SenderID = userID // Ensure sender is authenticated user
	message.Status = models.MessageStatusSent
	message.Timestamp = time.Now()

	// Save message to database
	if err := h.chatService.SendMessage(message); err != nil {
		log.Printf("Failed to save message: %v", err)
		h.sendMessageToUser(userID, WSMessage{
			Type:      WSMessageTypeMessageFailed,
			Data:      mustMarshal(message),
			ChatID:    message.ChatID,
			Timestamp: time.Now(),
			RequestID: wsMsg.RequestID,
		})
		return
	}

	// Broadcast to all chat participants
	h.broadcastToChatParticipants(message.ChatID, WSMessage{
		Type:      WSMessageTypeMessageReceived,
		Data:      mustMarshal(message),
		ChatID:    message.ChatID,
		Timestamp: time.Now(),
	}, userID) // Exclude sender

	// Confirm to sender
	h.sendMessageToUser(userID, WSMessage{
		Type:      WSMessageTypeMessageSent,
		Data:      mustMarshal(message),
		ChatID:    message.ChatID,
		Timestamp: time.Now(),
		RequestID: wsMsg.RequestID,
	})

	log.Printf("Message sent from %s to chat %s", userID, message.ChatID)
}

func (h *WebSocketChatHandler) handleTyping(userID string, wsMsg WSMessage) {
	var typingData WSTypingData
	if err := json.Unmarshal(wsMsg.Data, &typingData); err != nil {
		return
	}

	typingData.UserID = userID // Ensure correct user ID

	// Broadcast typing status to other chat participants
	h.broadcastToChatParticipants(typingData.ChatID, WSMessage{
		Type:      wsMsg.Type,
		Data:      mustMarshal(typingData),
		ChatID:    typingData.ChatID,
		Timestamp: time.Now(),
	}, userID) // Exclude sender
}

// Connection manager methods
func (cm *ConnectionManager) addConnection(userID string, conn *websocket.Conn) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.connections[userID] = conn
}

func (cm *ConnectionManager) removeConnection(userID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.connections, userID)
	delete(cm.userChats, userID)

	// Remove user from chat participants
	for chatID, users := range cm.chatUsers {
		for i, user := range users {
			if user == userID {
				cm.chatUsers[chatID] = append(users[:i], users[i+1:]...)
				break
			}
		}
	}
}

func (cm *ConnectionManager) joinChats(userID string, chatIDs []string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.userChats[userID] = chatIDs

	for _, chatID := range chatIDs {
		users := cm.chatUsers[chatID]
		// Add user if not already in chat
		found := false
		for _, user := range users {
			if user == userID {
				found = true
				break
			}
		}
		if !found {
			cm.chatUsers[chatID] = append(users, userID)
		}
	}
}

func (cm *ConnectionManager) leaveChats(userID string, chatIDs []string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Remove from user's chat list
	if userChats, exists := cm.userChats[userID]; exists {
		filtered := make([]string, 0)
		for _, chatID := range userChats {
			shouldRemove := false
			for _, leaveChatID := range chatIDs {
				if chatID == leaveChatID {
					shouldRemove = true
					break
				}
			}
			if !shouldRemove {
				filtered = append(filtered, chatID)
			}
		}
		cm.userChats[userID] = filtered
	}

	// Remove from chat participants
	for _, chatID := range chatIDs {
		if users, exists := cm.chatUsers[chatID]; exists {
			filtered := make([]string, 0)
			for _, user := range users {
				if user != userID {
					filtered = append(filtered, user)
				}
			}
			cm.chatUsers[chatID] = filtered
		}
	}
}

func (cm *ConnectionManager) getChatParticipants(chatID string) []string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	if users, exists := cm.chatUsers[chatID]; exists {
		return append([]string(nil), users...) // Return copy
	}
	return []string{}
}

func (cm *ConnectionManager) getConnection(userID string) *websocket.Conn {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.connections[userID]
}

func (cm *ConnectionManager) getAllConnections() map[string]*websocket.Conn {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	result := make(map[string]*websocket.Conn)
	for userID, conn := range cm.connections {
		result[userID] = conn
	}
	return result
}

// Broadcasting methods
func (h *WebSocketChatHandler) broadcastToChatParticipants(chatID string, message WSMessage, excludeUserID string) {
	participants := h.connManager.getChatParticipants(chatID)

	for _, userID := range participants {
		if userID != excludeUserID {
			h.sendMessageToUser(userID, message)
		}
	}
}

func (h *WebSocketChatHandler) broadcastUserStatus(userID string, isOnline bool) {
	statusData := WSUserStatusData{
		UserID:   userID,
		IsOnline: isOnline,
	}

	message := WSMessage{
		Type:      WSMessageTypeUserOnline,
		Data:      mustMarshal(statusData),
		Timestamp: time.Now(),
	}

	if !isOnline {
		message.Type = WSMessageTypeUserOffline
	}

	// Broadcast to all connected users
	connections := h.connManager.getAllConnections()
	for otherUserID, conn := range connections {
		if otherUserID != userID {
			h.sendMessage(conn, message)
		}
	}
}

func (h *WebSocketChatHandler) sendMessageToUser(userID string, message WSMessage) {
	conn := h.connManager.getConnection(userID)
	if conn != nil {
		h.sendMessage(conn, message)
	}
}

func (h *WebSocketChatHandler) sendMessage(conn *websocket.Conn, message WSMessage) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(message)
}

func (h *WebSocketChatHandler) sendError(conn *websocket.Conn, message, code string) {
	errorMsg := WSMessage{
		Type: WSMessageTypeError,
		Data: mustMarshal(WSErrorData{
			Message: message,
			Code:    code,
		}),
		Timestamp: time.Now(),
	}
	h.sendMessage(conn, errorMsg)
}

func (h *WebSocketChatHandler) sendErrorToUser(userID, message, code string) {
	conn := h.connManager.getConnection(userID)
	if conn != nil {
		h.sendError(conn, message, code)
	}
}

// Utility function
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		return json.RawMessage(`{}`)
	}
	return data
}

// Public methods for external message broadcasting
func (h *WebSocketChatHandler) BroadcastChatUpdate(chatID string, chat *models.Chat) {
	message := WSMessage{
		Type:      WSMessageTypeChatUpdated,
		Data:      mustMarshal(chat),
		ChatID:    chatID,
		Timestamp: time.Now(),
	}

	h.broadcastToChatParticipants(chatID, message, "")
}

func (h *WebSocketChatHandler) BroadcastMessage(message *models.Message) {
	wsMessage := WSMessage{
		Type:      WSMessageTypeMessageReceived,
		Data:      mustMarshal(message),
		ChatID:    message.ChatID,
		Timestamp: time.Now(),
	}

	h.broadcastToChatParticipants(message.ChatID, wsMessage, message.SenderID)
}

// Get connection statistics
func (h *WebSocketChatHandler) GetStats() map[string]interface{} {
	h.connManager.mutex.RLock()
	defer h.connManager.mutex.RUnlock()

	return map[string]interface{}{
		"active_connections": len(h.connManager.connections),
		"active_chats":       len(h.connManager.chatUsers),
		"total_participants": func() int {
			total := 0
			for _, users := range h.connManager.chatUsers {
				total += len(users)
			}
			return total
		}(),
	}
}
