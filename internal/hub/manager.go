package hub

import (
	"Confeet/internal/event"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	shardCount = 64 // tune: 16/64/128 depending on load
)

type inboundMessage struct {
	event  event.WsEvent
	client *Client
}

type clientBucket struct {
	sync.RWMutex
	rooms map[string]map[string]*Client
}

type Hub struct {
	shards     [shardCount]*clientBucket
	register   chan *Client
	unregister chan *Client
	broadcast  chan event.WsEvent
	inbound    chan inboundMessage
	mu         sync.RWMutex
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		register:   make(chan *Client, 1024),
		unregister: make(chan *Client, 1024),
		broadcast:  make(chan event.WsEvent, 1024), // buffer size for broadcast
		inbound:    make(chan inboundMessage, 4096), // buffer for burst handling
		ctx:        ctx,
		cancel:     cancel,
	}

	for i := 0; i < shardCount; i++ {
		h.shards[i] = &clientBucket{
			rooms: make(map[string]map[string]*Client),
		}
	}

	// run manager loop
	go h.run()

	// start worker loop
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

	return h
}

func (h *Hub) handleEvent(ev event.WsEvent, c *Client) {
	switch ev.Event {
	case event.EventClientMessage:
		var message event.Message
		if err := json.Unmarshal(ev.Message, &message); err != nil {
			log.Printf("failed to unmarshal client message: %v", err)
			return
		}

		log.Printf("New message from %s: %s\n", message.SenderId, message.Body)
		h.publishToRoom(ev, c.ChannelId)
	default:
		log.Printf("unknown event type: %s", ev.Event)
	}
}

func (h *Hub) publishToRoom(ev event.WsEvent, channelId string) {
	sh := getShard(channelId)
	b := h.shards[sh]

	// collect clients while holding RLock
	b.RLock()
	room, ok := b.rooms[channelId]
	if !ok || len(room) == 0 {
		b.RUnlock()
		return
	}

	clients := make([]*Client, 0, len(room))
	for _, c := range room {
		clients = append(clients, c)
	}
	b.RUnlock()

	ev.Event = event.EventServerMessage

	// deliver to clients without holding lock
	for _, c := range clients {
		// try enqueue with timeout
		select {
		case c.egress <- ev:
			// enqueued
		case <-time.After(sendTimeout):
			// egress full -> apply policy
			log.Printf("egress full for client %s in channel %s", c.ID, channelId)
			if kickOnFull {
				// Unregister (safe async)
				h.unregister <- c
			} else {
				// drop message (do nothing)
			}
		}
	}
}

func getShard(channelId string) uint32 {
	if channelId == "" {
		return 0
	}

	h := sha1.Sum([]byte(channelId))
	return binary.BigEndian.Uint32(h[:4]) % shardCount
}

func (h *Hub) addClient(c *Client) {
	sh := getShard(c.ChannelId)
	b := h.shards[sh]
	b.Lock()
	defer b.Unlock()

	room, ok := b.rooms[c.ChannelId]
	if !ok {
		room = make(map[string]*Client)
		b.rooms[c.ChannelId] = room
	}

	room[c.ID] = c
	log.Printf("client %s registered in channel %s (shard %d)", c.ID, c.ChannelId, sh)
}

func (h *Hub) Stop() {
	h.cancel()

	// Close all client connections
	for _, shard := range h.shards {
		shard.RLock()
		for _, room := range shard.rooms {
			for _, client := range room {
				client.Close()
			}
		}
		shard.RUnlock()
	}

	close(h.inbound)
	h.wg.Wait()
}

func (h *Hub) removeClient(c *Client) {
	sh := getShard(c.ChannelId)
	b := h.shards[sh]
	b.Lock()
	defer b.Unlock()

	if room, ok := b.rooms[c.ChannelId]; ok {
		if _, exists := room[c.ID]; exists {
			delete(room, c.ID)
		}

		if len(room) == 0 {
			delete(b.rooms, c.ChannelId)
		}

		c.Close()
		log.Printf("client %s removed from channel %s (shard %d)", c.ID, c.ChannelId, sh)
	}
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

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, clientId string, channelId string) {
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	RegisterClient(clientId, channelId, conn, h)
}
