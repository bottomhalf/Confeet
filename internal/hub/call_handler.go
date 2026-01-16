package hub

import (
	"Confeet/internal/event"
	"Confeet/internal/model"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// CallHandler manages call signaling between clients
type CallHandler struct {
	hub *Hub

	// Active calls - maps conversationID to call state
	activeCalls   map[string]*model.ActiveGroupCall
	activeCallsMu sync.RWMutex
}

// NewCallHandler creates a new call handler instance
// Note: Call SetHub() after creating Hub to complete the initialization
func NewCallHandler() *CallHandler {
	return &CallHandler{
		activeCalls: make(map[string]*model.ActiveGroupCall),
	}
}

// SetHub sets the hub reference. Must be called after Hub is created.
func (ch *CallHandler) SetHub(hub *Hub) {
	ch.hub = hub
}

// HandleCallEvent processes call-related WebSocket events
func (ch *CallHandler) HandleCallEvent(ev event.WsEvent, c *Client) {
	switch ev.Event {
	case event.EventCallInitiate:
		ch.handleCallInitiate(ev, c, false)
	case event.EventCallAccept:
		ch.handleCallAccept(ev, c)
	case event.EventCallReject:
		ch.handleCallReject(ev, c)
	case event.EventCallDismiss:
		ch.handleCallDismiss(ev, c)
	case event.EventCallCancel:
		ch.handleCallCancel(ev, c)
	case event.EventCallTimeout:
		ch.handleCallTimeout(ev, c)
	case event.EventCallEnd:
		ch.handleCallEnd(ev, c)
	case event.EventCallStarted:
		ch.handleCallInitiate(ev, c, true)
	case event.EventJoiningRequest:
		ch.raiseJoiningRequest(ev, c)
	case event.EventSendGroupNotification:
		ch.sendGroupNotification(ev, c)
	default:
		log.Printf("unknown call event type: %s", ev.Event)
	}
}

// sendGroupNotification sends a notification to all room participants except the sender
func (ch *CallHandler) sendGroupNotification(ev event.WsEvent, c *Client) {
	var payload model.GroupNotificationPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal group notification payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse group notification request")
		return
	}

	// Validate payload
	if payload.ConversationID == "" {
		ch.sendCallError(c, "", "invalid_conversation_id", "ConversationID is required")
		return
	}

	if payload.NotificationType == "" {
		ch.sendCallError(c, "", "invalid_notification_type", "NotificationType is required")
		return
	}

	// Get room
	room := ch.hub.GetRoom(payload.ConversationID)
	if room == nil {
		log.Printf("room %s not found, cannot send group notification", payload.ConversationID)
		return
	}

	// Get list of members (excluding the sender)
	var members []string
	room.mu.RLock()
	for memberID := range room.Members {
		if memberID != c.userId {
			members = append(members, memberID)
		}
	}
	room.mu.RUnlock()

	log.Printf("Group notification sent: %s to %d members (type: %s)",
		payload.ConversationID, len(members), payload.NotificationType)

	// Send notification to all other members
	ch.sendNotification(payload.ConversationID, payload.NotificationType, members)
}

