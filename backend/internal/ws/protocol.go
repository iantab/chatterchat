package ws

import "time"

// InboundMessage is the base shape of messages sent by the client.
type InboundMessage struct {
	Action string `json:"action"`
	RoomID string `json:"room_id,omitempty"`
	Body   string `json:"body,omitempty"`
}

// ChatMessage is broadcast to all members of a room when someone sends a message.
type ChatMessage struct {
	Type      string    `json:"type"`      // "message"
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// JoinedAck is sent to the client that just joined a room.
type JoinedAck struct {
	Type     string `json:"type"`      // "joined"
	RoomID   string `json:"room_id"`
	RoomName string `json:"room_name"`
}

// UserEvent is broadcast to all room members when someone joins or leaves.
type UserEvent struct {
	Type     string `json:"type"`      // "user_joined" | "user_left"
	Username string `json:"username"`
	RoomID   string `json:"room_id"`
}

// ErrorMessage is sent to the client when their request cannot be fulfilled.
type ErrorMessage struct {
	Type    string `json:"type"`    // "error"
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Pong is sent in response to a ping.
type Pong struct {
	Type string `json:"type"` // "pong"
}
