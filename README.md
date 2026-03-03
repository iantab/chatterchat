# ChatterChat

Real-time chat application. Go backend on AWS Lambda, Vanilla JS frontend on S3 + CloudFront.

```
Browser (Vanilla JS)
   │
   ├── PKCE → Cognito Hosted UI (auth)
   ├── HTTPS → API Gateway HTTP API → Lambda (http-api) → RDS
   └── WSS  → API Gateway WebSocket API → Lambda Authorizer → Lambda (ws-handler) → RDS
                                                                      │
                                                         API GW Management API (broadcast)
```

## Project Structure

```
chatterchat/
├── backend/
│   ├── cmd/
│   │   ├── ws-handler/     # WebSocket Lambda
│   │   ├── http-api/       # REST Lambda
│   │   ├── ws-authorizer/  # Lambda Authorizer ($connect)
│   │   └── local/          # Single-binary local dev server
│   ├── internal/
│   │   ├── auth/           # JWT validation + HTTP middleware
│   │   ├── db/             # DB pool, secrets, queries
│   │   ├── models/         # Shared data structs
│   │   └── ws/             # WS protocol, broadcast, handlers
│   ├── migrations/         # SQL files, run in order
│   ├── go.mod
│   └── Makefile
├── frontend/
│   ├── index.html          # Login page
│   ├── chat.html           # Chat UI
│   ├── style.css
│   ├── app.js              # PKCE auth + WebSocket + REST
│   ├── config.js           # Your config values (gitignored — copy from example)
│   └── config.js.example   # Config template
└── infra/                  # Terraform (AWS infrastructure)
```

---

## Local Development

No AWS account needed. Runs fully locally with Docker for Postgres.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Go 1.22+](https://go.dev/dl/)
- Python 3 (for serving the frontend)

### 1. Start Postgres + run migrations

```bash
docker-compose up -d postgres
docker-compose run --rm migrate
```

### 2. Start the backend

```bash
cd backend
make run-local
```

The server listens on `http://localhost:8080`. Auth is bypassed — a fake user is injected via `LOCAL_DEV_USER`.

### 3. Configure the frontend

```bash
cp frontend/config.js.example frontend/config.js
```

Open `frontend/config.js` and set `localDev: true`.

### 4. Serve the frontend

```bash
cd frontend
python -m http.server 3000
```

Open `http://localhost:3000`.

---

## Deployment

### Prerequisites

Install these tools first:

- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) + credentials configured (`aws configure`)
- [Terraform](https://developer.hashicorp.com/terraform/install)
- [Go 1.22+](https://go.dev/dl/) — in WSL on Windows (builds target Linux ARM64)
- [Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)
- `psql` — `sudo apt install postgresql-client` in WSL

### 1. Build the Lambda zips

Run this in WSL (required — builds Linux ARM64 binaries):

```bash
cd backend
make build
```

Produces:
- `backend/dist/ws-handler/ws-handler.zip`
- `backend/dist/ws-authorizer/ws-authorizer.zip`
- `backend/dist/http-api/http-api.zip`

### 2. Configure Terraform

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
aws_region            = "us-east-1"
app_name              = "chatterchat"
db_password           = "SomeStrongPassword1!"
cognito_domain_prefix = "chatterchat-yourname"  # must be globally unique
```

### 3. Deploy infrastructure

```bash
cd infra
terraform init
terraform apply
```

Takes 15–25 minutes (RDS + CloudFront). At the end, Terraform prints outputs — save them.

### 4. Run database migrations

RDS is in a private VPC. Use SSM port forwarding to tunnel to it from your machine.

**Open the tunnel** (keep this terminal open):

```bash
aws ssm start-session \
  --target <bastion_instance_id from outputs> \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters '{"host":["<db_endpoint from outputs>"],"portNumber":["5432"],"localPortNumber":["5433"]}'
```

**In a new terminal, get the DB password:**

```bash
aws secretsmanager get-secret-value \
  --secret-id chatterchat/db-credentials \
  --query SecretString --output text
```

**Run the migrations:**

```bash
cd backend
for f in migrations/*.sql; do
  echo "Running $f..."
  psql "host=localhost port=5433 dbname=chatterchat user=chatterchat password=<password> sslmode=require" -f "$f"
done
```

Close the tunnel (`Ctrl+C`) when done. You can also terminate the bastion EC2 instance in the AWS Console to stop paying for it (~$1.50/month).

### 5. Configure and deploy the frontend

```bash
cp frontend/config.js.example frontend/config.js
```

Fill in `frontend/config.js` with the values from `terraform output app_js_config`. Make sure `localDev: false`.

Upload to S3:

```bash
aws s3 sync frontend/ s3://$(terraform -chdir=infra output -raw s3_bucket_name)/ --delete
```

### 6. Open the app

```bash
terraform -chdir=infra output cloudfront_domain
```

Visit that URL in your browser.

---

## Redeploying after code changes

**Backend:**

```bash
cd backend && make build
cd infra && terraform apply
```

**Frontend:**

```bash
aws s3 sync frontend/ s3://$(terraform -chdir=infra output -raw s3_bucket_name)/ --delete
```

---

## Tearing down

```bash
cd infra
terraform destroy
```

Deletes everything including the database.
