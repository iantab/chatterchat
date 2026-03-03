package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type dbSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbname"`
}

var (
	secretOnce  sync.Once
	cachedDSN   string
	secretError error
)

// GetDSN returns a cached PostgreSQL DSN fetched from Secrets Manager.
func GetDSN(ctx context.Context) (string, error) {
	secretOnce.Do(func() {
		arn := os.Getenv("DB_SECRET_ARN")
		if arn == "" {
			secretError = fmt.Errorf("DB_SECRET_ARN env var not set")
			return
		}

		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			secretError = fmt.Errorf("load aws config: %w", err)
			return
		}

		svc := secretsmanager.NewFromConfig(cfg)
		out, err := svc.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(arn),
		})
		if err != nil {
			secretError = fmt.Errorf("get secret value: %w", err)
			return
		}

		var s dbSecret
		if err := json.Unmarshal([]byte(*out.SecretString), &s); err != nil {
			secretError = fmt.Errorf("unmarshal secret: %w", err)
			return
		}

		cachedDSN = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=require",
			s.Host, s.Port, s.Username, s.Password, s.DBName,
		)
	})
	return cachedDSN, secretError
}