// handleCallInitiate processes a call initiation request
func (ch *CallHandler) raiseJoiningRequest(ev event.WsEvent, c *Client) {
	var payload model.CallInitiatePayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call initiate payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call initiate request")
		return
	}

	// Validate payload
	if payload.ConversationID == "" {
		ch.sendCallError(c, "", "invalid_conversation_id", "ConversationID is required")
		return
	}

	if payload.CallType != event.CallTypeAudio && payload.CallType != event.CallTypeVideo {
		ch.sendCallError(c, payload.ConversationID, "invalid_call_type", "Conversation must be 'audio' or 'video'")
		return
	}

	// room := ch.hub.GetRoom(payload.ConversationID)
	// if room == nil {
	// 	log.Printf("room %s not found, cannot publish message", payload.ConversationID)
	// 	return
	// }

	// // Get list of members
	// room.mu.RLock()
	// payload.CalleeIDs = make([]string, 0, len(room.Members))
	// for memberID := range room.Members {
	// 	payload.CalleeIDs = append(payload.CalleeIDs, memberID)
	// }
	// room.mu.RUnlock()

	// Set default timeout
	timeout := payload.Timeout
	if timeout <= 0 {
		timeout = event.DefaultCallTimeout
	}
	if timeout > event.MaxCallTimeout {
		timeout = event.MaxCallTimeout
	}

	// Generate LiveKit room name
	roomName := "room_" + payload.ConversationID

	// Create participants map
	participants := ch.GetUserDetail(append(payload.CalleeIDs, c.userId))

	// Create active call record
	activeCall := &model.ActiveGroupCall{
		ConversationID: payload.ConversationID,
		CallerID:       c.userId,
		CallType:       payload.CallType,
		Status:         event.CallStatusRinging,
		Timeout:        timeout,
		CreatedAt:      time.Now(),
		Participants:   participants,
		RoomName:       roomName,
	}

	// Change user status as busy
	ch.setUserBusy(c.userId, payload.ConversationID)

	// Start server timer go routine to handle participant call status
	go ch.startCallTimeoutWatcher(activeCall)

	// Mark all callees as having incoming call
	for _, calleeID := range payload.CalleeIDs {
		if calleeID != c.userId {
			ch.setUserHavingCall(calleeID, payload.ConversationID)
		}
	}

	log.Printf("Call initiated: %s from %s to %v (type: %s, timeout: %ds)",
		payload.ConversationID, c.userId, payload.CalleeIDs, payload.CallType, timeout)
	// Send incoming call notification to all callees
	ch.notifyCallee(activeCall, c.userId)
}

// handleCallInitiate processes a call initiation request
func (ch *CallHandler) handleCallInitiate(ev event.WsEvent, c *Client, isJoinRequest bool) {
	var payload model.CallInitiatePayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call initiate payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call initiate request")
		return
	}

	// Validate payload
	if payload.ConversationID == "" {
		ch.sendCallError(c, "", "invalid_conversation_id", "ConversationID is required")
		return
	}

	if payload.CallType != event.CallTypeAudio && payload.CallType != event.CallTypeVideo {
		ch.sendCallError(c, payload.ConversationID, "invalid_call_type", "Conversation must be 'audio' or 'video'")
		return
	}

	room := ch.hub.GetRoom(payload.ConversationID)
	if room == nil {
		log.Printf("room %s not found, cannot publish message", payload.ConversationID)
		return
	}

	// Get list of members
	room.mu.RLock()
	payload.CalleeIDs = make([]string, 0, len(room.Members))
	for memberID := range room.Members {
		payload.CalleeIDs = append(payload.CalleeIDs, memberID)
	}
	room.mu.RUnlock()

	// Set default timeout
	timeout := payload.Timeout
	if timeout <= 0 {
		timeout = event.DefaultCallTimeout
	}
	if timeout > event.MaxCallTimeout {
		timeout = event.MaxCallTimeout
	}

	// Check if caller is already in a call
	if ch.isUserBusy(c.userId) {
		ch.sendCallError(c, payload.ConversationID, "caller_busy", "You are already in a call")
		return
	}

	// Check if any callee is busy
	busyUsers := ch.getBusyCallees(payload.CalleeIDs)
	if len(busyUsers) > 0 {
		// Notify busy users about the missed call (Option A: Teams-like behavior)
		ch.notifyBusyCallees(payload.ConversationID, c.userId, payload.CallType, busyUsers)

		// For 1-to-1 call, send busy signal to caller
		if len(payload.CalleeIDs) == 1 {
			ch.sendBusySignal(c, payload.ConversationID, busyUsers[0])
			return
		}
		// For group call, remove busy users and continue with available ones
		payload.CalleeIDs = ch.filterBusyUsers(payload.CalleeIDs, busyUsers)
		if len(payload.CalleeIDs) == 0 {
			ch.sendCallError(c, payload.ConversationID, "all_busy", "All callees are busy")
			return
		}
	}

	// Generate LiveKit room name
	roomName := "room_" + payload.ConversationID

	// Create participants map
	participants := ch.GetUserDetail(payload.CalleeIDs)
	currentParticipant := participants[c.userId]
	currentParticipant.Status = model.ParticipantStatusAccepted

	// Create active call record
	activeCall := &model.ActiveGroupCall{
		ConversationID: payload.ConversationID,
		CallerID:       c.userId,
		CallType:       payload.CallType,
		Status:         event.CallStatusRinging,
		Timeout:        timeout,
		CreatedAt:      time.Now(),
		Participants:   participants,
		RoomName:       roomName,
	}

	// Register call and mark caller as busy
	ch.registerCall(activeCall)

	// Start server timer go routine to handle participant call status
	go ch.startCallTimeoutWatcher(activeCall)

	ch.setUserBusy(c.userId, payload.ConversationID)

	// Mark all callees as having incoming call
	for _, calleeID := range payload.CalleeIDs {
		if calleeID != c.userId {
			ch.setUserHavingCall(calleeID, payload.ConversationID)
		}
	}

	log.Printf("Call initiated: %s from %s to %v (type: %s, timeout: %ds)",
		payload.ConversationID, c.userId, payload.CalleeIDs, payload.CallType, timeout)
	// Send incoming call notification to all callees
	ch.notifyCallees(activeCall, c.userId, isJoinRequest)
}

