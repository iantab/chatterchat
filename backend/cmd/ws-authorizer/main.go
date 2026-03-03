package main

import (
	"context"
	"fmt"
	"log"

	"chatterchat/internal/auth"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// handler validates the Cognito ID token passed as a query-string parameter
// and returns an IAM policy allowing or denying the $connect route.
func handler(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	token := event.QueryStringParameters["token"]
	if token == "" {
		log.Println("ws-authorizer: missing token query parameter")
		return deny(), nil
	}

	claims, err := auth.ValidateToken(ctx, token)
	if err != nil {
		log.Printf("ws-authorizer: invalid token: %v", err)
		return deny(), nil
	}

	arn := fmt.Sprintf("arn:aws:execute-api:*:*:%s/%s/$connect",
		event.RequestContext.APIID, event.RequestContext.Stage)

	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: claims.Sub,
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   "Allow",
					Resource: []string{arn},
				},
			},
		},
		Context: map[string]interface{}{
			"sub":      claims.Sub,
			"username": claims.Username,
			"email":    claims.Email,
		},
	}, nil
}

func deny() events.APIGatewayCustomAuthorizerResponse {
	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: "denied",
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   "Deny",
					Resource: []string{"arn:aws:execute-api:*:*:*"},
				},
			},
		},
	}
}

func main() {
	lambda.Start(handler)
}
