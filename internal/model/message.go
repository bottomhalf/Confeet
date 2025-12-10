package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	MessageRecievedId = 1
	MessageSentId     = 2
	MessageSeenId     = 3
	MessageEditedId   = 4
	MessageDeletedId  = 5
)

// Message represents a chat message in MongoDB
type Message struct {
	ID             primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	MessageId      string              `json:"messageId" bson:"message_id"`
	ConversationID primitive.ObjectID  `json:"conversationId" bson:"conversation_id"`
	SenderID       int64               `json:"senderId" bson:"sender_id"`
	Type           string              `json:"type" bson:"type"`
	Body           string              `json:"body" bson:"body"`
	FileURL        *string             `json:"fileUrl" bson:"file_url"`
	ReplyTo        *primitive.ObjectID `json:"replyTo" bson:"reply_to"`
	Mentions       []int64             `json:"mentions" bson:"mentions"`
	Reactions      []Reaction          `json:"reactions" bson:"reactions"`
	ClientType     string              `json:"clientType" bson:"client_type"`
	CreatedAt      time.Time           `json:"createdAt" bson:"created_at"`
	EditedAt       *time.Time          `json:"editedAt" bson:"edited_at"`
	Status         int                 `json:"status" bson:"status"`
}

// Reaction represents a reaction on a message
type Reaction struct {
	UserID int64  `json:"userId" bson:"user_id"`
	Emoji  string `json:"emoji" bson:"emoji"`
}

// ErrorPayload represents an error response sent to client via WebSocket
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
