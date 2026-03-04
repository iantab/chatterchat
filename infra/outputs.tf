# ---------------------------------------------------------------------------
# Outputs — after `terraform apply`, paste these into frontend/config.js
# ---------------------------------------------------------------------------

output "app_js_config" {
  description = "Paste these values into frontend/config.js CONFIG"
  value = {
    apiBase       = aws_apigatewayv2_api.http.api_endpoint
    wsBase        = "${aws_apigatewayv2_stage.ws_prod.invoke_url}"
    cognitoDomain = "${var.cognito_domain_prefix}.auth.${var.aws_region}.amazoncognito.com"
    clientId      = aws_cognito_user_pool_client.main.id
    userPoolId    = aws_cognito_user_pool.main.id
    redirectUri   = "https://${aws_cloudfront_distribution.frontend.domain_name}/chat.html"
  }
}

output "s3_bucket_name" {
  description = "Upload frontend files to this S3 bucket"
  value       = aws_s3_bucket.frontend.bucket
}

output "cloudfront_domain" {
  description = "Your app's public URL"
  value       = "https://${aws_cloudfront_distribution.frontend.domain_name}"
}

output "dynamodb_table_names" {
  description = "DynamoDB table names (set automatically as Lambda env vars)"
  value = {
    users       = aws_dynamodb_table.users.name
    rooms       = aws_dynamodb_table.rooms.name
    messages    = aws_dynamodb_table.messages.name
    connections = aws_dynamodb_table.connections.name
  }
}
