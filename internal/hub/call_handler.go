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
	activeCalls   map[string]*ActiveGroupCall
	activeCallsMu sync.RWMutex
}

// ParticipantStatus tracks individual callee state in a group call
type ParticipantStatus int

const (
	ParticipantStatusRinging  ParticipantStatus = 1
	ParticipantStatusAccepted ParticipantStatus = 2
	ParticipantStatusRejected ParticipantStatus = 3
	ParticipantStatusTimeout  ParticipantStatus = 4
	ParticipantStatusLeft     ParticipantStatus = 5
)

// CallParticipant tracks state of each callee in a group call
type CallParticipant struct {
	UserID       string
	Status       ParticipantStatus
	JoinedAt     *time.Time // When they accepted/joined
	LeftAt       *time.Time // When they left/rejected
	LiveKitToken string     // Their individual LiveKit token
}

// ActiveGroupCall tracks the state of an ongoing call (1-to-1 or group)
type ActiveGroupCall struct {
	ConversationID string
	CallerID       string
	CallType       string
	Status         int // Overall call status
	Timeout        int
	CreatedAt      time.Time
	Participants   map[string]*CallParticipant // Maps userID to their participation state
	RoomName       string                      // LiveKit room name
	mu             sync.RWMutex
}

// NewCallHandler creates a new call handler instance
// Note: Call SetHub() after creating Hub to complete the initialization
func NewCallHandler() *CallHandler {
	return &CallHandler{
		activeCalls: make(map[string]*ActiveGroupCall),
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
	case event.EventCallCancel:
		ch.handleCallCancel(ev, c)
	case event.EventCallTimeout:
		ch.handleCallTimeout(ev, c)
	case event.EventCallEnd:
		ch.handleCallEnd(ev, c)
	case event.EventCallStarted:
		ch.handleCallInitiate(ev, c, true)
	default:
		log.Printf("unknown call event type: %s", ev.Event)
	}
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
	participants := make(map[string]*CallParticipant)
	for _, calleeID := range payload.CalleeIDs {
		participants[calleeID] = &CallParticipant{
			UserID: calleeID,
			Status: ParticipantStatusAccepted,
		}
	}

	// Create active call record
	activeCall := &ActiveGroupCall{
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

	activeCall.mu.Lock()

	// Verify callee is part of this call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "not_callee", "You are not a callee of this call")
		return
	}

	// Check if this participant has already accepted or left
	if participant.Status != ParticipantStatusRinging {
		activeCall.mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "already_responded", "You have already responded to this call")
		return
	}

	// Check if call is still active (not ended/cancelled)
	if activeCall.Status == event.CallStatusEnded || activeCall.Status == event.CallStatusCancelled {
		activeCall.mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "call_ended", "Call has already ended")
		return
	}

	// Update participant status to accepted
	now := time.Now()
	participant.Status = ParticipantStatusAccepted
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

	activeCall.mu.Unlock()

	log.Printf("Call accepted: %s by %s (first accept: %v)", payload.ConversationID, c.userId, isFirstAccept)

	// Generate caller token
	callerToken := ch.generateLiveKitToken(activeCall.RoomName, activeCall.CallerID)

	// Notify caller that call was accepted (send room info if first accept)
	ch.notifyCallAccepted(payload.ConversationID, activeCall.CallerID, c.userId, activeCall.RoomName, callerToken)

	// Send room info to accepting callee
	ch.sendRoomInfo(c, payload.ConversationID, activeCall.CallType, activeCall.RoomName, calleeToken)

	// For 1-to-1 calls, we're done - no need to notify other callees
	if is1to1Call {
		return
	}

	// For group calls: notify OTHER participants who are still ringing
	// that someone joined (they can still join too)
	activeCall.mu.RLock()
	for userID, p := range activeCall.Participants {
		if userID != c.userId && p.Status == ParticipantStatusRinging {
			ch.notifyParticipantJoined(payload.ConversationID, userID, c.userId)
		}
	}
	activeCall.mu.RUnlock()
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
		ch.clearUserBusy(c.userId)
		return
	}

	activeCall.mu.Lock()

	// Verify callee is part of this call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.mu.Unlock()
		ch.sendCallError(c, payload.ConversationID, "not_callee", "You are not a callee of this call")
		return
	}

	// Update participant status
	now := time.Now()
	participant.Status = ParticipantStatusRejected
	participant.LeftAt = &now

	// Check how many are still ringing or have accepted
	ringingCount, acceptedCount := ch.countParticipantStates(activeCall)
	is1to1Call := len(activeCall.Participants) == 1

	activeCall.mu.Unlock()

	log.Printf("Call rejected: %s by %s (reason: %s)", payload.ConversationID, c.userId, payload.Reason)

	// Clear this user's busy status
	ch.clearUserBusy(c.userId)

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
		ch.clearUserBusy(c.userId)
		return
	}

	// Verify caller is the one cancelling
	if activeCall.CallerID != c.userId {
		ch.sendCallError(c, payload.ConversationID, "not_caller", "Only caller can cancel the call")
		return
	}

	log.Printf("Call cancelled: %s by caller %s", payload.ConversationID, c.userId)

	// Notify all participants that call was cancelled
	activeCall.mu.RLock()
	for userID := range activeCall.Participants {
		ch.notifyCallCancelled(payload.ConversationID, userID, c.userId)
		ch.clearUserBusy(userID)
	}
	activeCall.mu.RUnlock()

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
		ch.clearUserBusy(c.userId)
		return
	}

	activeCall.mu.Lock()

	// Verify callee is part of this call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.mu.Unlock()
		ch.clearUserBusy(c.userId)
		return
	}

	// Update participant status
	now := time.Now()
	participant.Status = ParticipantStatusTimeout
	participant.LeftAt = &now

	// Check how many are still ringing or have accepted
	ringingCount, acceptedCount := ch.countParticipantStates(activeCall)
	is1to1Call := len(activeCall.Participants) == 1

	activeCall.mu.Unlock()

	log.Printf("Call timeout: %s reported by %s", payload.ConversationID, c.userId)

	// Clear this user's busy status
	ch.clearUserBusy(c.userId)

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
		ch.clearUserBusy(c.userId)
		return
	}

	reason := payload.Reason
	if reason == "" {
		reason = event.CallEndReasonNormal
	}

	activeCall.mu.Lock()

	// Check if caller is ending the call (ends for everyone)
	if c.userId == activeCall.CallerID {
		activeCall.mu.Unlock()

		log.Printf("Call ended by caller: %s by %s (reason: %s)", payload.ConversationID, c.userId, reason)

		// Calculate duration from first accepted participant
		duration := ch.calculateCallDuration(activeCall)

		// Notify all participants that call has ended
		for userID, p := range activeCall.Participants {
			if p.Status == ParticipantStatusAccepted || p.Status == ParticipantStatusRinging {
				ch.notifyCallEnded(payload.ConversationID, userID, c.userId, reason, duration)
			}
		}

		// End the call
		ch.endCall(activeCall, reason)
		return
	}

	// A participant is leaving the call
	participant, exists := activeCall.Participants[c.userId]
	if !exists {
		activeCall.mu.Unlock()
		ch.clearUserBusy(c.userId)
		return
	}

	// Calculate duration for this participant
	duration := 0
	if participant.JoinedAt != nil {
		duration = int(time.Since(*participant.JoinedAt).Seconds())
	}

	// Mark participant as left
	now := time.Now()
	participant.Status = ParticipantStatusLeft
	participant.LeftAt = &now

	// Count remaining active participants (still in call)
	_, acceptedCount := ch.countParticipantStates(activeCall)
	is1to1Call := len(activeCall.Participants) == 1

	activeCall.mu.Unlock()

	log.Printf("Participant left call: %s by %s (reason: %s)", payload.ConversationID, c.userId, reason)

	// Clear this user's busy status
	ch.clearUserBusy(c.userId)

	// Notify caller that participant left
	ch.notifyParticipantLeft(payload.ConversationID, activeCall.CallerID, c.userId, reason, duration)

	// Notify other active participants
	activeCall.mu.RLock()
	for userID, p := range activeCall.Participants {
		if userID != c.userId && p.Status == ParticipantStatusAccepted {
			ch.notifyParticipantLeft(payload.ConversationID, userID, c.userId, reason, duration)
		}
	}
	activeCall.mu.RUnlock()

	// For 1-to-1 call or if no participants left, end the call
	if is1to1Call || acceptedCount == 0 {
		ch.endCall(activeCall, reason)
	}
}

