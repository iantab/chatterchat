package models

import (
	"time"
)

type User struct {
	ID         string    `db:"id"`
	CognitoSub string    `db:"cognito_sub"`
	Username   string    `db:"username"`
	Email      string    `db:"email"`
	CreatedAt  time.Time `db:"created_at"`
}

type Room struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}

type Message struct {
	ID        string    `db:"id"`
	RoomID    string    `db:"room_id"`
	UserID    string    `db:"user_id"`
	Username  string    `db:"username"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"created_at"`
}

type Connection struct {
	ConnectionID string    `db:"connection_id"`
	UserID       string    `db:"user_id"`
	Username     string    `db:"username"`
	RoomID       *string   `db:"room_id"`
	ConnectedAt  time.Time `db:"connected_at"`
}
