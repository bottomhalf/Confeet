package hub

import (
	"Confeet/internal/model"
	"time"
)

// MonitorService provides methods to gather hub statistics
type MonitorService struct {
	hub *Hub
}

// NewMonitorService creates a new monitor service
func NewMonitorService(hub *Hub) *MonitorService {
	return &MonitorService{hub: hub}
}

// GetStats gathers and returns all hub statistics
func (ms *MonitorService) GetStats() model.MonitorResponse {
	connectionStats := ms.getConnectionStats()
	roomStats := ms.getRoomStats()
	callStats := ms.getCallStats()
	roomParticipantsStats := ms.getCurrentRoomClientsDetail()
	clients := ms.getClientList()
	statusCount := ms.getStatusCount()

	// Determine overall health status
	status := "healthy"
	if connectionStats.TotalConnected == 0 {
		status = "idle"
	}

	return model.MonitorResponse{
		Status:                status,
		Connections:           connectionStats,
		RoomParticipantsStats: roomParticipantsStats,
		Rooms:                 roomStats,
		Calls:                 callStats,
		Clients:               clients,
		StatusCount:           statusCount,
	}
}

// getConnectionStats returns connection statistics
func (ms *MonitorService) getConnectionStats() model.ConnectionStats {
	ms.hub.onlineUsersMu.RLock()
	defer ms.hub.onlineUsersMu.RUnlock()

	stats := model.ConnectionStats{
		TotalConnected: len(ms.hub.onlineUsers),
	}

	for _, client := range ms.hub.onlineUsers {
		switch client.GetStatus() {
		case StatusOnline:
			stats.TotalOnline++
		case StatusBusy:
			stats.TotalBusy++
		case StatusInCall:
			stats.TotalInCall++
		case StatusAway:
			stats.TotalAway++
		}
	}

	return stats
}

// getRoomStats returns room/conversation statistics
func (ms *MonitorService) getRoomStats() model.RoomStats {
	stats := model.RoomStats{
		RoomDetails: make([]model.RoomInfo, 0),
	}

	// Iterate through all shards to collect room info
	for _, bucket := range ms.hub.shards {
		bucket.RLock()
		for _, room := range bucket.rooms {
			room.mu.RLock()

			memberIDs := make([]string, 0, len(room.Members))
			for memberID := range room.Members {
				memberIDs = append(memberIDs, memberID)
			}

			// Count online members
			onlineCount := 0
			ms.hub.onlineUsersMu.RLock()
			for _, memberID := range memberIDs {
				if _, online := ms.hub.onlineUsers[memberID]; online {
					onlineCount++
				}
			}
			ms.hub.onlineUsersMu.RUnlock()

			roomInfo := model.RoomInfo{
				ConversationID: room.ConversationID,
				TotalMembers:   len(room.Members),
				OnlineMembers:  onlineCount,
				MemberIDs:      memberIDs,
			}

			stats.RoomDetails = append(stats.RoomDetails, roomInfo)
			stats.TotalRooms++

			if onlineCount > 0 {
				stats.ActiveRooms++
			}

			room.mu.RUnlock()
		}
		bucket.RUnlock()
	}

	return stats
}

// getClientList returns list of all connected clients
func (ms *MonitorService) getCurrentRoomClientsDetail() []*model.ActiveGroupCall {
	ms.hub.callHandler.activeCallsMu.RLock()
	defer ms.hub.callHandler.activeCallsMu.RUnlock()

	call := make([]*model.ActiveGroupCall, 0, len(ms.hub.callHandler.activeCalls))

	for _, activeCall := range ms.hub.callHandler.activeCalls {
		call = append(call, activeCall)
	}

	return call
}

// getCallStats returns active call statistics
func (ms *MonitorService) getCallStats() model.CallStats {
	stats := model.CallStats{
		CallDetails: make([]model.CallInfo, 0),
	}

	if ms.hub.callHandler == nil {
		return stats
	}

	ms.hub.callHandler.activeCallsMu.RLock()
	defer ms.hub.callHandler.activeCallsMu.RUnlock()

	for _, call := range ms.hub.callHandler.activeCalls {
		call.Mu.RLock()

		// Extract participant IDs from the Participants map
		calleeIDs := make([]string, 0, len(call.Participants))
		for userID := range call.Participants {
			calleeIDs = append(calleeIDs, userID)
		}

		callInfo := model.CallInfo{
			ConversationID: call.ConversationID,
			CallerID:       call.CallerID,
			CalleeIDs:      calleeIDs,
			CallType:       call.CallType,
			Status:         call.Status,
			StartedAt:      call.CreatedAt.Format(time.RFC3339),
		}

		stats.CallDetails = append(stats.CallDetails, callInfo)
		stats.TotalActiveCalls++

		call.Mu.RUnlock()
	}

	return stats
}

// getClientList returns list of all connected clients
func (ms *MonitorService) getClientList() []model.ClientInfo {
	ms.hub.onlineUsersMu.RLock()
	defer ms.hub.onlineUsersMu.RUnlock()

	clients := make([]model.ClientInfo, 0, len(ms.hub.onlineUsers))

	for _, client := range ms.hub.onlineUsers {
		clientInfo := model.ClientInfo{
			ClientID:              client.ID,
			UserID:                client.userId,
			Status:                client.GetStatus(),
			CurrentConversationID: client.GetCurrentConversationID(),
		}
		clients = append(clients, clientInfo)
	}

	return clients
}

// getStatusCount returns count of clients by status
func (ms *MonitorService) getStatusCount() map[string]int {
	ms.hub.onlineUsersMu.RLock()
	defer ms.hub.onlineUsersMu.RUnlock()

	statusCount := map[string]int{
		StatusOnline: 0,
		StatusBusy:   0,
		StatusInCall: 0,
		StatusAway:   0,
	}

	for _, client := range ms.hub.onlineUsers {
		status := client.GetStatus()
		statusCount[status]++
	}

	return statusCount
}
