package event

import "encoding/json"

const (
	EventClientMessage = "client_message"
	EventServerMessage = "server_message"
)

type WsEvent struct {
	Event     string          `json:"event"`
	ChannelId string          `json:"channelId"`
	Message   json.RawMessage `json:"message"`
	MessageId string          `json:"messageId"`
}

type Message struct {
	ChannelId string   `json:"channelId"`
	SenderId  string   `json:"senderId"`
	Type      string   `json:"type"`
	Body      string   `json:"body"`
	FileLink  string   `json:"fileLink"`
	Metadata  Metadata `json:"metadata"`
	Timestamp int64    `json:"timestamp"`
	Status    int      `json:"status"`
}

type Metadata struct {
	ReplyTo    *string  `json:"replyTo"`  // nullable
	Mentions   []string `json:"mentions"` // array, not single string
	ClientType string   `json:"clientType"`
}
