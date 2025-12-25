package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Call represents a call session (can be stored in DB for call history)
type Call struct {
	ID             primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ConversationID primitive.ObjectID `json:"conversationId" bson:"conversationId"` // Associated conversation
	CallerID       string             `json:"callerId" bson:"callerId"`             // User who initiated the call
	CalleeIDs      []string           `json:"calleeIds" bson:"calleeIds"`           // User(s) being called
	CallType       string             `json:"callType" bson:"callType"`             // "audio" or "video"
	Status         int                `json:"status" bson:"status"`                 // Current call status
	RoomName       string             `json:"roomName,omitempty" bson:"roomName"`   // LiveKit room name
	StartedAt      *time.Time         `json:"startedAt,omitempty" bson:"startedAt"` // When call was accepted
	EndedAt        *time.Time         `json:"endedAt,omitempty" bson:"endedAt"`     // When call ended
	Duration       int                `json:"duration" bson:"duration"`             // Call duration in seconds
	EndReason      string             `json:"endReason,omitempty" bson:"endReason"` // Why call ended
	CreatedAt      time.Time          `json:"createdAt" bson:"createdAt"`           // When call was initiated
	UpdatedAt      time.Time          `json:"updatedAt" bson:"updatedAt"`           // Last update time
}

// CallParticipant tracks individual participant status in a call
type CallParticipant struct {
	UserID    string     `json:"userId" bson:"userId"`
	Status    int        `json:"status" bson:"status"`       // Participant-specific status
	JoinedAt  *time.Time `json:"joinedAt" bson:"joinedAt"`   // When they joined
	LeftAt    *time.Time `json:"leftAt" bson:"leftAt"`       // When they left
	EndReason string     `json:"endReason" bson:"endReason"` // Why they left
}

// -----------------------------------------------------------------
// WebSocket Event Payloads - Client to Server
// -----------------------------------------------------------------

// CallInitiatePayload is sent by caller to initiate a call
type CallInitiatePayload struct {
	ConversationID string   `json:"conversationId"`    // Conversation/room ID
	CalleeIDs      []string `json:"calleeIds"`         // User(s) to call
	CallType       string   `json:"callType"`          // "audio" or "video"
	Timeout        int      `json:"timeout,omitempty"` // Ring timeout in seconds (default 40)
}

// CallAcceptPayload is sent by callee to accept a call
type CallAcceptPayload struct {
	ConversationID string `json:"conversationId"`
	CallerID       string `json:"callerId"`
}

// CallRejectPayload is sent by callee to reject a call
type CallRejectPayload struct {
	ConversationID string `json:"conversationId"`
	CallerID       string `json:"callerId"`
	Reason         string `json:"reason,omitempty"` // Optional rejection reason
}

// CallCancelPayload is sent by caller to cancel before answer
type CallCancelPayload struct {
	ConversationID string   `json:"conversationId"`
	CalleeIDs      []string `json:"calleeIds"`
}

// CallTimeoutPayload is sent by callee when ring timeout occurs
type CallTimeoutPayload struct {
	ConversationID string `json:"conversationId"`
	CallerID       string `json:"callerId"`
}

// CallEndPayload is sent to end an ongoing call
type CallEndPayload struct {
	ConversationID string `json:"conversationId"`
	Reason         string `json:"reason,omitempty"`
}

// -----------------------------------------------------------------
// WebSocket Event Payloads - Server to Client
// -----------------------------------------------------------------

// CallIncomingEvent is sent to callee(s) when receiving a call
type CallIncomingEvent struct {
	ConversationID string `json:"conversationId"`
	CallerID       string `json:"callerId"`
	CallerName     string `json:"callerName,omitempty"`   // Display name
	CallerAvatar   string `json:"callerAvatar,omitempty"` // Avatar URL
	CallType       string `json:"callType"`               // "audio" or "video"
	Timeout        int    `json:"timeout"`                // Seconds until timeout
	Timestamp      int64  `json:"timestamp"`              // Unix timestamp
}

// CallAcceptedEvent is sent to caller when callee accepts
type CallAcceptedEvent struct {
	CallID     string `json:"callId"`
	AcceptedBy string `json:"acceptedBy"` // UserID who accepted
	RoomName   string `json:"roomName"`   // LiveKit room name
	Token      string `json:"token"`      // LiveKit access token for caller
	Timestamp  int64  `json:"timestamp"`
}

// CallRejectedEvent is sent to caller when callee rejects
type CallRejectedEvent struct {
	CallID     string `json:"callId"`
	RejectedBy string `json:"rejectedBy"`
	Reason     string `json:"reason,omitempty"`
	Timestamp  int64  `json:"timestamp"`
}

// CallCancelledEvent is sent to callee when caller cancels
type CallCancelledEvent struct {
	CallID      string `json:"callId"`
	CancelledBy string `json:"cancelledBy"`
	Timestamp   int64  `json:"timestamp"`
}

// CallTimedOutEvent is sent to caller when call times out
type CallTimedOutEvent struct {
	CallID    string `json:"callId"`
	Timestamp int64  `json:"timestamp"`
}

// CallEndedEvent is sent when call ends
type CallEndedEvent struct {
	CallID    string `json:"callId"`
	EndedBy   string `json:"endedBy"`
	Reason    string `json:"reason"`
	Duration  int    `json:"duration"` // Call duration in seconds
	Timestamp int64  `json:"timestamp"`
}

// CallBusyEvent is sent to caller when callee is busy
type CallBusyEvent struct {
	ConversationID string `json:"conversationId"`
	BusyUser       string `json:"busyUser"`
	Timestamp      int64  `json:"timestamp"`
}

// CallErrorEvent is sent when a call error occurs
type CallErrorEvent struct {
	CallID    string `json:"callId,omitempty"`
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// -----------------------------------------------------------------
// LiveKit Integration
// -----------------------------------------------------------------

// LiveKitRoomInfo contains LiveKit room details for joining a call
type LiveKitRoomInfo struct {
	RoomName string `json:"roomName"`
	Token    string `json:"token"` // JWT token for participant
	URL      string `json:"url"`   // LiveKit server URL
}

// CallJoinInfo is sent to participants when they should join the call
type CallJoinInfo struct {
	CallID   string          `json:"callId"`
	CallType string          `json:"callType"`
	LiveKit  LiveKitRoomInfo `json:"livekit"`
}
