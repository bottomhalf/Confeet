package hub

import (
	"Confeet/internal/event"
	"Confeet/internal/model"
	"encoding/json"
	"log"
	"time"
)

// -----------------------------------------------------------------
// Notification Methods - Send Events to Clients
// -----------------------------------------------------------------

// getBusyDetail returns list of userIDs that are currently busy/in a call
func (ch *CallHandler) GetUserDetail(ids []string) map[string]*model.CallParticipant {
	ch.hub.onlineUsersMu.RLock()
	defer ch.hub.onlineUsersMu.RUnlock()

	participants := make(map[string]*model.CallParticipant)
	for _, id := range ids {
		user, err := ch.hub.userRepository.GetUser(id)
		if err != nil {
			log.Printf("failed to get user details for %s: %v", id, err)
			continue
		}
		participants[id] = &model.CallParticipant{
			UserID: id,
			Name:   user.FirstName + " " + user.LastName,
			Avatar: user.Avatar,
			Email:  user.Email,
			Status: model.ParticipantStatusRinging,
		}
	}
	return participants
}

func (ch *CallHandler) notifyCallees(call *model.ActiveGroupCall, callerID string, isJoinRequest bool) {
	incomingEvent := model.CallIncomingEvent{
		ConversationID: call.ConversationID,
		CallerID:       callerID,
		Participants:   call.Participants,
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

	call.Mu.RLock()
	for userID := range call.Participants {
		ch.sendToUser(userID, ev)
	}
	call.Mu.RUnlock()
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

func (ch *CallHandler) notifyCallDismissed(callID, callerID, dismissedBy, reason string) {
	dismissEvent := model.CallDismissedEvent{
		CallID:      callID,
		DismissedBy: dismissedBy,
		Reason:      reason,
		Timestamp:   time.Now().Unix(),
	}

	payload, _ := json.Marshal(dismissEvent)
	ev := event.WsEvent{
		Event:   event.EventCallDismissed,
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
