package hub

import (
	"Confeet/internal/event"
	"Confeet/internal/model"
	"Confeet/internal/repo"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	shardCount    = 64               // tune: 16/64/128 depending on load
	roomTTL       = 30 * time.Minute // TTL for room cache eviction
	evictInterval = 5 * time.Minute  // How often to check for expired rooms
	Direct        = "direct"
	Group         = "group"
)

type inboundMessage struct {
	event  event.WsEvent
	client *Client
}

// Room represents a cached conversation room with its members (userIds)
type Room struct {
	ConversationID string
	Members        map[string]bool // userIds who are members of this room
	LastAccess     time.Time       // For TTL-based eviction
	mu             sync.RWMutex
}

type roomBucket struct {
	sync.RWMutex
	rooms map[string]*Room // conversationID -> Room
}

func generateaUUID(firstUserId string, secondUserId string) string {
	// Your input value
	var value string

	for i := 0; i < len(firstUserId); i++ {
		if firstUserId[i] < secondUserId[i] {
			value = secondUserId + firstUserId
			break
		} else {
			value = firstUserId + secondUserId
			break
		}
	}

	// Define a namespace (you can use a predefined one or create your own)
	namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // UUID namespace for DNS (or use your own)

	// Generate deterministic UUID
	clientID := uuid.NewSHA1(namespace, []byte(value)).String()

	log.Println(clientID)
	// This will ALWAYS produce the same UUID for "1535"

	return clientID
}

type Hub struct {
	shards [shardCount]*roomBucket

	// Online users - maps userId to their Client connection
	onlineUsers   map[string]*Client
	onlineUsersMu sync.RWMutex

	register   chan *Client
	unregister chan *Client
	broadcast  chan event.WsEvent
	inbound    chan inboundMessage
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc

	// Repositories
	messageRepo      repo.MessageRepository
	conversationRepo repo.ConversationRepository
}

func NewHub(messageRepo repo.MessageRepository, conversationRepo repo.ConversationRepository) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		onlineUsers:      make(map[string]*Client),
		register:         make(chan *Client, 1024),
		unregister:       make(chan *Client, 1024),
		broadcast:        make(chan event.WsEvent, 1024),
		inbound:          make(chan inboundMessage, 4096),
		messageRepo:      messageRepo,
		conversationRepo: conversationRepo,
		ctx:              ctx,
		cancel:           cancel,
	}

	for i := 0; i < shardCount; i++ {
		h.shards[i] = &roomBucket{
			rooms: make(map[string]*Room),
		}
	}

	// run manager loop
	go h.run()

	// start worker pool for processing inbound messages
	for i := 0; i < workerPoolSize; i++ {
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			for {
				select {
				case <-h.ctx.Done():
					return
				case in, ok := <-h.inbound:
					if !ok {
						return
					}
					h.handleEvent(in.event, in.client)
				}
			}
		}()
	}

	// start TTL eviction goroutine
	go h.evictExpiredRooms()

	return h
}

func (h *Hub) handleEvent(ev event.WsEvent, c *Client) {
	switch ev.Event {
	case event.EventSendMessage:
		var message model.Message
		if err := json.Unmarshal(ev.Payload, &message); err != nil {
			log.Printf("failed to unmarshal client message: %v", err)
			h.sendErrorToClient(c, "invalid_message", "Failed to parse message")
			return
		}

		// Get conversation ID from the message
		conversationID := message.ConversationID.Hex()
		if conversationID == "" || conversationID == "000000000000000000000000" {
			h.sendErrorToClient(c, "invalid_message", "ConversationID is required")
			return
		}

		log.Printf("New message from %s in conversation %s: %s\n", message.SenderID, conversationID, message.Body)
		message.Status = model.MessageSentId

		// Save message to MongoDB before publishing
		ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
		insertedID, err := h.messageRepo.InsertMessage(ctx, &message)
		cancel()

		if err != nil {
			log.Printf("failed to save message to MongoDB: %v", err)
			h.sendErrorToClient(c, "save_failed", "Failed to save message, please retry")
			return
		}

		log.Printf("Message saved to MongoDB with ID: %s", insertedID)

		ev.Payload, _ = json.Marshal(message)
		h.publishToRoom(ev, conversationID)

	case event.EventTyping:
		var typing model.TypingIndicator
		if err := json.Unmarshal(ev.Payload, &typing); err != nil {
			log.Printf("failed to unmarshal typing indicator: %v", err)
			return
		}

		if typing.ConversationID == "" {
			return
		}

		log.Printf("User %s is typing in conversation %s\n", typing.UserID, typing.ConversationID)
		h.publishToRoom(ev, typing.ConversationID)

	default:
		log.Printf("unknown event type: %s", ev.Event)
	}
}

