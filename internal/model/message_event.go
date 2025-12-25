package model

// MessageDelivered - lightweight event for delivery confirmation
type MessageDelivered struct {
	Id             string `json:"id"`
	UserId         string `json:"userId"`
	ConversationID string `json:"conversationId"`
	DeliveredTo    string `json:"deliveredTo"`
	DeliveredAt    string `json:"deliveredAt"`
	Status         int    `json:"status"`
}

// MessageSeen - for read receipts
type MessageSeen struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
	SeenBy         string `json:"seenBy"`
	SeenAt         string `json:"seenAt"`
}

// TypingIndicator - for typing status
type TypingIndicator struct {
	ConversationID string `json:"conversationId"`
	UserID         string `json:"userId"`
	Type           string `json:"type"` // "start" or "stop"
	IsTyping       bool   `json:"isTyping"`
}
