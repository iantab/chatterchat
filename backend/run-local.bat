@echo off
set USERS_TABLE=chatterchat-users
set ROOMS_TABLE=chatterchat-rooms
set MESSAGES_TABLE=chatterchat-messages
set CONNECTIONS_TABLE=chatterchat-connections
set LOCAL_DEV_USER=dev-001:alice:alice@local.dev
go run ./cmd/local/