// -----------------------------------------------------------------
// Helper Methods - Call State Management
// -----------------------------------------------------------------

func (ch *CallHandler) registerCall(call *ActiveGroupCall) {
	ch.activeCallsMu.Lock()
	ch.activeCalls[call.ConversationID] = call
	ch.activeCallsMu.Unlock()
}

func (ch *CallHandler) getActiveCall(conversationID string) *ActiveGroupCall {
	ch.activeCallsMu.RLock()
	call := ch.activeCalls[conversationID]
	ch.activeCallsMu.RUnlock()
	return call
}

func (ch *CallHandler) endCall(call *ActiveGroupCall, reason string) {
	ch.activeCallsMu.Lock()
	delete(ch.activeCalls, call.ConversationID)
	ch.activeCallsMu.Unlock()

	// Clear caller busy status
	ch.clearUserBusy(call.CallerID)

	// Clear all participants busy status
	call.mu.RLock()
	for userID := range call.Participants {
		ch.clearUserBusy(userID)
	}
	call.mu.RUnlock()

	log.Printf("Call %s ended with reason: %s", call.ConversationID, reason)
}

// countParticipantStates counts participants in ringing and accepted states
// IMPORTANT: Must be called with call.mu held (either RLock or Lock)
func (ch *CallHandler) countParticipantStates(call *ActiveGroupCall) (ringingCount, acceptedCount int) {
	for _, p := range call.Participants {
		switch p.Status {
		case ParticipantStatusRinging:
			ringingCount++
		case ParticipantStatusAccepted:
			acceptedCount++
		}
	}
	return
}

