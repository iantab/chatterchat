package db

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"chatterchat/internal/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

var (
	usersTable       = os.Getenv("USERS_TABLE")
	roomsTable       = os.Getenv("ROOMS_TABLE")
	messagesTable    = os.Getenv("MESSAGES_TABLE")
	connectionsTable = os.Getenv("CONNECTIONS_TABLE")
)

// UpsertUser inserts or updates a user by cognito_sub. Returns the persisted user.
// On insert: generates a new UUID id and sets created_at.
// On update: only updates username and email; preserves id, created_at, display_name.
func UpsertUser(ctx context.Context, client *dynamodb.Client, cognitoSub, username, email string) (*models.User, error) {
	out, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(usersTable),
		Key: map[string]types.AttributeValue{
			"cognito_sub": &types.AttributeValueMemberS{Value: cognitoSub},
		},
		UpdateExpression: aws.String(
			"SET username = :u, email = :e, " +
				"id = if_not_exists(id, :id), " +
				"created_at = if_not_exists(created_at, :cat)",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":u":   &types.AttributeValueMemberS{Value: username},
			":e":   &types.AttributeValueMemberS{Value: email},
			":id":  &types.AttributeValueMemberS{Value: uuid.NewString()},
			":cat": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	var u models.User
	if err := attributevalue.UnmarshalMap(out.Attributes, &u); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &u, nil
}

// GetUserBySub returns a user by their Cognito sub.
func GetUserBySub(ctx context.Context, client *dynamodb.Client, cognitoSub string) (*models.User, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(usersTable),
		Key: map[string]types.AttributeValue{
			"cognito_sub": &types.AttributeValueMemberS{Value: cognitoSub},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get user by sub: %w", err)
	}
	if out.Item == nil {
		return nil, fmt.Errorf("get user by sub: not found")
	}
	var u models.User
	if err := attributevalue.UnmarshalMap(out.Item, &u); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &u, nil
}

// UpdateDisplayName sets the display_name for a user by cognito_sub.
func UpdateDisplayName(ctx context.Context, client *dynamodb.Client, cognitoSub, displayName string) (*models.User, error) {
	out, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(usersTable),
		Key: map[string]types.AttributeValue{
			"cognito_sub": &types.AttributeValueMemberS{Value: cognitoSub},
		},
		UpdateExpression: aws.String("SET display_name = :dn"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":dn": &types.AttributeValueMemberS{Value: displayName},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return nil, fmt.Errorf("update display name: %w", err)
	}
	var u models.User
	if err := attributevalue.UnmarshalMap(out.Attributes, &u); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &u, nil
}

// GetRooms returns all rooms sorted by name.
func GetRooms(ctx context.Context, client *dynamodb.Client) ([]models.Room, error) {
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(roomsTable),
	})
	if err != nil {
		return nil, fmt.Errorf("get rooms: %w", err)
	}
	rooms := make([]models.Room, 0, len(out.Items))
	for _, item := range out.Items {
		var r models.Room
		if err := attributevalue.UnmarshalMap(item, &r); err != nil {
			return nil, fmt.Errorf("unmarshal room: %w", err)
		}
		rooms = append(rooms, r)
	}
	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i].Name < rooms[j].Name
	})
	return rooms, nil
}

// GetRoomByID returns a single room by ID.
func GetRoomByID(ctx context.Context, client *dynamodb.Client, roomID string) (*models.Room, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(roomsTable),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: roomID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get room by id: %w", err)
	}
	if out.Item == nil {
		return nil, fmt.Errorf("get room by id: not found")
	}
	var r models.Room
	if err := attributevalue.UnmarshalMap(out.Item, &r); err != nil {
		return nil, fmt.Errorf("unmarshal room: %w", err)
	}
	return &r, nil
}