// handleCallAccept processes a call acceptance
func (ch *CallHandler) handleCallAccept(ev event.WsEvent, c *Client) {
	var payload model.CallAcceptPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call accept payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call accept request")
		return
	}

	// Get active call
	activeCall := ch.getActiveCall(payload.ConversationID)
	if activeCall == nil {
		ch.sendCallError(c, payload.ConversationID, "call_not_found", "Call not found or already ended")
		return
	}

	activeCall.Mu.Lock()

	// Verify callee is part of this call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.Mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "not_callee", "You are not a callee of this call")
		return
	}

	// Check if this participant has already accepted or left
	if participant.Status != model.ParticipantStatusRinging {
		activeCall.Mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "already_responded", "You have already responded to this call")
		return
	}

	// Check if call is still active (not ended/cancelled)
	if activeCall.Status == event.CallStatusEnded || activeCall.Status == event.CallStatusCancelled {
		activeCall.Mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "call_ended", "Call has already ended")
		return
	}

	// Update participant status to accepted
	now := time.Now()
	participant.Status = model.ParticipantStatusAccepted
	participant.JoinedAt = &now

	// Generate LiveKit token for this participant
	calleeToken := ch.generateLiveKitToken(activeCall.RoomName, c.userId)
	participant.LiveKitToken = calleeToken

	// Check if this is the first participant to accept
	isFirstAccept := activeCall.Status == event.CallStatusRinging
	if isFirstAccept {
		// Transition call from Ringing to Accepted (call is now active)
		activeCall.Status = event.CallStatusAccepted
	}

	// Check if this is a 1-to-1 call (only one participant)
	is1to1Call := len(activeCall.Participants) == 1

	activeCall.Mu.Unlock()

	log.Printf("Call accepted: %s by %s (first accept: %v)", payload.ConversationID, c.userId, isFirstAccept)

	// Generate caller token
	callerToken := ch.generateLiveKitToken(activeCall.RoomName, activeCall.CallerID)

	log.Printf("Change client: %s status from ringing to accepted", c.userId)
	// Send incoming call notification to all callees
	ch.setUserBusy(c.userId, payload.ConversationID)

	// Notify caller that call was accepted (send room info if first accept)
	ch.notifyCallAccepted(payload.ConversationID, activeCall.CallerID, c.userId, activeCall.RoomName, callerToken)

	// For 1-to-1 calls, we're done - no need to notify other callees
	if is1to1Call {
		return
	}

	// For group calls: notify OTHER participants who are still ringing
	// that someone joined (they can still join too)
	activeCall.Mu.RLock()
	for userID, p := range activeCall.Participants {
		if userID != c.userId && p.Status == model.ParticipantStatusAccepted {
			ch.notifyParticipantJoined(payload.ConversationID, userID, c.userId)
		}
	}
	activeCall.Mu.RUnlock()
}

