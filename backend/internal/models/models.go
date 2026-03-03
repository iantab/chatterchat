package models

import (
	"time"
)

type User struct {
	ID         string    `db:"id"          json:"id"`
	CognitoSub string    `db:"cognito_sub" json:"cognito_sub"`
	Username   string    `db:"username"    json:"username"`
	Email      string    `db:"email"       json:"email"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}

type Room struct {
	ID          string    `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	Description *string   `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
}

type Message struct {
	ID        string    `db:"id"         json:"id"`
	RoomID    string    `db:"room_id"    json:"room_id"`
	UserID    string    `db:"user_id"    json:"user_id"`
	Username  string    `db:"username"   json:"username"`
	Body      string    `db:"body"       json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Connection struct {
	ConnectionID string    `db:"connection_id" json:"connection_id"`
	UserID       string    `db:"user_id"       json:"user_id"`
	Username     string    `db:"username"      json:"username"`
	RoomID       *string   `db:"room_id"       json:"room_id"`
	ConnectedAt  time.Time `db:"connected_at"  json:"connected_at"`
}
