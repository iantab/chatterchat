package db

import (
	"context"
	"fmt"

	"chatterchat/internal/models"

	"github.com/jmoiron/sqlx"
)

// UpsertUser inserts or updates a user by cognito_sub. Returns the persisted user.
func UpsertUser(ctx context.Context, db *sqlx.DB, cognitoSub, username, email string) (*models.User, error) {
	const q = `
		INSERT INTO users (cognito_sub, username, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (cognito_sub) DO UPDATE
			SET username = EXCLUDED.username,
			    email    = EXCLUDED.email
		RETURNING id, cognito_sub, username, email, display_name, created_at`

	var u models.User
	if err := db.QueryRowxContext(ctx, q, cognitoSub, username, email).StructScan(&u); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &u, nil
}

// GetUserBySub returns a user by their Cognito sub.
func GetUserBySub(ctx context.Context, db *sqlx.DB, cognitoSub string) (*models.User, error) {
	const q = `SELECT id, cognito_sub, username, email, display_name, created_at FROM users WHERE cognito_sub = $1`
	var u models.User
	if err := db.QueryRowxContext(ctx, q, cognitoSub).StructScan(&u); err != nil {
		return nil, fmt.Errorf("get user by sub: %w", err)
	}
	return &u, nil
}

// UpdateDisplayName sets the display_name for a user by cognito_sub.
func UpdateDisplayName(ctx context.Context, db *sqlx.DB, cognitoSub, displayName string) (*models.User, error) {
	const q = `
		UPDATE users SET display_name = $2 WHERE cognito_sub = $1
		RETURNING id, cognito_sub, username, email, display_name, created_at`
	var u models.User
	if err := db.QueryRowxContext(ctx, q, cognitoSub, displayName).StructScan(&u); err != nil {
		return nil, fmt.Errorf("update display name: %w", err)
	}
	return &u, nil
}

// GetRooms returns all rooms ordered by name.
func GetRooms(ctx context.Context, db *sqlx.DB) ([]models.Room, error) {
	const q = `SELECT id, name, description, created_at FROM rooms ORDER BY name`
	var rooms []models.Room
	if err := db.SelectContext(ctx, &rooms, q); err != nil {
		return nil, fmt.Errorf("get rooms: %w", err)
	}
	return rooms, nil
}

// GetRoomByID returns a single room by ID.
func GetRoomByID(ctx context.Context, db *sqlx.DB, roomID string) (*models.Room, error) {
	const q = `SELECT id, name, description, created_at FROM rooms WHERE id = $1`
	var r models.Room
	if err := db.QueryRowxContext(ctx, q, roomID).StructScan(&r); err != nil {
		return nil, fmt.Errorf("get room by id: %w", err)
	}
	return &r, nil
}

// CreateRoom inserts a new room and returns it.
func CreateRoom(ctx context.Context, db *sqlx.DB, name, description string) (*models.Room, error) {
	const q = `
		INSERT INTO rooms (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, created_at`
	var r models.Room
	if err := db.QueryRowxContext(ctx, q, name, description).StructScan(&r); err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return &r, nil
}

// GetMessagesByRoom returns up to limit messages for a room, ordered newest-first.
func GetMessagesByRoom(ctx context.Context, db *sqlx.DB, roomID string, limit int) ([]models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	const q = `
		SELECT id, room_id, user_id, username, body, created_at
		FROM messages
		WHERE room_id = $1
		ORDER BY created_at DESC
		LIMIT $2`
	var msgs []models.Message
	if err := db.SelectContext(ctx, &msgs, q, roomID, limit); err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	return msgs, nil
}

// InsertMessage persists a chat message and returns it with generated fields.
func InsertMessage(ctx context.Context, db *sqlx.DB, roomID, userID, username, body string) (*models.Message, error) {
	const q = `
		INSERT INTO messages (room_id, user_id, username, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, room_id, user_id, username, body, created_at`
	var m models.Message
	if err := db.QueryRowxContext(ctx, q, roomID, userID, username, body).StructScan(&m); err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}
	return &m, nil
}

// InsertConnection records a new WebSocket connection.
func InsertConnection(ctx context.Context, db *sqlx.DB, connID, userID, username string) error {
	const q = `
		INSERT INTO connections (connection_id, user_id, username)
		VALUES ($1, $2, $3)
		ON CONFLICT (connection_id) DO NOTHING`
	if _, err := db.ExecContext(ctx, q, connID, userID, username); err != nil {
		return fmt.Errorf("insert connection: %w", err)
	}
	return nil
}

// DeleteConnection removes a connection record.
func DeleteConnection(ctx context.Context, db *sqlx.DB, connID string) error {
	const q = `DELETE FROM connections WHERE connection_id = $1`
	if _, err := db.ExecContext(ctx, q, connID); err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}

// SetConnectionRoom updates the room a connection is joined to.
func SetConnectionRoom(ctx context.Context, db *sqlx.DB, connID, roomID string) error {
	const q = `UPDATE connections SET room_id = $1 WHERE connection_id = $2`
	if _, err := db.ExecContext(ctx, q, roomID, connID); err != nil {
		return fmt.Errorf("set connection room: %w", err)
	}
	return nil
}

// GetConnectionsByRoom returns all connections currently in a room.
func GetConnectionsByRoom(ctx context.Context, db *sqlx.DB, roomID string) ([]models.Connection, error) {
	const q = `
		SELECT connection_id, user_id, username, room_id, connected_at
		FROM connections
		WHERE room_id = $1`
	var conns []models.Connection
	if err := db.SelectContext(ctx, &conns, q, roomID); err != nil {
		return nil, fmt.Errorf("get connections by room: %w", err)
	}
	return conns, nil
}

// GetConnection returns a single connection by ID.
func GetConnection(ctx context.Context, db *sqlx.DB, connID string) (*models.Connection, error) {
	const q = `
		SELECT connection_id, user_id, username, room_id, connected_at
		FROM connections WHERE connection_id = $1`
	var c models.Connection
	if err := db.QueryRowxContext(ctx, q, connID).StructScan(&c); err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	return &c, nil
}
