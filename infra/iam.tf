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

# ---- DynamoDB access ----

data "aws_iam_policy_document" "dynamodb_access" {
  statement {
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:DeleteItem",
      "dynamodb:Query",
      "dynamodb:Scan",
    ]
    resources = [
      aws_dynamodb_table.users.arn,
      aws_dynamodb_table.rooms.arn,
      "${aws_dynamodb_table.rooms.arn}/index/*",
      aws_dynamodb_table.messages.arn,
      aws_dynamodb_table.connections.arn,
      "${aws_dynamodb_table.connections.arn}/index/*",
    ]
  }
}

resource "aws_iam_role_policy" "dynamodb_access" {
  name   = "dynamodb-access"
  role   = aws_iam_role.lambda.id
  policy = data.aws_iam_policy_document.dynamodb_access.json
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
