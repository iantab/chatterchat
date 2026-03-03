# ---------------------------------------------------------------------------
# API Gateway — HTTP API
# JWT authorizer backed by Cognito; CORS allows CloudFront origin
# ---------------------------------------------------------------------------

resource "aws_apigatewayv2_api" "http" {
  name          = "${var.app_name}-http"
  protocol_type = "HTTP"

  cors_configuration {
    allow_origins = ["https://${aws_cloudfront_distribution.frontend.domain_name}"]
    allow_methods = ["GET", "POST", "PUT", "OPTIONS"]
    allow_headers = ["Content-Type", "Authorization"]
    max_age       = 300
  }
}

# ---- JWT authorizer (Cognito) ----

resource "aws_apigatewayv2_authorizer" "http_jwt" {
  api_id           = aws_apigatewayv2_api.http.id
  authorizer_type  = "JWT"
  identity_sources = ["$request.header.Authorization"]
  name             = "${var.app_name}-jwt"

  jwt_configuration {
    issuer   = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.main.id}"
    audience = [aws_cognito_user_pool_client.main.id]
  }
}

# ---- Lambda integration (http-api) ----

resource "aws_apigatewayv2_integration" "http_api" {
  api_id                 = aws_apigatewayv2_api.http.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.http_api.invoke_arn
  payload_format_version = "2.0"
}

# ---- Routes ----

# Public health check (no auth)
resource "aws_apigatewayv2_route" "health" {
  api_id    = aws_apigatewayv2_api.http.id
  route_key = "GET /health"
  target    = "integrations/${aws_apigatewayv2_integration.http_api.id}"
}

# Authenticated routes
resource "aws_apigatewayv2_route" "list_rooms" {
  api_id             = aws_apigatewayv2_api.http.id
  route_key          = "GET /rooms"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.http_jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.http_api.id}"
}

resource "aws_apigatewayv2_route" "create_room" {
  api_id             = aws_apigatewayv2_api.http.id
  route_key          = "POST /rooms"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.http_jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.http_api.id}"
}

resource "aws_apigatewayv2_route" "get_room" {
  api_id             = aws_apigatewayv2_api.http.id
  route_key          = "GET /rooms/{id}"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.http_jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.http_api.id}"
}

resource "aws_apigatewayv2_route" "get_messages" {
  api_id             = aws_apigatewayv2_api.http.id
  route_key          = "GET /rooms/{id}/messages"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.http_jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.http_api.id}"
}

resource "aws_apigatewayv2_route" "get_me" {
  api_id             = aws_apigatewayv2_api.http.id
  route_key          = "GET /users/me"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.http_jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.http_api.id}"
}

resource "aws_apigatewayv2_route" "update_me" {
  api_id             = aws_apigatewayv2_api.http.id
  route_key          = "PUT /users/me"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.http_jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.http_api.id}"
}

# ---- Stage ----

resource "aws_apigatewayv2_stage" "http_default" {
  api_id      = aws_apigatewayv2_api.http.id
  name        = "$default"
  auto_deploy = true
}

# ---- Permission for API GW to invoke http-api Lambda ----

resource "aws_lambda_permission" "http_api_invoke" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.http_api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.http.execution_arn}/*/*"
}