// publishToRoom sends message to all ONLINE members of a room
func (h *Hub) publishToRoom(ev event.WsEvent, groupConversationID string) {
	// Get the room from cache
	room := h.GetRoom(groupConversationID)
	if room == nil {
		log.Printf("room %s not found in cache - fetching from database", groupConversationID)

		// Fetch room details from database
		ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
		conversation, err := h.conversationRepo.GetRoomDetail(ctx, groupConversationID)
		cancel()

		if err != nil {
			log.Printf("failed to fetch room %s from database: %v", groupConversationID, err)
			return
		}

		if conversation == nil {
			log.Printf("room %s not found in database", groupConversationID)
			return
		}

		// Set room members in cache from the fetched conversation
		h.SetRoomMembers(groupConversationID, conversation.ParticipantIds)
		room = h.GetRoom(groupConversationID)
		if room == nil {
			log.Printf("failed to create room %s in cache", groupConversationID)
			return
		}
	}

	// Get list of members
	room.mu.RLock()
	memberIDs := make([]string, 0, len(room.Members))
	for memberID := range room.Members {
		memberIDs = append(memberIDs, memberID)
	}
	room.mu.RUnlock()

	ev.Event = event.EventMessageSent

	// Find online clients for each member and send
	h.onlineUsersMu.RLock()
	onlineClients := make([]*Client, 0)
	for _, memberID := range memberIDs {
		if client, online := h.onlineUsers[memberID]; online {
			onlineClients = append(onlineClients, client)
		}
		// TODO: For offline members, queue message to Kafka/Redis for later delivery
	}
	h.onlineUsersMu.RUnlock()

	// Deliver to online clients without holding lock
	for _, client := range onlineClients {
		select {
		case <-client.ctx.Done():
			// client already closed, skip
			log.Printf("client %s already closed, skipping", client.ID)
		case client.egress <- ev:
			// enqueued
		case <-time.After(sendTimeout):
			log.Printf("egress full for client %s in conversation %s", client.ID, groupConversationID)
			if kickOnFull {
				h.unregister <- client
			}
		}
	}

	log.Printf("message published to %d/%d members in room %s", len(onlineClients), len(memberIDs), groupConversationID)
}

// evictExpiredRooms periodically removes rooms that haven't been accessed recently
func (h *Hub) evictExpiredRooms() {
	ticker := time.NewTicker(evictInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			evicted := 0

			for _, bucket := range h.shards {
				bucket.Lock()
				for roomID, room := range bucket.rooms {
					room.mu.RLock()
					expired := now.Sub(room.LastAccess) > roomTTL
					room.mu.RUnlock()

					if expired {
						delete(bucket.rooms, roomID)
						evicted++
					}
				}
				bucket.Unlock()
			}

			if evicted > 0 {
				log.Printf("evicted %d expired rooms from cache", evicted)
			}
		}
	}
}

// GetRoom gets a room from cache, returns nil if not found
func (h *Hub) GetRoom(conversationID string) *Room {
	sh := getShard(conversationID)
	bucket := h.shards[sh]

	bucket.RLock()
	room, exists := bucket.rooms[conversationID]
	bucket.RUnlock()

	if exists {
		room.mu.Lock()
		room.LastAccess = time.Now()
		room.mu.Unlock()
		return room
	}

	return nil
}

