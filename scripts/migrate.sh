#!/usr/bin/env bash
# Run all pending DB migrations via the bastion (SSM — no SSH needed).
# Usage: ./scripts/migrate.sh

set -euo pipefail

INFRA_DIR="$(cd "$(dirname "$0")/../infra" && pwd)"
MIGRATIONS_DIR="$(cd "$(dirname "$0")/../backend/migrations" && pwd)"

echo "==> Reading Terraform outputs..."
BASTION_ID=$(cd "$INFRA_DIR" && terraform output -raw bastion_instance_id)
SECRET_ARN=$(cd "$INFRA_DIR" && terraform output -raw db_secret_arn)
DB_HOST=$(cd "$INFRA_DIR" && terraform output -raw db_endpoint)

echo "    Bastion : $BASTION_ID"
echo "    DB host : $DB_HOST"

# Concatenate all migration files
MIGRATION_SQL=$(cat "$MIGRATIONS_DIR"/*.sql)

# Build the script that will run on the bastion.
# We embed the secret ARN, host, and SQL directly — no runtime variable substitution needed.
REMOTE_SCRIPT=$(cat <<SCRIPT
#!/bin/bash
set -e
SECRET=\$(aws secretsmanager get-secret-value --secret-id '$SECRET_ARN' --query SecretString --output text)
DB_PASS=\$(echo "\$SECRET" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['password'])")
DB_USER=\$(echo "\$SECRET" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['username'])")
DB_NAME=\$(echo "\$SECRET" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['dbname'])")
PGPASSWORD="\$DB_PASS" psql -h '$DB_HOST' -U "\$DB_USER" -d "\$DB_NAME" <<'SQL'
$MIGRATION_SQL
SQL
SCRIPT
)

# Base64-encode the script so SSM never sees any shell special characters.
ENCODED=$(printf '%s' "$REMOTE_SCRIPT" | base64 | tr -d '\n')

# Build the SSM input as JSON via Python to avoid any shell quoting issues.
SSM_INPUT=$(python3 -c "
import json, sys
doc = {
    'InstanceIds': ['$BASTION_ID'],
    'DocumentName': 'AWS-RunShellScript',
    'Comment': 'chatterchat DB migrations',
    'Parameters': {'commands': ['echo $ENCODED | base64 -d | bash']},
    'TimeoutSeconds': 60
}
print(json.dumps(doc))
")

echo "==> Sending migration command via SSM..."
CMD_ID=$(aws ssm send-command \
  --cli-input-json "$SSM_INPUT" \
  --query "Command.CommandId" \
  --output text)

echo "    Command ID: $CMD_ID"
echo "==> Waiting for result..."

for i in $(seq 1 24); do
  STATUS=$(aws ssm get-command-invocation \
    --command-id "$CMD_ID" \
    --instance-id "$BASTION_ID" \
    --query "Status" --output text 2>/dev/null || echo "Pending")

  if [[ "$STATUS" == "Success" ]]; then
    echo "==> Migration succeeded!"
    aws ssm get-command-invocation \
      --command-id "$CMD_ID" \
      --instance-id "$BASTION_ID" \
      --query "StandardOutputContent" --output text
    exit 0
  elif [[ "$STATUS" == "Failed" || "$STATUS" == "TimedOut" || "$STATUS" == "Cancelled" ]]; then
    echo "==> Migration FAILED (status: $STATUS)"
    aws ssm get-command-invocation \
      --command-id "$CMD_ID" \
      --instance-id "$BASTION_ID" \
      --query "StandardErrorContent" --output text
    exit 1
  fi

  echo "    Status: $STATUS — waiting..."
  sleep 5
done

echo "==> Timed out waiting for result. Check AWS console for command $CMD_ID."
exit 1
