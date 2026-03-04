package models

import (
	"time"
)

type User struct {
	ID          string    `dynamodbav:"id"                     json:"id"`
	CognitoSub  string    `dynamodbav:"cognito_sub"            json:"cognito_sub"`
	Username    string    `dynamodbav:"username"               json:"username"`
	Email       string    `dynamodbav:"email"                  json:"email"`
	DisplayName *string   `dynamodbav:"display_name,omitempty" json:"display_name"`
	CreatedAt   time.Time `dynamodbav:"created_at"             json:"created_at"`
}

type Room struct {
	ID          string    `dynamodbav:"id"                     json:"id"`
	Name        string    `dynamodbav:"name"                   json:"name"`
	Description *string   `dynamodbav:"description,omitempty"  json:"description"`
	CreatedAt   time.Time `dynamodbav:"created_at"             json:"created_at"`
}

type Message struct {
	ID        string    `dynamodbav:"id"         json:"id"`
	RoomID    string    `dynamodbav:"room_id"    json:"room_id"`
	TsID      string    `dynamodbav:"ts_id"      json:"-"`
	UserID    string    `dynamodbav:"user_id"    json:"user_id"`
	Username  string    `dynamodbav:"username"   json:"username"`
	Body      string    `dynamodbav:"body"       json:"body"`
	CreatedAt time.Time `dynamodbav:"created_at" json:"created_at"`
}

type Connection struct {
	ConnectionID string    `dynamodbav:"connection_id"          json:"connection_id"`
	UserID       string    `dynamodbav:"user_id"                json:"user_id"`
	Username     string    `dynamodbav:"username"               json:"username"`
	RoomID       *string   `dynamodbav:"room_id,omitempty"      json:"room_id"`
	ConnectedAt  time.Time `dynamodbav:"connected_at"           json:"connected_at"`
}