// SetRoomMembers sets the members for a room (call this from your service after loading from DB)
func (h *Hub) SetRoomMembers(conversationID string, memberIDs []string) {
	sh := getShard(conversationID)
	bucket := h.shards[sh]

	bucket.Lock()
	defer bucket.Unlock()

	room, exists := bucket.rooms[conversationID]
	if !exists {
		room = &Room{
			ConversationID: conversationID,
			Members:        make(map[string]bool),
			LastAccess:     time.Now(),
		}
		bucket.rooms[conversationID] = room
	}

	room.mu.Lock()
	room.Members = make(map[string]bool)
	for _, memberID := range memberIDs {
		room.Members[memberID] = true
	}
	room.LastAccess = time.Now()
	room.mu.Unlock()

	log.Printf("room %s updated with %d members (shard %d)", conversationID, len(memberIDs), sh)
}

// AddMemberToRoom adds a member to room cache (call after persisting to DB)
func (h *Hub) AddMemberToRoom(conversationID string, userID string) {
	sh := getShard(conversationID)
	bucket := h.shards[sh]

	bucket.RLock()
	room, exists := bucket.rooms[conversationID]
	bucket.RUnlock()

	if exists {
		room.mu.Lock()
		room.Members[userID] = true
		room.LastAccess = time.Now()
		room.mu.Unlock()
	}
}

// RemoveMemberFromRoom removes a member from room cache (call after persisting to DB)
func (h *Hub) RemoveMemberFromRoom(conversationID string, userID string) {
	sh := getShard(conversationID)
	bucket := h.shards[sh]

	bucket.RLock()
	room, exists := bucket.rooms[conversationID]
	bucket.RUnlock()

	if exists {
		room.mu.Lock()
		delete(room.Members, userID)
		room.LastAccess = time.Now()
		room.mu.Unlock()
	}
}

// sendErrorToClient sends an error event back to the specific client
func (h *Hub) sendErrorToClient(c *Client, code string, message string) {
	errorPayload := model.ErrorPayload{
		Code:    code,
		Message: message,
	}

	payload, err := json.Marshal(errorPayload)
	if err != nil {
		log.Printf("failed to marshal error payload: %v", err)
		return
	}

	errorEvent := event.WsEvent{
		Event:   event.EventError,
		Payload: payload,
	}

	select {
	case c.egress <- errorEvent:
		// sent
	case <-time.After(sendTimeout):
		log.Printf("failed to send error to client %s: timeout", c.ID)
	}
}

func getShard(conversationID string) uint32 {
	if conversationID == "" {
		return 0
	}

	h := sha1.Sum([]byte(conversationID))
	return binary.BigEndian.Uint32(h[:4]) % shardCount
}

// addClient is called when a client is registered
func (h *Hub) addClient(c *Client) {
	h.onlineUsersMu.Lock()
	h.onlineUsers[c.userId] = c
	h.onlineUsersMu.Unlock()

	log.Printf("client %s (user: %s) added to online users", c.ID, c.userId)
}

func (h *Hub) Stop() {
	h.cancel()

	// Close all online client connections
	h.onlineUsersMu.RLock()
	for _, client := range h.onlineUsers {
		client.Close()
	}
	h.onlineUsersMu.RUnlock()

	close(h.inbound)
	h.wg.Wait()
}

// removeClient removes the client from online users (does NOT remove from rooms)
func (h *Hub) removeClient(c *Client) {
	h.onlineUsersMu.Lock()
	delete(h.onlineUsers, c.userId)
	h.onlineUsersMu.Unlock()

	// Close the client connection
	c.Close()
	log.Printf("client %s (user: %s) removed from online users", c.ID, c.userId)
}

func (h *Hub) run() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case c := <-h.register:
			h.addClient(c)
		case c := <-h.unregister:
			h.removeClient(c)
		}
	}
}

var (
	websocketUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkOrigin,
	}
)

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	switch origin {
	case "http://localhost:4200":
		return true
	case "https://www.confeet.com":
		return true
	default:
		return false
	}
}

// ServeWS handles WebSocket connection requests
func (h *Hub) ServeWS(c *gin.Context, userId string) {
	conn, err := websocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	RegisterClient(userId, conn, h)
}
