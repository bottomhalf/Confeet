package model

// -----------------------------------------------------------------
// Monitor API Response Models
// -----------------------------------------------------------------

// MonitorResponse is the main response for the monitor API
type MonitorResponse struct {
	Status      string             `json:"status"`      // "healthy", "degraded", "unhealthy"
	Connections ConnectionStats    `json:"connections"` // Client connection stats
	Rooms       RoomStats          `json:"rooms"`       // Room/conversation stats
	Calls       CallStats          `json:"calls"`       // Active call stats
	Clients     []ClientInfo       `json:"clients"`     // List of connected clients
	StatusCount map[string]int     `json:"statusCount"` // Count by status (online, busy, in_call, away)
}

// ConnectionStats holds connection-related statistics
type ConnectionStats struct {
	TotalConnected int `json:"totalConnected"` // Total clients currently connected
	TotalOnline    int `json:"totalOnline"`    // Clients with status "online"
	TotalBusy      int `json:"totalBusy"`      // Clients with status "busy"
	TotalInCall    int `json:"totalInCall"`    // Clients with status "in_call"
	TotalAway      int `json:"totalAway"`      // Clients with status "away"
}

// RoomStats holds room/conversation statistics
type RoomStats struct {
	TotalRooms   int        `json:"totalRooms"`   // Total rooms in cache
	ActiveRooms  int        `json:"activeRooms"`  // Rooms with at least one online member
	RoomDetails  []RoomInfo `json:"roomDetails"`  // Details of each room
}

// RoomInfo contains information about a single room
type RoomInfo struct {
	ConversationID string   `json:"conversationId"`
	TotalMembers   int      `json:"totalMembers"`   // Total members in room
	OnlineMembers  int      `json:"onlineMembers"`  // Currently online members
	MemberIDs      []string `json:"memberIds"`      // List of member user IDs
}

// CallStats holds active call statistics
type CallStats struct {
	TotalActiveCalls int        `json:"totalActiveCalls"` // Number of ongoing calls
	CallDetails      []CallInfo `json:"callDetails"`      // Details of each active call
}

// CallInfo contains information about a single active call
type CallInfo struct {
	ConversationID string   `json:"conversationId"`
	CallerID       string   `json:"callerId"`
	CalleeIDs      []string `json:"calleeIds"`
	CallType       string   `json:"callType"` // "audio" or "video"
	Status         int      `json:"status"`
	StartedAt      string   `json:"startedAt"` // ISO timestamp
}

// ClientInfo contains information about a connected client
type ClientInfo struct {
	ClientID              string `json:"clientId"`
	UserID                string `json:"userId"`
	Status                string `json:"status"`                          // "online", "busy", "in_call", "away"
	CurrentConversationID string `json:"currentConversationId,omitempty"` // If in a call
}
