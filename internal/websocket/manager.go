// ===============================
// internal/websocket/manager.go - WebSocket Connection Manager
// ===============================

package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jmoiron/sqlx"
)

// ===============================
// MESSAGE TYPES
// ===============================

type MessageType string

const (
	// Connection events
	TypeConnectionEstablished MessageType = "connection_established"
	TypePing                  MessageType = "ping"
	TypePong                  MessageType = "pong"
	TypeError                 MessageType = "error"

	// Chat events
	TypeChatCreated MessageType = "chat_created"
	TypeChatUpdated MessageType = "chat_updated"
	TypeChatDeleted MessageType = "chat_deleted"

	// Message events
	TypeMessageReceived  MessageType = "message_received"
	TypeMessageSent      MessageType = "message_sent"
	TypeMessageUpdated   MessageType = "message_updated"
	TypeMessageDeleted   MessageType = "message_deleted"
	TypeMessageDelivered MessageType = "message_delivered"
	TypeMessageRead      MessageType = "message_read"

	// Typing events
	TypeUserTyping        MessageType = "user_typing"
	TypeUserStoppedTyping MessageType = "user_stopped_typing"

	// Presence events
	TypeUserOnline  MessageType = "user_online"
	TypeUserOffline MessageType = "user_offline"

	// Reaction events
	TypeReactionAdded   MessageType = "reaction_added"
	TypeReactionRemoved MessageType = "reaction_removed"

	// Client actions
	TypeSubscribeChat      MessageType = "subscribe_chat"
	TypeUnsubscribeChat    MessageType = "unsubscribe_chat"
	TypeSubscribeUserChats MessageType = "subscribe_user_chats"
	TypeSendMessage        MessageType = "send_message"
	TypeUpdateMessage      MessageType = "update_message"
	TypeDeleteMessage      MessageType = "delete_message"
	TypeMarkDelivered      MessageType = "message_delivered"
	TypeMarkRead           MessageType = "message_read"
	TypeChatRead           MessageType = "chat_read"
	TypeTyping             MessageType = "typing"
	TypePresence           MessageType = "presence"
	TypePinMessage         MessageType = "pin_message"
	TypeUnpinMessage       MessageType = "unpin_message"
	TypeAddReaction        MessageType = "add_reaction"
	TypeRemoveReaction     MessageType = "remove_reaction"
	TypeCreateChat         MessageType = "create_chat"
)

// ===============================
// WEBSOCKET MESSAGE
// ===============================

