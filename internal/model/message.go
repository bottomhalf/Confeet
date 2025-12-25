package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	NEW_MESSAGE       = 1
	MESSAGE_SENT      = 2
	MESSAGE_SEEN      = 3
	MESSAGE_EDITED    = 4
	MESSAGE_DELETED   = 5
	MESSAGE_DELIVERED = 6
	USER_TYPING       = 7
	ERROR             = 8
)

// Message represents a chat message in MongoDB
type Message struct {
	ID                primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	MessageId         string              `json:"messageId" bson:"message_id"`
	ConversationID    primitive.ObjectID  `json:"conversationId" bson:"conversation_id"`
	SenderID          string              `json:"senderId" bson:"sender_id"`
	RecievedID        string              `json:"recievedId" bson:"recieved_id"`
	Type              string              `json:"type" bson:"type"`
	Avatar            string              `json:"avatar" bson:"avatar"`
	Body              string              `json:"body" bson:"body"`
	FileURL           *string             `json:"fileUrl" bson:"file_url"`
	ReplyTo           *primitive.ObjectID `json:"replyTo" bson:"reply_to"`
	Mentions          []int64             `json:"mentions" bson:"mentions"`
	Reactions         []Reaction          `json:"reactions" bson:"reactions"`
	ClientType        string              `json:"clientType" bson:"client_type"`
	CreatedAt         time.Time           `json:"createdAt" bson:"created_at"`
	EditedAt          *time.Time          `json:"editedAt" bson:"edited_at"`
	Status            int                 `json:"status" bson:"status"`
	IsNewConversation bool                `json:"isNewConversation" bson:"is_new_conversation"`
}

// Reaction represents a reaction on a message
type MessageStatus struct {
	Recieved int64 `json:"recieved" bson:"recieved"`
	Sent     int64 `json:"sent" bson:"sent"`
	Seen     int64 `json:"seen" bson:"seen"`
	Edited   int64 `json:"edited" bson:"edited"`
	Deleted  int64 `json:"deleted" bson:"deleted"`
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