// handleCallReject processes a call rejection
func (ch *CallHandler) handleCallReject(ev event.WsEvent, c *Client) {
	var payload model.CallRejectPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call reject payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call reject request")
		return
	}

	// Get active call
	activeCall := ch.getActiveCall(payload.ConversationID)
	if activeCall == nil {
		// Call might have already ended, just clear user status
		ch.clearUserBusy(c.userId, payload.ConversationID)
		return
	}

	activeCall.Mu.Lock()

	// Verify callee is part of this call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.Mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "not_callee", "You are not a callee of this call")
		return
	}

	// Update participant status
	now := time.Now()
	participant.Status = model.ParticipantStatusRejected
	participant.LeftAt = &now

	// Check how many are still ringing or have accepted
	ringingCount, acceptedCount := ch.countParticipantStates(activeCall)
	is1to1Call := len(activeCall.Participants) == 1

	activeCall.Mu.Unlock()

	log.Printf("Call rejected: %s by %s (reason: %s)", payload.ConversationID, c.userId, payload.Reason)

	// Clear this user's busy status
	ch.clearUserBusy(c.userId, payload.ConversationID)

	// Notify caller about rejection
	ch.notifyCallRejected(payload.ConversationID, activeCall.CallerID, c.userId, payload.Reason)

	// For 1-to-1 call, end the call entirely
	if is1to1Call {
		ch.endCall(activeCall, event.CallEndReasonRejected)
		return
	}

	// For group call: if no one is ringing and no one has accepted, end the call
	if ringingCount == 0 && acceptedCount == 0 {
		ch.endCall(activeCall, event.CallEndReasonRejected)
	}
}

// handleCallDismiss processes a call rejection
func (ch *CallHandler) handleCallDismiss(ev event.WsEvent, c *Client) {
	var payload model.CallDismissPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call dismiss payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call dismiss request")
		return
	}

	// Get active call
	activeCall := ch.getActiveCall(payload.ConversationID)
	if activeCall == nil {
		// Call might have already ended, just clear user status
		ch.clearUserBusy(c.userId, payload.ConversationID)
		return
	}

	activeCall.Mu.Lock()

	// Verify callee is part of this call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.Mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "not_callee", "You are not a callee of this call")
		return
	}

	// Update participant status
	now := time.Now()
	participant.Status = model.ParticipantStatusDismissed
	participant.LeftAt = &now

	activeCall.Mu.Unlock()

	log.Printf("Call dismissed: %s by %s (reason: %s)", payload.ConversationID, c.userId, payload.Reason)

	// notify caller about dismissal
	ch.notifyCallDismissed(payload.ConversationID, payload.CallerID, c.userId, payload.Reason)
}

// handleCallCancel processes a call cancellation by caller
func (ch *CallHandler) handleCallCancel(ev event.WsEvent, c *Client) {
	var payload model.CallCancelPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call cancel payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call cancel request")
		return
	}

	// Get active call
	activeCall := ch.getActiveCall(payload.ConversationID)
	if activeCall == nil {
		ch.clearUserBusy(c.userId, payload.ConversationID)
		return
	}

	// Verify caller is the one cancelling
	if activeCall.CallerID != c.userId {
		ch.sendCallError(c, payload.ConversationID, "not_caller", "Only caller can cancel the call")
		return
	}

	log.Printf("Call cancelled: %s by caller %s", payload.ConversationID, c.userId)

	// Notify all participants that call was cancelled
	activeCall.Mu.RLock()
	for userID := range activeCall.Participants {
		ch.notifyCallCancelled(payload.ConversationID, userID, c.userId)
		ch.clearUserBusy(userID, payload.ConversationID)
	}
	activeCall.Mu.RUnlock()

	// End the call
	ch.endCall(activeCall, event.CallEndReasonCancelled)
}

// handleCallTimeout processes a call timeout from callee
func (ch *CallHandler) handleCallTimeout(ev event.WsEvent, c *Client) {
	var payload model.CallTimeoutPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call timeout payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call timeout request")
		return
	}

	// Get active call
	activeCall := ch.getActiveCall(payload.ConversationID)
	if activeCall == nil {
		ch.clearUserBusy(c.userId, payload.ConversationID)
		return
	}

	activeCall.Mu.Lock()

	// Verify callee is part of this call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.Mu.Unlock()
		ch.clearUserBusy(c.userId, payload.ConversationID)
		return
	}

	// Update participant status
	now := time.Now()
	participant.Status = model.ParticipantStatusTimeout
	participant.LeftAt = &now

	// Check how many are still ringing or have accepted
	ringingCount, acceptedCount := ch.countParticipantStates(activeCall)
	is1to1Call := len(activeCall.Participants) == 1

	activeCall.Mu.Unlock()

	log.Printf("Call timeout: %s reported by %s", payload.ConversationID, c.userId)

	// Clear this user's busy status
	ch.clearUserBusy(c.userId, payload.ConversationID)

	// For 1-to-1 call, notify caller and end call
	if is1to1Call {
		ch.notifyCallTimedOut(payload.ConversationID, activeCall.CallerID)
		ch.endCall(activeCall, event.CallEndReasonTimeout)
		return
	}

	// For group call: if no one is ringing and no one has accepted, end the call
	if ringingCount == 0 && acceptedCount == 0 {
		ch.notifyCallTimedOut(payload.ConversationID, activeCall.CallerID)
		ch.endCall(activeCall, event.CallEndReasonTimeout)
	}
}

