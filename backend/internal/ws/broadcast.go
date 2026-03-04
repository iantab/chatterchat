package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"chatterchat/internal/db"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// LocalSender, if non-nil, bypasses PostToConnection for local dev.
// Set once at startup by cmd/local/main.go before any connections arrive.
var LocalSender func(ctx context.Context, connID string, data []byte) error

// newAPIGWClient creates an API Gateway Management API client for the given endpoint.
func newAPIGWClient(ctx context.Context, endpoint string) (*apigatewaymanagementapi.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	return client, nil
}

// BroadcastToRoom sends payload to all connections in roomID.
// Stale connections (410 Gone) are removed from the DB.
func BroadcastToRoom(ctx context.Context, client *dynamodb.Client, endpoint, roomID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	conns, err := db.GetConnectionsByRoom(ctx, client, roomID)
	if err != nil {
		return fmt.Errorf("get room connections: %w", err)
	}

	if LocalSender != nil {
		for _, conn := range conns {
			if err := LocalSender(ctx, conn.ConnectionID, data); err != nil {
				log.Printf("local send to %s failed: %v", conn.ConnectionID, err)
			}
		}
		return nil
	}

	apiClient, err := newAPIGWClient(ctx, endpoint)
	if err != nil {
		return err
	}

	for _, conn := range conns {
		_, err := apiClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(conn.ConnectionID),
			Data:         data,
		})
		if err != nil {
			// Check for 410 Gone — connection no longer exists.
			if isGone(err) {
				log.Printf("stale connection %s, removing from DB", conn.ConnectionID)
				if delErr := db.DeleteConnection(ctx, client, conn.ConnectionID); delErr != nil {
					log.Printf("failed to delete stale connection %s: %v", conn.ConnectionID, delErr)
				}
			} else {
				log.Printf("post to connection %s failed: %v", conn.ConnectionID, err)
			}
		}
	}
	return nil
}

// SendToConnection sends payload to a single connection.
func SendToConnection(ctx context.Context, endpoint, connID string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	if LocalSender != nil {
		return LocalSender(ctx, connID, data)
	}

	client, err := newAPIGWClient(ctx, endpoint)
	if err != nil {
		return err
	}

	_, err = client.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connID),
		Data:         data,
	})
	return err
}

// isGone checks whether an API Gateway Management API error is a 410 Gone response.
func isGone(err error) bool {
	if err == nil {
		return false
	}
	// The SDK wraps HTTP errors; check the status code via error string as a fallback.
	type httpErr interface {
		HTTPStatusCode() int
	}
	if he, ok := err.(httpErr); ok {
		return he.HTTPStatusCode() == 410
	}
	return false
}