// CreateRoom inserts a new room. Returns an error if the name already exists.
func CreateRoom(ctx context.Context, client *dynamodb.Client, name, description string) (*models.Room, error) {
	// Check name uniqueness via GSI.
	qOut, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(roomsTable),
		IndexName:              aws.String("name-index"),
		KeyConditionExpression: aws.String("#n = :name"),
		ExpressionAttributeNames: map[string]string{
			"#n": "name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: name},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("check room name: %w", err)
	}
	if qOut.Count > 0 {
		return nil, fmt.Errorf("room with name %q already exists", name)
	}

	r := models.Room{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	if description != "" {
		r.Description = &description
	}

	item, err := attributevalue.MarshalMap(r)
	if err != nil {
		return nil, fmt.Errorf("marshal room: %w", err)
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(roomsTable),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return &r, nil
}

// GetMessagesByRoom returns up to limit messages for a room, ordered newest-first.
func GetMessagesByRoom(ctx context.Context, client *dynamodb.Client, roomID string, limit int) ([]models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(messagesTable),
		KeyConditionExpression: aws.String("room_id = :rid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":rid": &types.AttributeValueMemberS{Value: roomID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	msgs := make([]models.Message, 0, len(out.Items))
	for _, item := range out.Items {
		var m models.Message
		if err := attributevalue.UnmarshalMap(item, &m); err != nil {
			return nil, fmt.Errorf("unmarshal message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// InsertMessage persists a chat message and returns it with generated fields.
func InsertMessage(ctx context.Context, client *dynamodb.Client, roomID, userID, username, body string) (*models.Message, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	// ts_id is lexicographically sortable: RFC3339Nano timestamp + UUID for uniqueness.
	tsID := now.Format(time.RFC3339Nano) + "#" + id

	m := models.Message{
		ID:        id,
		RoomID:    roomID,
		TsID:      tsID,
		UserID:    userID,
		Username:  username,
		Body:      body,
		CreatedAt: now,
	}
	item, err := attributevalue.MarshalMap(m)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(messagesTable),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}
	return &m, nil
}

// InsertConnection records a new WebSocket connection.
func InsertConnection(ctx context.Context, client *dynamodb.Client, connID, userID, username string) error {
	c := models.Connection{
		ConnectionID: connID,
		UserID:       userID,
		Username:     username,
		ConnectedAt:  time.Now().UTC(),
	}
	item, err := attributevalue.MarshalMap(c)
	if err != nil {
		return fmt.Errorf("marshal connection: %w", err)
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(connectionsTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("insert connection: %w", err)
	}
	return nil
}

// DeleteConnection removes a connection record.
func DeleteConnection(ctx context.Context, client *dynamodb.Client, connID string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(connectionsTable),
		Key: map[string]types.AttributeValue{
			"connection_id": &types.AttributeValueMemberS{Value: connID},
		},
	})
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}

// SetConnectionRoom updates the room a connection is joined to.
func SetConnectionRoom(ctx context.Context, client *dynamodb.Client, connID, roomID string) error {
	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(connectionsTable),
		Key: map[string]types.AttributeValue{
			"connection_id": &types.AttributeValueMemberS{Value: connID},
		},
		UpdateExpression: aws.String("SET room_id = :rid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":rid": &types.AttributeValueMemberS{Value: roomID},
		},
	})
	if err != nil {
		return fmt.Errorf("set connection room: %w", err)
	}
	return nil
}

// GetConnectionsByRoom returns all connections currently in a room.
func GetConnectionsByRoom(ctx context.Context, client *dynamodb.Client, roomID string) ([]models.Connection, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(connectionsTable),
		IndexName:              aws.String("room-index"),
		KeyConditionExpression: aws.String("room_id = :rid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":rid": &types.AttributeValueMemberS{Value: roomID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get connections by room: %w", err)
	}
	conns := make([]models.Connection, 0, len(out.Items))
	for _, item := range out.Items {
		var c models.Connection
		if err := attributevalue.UnmarshalMap(item, &c); err != nil {
			return nil, fmt.Errorf("unmarshal connection: %w", err)
		}
		conns = append(conns, c)
	}
	return conns, nil
}

// GetConnection returns a single connection by ID.
func GetConnection(ctx context.Context, client *dynamodb.Client, connID string) (*models.Connection, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(connectionsTable),
		Key: map[string]types.AttributeValue{
			"connection_id": &types.AttributeValueMemberS{Value: connID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	if out.Item == nil {
		return nil, fmt.Errorf("get connection: not found")
	}
	var c models.Connection
	if err := attributevalue.UnmarshalMap(out.Item, &c); err != nil {
		return nil, fmt.Errorf("unmarshal connection: %w", err)
	}
	return &c, nil
}
