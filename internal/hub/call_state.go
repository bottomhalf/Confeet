package hub

import (
	"Confeet/internal/model"
	"log"
	"time"
)

// -----------------------------------------------------------------
// Call State Management - Registration and Lifecycle
// -----------------------------------------------------------------

func (ch *CallHandler) registerCall(call *model.ActiveGroupCall) {
	ch.activeCallsMu.Lock()
	ch.activeCalls[call.ConversationID] = call
	ch.activeCallsMu.Unlock()
}

func (ch *CallHandler) getActiveCall(conversationID string) *model.ActiveGroupCall {
	ch.activeCallsMu.RLock()
	call := ch.activeCalls[conversationID]
	ch.activeCallsMu.RUnlock()
	return call
}

func (ch *CallHandler) endCall(call *model.ActiveGroupCall, reason string) {
	ch.activeCallsMu.Lock()
	delete(ch.activeCalls, call.ConversationID)
	ch.activeCallsMu.Unlock()

	// Clear caller busy status
	ch.clearUserBusy(call.CallerID)

	// Clear all participants busy status
	call.Mu.RLock()
	for userID := range call.Participants {
		ch.clearUserBusy(userID)
	}
	call.Mu.RUnlock()
	log.Printf("Call %s ended with reason: %s", call.ConversationID, reason)
}

// countParticipantStates counts participants in ringing and accepted states
// IMPORTANT: Must be called with call.mu held (either RLock or Lock)
func (ch *CallHandler) countParticipantStates(call *model.ActiveGroupCall) (ringingCount, acceptedCount int) {
	for _, p := range call.Participants {
		switch p.Status {
		case model.ParticipantStatusRinging:
			ringingCount++
		case model.ParticipantStatusAccepted:
			acceptedCount++
		}
	}
	return
}

// calculateCallDuration calculates duration from the earliest participant join time
func (ch *CallHandler) calculateCallDuration(call *model.ActiveGroupCall) int {
	call.Mu.RLock()
	defer call.Mu.RUnlock()

	var earliestJoin *time.Time
	for _, p := range call.Participants {
		if p.JoinedAt != nil {
			if earliestJoin == nil || p.JoinedAt.Before(*earliestJoin) {
				earliestJoin = p.JoinedAt
			}
		}
	}

	if earliestJoin == nil {
		return 0
	}
	return int(time.Since(*earliestJoin).Seconds())
}

// -----------------------------------------------------------------
// User Busy Status Management (using Client.status)
// -----------------------------------------------------------------

// setUserBusy marks a user as in a call using their Client status
func (ch *CallHandler) setUserBusy(userID string, conversationID string) {
	ch.hub.onlineUsersMu.RLock()
	client, online := ch.hub.onlineUsers[userID]
	ch.hub.onlineUsersMu.RUnlock()

	if online {
		client.SetCallStatusAsInCall(conversationID)
	}
}

// setUserHavingCall marks a user as having an incoming call
func (ch *CallHandler) setUserHavingCall(userID string, conversationID string) {
	ch.hub.onlineUsersMu.RLock()
	client, online := ch.hub.onlineUsers[userID]
	ch.hub.onlineUsersMu.RUnlock()

	if online {
		client.SetGettingCallStatus(conversationID)
	}
}

// clearUserBusy clears a user's call status back to online
func (ch *CallHandler) clearUserBusy(userID string) {
	ch.hub.onlineUsersMu.RLock()
	client, online := ch.hub.onlineUsers[userID]
	ch.hub.onlineUsersMu.RUnlock()

	if online {
		client.ClearCallStatus()
	}
}

// isUserBusy checks if a user is currently in a call
func (ch *CallHandler) isUserBusy(userID string) bool {
	ch.hub.onlineUsersMu.RLock()
	client, online := ch.hub.onlineUsers[userID]
	ch.hub.onlineUsersMu.RUnlock()

	if !online {
		return false
	}
	return client.IsInCall()
}

// getBusyCallees returns list of userIDs that are currently busy/in a call
func (ch *CallHandler) getBusyCallees(calleeIDs []string) []string {
	ch.hub.onlineUsersMu.RLock()
	defer ch.hub.onlineUsersMu.RUnlock()

	busy := make([]string, 0)
	for _, id := range calleeIDs {
		if client, online := ch.hub.onlineUsers[id]; online {
			if client.IsInCall() {
				busy = append(busy, id)
			}
		}
	}
	return busy
}

// filterBusyUsers removes busy users from the calleeIDs list
func (ch *CallHandler) filterBusyUsers(calleeIDs []string, busyUsers []string) []string {
	busyMap := make(map[string]bool)
	for _, id := range busyUsers {
		busyMap[id] = true
	}

	available := make([]string, 0)
	for _, id := range calleeIDs {
		if !busyMap[id] {
			available = append(available, id)
		}
	}
	return available
}

// -----------------------------------------------------------------
// LiveKit Integration
// -----------------------------------------------------------------

// generateLiveKitToken generates a LiveKit access token for a participant
// TODO: Replace with actual LiveKit SDK token generation
func (ch *CallHandler) generateLiveKitToken(roomName, userID string) string {
	// This is a placeholder. In production, use livekit-server-sdk-go:
	//
	// import "github.com/livekit/protocol/auth"
	//
	// at := auth.NewAccessToken(apiKey, apiSecret)
	// grant := &auth.VideoGrant{
	//     RoomJoin: true,
	//     Room:     roomName,
	// }
	// at.AddGrant(grant).SetIdentity(userID).SetValidFor(time.Hour)
	// token, _ := at.ToJWT()
	// return token

	return "placeholder_token_" + roomName + "_" + userID
}
