# ---------------------------------------------------------------------------
# Lambda functions — build zips first with: cd backend && make build
# Zips are expected at backend/dist/{name}/{name}.zip
# ---------------------------------------------------------------------------

locals {
  lambda_env = {
    DB_SECRET_ARN        = aws_secretsmanager_secret.db.arn
    COGNITO_REGION       = var.aws_region
    COGNITO_USER_POOL_ID = aws_cognito_user_pool.main.id
    COGNITO_CLIENT_ID    = aws_cognito_user_pool_client.main.id
  }

  lambda_vpc_config = {
    subnet_ids         = aws_subnet.private[*].id
    security_group_ids = [aws_security_group.lambda.id]
  }
}

# ---- ws-handler ----

resource "aws_lambda_function" "ws_handler" {
  function_name = "${var.app_name}-ws-handler"
  role          = aws_iam_role.lambda.arn
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  handler       = "bootstrap"
  filename      = "${path.module}/../backend/dist/ws-handler/ws-handler.zip"
  source_code_hash = try(
    filebase64sha256("${path.module}/../backend/dist/ws-handler/ws-handler.zip"),
    null
  )
  timeout      = 30
  memory_size  = 128

  environment {
    variables = local.lambda_env
  }

  vpc_config {
    subnet_ids         = local.lambda_vpc_config.subnet_ids
    security_group_ids = local.lambda_vpc_config.security_group_ids
  }

  tags = { Name = "${var.app_name}-ws-handler" }
}

# ---- ws-authorizer ----

resource "aws_lambda_function" "ws_authorizer" {
  function_name = "${var.app_name}-ws-authorizer"
  role          = aws_iam_role.lambda.arn
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  handler       = "bootstrap"
  filename      = "${path.module}/../backend/dist/ws-authorizer/ws-authorizer.zip"
  source_code_hash = try(
    filebase64sha256("${path.module}/../backend/dist/ws-authorizer/ws-authorizer.zip"),
    null
  )
  timeout     = 10
  memory_size = 128

  environment {
    variables = local.lambda_env
  }

  vpc_config {
    subnet_ids         = local.lambda_vpc_config.subnet_ids
    security_group_ids = local.lambda_vpc_config.security_group_ids
  }

  tags = { Name = "${var.app_name}-ws-authorizer" }
}

# ---- http-api ----

resource "aws_lambda_function" "http_api" {
  function_name = "${var.app_name}-http-api"
  role          = aws_iam_role.lambda.arn
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  handler       = "bootstrap"
  filename      = "${path.module}/../backend/dist/http-api/http-api.zip"
  source_code_hash = try(
    filebase64sha256("${path.module}/../backend/dist/http-api/http-api.zip"),
    null
  )
  timeout     = 30
  memory_size = 128

  environment {
    variables = local.lambda_env
  }

  vpc_config {
    subnet_ids         = local.lambda_vpc_config.subnet_ids
    security_group_ids = local.lambda_vpc_config.security_group_ids
  }

  tags = { Name = "${var.app_name}-http-api" }
}
