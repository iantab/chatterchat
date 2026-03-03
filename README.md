# ChatterChat

Real-time chat application. Go backend on AWS Lambda, Vanilla JS frontend on S3 + CloudFront.

```
Browser (Vanilla JS)
   │
   ├── SRP → Cognito (custom auth UI, no redirect)
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
│   ├── index.html          # Login page (Sign In / Create Account / Verify)
│   ├── chat.html           # Chat UI
│   ├── style.css
│   ├── app.js              # Auth + WebSocket + REST + display name
│   ├── config.js           # Your config values (gitignored — copy from example)
│   └── config.js.example   # Config template
├── infra/                  # Terraform (AWS infrastructure)
└── scripts/
    └── migrate.sh          # Run DB migrations via SSM (no SSH needed)
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

- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) + credentials configured (`aws configure`)
- [Terraform](https://developer.hashicorp.com/terraform/install)
- [Go 1.22+](https://go.dev/dl/) — in WSL on Windows (builds target Linux ARM64)

### 1. Build the Lambda zips

Run this in WSL (required — builds Linux ARM64 binaries):

```bash
cd backend
make build
```

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

Takes 15–25 minutes (RDS + CloudFront). When done, grab your config values:

```bash
terraform output app_js_config
```

### 4. Run database migrations

RDS is in a private VPC. The migration script handles the SSM tunnel automatically — no manual steps needed:

```bash
./scripts/migrate.sh
```

This SSMs into the bastion, reads DB credentials from Secrets Manager, and runs all `backend/migrations/*.sql` files in order. You can terminate the bastion EC2 instance in the AWS Console afterwards to stop paying for it (~$1.50/month).

### 5. Configure and deploy the frontend

```bash
cp frontend/config.js.example frontend/config.js
```

Fill in `frontend/config.js` with the values from `terraform output app_js_config`. All fields are required:

| Field | Where to get it |
|---|---|
| `apiBase` | `terraform output app_js_config` |
| `wsBase` | `terraform output app_js_config` |
| `cognitoDomain` | `terraform output app_js_config` |
| `clientId` | `terraform output app_js_config` |
| `userPoolId` | `terraform output app_js_config` |
| `redirectUri` | `terraform output app_js_config` |

Make sure `localDev: false`.

Upload to S3:

```bash
aws s3 sync frontend/ s3://$(terraform -chdir=infra output -raw s3_bucket_name)/ --delete
aws cloudfront create-invalidation \
  --distribution-id $(aws cloudfront list-distributions \
    --query "DistributionList.Items[?Comment=='chatterchat frontend'].Id" --output text) \
  --paths "/*"
```

### 6. Open the app

```bash
terraform -chdir=infra output cloudfront_domain
```

Visit that URL. You'll see the custom Sign In / Create Account page — no redirect to Cognito's hosted UI.

---

## Redeploying after code changes

**Backend** (also handles new API routes / infra changes):

```bash
cd backend && make build
cd infra && terraform apply
```

**Frontend:**

```bash
aws s3 sync frontend/ s3://$(terraform -chdir=infra output -raw s3_bucket_name)/ --delete
```

**Database migrations** (when new `.sql` files are added):

```bash
./scripts/migrate.sh
```

---

## Tearing down

```bash
cd infra
terraform destroy
```

Deletes everything including the database.
