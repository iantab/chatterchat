@echo off
set DATABASE_URL=host=localhost port=5432 user=chatterchat password=chatterchat dbname=chatterchat sslmode=disable
set LOCAL_DEV_USER=dev-001:alice:alice@local.dev
go run ./cmd/local/
