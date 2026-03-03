package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"chatterchat/internal/db"
	"chatterchat/internal/ws"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	rc := event.RequestContext
	connID := rc.ConnectionID
	domain := rc.DomainName
	stage := rc.Stage

	sqlDB, err := db.Get(ctx)
	if err != nil {
		log.Printf("db init error: %v", err)
		return serverError(), nil
	}

	switch rc.RouteKey {
	case "$connect":
		sub := stringFromCtx(rc.Authorizer, "sub")
		username := stringFromCtx(rc.Authorizer, "username")
		email := stringFromCtx(rc.Authorizer, "email")
		if err := ws.HandleConnect(ctx, sqlDB, connID, sub, username, email); err != nil {
			log.Printf("HandleConnect error: %v", err)
			return serverError(), nil
		}

	case "$disconnect":
		if err := ws.HandleDisconnect(ctx, sqlDB, domain, stage, connID); err != nil {
			log.Printf("HandleDisconnect error: %v", err)
		}

	case "ping":
		// Pong handled inline — no DB needed.
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: `{"type":"pong"}`}, nil

	default:
		// joinRoom / sendMessage — decode inbound message.
		var msg ws.InboundMessage
		if err := json.Unmarshal([]byte(event.Body), &msg); err != nil {
			log.Printf("unmarshal body error: %v", err)
			return events.APIGatewayProxyResponse{StatusCode: 400}, nil
		}

		switch msg.Action {
		case "joinRoom":
			if msg.RoomID == "" {
				return events.APIGatewayProxyResponse{StatusCode: 400, Body: `{"error":"room_id required"}`}, nil
			}
			if err := ws.HandleJoinRoom(ctx, sqlDB, domain, stage, connID, msg.RoomID); err != nil {
				log.Printf("HandleJoinRoom error: %v", err)
				return serverError(), nil
			}

		case "sendMessage":
			if msg.RoomID == "" || msg.Body == "" {
				return events.APIGatewayProxyResponse{StatusCode: 400, Body: `{"error":"room_id and body required"}`}, nil
			}
			if err := ws.HandleSendMessage(ctx, sqlDB, domain, stage, connID, msg.RoomID, msg.Body); err != nil {
				log.Printf("HandleSendMessage error: %v", err)
				return serverError(), nil
			}

		default:
			log.Printf("unknown action: %s", msg.Action)
			return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf(`{"error":"unknown action %q"}`, msg.Action)}, nil
		}
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func serverError() events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{StatusCode: 500, Body: `{"error":"internal server error"}`}
}

// stringFromCtx extracts a string value from the Lambda authorizer context map.
func stringFromCtx(authCtx interface{}, key string) string {
	m, ok := authCtx.(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func main() {
	lambda.Start(handler)
}
