# ---------------------------------------------------------------------------
# API Gateway — WebSocket API
# Route selection: $request.body.action
# Routes: $connect, $disconnect, ping, joinRoom, sendMessage, $default
# ---------------------------------------------------------------------------

resource "aws_apigatewayv2_api" "websocket" {
  name                       = "${var.app_name}-ws"
  protocol_type              = "WEBSOCKET"
  route_selection_expression = "$request.body.action"
}

# ---- Lambda integration (ws-handler handles all routes) ----

resource "aws_apigatewayv2_integration" "ws_handler" {
  api_id                    = aws_apigatewayv2_api.websocket.id
  integration_type          = "AWS_PROXY"
  integration_uri           = aws_lambda_function.ws_handler.invoke_arn
  content_handling_strategy = "CONVERT_TO_TEXT"
}

# ---- Lambda authorizer ($connect uses this to validate the token) ----

resource "aws_apigatewayv2_authorizer" "ws" {
  api_id                            = aws_apigatewayv2_api.websocket.id
  authorizer_type                   = "REQUEST"
  authorizer_uri                    = aws_lambda_function.ws_authorizer.invoke_arn
  identity_sources                  = ["route.request.querystring.token"]
  name                              = "${var.app_name}-ws-authorizer"
  authorizer_credentials_arn        = aws_iam_role.apigw_invoke_authorizer.arn
}

# IAM role so API Gateway can invoke the authorizer Lambda
resource "aws_iam_role" "apigw_invoke_authorizer" {
  name = "${var.app_name}-apigw-authorizer-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "apigateway.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy" "apigw_invoke_authorizer" {
  name = "invoke-authorizer"
  role = aws_iam_role.apigw_invoke_authorizer.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action   = "lambda:InvokeFunction"
      Effect   = "Allow"
      Resource = aws_lambda_function.ws_authorizer.arn
    }]
  })
}

# ---- Routes ----

resource "aws_apigatewayv2_route" "ws_connect" {
  api_id             = aws_apigatewayv2_api.websocket.id
  route_key          = "$connect"
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.ws.id
  target             = "integrations/${aws_apigatewayv2_integration.ws_handler.id}"
}

resource "aws_apigatewayv2_route" "ws_disconnect" {
  api_id    = aws_apigatewayv2_api.websocket.id
  route_key = "$disconnect"
  target    = "integrations/${aws_apigatewayv2_integration.ws_handler.id}"
}

resource "aws_apigatewayv2_route" "ws_ping" {
  api_id    = aws_apigatewayv2_api.websocket.id
  route_key = "ping"
  target    = "integrations/${aws_apigatewayv2_integration.ws_handler.id}"
}

resource "aws_apigatewayv2_route" "ws_join_room" {
  api_id    = aws_apigatewayv2_api.websocket.id
  route_key = "joinRoom"
  target    = "integrations/${aws_apigatewayv2_integration.ws_handler.id}"
}

resource "aws_apigatewayv2_route" "ws_send_message" {
  api_id    = aws_apigatewayv2_api.websocket.id
  route_key = "sendMessage"
  target    = "integrations/${aws_apigatewayv2_integration.ws_handler.id}"
}

resource "aws_apigatewayv2_route" "ws_default" {
  api_id    = aws_apigatewayv2_api.websocket.id
  route_key = "$default"
  target    = "integrations/${aws_apigatewayv2_integration.ws_handler.id}"
}

# ---- Stage + deployment ----

resource "aws_apigatewayv2_stage" "ws_prod" {
  api_id      = aws_apigatewayv2_api.websocket.id
  name        = "prod"
  auto_deploy = true
}

# ---- Permission for API GW to invoke ws-handler ----

resource "aws_lambda_permission" "ws_handler_invoke" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ws_handler.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket.execution_arn}/*/*"
}
