package model

// MessageDelivered - lightweight event for delivery confirmation
type MessageDelivered struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
	DeliveredTo    string `json:"deliveredTo"`
	DeliveredAt    string `json:"deliveredAt"`
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
	IsTyping       bool   `json:"isTyping"`
}
