# ---------------------------------------------------------------------------
# IAM — Lambda execution roles and policies
# ---------------------------------------------------------------------------

# ---- Shared assume-role policy for all Lambda functions ----

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

# ---- Shared role (used by all 3 Lambdas) ----

resource "aws_iam_role" "lambda" {
  name               = "${var.app_name}-lambda-role"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

# Basic Lambda execution (CloudWatch Logs)
resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# VPC access (create/delete ENIs so Lambda can reach private subnets)
resource "aws_iam_role_policy_attachment" "lambda_vpc" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

# ---- Secrets Manager — read the DB secret ----

data "aws_iam_policy_document" "read_db_secret" {
  statement {
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [aws_secretsmanager_secret.db.arn]
  }
}

resource "aws_iam_role_policy" "read_db_secret" {
  name   = "read-db-secret"
  role   = aws_iam_role.lambda.id
  policy = data.aws_iam_policy_document.read_db_secret.json
}

# ---- WebSocket push (ws-handler only) ----
# Allows PostToConnection on the WebSocket API Management API

data "aws_iam_policy_document" "ws_manage_connections" {
  statement {
    actions = ["execute-api:ManageConnections"]
    resources = [
      "arn:aws:execute-api:${var.aws_region}:${data.aws_caller_identity.current.account_id}:${aws_apigatewayv2_api.websocket.id}/*"
    ]
  }
}

resource "aws_iam_role_policy" "ws_manage_connections" {
  name   = "ws-manage-connections"
  role   = aws_iam_role.lambda.id
  policy = data.aws_iam_policy_document.ws_manage_connections.json
}