// handleCallEnd processes a call end request (participant leaving or ending call)
func (ch *CallHandler) handleCallEnd(ev event.WsEvent, c *Client) {
	var payload model.CallEndPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		log.Printf("failed to unmarshal call end payload: %v", err)
		ch.sendCallError(c, "", "invalid_payload", "Failed to parse call end request")
		return
	}

	// Get active call
	activeCall := ch.getActiveCall(payload.ConversationID)
	if activeCall == nil {
		ch.clearUserBusy(c.userId, payload.ConversationID)
		return
	}

	reason := payload.Reason
	if reason == "" {
		reason = event.CallEndReasonNormal
	}

	activeCall.Mu.Lock()

	// // Check if caller is ending the call (ends for everyone)
	// if c.userId == activeCall.CallerID {
	// 	activeCall.Mu.Unlock()
	// 	log.Printf("Call ended by caller: %s by %s (reason: %s)", payload.ConversationID, c.userId, reason)

	// 	// Calculate duration from first accepted participant
	// 	duration := ch.calculateCallDuration(activeCall)

	// 	// Notify all participants that call has ended
	// 	for userID, p := range activeCall.Participants {
	// 		if p.Status == ParticipantStatusAccepted || p.Status == ParticipantStatusRinging {
	// 			ch.notifyCallEnded(payload.ConversationID, userID, c.userId, reason, duration)
	// 		}
	// 	}

	// 	// End the call
	// 	ch.endCall(activeCall, reason)
	// 	return
	// }

	// A participant is leaving the call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.Mu.Unlock()
		ch.clearUserBusy(c.userId, payload.ConversationID)
		return
	}

	// Calculate duration for this participant
	duration := 0
	if participant.JoinedAt != nil {
		duration = int(time.Since(*participant.JoinedAt).Seconds())
	}

	// Mark participant as left
	now := time.Now()
	participant.Status = model.ParticipantStatusLeft
	participant.LeftAt = &now

	// Count remaining active participants (still in call)
	_, acceptedCount := ch.countParticipantStates(activeCall)
	is1to1Call := len(activeCall.Participants) == 1

	activeCall.Mu.Unlock()

	log.Printf("Participant left call: %s by %s (reason: %s)", payload.ConversationID, c.userId, reason)

	// Clear this user's busy status
	ch.clearUserBusy(c.userId, payload.ConversationID)

	// Notify caller that participant left
	ch.notifyParticipantLeft(payload.ConversationID, activeCall.CallerID, c.userId, reason, duration)

	// Notify other active participants
	activeCall.Mu.RLock()
	for userID, p := range activeCall.Participants {
		if userID != c.userId && p.Status == model.ParticipantStatusAccepted {
			ch.notifyParticipantLeft(payload.ConversationID, userID, c.userId, reason, duration)
		}
	}
	activeCall.Mu.RUnlock()

	// For 1-to-1 call or if no participants left, end the call
	if is1to1Call || acceptedCount == 0 {
		ch.endCall(activeCall, reason)
	}
}

// IsCallEvent checks if an event is a call-related event
func IsCallEvent(eventType string) bool {
	switch eventType {
	case event.EventCallInitiate,
		event.EventCallAccept,
		event.EventCallReject,
		event.EventCallCancel,
		event.EventCallTimeout,
		event.EventCallStarted,
		event.EventCallDismiss,
		event.EventCallEnd,
		event.EventJoiningRequest:
		return true
	default:
		return false
	}
}
