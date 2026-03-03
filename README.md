# ChatterChat

Real-time chat application built with a Go backend on AWS Lambda and a Vanilla JS frontend.

## Architecture

```
Browser (Vanilla JS)
   │
   ├── PKCE → Cognito Hosted UI (auth)
   ├── HTTPS → API Gateway HTTP API → Lambda (http-api) → RDS Proxy → RDS
   └── WSS  → API Gateway WebSocket API → Lambda Authorizer → Lambda (ws-handler) → RDS Proxy → RDS
                                                                        │
                                                           API GW Management API (broadcast)
```

## Project Structure

```
chatterchat/
├── backend/
│   ├── cmd/
│   │   ├── ws-handler/main.go      # WebSocket Lambda
│   │   ├── http-api/main.go        # REST Lambda
│   │   └── ws-authorizer/main.go   # Lambda Authorizer ($connect)
│   ├── internal/
│   │   ├── auth/                   # JWT validation + HTTP middleware
│   │   ├── db/                     # DB pool, secrets, queries
│   │   ├── models/                 # Shared data structs
│   │   └── ws/                     # WS protocol, broadcast, handlers
│   ├── migrations/                 # SQL migration files (run in order)
│   ├── go.mod
│   └── Makefile
└── frontend/
    ├── index.html      # Login page
    ├── chat.html       # Chat UI
    ├── style.css
    └── app.js          # PKCE auth + WebSocket + REST
```

## Setup

### 1. AWS Prerequisites (manual)

Create the following resources in order:

| Resource | Notes |
|---|---|
| Cognito User Pool | Enable email sign-up |
| Cognito App Client | Public, Auth Code + PKCE, redirect = `https://<cf-domain>/chat.html` |
| Cognito Hosted Domain | `chatterchat.auth.<region>.amazoncognito.com` |
| RDS PostgreSQL | `db.t4g.micro`, private subnet, port 5432 |
| Secrets Manager secret | RDS credentials — note the ARN |
| RDS Proxy | Target = RDS, secret = above |
| Lambda IAM Role | See permissions below |
| Lambda: `ws-handler` | `provided.al2023`, arm64, 256 MB, 30 s, VPC |
| Lambda: `http-api` | Same |
| Lambda: `ws-authorizer` | Same, can be smaller |
| API GW WebSocket API | Route key: `$request.body.action` |
| API GW HTTP API | JWT Authorizer pointing at Cognito |
| S3 Bucket | Block all public access |
| CloudFront | Origin = S3 via OAC, default root = `index.html` |

### 2. Lambda Environment Variables

Set on **all three** Lambda functions:

```
DB_SECRET_ARN=arn:aws:secretsmanager:<region>:<account>:secret:...
COGNITO_REGION=us-east-1
COGNITO_USER_POOL_ID=us-east-1_XXXXXXXXX
COGNITO_CLIENT_ID=<app-client-id>
```

### 3. Lambda IAM Permissions

**All Lambdas:**
- `secretsmanager:GetSecretValue` on the DB secret ARN
- `logs:CreateLogGroup`, `logs:CreateLogStream`, `logs:PutLogEvents`
- `ec2:CreateNetworkInterface`, `ec2:DescribeNetworkInterfaces`, `ec2:DeleteNetworkInterface`

**ws-handler only:**
- `execute-api:ManageConnections` on `arn:aws:execute-api:<region>:<account>:<ws-api-id>/*/@connections/*`

### 4. Run Database Migrations

Connect to RDS through a bastion host or SSM port forwarding, then run each file in order:

```bash
psql "host=<rds-endpoint> user=<user> dbname=<db> sslmode=require" \
  -f migrations/001_create_users.sql \
  -f migrations/002_create_rooms.sql \
  -f migrations/003_create_messages.sql \
  -f migrations/004_create_connections.sql \
  -f migrations/005_seed_default_rooms.sql
```

### 5. Build & Deploy Backend

```bash
cd backend
go mod tidy
make build

# Upload each zip to its Lambda function:
# dist/ws-handler/ws-handler.zip   → ws-handler Lambda
# dist/http-api/http-api.zip       → http-api Lambda
# dist/ws-authorizer/ws-authorizer.zip → ws-authorizer Lambda
```

### 6. Configure Frontend

Edit `frontend/app.js` and fill in the `CONFIG` object:

```javascript
const CONFIG = {
  apiBase:       'https://<http-api-id>.execute-api.<region>.amazonaws.com',
  wsBase:        'wss://<ws-api-id>.execute-api.<region>.amazonaws.com/prod',
  cognitoDomain: 'chatterchat.auth.<region>.amazoncognito.com',
  clientId:      '<app-client-id>',
  redirectUri:   'https://<cf-domain>/chat.html',
};
```

### 7. Deploy Frontend

```bash
aws s3 sync frontend/ s3://<bucket-name>/ --delete
aws cloudfront create-invalidation --distribution-id <dist-id> --paths "/*"
```

## API GW WebSocket Routes

| Route | Description |
|---|---|
| `$connect` | Authorizer validates JWT query param `?token=<idToken>` |
| `$disconnect` | Remove connection, notify room |
| `joinRoom` | `{ "action": "joinRoom", "room_id": "uuid" }` |
| `sendMessage` | `{ "action": "sendMessage", "room_id": "uuid", "body": "..." }` |
| `ping` | `{ "action": "ping" }` → `{ "type": "pong" }` |

## REST Endpoints (HTTP API)

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Health check (public) |
| GET | `/rooms` | List all rooms |
| POST | `/rooms` | Create a room `{ name, description }` |
| GET | `/rooms/{id}` | Get room details |
| GET | `/rooms/{id}/messages?limit=N` | Get message history (max 100, newest→oldest reversed to chronological) |

## Verification

```bash
# REST smoke test
curl -H "Authorization: Bearer <id-token>" https://<api>/rooms

# WebSocket test (requires wscat: npm i -g wscat)
wscat -c "wss://<ws-api>.execute-api.<region>.amazonaws.com/prod?token=<id-token>"
# Then send:
# {"action":"joinRoom","room_id":"<uuid>"}
# {"action":"sendMessage","room_id":"<uuid>","body":"Hello!"}
# {"action":"ping"}
```
