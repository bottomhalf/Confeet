package hub

import (
	"Confeet/internal/event"
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type ClientList map[*Client]bool

// Client status constants
const (
	StatusOnline      = "online"
	StatusBusy        = "busy"
	StatusInCall      = "in_call"
	StatusGettingCall = "ringing"
	StatusAway        = "away"
	StatusOffline     = "offline"
)

type Client struct {
	ID      string
	userId  string
	conn    *websocket.Conn
	manager *Hub
	egress  chan event.WsEvent

	// User status for calls
	status                string // "online", "busy", "in_call", "away"
	currentConversationID string // conversationID if in a call
	statusMu              sync.RWMutex

	// cancel or stop goroutine
	cancel         context.CancelFunc
	ctx            context.Context
	once           sync.Once
	connClosed     chan struct{}
	connClosedOnce sync.Once
	closed         bool         // tracks if client is closed
	closedMu       sync.RWMutex // protects closed flag
}

var (
	// tuning parameters
	writeWait          = 10 * time.Second       // time allowed to write a message to the peer
	pongWait           = 20 * time.Second       // time allowed to read the next pong message from the peer
	pingInterval       = (pongWait * 9) / 10    // send pings to peer with this period
	maxMessageSize     = 64 * 1024              // max inbound message size (64KB)
	sendBufSize        = 256                    // per-connection outbound buffer size
	workerPoolSize     = 16                     // number of workers to process inbound messages
	sendTimeout        = 2 * time.Second        // timeout for enqueuing outbound messages
	kickOnFull         = true                   // when true, disconnect client when egress is full
	registerTimeout    = 5 * time.Second        // timeout for client registration
	unregisterTimeout  = 5 * time.Second        // timeout for client unregistration
	inboundSendTimeout = 500 * time.Millisecond // timeout for sending to inbound channel
)

// RegisterClient creates a new client with a single WebSocket connection
func RegisterClient(userId string, conn *websocket.Conn, h *Hub) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	clientID := uuid.New().String()

	client := &Client{
		ID:             clientID,
		userId:         userId,
		conn:           conn,
		manager:        h,
		egress:         make(chan event.WsEvent, sendBufSize),
		status:         StatusOnline,
		cancel:         cancel,
		ctx:            ctx,
		once:           sync.Once{},
		connClosed:     make(chan struct{}),
		connClosedOnce: sync.Once{},
	}

	select {
	case h.register <- client:
		// registered
		go client.ReadMessages()
		go client.WriteMessage()
		log.Printf("client %s registered for user %s", clientID, userId)
		return client
	case <-time.After(registerTimeout):
		log.Printf("failed to register client %s: timeout", clientID)
		cancel()
		conn.Close()
		return nil
	}
}

func (c *Client) ReadMessages() {
	defer func() {
		select {
		case c.manager.unregister <- c:
			// unregistered successfully
		case <-time.After(unregisterTimeout):
			log.Printf("failed to unregister client %s: timeout", c.ID)
		}
		c.Close()
	}()

	c.conn.SetReadLimit(int64(maxMessageSize))
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(c.pongHandler)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			var ev event.WsEvent

			if err := c.conn.ReadJSON(&ev); err != nil {

				if websocket.IsCloseError(err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
				) {
					log.Printf("client disconnected: %v", c.ID)
					return
				}

				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseInternalServerErr,
					websocket.CloseProtocolError,
				) {
					log.Printf("unexpected close for %s: %v", c.ID, err)
				}

				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					log.Printf("client %s timed out - closing connection", c.ID)
					return
				}

				// For other errors, log and exit (cleanup will happen in defer)
				log.Printf("error reading from client %s: %v", c.ID, err)
				return
			}

			// Non-blocking send into inbound processing queue to avoid blocking reader
			select {
			case c.manager.inbound <- inboundMessage{client: c, event: ev}:
				// accepted for processing
			case <-time.After(inboundSendTimeout):
				log.Printf("inbound send timeout: dropping client %s", c.ID)
				c.cancel()
				c.conn.Close()
			case <-c.ctx.Done():
				log.Printf("Closing read message: for client: %s", c.ID)
				return
			}
		}
	}
}

