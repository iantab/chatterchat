package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var (
	ddbOnce   sync.Once
	ddbClient *dynamodb.Client
	ddbError  error
)

// Get returns the singleton DynamoDB client, initializing it on first call.
// The client uses the Lambda IAM role automatically (no credentials needed).
func Get(ctx context.Context) (*dynamodb.Client, error) {
	ddbOnce.Do(func() {
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			ddbError = fmt.Errorf("load aws config: %w", err)
			return
		}
		ddbClient = dynamodb.NewFromConfig(cfg)
	})
	return ddbClient, ddbError
}