type Message struct {
	Type      MessageType            `json:"type"`
	Data      map[string]interface{} `json:"data"`
	ID        string                 `json:"id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ===============================
// CLIENT CONNECTION
// ===============================

type Client struct {
	ID            string
	UserID        string
	Conn          *websocket.Conn
	Manager       *Manager
	Send          chan []byte
	Subscriptions map[string]bool // chat_id -> subscribed
	mutex         sync.RWMutex
}

func NewClient(userID string, conn *websocket.Conn, manager *Manager) *Client {
	return &Client{
		ID:            uuid.New().String(),
		UserID:        userID,
		Conn:          conn,
		Manager:       manager,
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
}

// Subscribe to a chat
func (c *Client) SubscribeToChat(chatID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Subscriptions[chatID] = true
	log.Printf("Client %s subscribed to chat %s", c.UserID, chatID)
}

// Unsubscribe from a chat
func (c *Client) UnsubscribeFromChat(chatID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.Subscriptions, chatID)
	log.Printf("Client %s unsubscribed from chat %s", c.UserID, chatID)
}

// Check if subscribed to a chat
func (c *Client) IsSubscribedTo(chatID string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Subscriptions[chatID]
}

// Read messages from client
func (c *Client) ReadPump() {
	defer func() {
		c.Manager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for user %s: %v", c.UserID, err)
			}
			break
		}

		// Parse message
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to parse message from user %s: %v", c.UserID, err)
			continue
		}

		// Handle message
		c.Manager.HandleMessage(c, &msg)
	}
}

// Write messages to client
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current websocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ===============================
// WEBSOCKET MANAGER
// ===============================

type Manager struct {
	Clients     map[string]*Client   // socket_id -> client
	UserClients map[string][]*Client // user_id -> clients
	ChatRooms   map[string][]*Client // chat_id -> clients
	Register    chan *Client
	Unregister  chan *Client
	Broadcast   chan *BroadcastMessage
	DB          *sqlx.DB
	mutex       sync.RWMutex
}

type BroadcastMessage struct {
	ChatID  string
	Message *Message
	Exclude string // Exclude this client ID
}

func NewManager(db *sqlx.DB) *Manager {
	return &Manager{
		Clients:     make(map[string]*Client),
		UserClients: make(map[string][]*Client),
		ChatRooms:   make(map[string][]*Client),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Broadcast:   make(chan *BroadcastMessage, 256),
		DB:          db,
	}
}

// Run the manager
func (m *Manager) Run() {
	// Cleanup routine for stale connections
	go m.cleanupRoutine()

	for {
		select {
		case client := <-m.Register:
			m.registerClient(client)

		case client := <-m.Unregister:
			m.unregisterClient(client)

		case broadcast := <-m.Broadcast:
			m.broadcastToChat(broadcast)
		}
	}
}

// Register a new client
func (m *Manager) registerClient(client *Client) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Store client by socket ID
	m.Clients[client.ID] = client

	// Store client by user ID
	m.UserClients[client.UserID] = append(m.UserClients[client.UserID], client)

	// Store in database
	m.storeConnectionInDB(client)

	// Send connection established message
	msg := Message{
		Type:      TypeConnectionEstablished,
		Data:      map[string]interface{}{"clientId": client.ID, "userId": client.UserID},
		Timestamp: time.Now(),
	}
	m.sendToClient(client, &msg)

	// Notify user is online
	m.broadcastUserPresence(client.UserID, true)

	log.Printf("Client registered: %s (User: %s)", client.ID, client.UserID)
}

// Unregister a client
func (m *Manager) unregisterClient(client *Client) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Remove from clients map
	delete(m.Clients, client.ID)

	// Remove from user clients
	if clients, ok := m.UserClients[client.UserID]; ok {
		for i, c := range clients {
			if c.ID == client.ID {
				m.UserClients[client.UserID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		if len(m.UserClients[client.UserID]) == 0 {
			delete(m.UserClients, client.UserID)
		}
	}

	// Remove from chat rooms
	for chatID := range client.Subscriptions {
		m.removeClientFromChatRoom(chatID, client)
	}

	// Close send channel
	close(client.Send)

	// Update database
	m.deactivateConnectionInDB(client)

	// Notify user is offline (only if no other clients)
	if len(m.UserClients[client.UserID]) == 0 {
		m.broadcastUserPresence(client.UserID, false)
	}

	log.Printf("Client unregistered: %s (User: %s)", client.ID, client.UserID)
}

// Broadcast to chat room
func (m *Manager) broadcastToChat(broadcast *BroadcastMessage) {
	m.mutex.RLock()
	clients, ok := m.ChatRooms[broadcast.ChatID]
	m.mutex.RUnlock()

	if !ok {
		return
	}

	messageBytes, err := json.Marshal(broadcast.Message)
	if err != nil {
		log.Printf("Failed to marshal broadcast message: %v", err)
		return
	}

	for _, client := range clients {
		if client.ID != broadcast.Exclude {
			select {
			case client.Send <- messageBytes:
			default:
				// Client's send buffer is full, close connection
				close(client.Send)
				m.Unregister <- client
			}
		}
	}
}

// Send message to specific client
func (m *Manager) sendToClient(client *Client, msg *Message) {
	messageBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	select {
	case client.Send <- messageBytes:
	default:
		close(client.Send)
		m.Unregister <- client
	}
}

// Send message to all clients of a user
func (m *Manager) SendToUser(userID string, msg *Message) {
	m.mutex.RLock()
	clients := m.UserClients[userID]
	m.mutex.RUnlock()

	for _, client := range clients {
		m.sendToClient(client, msg)
	}
}

// Add client to chat room
func (m *Manager) addClientToChatRoom(chatID string, client *Client) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.ChatRooms[chatID] = append(m.ChatRooms[chatID], client)
}

// Remove client from chat room
func (m *Manager) removeClientFromChatRoom(chatID string, client *Client) {
	if clients, ok := m.ChatRooms[chatID]; ok {
		for i, c := range clients {
			if c.ID == client.ID {
				m.ChatRooms[chatID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		if len(m.ChatRooms[chatID]) == 0 {
			delete(m.ChatRooms, chatID)
		}
	}
}

// Handle incoming message from client
func (m *Manager) HandleMessage(client *Client, msg *Message) {
	switch msg.Type {
	case TypePing:
		m.handlePing(client)

	case TypeSubscribeChat:
		m.handleSubscribeChat(client, msg)

	case TypeUnsubscribeChat:
		m.handleUnsubscribeChat(client, msg)

	case TypeSubscribeUserChats:
		m.handleSubscribeUserChats(client, msg)

	case TypeSendMessage:
		m.handleSendMessage(client, msg)

	case TypeUpdateMessage:
		m.handleUpdateMessage(client, msg)

	case TypeDeleteMessage:
		m.handleDeleteMessage(client, msg)

	case TypeMarkDelivered:
		m.handleMarkDelivered(client, msg)

	case TypeMarkRead:
		m.handleMarkRead(client, msg)

	case TypeChatRead:
		m.handleChatRead(client, msg)

	case TypeTyping:
		m.handleTyping(client, msg)

	case TypePresence:
		m.handlePresence(client, msg)

	case TypePinMessage:
		m.handlePinMessage(client, msg)

	case TypeUnpinMessage:
		m.handleUnpinMessage(client, msg)

	case TypeAddReaction:
		m.handleAddReaction(client, msg)

	case TypeRemoveReaction:
		m.handleRemoveReaction(client, msg)

	case TypeCreateChat:
		m.handleCreateChat(client, msg)

	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// ===============================
// MESSAGE HANDLERS
// ===============================

func (m *Manager) handlePing(client *Client) {
	msg := Message{
		Type:      TypePong,
		Data:      map[string]interface{}{},
		Timestamp: time.Now(),
	}
	m.sendToClient(client, &msg)

	// Update heartbeat in database
	m.updateHeartbeatInDB(client)
}

func (m *Manager) handleSubscribeChat(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		log.Printf("Invalid chatId in subscribe_chat message")
		return
	}

	client.SubscribeToChat(chatID)
	m.addClientToChatRoom(chatID, client)
}

func (m *Manager) handleUnsubscribeChat(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		log.Printf("Invalid chatId in unsubscribe_chat message")
		return
	}

	client.UnsubscribeFromChat(chatID)
	m.removeClientFromChatRoom(chatID, client)
}

func (m *Manager) handleSubscribeUserChats(client *Client, msg *Message) {
	userID, ok := msg.Data["userId"].(string)
	if !ok || userID != client.UserID {
		log.Printf("Invalid userId in subscribe_user_chats message")
		return
	}

	// Load user's chats and subscribe to them
	// This would query the database for user's chats
	// For now, just acknowledge
	log.Printf("User %s subscribed to their chats", userID)
}

func (m *Manager) handleSendMessage(client *Client, msg *Message) {
	// Validate message data
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		m.sendError(client, "Invalid chatId")
		return
	}

	// Create response with message ID
	response := Message{
		Type:      TypeMessageSent,
		Data:      msg.Data,
		ID:        msg.ID,
		Timestamp: time.Now(),
	}
	m.sendToClient(client, &response)

	// Broadcast to chat participants
	broadcast := Message{
		Type:      TypeMessageReceived,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
		Exclude: client.ID,
	}
}

func (m *Manager) handleUpdateMessage(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	broadcast := Message{
		Type:      TypeMessageUpdated,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleDeleteMessage(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	broadcast := Message{
		Type:      TypeMessageDeleted,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleMarkDelivered(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	msg.Data["userId"] = client.UserID
	msg.Data["deliveredAt"] = time.Now().Format(time.RFC3339)

	broadcast := Message{
		Type:      TypeMessageDelivered,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleMarkRead(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	msg.Data["userId"] = client.UserID
	msg.Data["readAt"] = time.Now().Format(time.RFC3339)

	broadcast := Message{
		Type:      TypeMessageRead,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleChatRead(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	// Mark all messages in chat as read for this user
	msg.Data["userId"] = client.UserID
	msg.Data["readAt"] = time.Now().Format(time.RFC3339)

	broadcast := Message{
		Type:      TypeChatRead,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleTyping(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	isTyping, ok := msg.Data["isTyping"].(bool)
	if !ok {
		return
	}

	msg.Data["userId"] = client.UserID

	messageType := TypeUserTyping
	if !isTyping {
		messageType = TypeUserStoppedTyping
	}

	broadcast := Message{
		Type:      messageType,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
		Exclude: client.ID,
	}
}

func (m *Manager) handlePresence(client *Client, msg *Message) {
	isOnline, ok := msg.Data["isOnline"].(bool)
	if !ok {
		return
	}

	m.broadcastUserPresence(client.UserID, isOnline)
}

func (m *Manager) handlePinMessage(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	broadcast := Message{
		Type:      TypeMessageUpdated,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleUnpinMessage(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	broadcast := Message{
		Type:      TypeMessageUpdated,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleAddReaction(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	msg.Data["userId"] = client.UserID

	broadcast := Message{
		Type:      TypeReactionAdded,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleRemoveReaction(client *Client, msg *Message) {
	chatID, ok := msg.Data["chatId"].(string)
	if !ok {
		return
	}

	msg.Data["userId"] = client.UserID

	broadcast := Message{
		Type:      TypeReactionRemoved,
		Data:      msg.Data,
		Timestamp: time.Now(),
	}
	m.Broadcast <- &BroadcastMessage{
		ChatID:  chatID,
		Message: &broadcast,
	}
}

func (m *Manager) handleCreateChat(client *Client, msg *Message) {
	// Create response with chat ID
	response := Message{
		Type:      TypeChatCreated,
		Data:      msg.Data,
		ID:        msg.ID,
		Timestamp: time.Now(),
	}
	m.sendToClient(client, &response)
}

// ===============================
// UTILITY METHODS
// ===============================

func (m *Manager) broadcastUserPresence(userID string, isOnline bool) {
	messageType := TypeUserOnline
	if !isOnline {
		messageType = TypeUserOffline
	}

	msg := Message{
		Type: messageType,
		Data: map[string]interface{}{
			"userId":   userID,
			"isOnline": isOnline,
		},
		Timestamp: time.Now(),
	}

	// Broadcast to all connected clients
	m.mutex.RLock()
	for _, client := range m.Clients {
		m.sendToClient(client, &msg)
	}
	m.mutex.RUnlock()
}

func (m *Manager) sendError(client *Client, errorMessage string) {
	msg := Message{
		Type: TypeError,
		Data: map[string]interface{}{
			"message": errorMessage,
		},
		Timestamp: time.Now(),
	}
	m.sendToClient(client, &msg)
}

// ===============================
// DATABASE OPERATIONS
// ===============================

func (m *Manager) storeConnectionInDB(client *Client) {
	query := `
		INSERT INTO websocket_connections (connection_id, user_id, socket_id, connected_at, last_heartbeat, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := m.DB.Exec(query, uuid.New().String(), client.UserID, client.ID, time.Now(), time.Now(), true)
	if err != nil {
		log.Printf("Failed to store connection in DB: %v", err)
	}
}

func (m *Manager) deactivateConnectionInDB(client *Client) {
	query := `UPDATE websocket_connections SET is_active = false WHERE socket_id = $1`
	_, err := m.DB.Exec(query, client.ID)
	if err != nil {
		log.Printf("Failed to deactivate connection in DB: %v", err)
	}
}

func (m *Manager) updateHeartbeatInDB(client *Client) {
	query := `UPDATE websocket_connections SET last_heartbeat = $1 WHERE socket_id = $2 AND is_active = true`
	_, err := m.DB.Exec(query, time.Now(), client.ID)
	if err != nil {
		log.Printf("Failed to update heartbeat in DB: %v", err)
	}
}

// Cleanup routine to remove stale connections
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		query := `
			UPDATE websocket_connections 
			SET is_active = false 
			WHERE is_active = true AND last_heartbeat < $1
		`
		_, err := m.DB.Exec(query, time.Now().Add(-5*time.Minute))
		if err != nil {
			log.Printf("Failed to cleanup stale connections: %v", err)
		}
	}
}

// Get active connections count
func (m *Manager) GetActiveConnectionsCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.Clients)
}

// Get user's active connections count
func (m *Manager) GetUserConnectionsCount(userID string) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.UserClients[userID])
}

// Check if user is online
func (m *Manager) IsUserOnline(userID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.UserClients[userID]) > 0
}

// Broadcast to specific users
func (m *Manager) BroadcastToUsers(userIDs []string, msg *Message) {
	for _, userID := range userIDs {
		m.SendToUser(userID, msg)
	}
}
