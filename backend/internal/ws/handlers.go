package ws

import (
	"context"
	"fmt"
	"log"

	"chatterchat/internal/db"

	"github.com/jmoiron/sqlx"
)

// mgmtEndpoint builds the API Gateway Management API base URL from domain+stage.
func mgmtEndpoint(domain, stage string) string {
	return fmt.Sprintf("https://%s/%s", domain, stage)
}

// HandleConnect upserts the user and records the connection.
func HandleConnect(ctx context.Context, sqlDB *sqlx.DB, connID, cognitoSub, username, email string) error {
	user, err := db.UpsertUser(ctx, sqlDB, cognitoSub, username, email)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	if err := db.InsertConnection(ctx, sqlDB, connID, user.ID, user.Username); err != nil {
		return fmt.Errorf("insert connection: %w", err)
	}
	log.Printf("connected: connID=%s user=%s", connID, user.Username)
	return nil
}

// HandleDisconnect removes the connection and notifies the room if the user was in one.
func HandleDisconnect(ctx context.Context, sqlDB *sqlx.DB, domain, stage, connID string) error {
	conn, err := db.GetConnection(ctx, sqlDB, connID)
	if err != nil {
		// Connection may not exist (e.g. auth failed before insert). Not fatal.
		log.Printf("disconnect: connection %s not found: %v", connID, err)
		return nil
	}

	if conn.RoomID != nil {
		event := UserEvent{
			Type:     "user_left",
			Username: conn.Username,
			RoomID:   *conn.RoomID,
		}
		endpoint := mgmtEndpoint(domain, stage)
		if err := BroadcastToRoom(ctx, sqlDB, endpoint, *conn.RoomID, event); err != nil {
			log.Printf("broadcast user_left failed: %v", err)
		}
	}

	if err := db.DeleteConnection(ctx, sqlDB, connID); err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	log.Printf("disconnected: connID=%s user=%s", connID, conn.Username)
	return nil
}

// HandleJoinRoom moves the connection into a room and broadcasts user_joined.
func HandleJoinRoom(ctx context.Context, sqlDB *sqlx.DB, domain, stage, connID, roomID string) error {
	endpoint := mgmtEndpoint(domain, stage)

	room, err := db.GetRoomByID(ctx, sqlDB, roomID)
	if err != nil {
		return sendError(ctx, endpoint, connID, "INVALID_ROOM", "Room not found")
	}

	conn, err := db.GetConnection(ctx, sqlDB, connID)
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}

	// Leave old room if necessary.
	if conn.RoomID != nil && *conn.RoomID != roomID {
		leaveEvent := UserEvent{Type: "user_left", Username: conn.Username, RoomID: *conn.RoomID}
		if err := BroadcastToRoom(ctx, sqlDB, endpoint, *conn.RoomID, leaveEvent); err != nil {
			log.Printf("broadcast user_left (old room) failed: %v", err)
		}
	}

	if err := db.SetConnectionRoom(ctx, sqlDB, connID, roomID); err != nil {
		return fmt.Errorf("set connection room: %w", err)
	}

	// Acknowledge join to the requester.
	ack := JoinedAck{Type: "joined", RoomID: room.ID, RoomName: room.Name}
	if err := SendToConnection(ctx, endpoint, connID, ack); err != nil {
		log.Printf("send joined ack failed: %v", err)
	}

	// Notify room members.
	joinEvent := UserEvent{Type: "user_joined", Username: conn.Username, RoomID: roomID}
	if err := BroadcastToRoom(ctx, sqlDB, endpoint, roomID, joinEvent); err != nil {
		log.Printf("broadcast user_joined failed: %v", err)
	}

	return nil
}

// HandleSendMessage persists a message and broadcasts it to the room.
func HandleSendMessage(ctx context.Context, sqlDB *sqlx.DB, domain, stage, connID, roomID, body string) error {
	endpoint := mgmtEndpoint(domain, stage)

	if body == "" {
		return sendError(ctx, endpoint, connID, "EMPTY_MESSAGE", "Message body cannot be empty")
	}

	conn, err := db.GetConnection(ctx, sqlDB, connID)
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}

	if conn.RoomID == nil || *conn.RoomID != roomID {
		return sendError(ctx, endpoint, connID, "NOT_IN_ROOM", "You must join the room before sending messages")
	}

	if _, err := db.GetRoomByID(ctx, sqlDB, roomID); err != nil {
		return sendError(ctx, endpoint, connID, "INVALID_ROOM", "Room not found")
	}

	msg, err := db.InsertMessage(ctx, sqlDB, roomID, conn.UserID, conn.Username, body)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	chatMsg := ChatMessage{
		Type:      "message",
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
		Username:  msg.Username,
		Body:      msg.Body,
		CreatedAt: msg.CreatedAt,
	}
	if err := BroadcastToRoom(ctx, sqlDB, endpoint, roomID, chatMsg); err != nil {
		log.Printf("broadcast message failed: %v", err)
	}

	return nil
}

// sendError sends an ErrorMessage to a single connection and returns nil
// (the error is communicated to the client, not treated as a Lambda error).
func sendError(ctx context.Context, endpoint, connID, code, message string) error {
	msg := ErrorMessage{Type: "error", Code: code, Message: message}
	if err := SendToConnection(ctx, endpoint, connID, msg); err != nil {
		log.Printf("send error to %s failed: %v", connID, err)
	}
	return nil
}