// calculateCallDuration calculates duration from the earliest participant join time
func (ch *CallHandler) calculateCallDuration(call *ActiveGroupCall) int {
	call.mu.RLock()
	defer call.mu.RUnlock()

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
// Helper Methods - User Busy Status (using Client.status)
// -----------------------------------------------------------------

// setUserBusy marks a user as in a call using their Client status
func (ch *CallHandler) setUserBusy(userID string, conversationID string) {
	ch.hub.onlineUsersMu.RLock()
	client, online := ch.hub.onlineUsers[userID]
	ch.hub.onlineUsersMu.RUnlock()

	if online {
		client.SetCallStatus(conversationID)
	}
}

// setUserBusy marks a user as in a call using their Client status
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
// Helper Methods - Send Events to Clients
// -----------------------------------------------------------------

func (ch *CallHandler) notifyCallees(call *ActiveGroupCall, callerID string, isJoinRequest bool) {
	incomingEvent := model.CallIncomingEvent{
		ConversationID: call.ConversationID,
		CallerID:       callerID,
		// CallerName and CallerAvatar can be fetched from user service if needed
		CallType:  call.CallType,
		Timeout:   call.Timeout,
		Timestamp: time.Now().Unix(),
	}

	callEventName := event.EventCallIncoming
	if isJoinRequest {
		callEventName = event.EventCallJoiningRequest
	}

	payload, _ := json.Marshal(incomingEvent)
	ev := event.WsEvent{
		Event:   callEventName,
		Payload: payload,
	}

	call.mu.RLock()
	for userID := range call.Participants {
		ch.sendToUser(userID, ev)
	}
	call.mu.RUnlock()
}

func (ch *CallHandler) notifyCallAccepted(callID, callerID, acceptedBy, roomName, token string) {
	acceptedEvent := model.CallAcceptedEvent{
		CallID:     callID,
		AcceptedBy: acceptedBy,
		RoomName:   roomName,
		Token:      token,
		Timestamp:  time.Now().Unix(),
	}

	payload, _ := json.Marshal(acceptedEvent)
	ev := event.WsEvent{
		Event:   event.EventCallAccepted,
		Payload: payload,
	}

	ch.sendToUser(callerID, ev)
}

func (ch *CallHandler) notifyCallRejected(callID, callerID, rejectedBy, reason string) {
	rejectedEvent := model.CallRejectedEvent{
		CallID:     callID,
		RejectedBy: rejectedBy,
		Reason:     reason,
		Timestamp:  time.Now().Unix(),
	}

	payload, _ := json.Marshal(rejectedEvent)
	ev := event.WsEvent{
		Event:   event.EventCallRejected,
		Payload: payload,
	}

	ch.sendToUser(callerID, ev)
}

func (ch *CallHandler) notifyCallCancelled(callID, calleeID, cancelledBy string) {
	cancelledEvent := model.CallCancelledEvent{
		CallID:      callID,
		CancelledBy: cancelledBy,
		Timestamp:   time.Now().Unix(),
	}

	payload, _ := json.Marshal(cancelledEvent)
	ev := event.WsEvent{
		Event:   event.EventCallCancelled,
		Payload: payload,
	}

	ch.sendToUser(calleeID, ev)
}

func (ch *CallHandler) notifyCallTimedOut(callID, callerID string) {
	timedOutEvent := model.CallTimedOutEvent{
		CallID:    callID,
		Timestamp: time.Now().Unix(),
	}

	payload, _ := json.Marshal(timedOutEvent)
	ev := event.WsEvent{
		Event:   event.EventCallTimedOut,
		Payload: payload,
	}

	ch.sendToUser(callerID, ev)
}

func (ch *CallHandler) notifyCallEnded(callID, userID, endedBy, reason string, duration int) {
	endedEvent := model.CallEndedEvent{
		CallID:    callID,
		EndedBy:   endedBy,
		Reason:    reason,
		Duration:  duration,
		Timestamp: time.Now().Unix(),
	}

	payload, _ := json.Marshal(endedEvent)
	ev := event.WsEvent{
		Event:   event.EventCallEnded,
		Payload: payload,
	}

	ch.sendToUser(userID, ev)
}

// notifyParticipantJoined notifies a user that someone joined the group call
func (ch *CallHandler) notifyParticipantJoined(callID, recipientID, joinedUserID string) {
	joinedEvent := model.CallParticipantJoinedEvent{
		CallID:    callID,
		UserID:    joinedUserID,
		Timestamp: time.Now().Unix(),
	}

	payload, _ := json.Marshal(joinedEvent)
	ev := event.WsEvent{
		Event:   event.EventCallParticipantJoined,
		Payload: payload,
	}

	ch.sendToUser(recipientID, ev)
}

// notifyParticipantLeft notifies a user that someone left the group call
func (ch *CallHandler) notifyParticipantLeft(callID, recipientID, leftUserID, reason string, duration int) {
	leftEvent := model.CallParticipantLeftEvent{
		CallID:    callID,
		UserID:    leftUserID,
		Reason:    reason,
		Duration:  duration,
		Timestamp: time.Now().Unix(),
	}

	payload, _ := json.Marshal(leftEvent)
	ev := event.WsEvent{
		Event:   event.EventCallParticipantLeft,
		Payload: payload,
	}

	ch.sendToUser(recipientID, ev)
}

func (ch *CallHandler) notifyCallAnsweredElsewhere(callID, calleeID string) {
	// Send a cancelled event with a special indicator
	cancelledEvent := model.CallCancelledEvent{
		CallID:      callID,
		CancelledBy: "answered_elsewhere",
		Timestamp:   time.Now().Unix(),
	}

	payload, _ := json.Marshal(cancelledEvent)
	ev := event.WsEvent{
		Event:   event.EventCallCancelled,
		Payload: payload,
	}

	ch.sendToUser(calleeID, ev)
}

func (ch *CallHandler) sendBusySignal(c *Client, conversationID, busyUserID string) {
	busyEvent := model.CallBusyEvent{
		ConversationID: conversationID,
		BusyUser:       busyUserID,
		Timestamp:      time.Now().Unix(),
	}

	payload, _ := json.Marshal(busyEvent)
	ev := event.WsEvent{
		Event:   event.EventCallBusy,
		Payload: payload,
	}

	c.SafeSend(ev, sendTimeout)
}

// notifyMissedCall sends a missed call notification to a busy user
func (ch *CallHandler) notifyMissedCall(conversationID, calleeID, callerID, callType, reason string) {
	missedEvent := model.CallMissedEvent{
		ConversationID: conversationID,
		CallerID:       callerID,
		// CallerName and CallerAvatar can be fetched from user service if needed
		CallType:  callType,
		Reason:    reason,
		Timestamp: time.Now().Unix(),
	}

	payload, _ := json.Marshal(missedEvent)
	ev := event.WsEvent{
		Event:   event.EventCallMissed,
		Payload: payload,
	}

	ch.sendToUser(calleeID, ev)
}

// notifyBusyCallees sends missed call notifications to all busy callees
func (ch *CallHandler) notifyBusyCallees(conversationID, callerID, callType string, busyUserIDs []string) {
	for _, busyUserID := range busyUserIDs {
		ch.notifyMissedCall(conversationID, busyUserID, callerID, callType, event.CallEndReasonBusy)
		log.Printf("Sent missed call notification to busy user %s for conversation %s", busyUserID, conversationID)
	}
}

func (ch *CallHandler) sendRoomInfo(c *Client, callID, callType, roomName, token string) {
	joinInfo := model.CallJoinInfo{
		CallID:   callID,
		CallType: callType,
		LiveKit: model.LiveKitRoomInfo{
			RoomName: roomName,
			Token:    token,
			// URL should come from config
		},
	}

	payload, _ := json.Marshal(joinInfo)
	ev := event.WsEvent{
		Event:   event.EventCallAccepted,
		Payload: payload,
	}

	c.SafeSend(ev, sendTimeout)
}

func (ch *CallHandler) sendCallError(c *Client, callID, code, message string) {
	errorEvent := model.CallErrorEvent{
		CallID:    callID,
		Error:     message,
		Code:      code,
		Timestamp: time.Now().Unix(),
	}

	payload, _ := json.Marshal(errorEvent)
	ev := event.WsEvent{
		Event:   event.EventCallError,
		Payload: payload,
	}

	c.SafeSend(ev, sendTimeout)
}

func (ch *CallHandler) sendToUser(userID string, ev event.WsEvent) {
	ch.hub.onlineUsersMu.RLock()
	client, online := ch.hub.onlineUsers[userID]
	ch.hub.onlineUsersMu.RUnlock()

	if online {
		if !client.SafeSend(ev, sendTimeout) {
			log.Printf("failed to send call event to user %s", userID)
		}
	} else {
		log.Printf("user %s is offline, cannot deliver call event", userID)
		// TODO: Send push notification for offline users
	}
}

// -----------------------------------------------------------------
// LiveKit Integration (Placeholder)
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

// IsCallEvent checks if an event is a call-related event
func IsCallEvent(eventType string) bool {
	switch eventType {
	case event.EventCallInitiate,
		event.EventCallAccept,
		event.EventCallReject,
		event.EventCallCancel,
		event.EventCallTimeout,
		event.EventCallStarted,
		event.EventCallEnd:
		return true
	default:
		return false
	}
}
