package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Conversation represents a chat conversation/room in MongoDB
type Conversation struct {
	ID               primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	ConversationType string               `json:"conversationType" bson:"conversation_type"`
	Participants     []Participant        `json:"participants" bson:"participants"`
	ConversationName string               `json:"conversationName" bson:"conversation_name"`
	CreatedAt        time.Time            `json:"createdAt" bson:"created_at"`
	LastMessageAt    time.Time            `json:"lastMessageAt" bson:"last_message_at"`
	LastMessage      *LastMessage         `json:"lastMessage" bson:"last_message"`
	IsActive         bool                 `json:"isActive" bson:"is_active"`
	Settings         ConversationSettings `json:"settings" bson:"settings"`
}

// Participant represents a user in a conversation
type Participant struct {
	UserID   primitive.ObjectID `json:"userId" bson:"user_id"`
	Username string             `json:"username" bson:"username"`
	JoinedAt time.Time          `json:"joinedAt" bson:"joined_at"`
	Role     string             `json:"role" bson:"role"`
}

// LastMessage stores the most recent message preview
type LastMessage struct {
	Content        string    `json:"content" bson:"content"`
	SenderUsername string    `json:"senderUsername" bson:"sender_username"`
	Timestamp      time.Time `json:"timestamp" bson:"timestamp"`
}

// ConversationSettings holds conversation-level settings
type ConversationSettings struct {
	AllowReactions bool `json:"allowReactions" bson:"allow_reactions"`
	AllowPinning   bool `json:"allowPinning" bson:"allow_pinning"`
	AdminOnlyPost  bool `json:"adminOnlyPost" bson:"admin_only_post"`
}
