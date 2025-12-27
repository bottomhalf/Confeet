package event

// Call Event Types - Client to Server
const (
	// EventCallInitiate - Caller initiates a call to callee(s)
	EventCallInitiate = "call:initiate"

	// EventCallAccept - Callee accepts the incoming call
	EventCallAccept = "call:accept"

	// EventCallReject - Callee rejects the incoming call
	EventCallReject = "call:reject"

	// EventCallCancel - Caller cancels before callee answers
	EventCallCancel = "call:cancel"

	// EventCallTimeout - Callee didn't answer within timeout period
	EventCallTimeout = "call:timeout"

	// EventCallEnd - Either party ends an ongoing call
	EventCallEnd = "call:end"
)

// Call Event Types - Server to Client
const (
	// EventCallIncoming - Notify callee of incoming call
	EventCallIncoming = "call:incoming"

	// EventCallAccepted - Notify caller that callee accepted
	EventCallAccepted = "call:accepted"

	// EventCallRejected - Notify caller that callee rejected
	EventCallRejected = "call:rejected"

	// EventCallCancelled - Notify callee that caller cancelled
	EventCallCancelled = "call:cancelled"

	// EventCallTimedOut - Notify caller that call timed out (no answer)
	EventCallTimedOut = "call:timed_out"

	// EventCallEnded - Notify other party that call ended
	EventCallEnded = "call:ended"

	// EventCallBusy - Notify caller that callee is busy
	EventCallBusy = "call:busy"

	// EventCallMissed - Notify busy callee that they missed a call
	EventCallMissed = "call:missed"

	// EventCallError - Notify of call-related errors
	EventCallError = "call:error"

	// EventCallParticipantJoined - Notify that a participant joined the group call
	EventCallParticipantJoined = "call:participant_joined"

	// EventCallParticipantLeft - Notify that a participant left the group call
	EventCallParticipantLeft = "call:participant_left"
)

// Call Types
const (
	CallTypeAudio = "audio"
	CallTypeVideo = "video"
)

// Call Status Constants
const (
	CallStatusInitiated  = 1
	CallStatusRinging    = 2
	CallStatusAccepted   = 3
	CallStatusRejected   = 4
	CallStatusCancelled  = 5
	CallStatusTimeout    = 6
	CallStatusEnded      = 7
	CallStatusBusy       = 8
	CallStatusFailed     = 9
	CallStatusMissed     = 10
)

// Call End Reasons
const (
	CallEndReasonNormal     = "normal"      // Normal hangup
	CallEndReasonBusy       = "busy"        // Callee was busy
	CallEndReasonTimeout    = "timeout"     // No answer
	CallEndReasonRejected   = "rejected"    // Callee rejected
	CallEndReasonCancelled  = "cancelled"   // Caller cancelled
	CallEndReasonError      = "error"       // Technical error
	CallEndReasonNoNetwork  = "no_network"  // Network issue
)

// Call Configuration
const (
	// DefaultCallTimeout is the default ring timeout in seconds
	DefaultCallTimeout = 40

	// MaxCallTimeout is the maximum allowed ring timeout in seconds
	MaxCallTimeout = 120
)