func (c *Client) WriteMessage() {
	ticker := time.NewTicker(pingInterval)

	defer func() {
		ticker.Stop()
		c.Close()
		_ = c.conn.Close()

		// Safe close of connClosed channel using sync.Once
		c.connClosedOnce.Do(func() {
			close(c.connClosed)
		})

		log.Println("WriteMessage goroutine exiting for client:", c.ID)
	}()

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("Closing write message: for client: %s", c.ID)
			return
		case ev, ok := <-c.egress:
			if !ok {
				if err := c.conn.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Printf("connection closed: %v", err)
				}
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteJSON(ev); err != nil {
				log.Println("marshal error: ", err)
				return
			}

			log.Println("message sent")
		case <-ticker.C:
			// log.Println("Ping sent to client:", c.ID)
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				log.Println("write message error: ", err)
				return
			}
		}
	}
}

func (c *Client) pongHandler(pongMsg string) error {
	// log.Println("Pong received from client:", c.ID)
	return c.conn.SetReadDeadline(time.Now().Add(pongWait))
}

func (c *Client) Send(ev event.WsEvent) {
	select {
	case c.egress <- ev:
		// message sent
	case <-time.After(sendTimeout):
		log.Printf("egress full, disconnecting client %s\n", c.ID)
		select {
		case c.manager.unregister <- c:
			// unregistered
		case <-time.After(unregisterTimeout):
			log.Printf("failed to unregister client %s: timeout", c.ID)
		}
	case <-c.ctx.Done():
		// client already closed
	}
}

func (c *Client) Close() {
	c.once.Do(func() {
		// Mark as closed BEFORE closing the channel
		c.closedMu.Lock()
		c.closed = true
		c.closedMu.Unlock()

		c.cancel()
		close(c.egress)

		// Wait for WriteMessage to close conn, or force close after timeout
		go func() {
			select {
			case <-c.connClosed:
				// WriteMessage closed it properly
			case <-time.After(5 * time.Second):
				_ = c.conn.Close()
				log.Printf("safety timeout: force closed connection for client %s", c.ID)
			}
		}()
	})
}

// IsClosed returns true if the client has been closed
func (c *Client) IsClosed() bool {
	c.closedMu.RLock()
	defer c.closedMu.RUnlock()
	return c.closed
}

// SafeSend attempts to send an event to the client's egress channel.
// Returns true if sent successfully, false if client is closed or timeout.
func (c *Client) SafeSend(ev event.WsEvent, timeout time.Duration) bool {
	// Check if closed first (fast path)
	if c.IsClosed() {
		return false
	}

	select {
	case <-c.ctx.Done():
		return false
	case c.egress <- ev:
		return true
	case <-time.After(timeout):
		return false
	}
}

// -----------------------------------------------------------------
// Status Management Methods
// -----------------------------------------------------------------

// GetStatus returns the current status of the client
func (c *Client) GetStatus() string {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status
}

// SetStatus sets the client status
func (c *Client) SetStatus(status string) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status = status
}

// GetCurrentConversationID returns the current call conversation ID
func (c *Client) GetCurrentConversationID() string {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.currentConversationID
}

// SetCallStatusAsInCall marks the client as in a call with the given conversation
func (c *Client) SetCallStatusAsInCall(conversationID string) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status = StatusInCall
	c.currentConversationID = conversationID
}

// SetCallStatus marks the client as in a call with the given conversation
func (c *Client) SetGettingCallStatus(conversationID string) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status = StatusGettingCall
	c.currentConversationID = conversationID
}

// ClearCallStatus resets the client status to online and clears conversation
func (c *Client) ClearCallStatus() {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status = StatusOnline
	c.currentConversationID = ""
}

// IsInCall returns true if the client is currently in a call
func (c *Client) IsInCall() bool {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status == StatusInCall
}

// IsAvailableForCall returns true if client can receive a call
func (c *Client) IsAvailableForCall() bool {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status == StatusOnline
}
