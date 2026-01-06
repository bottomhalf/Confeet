package event

import "encoding/json"

// Event types
const (
	// Client -> Server
	EventSendMessage   = "send_message"
	EventMarkDelivered = "mark_delivered"
	EventMarkSeen      = "mark_seen"
	EventTyping        = "typing"
	HeartBeat          = "heartbeat" // Client sends this periodically to prove it's alive

	// Server -> Client
	EventNewMessage  = "new_message"
	EventMessageSent = "message_sent"
	EventDelivered   = "delivered"
	EventSeen        = "seen"
	EventUserTyping  = "user_typing"
	EventError       = "error"
	Direct           = "direct"
	Group            = "group"
)

type WsEvent struct {
	Event   string          `json:"event"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
