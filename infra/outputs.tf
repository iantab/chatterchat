# ---------------------------------------------------------------------------
# Outputs — after `terraform apply`, these are the values you need to
# paste into frontend/app.js CONFIG and to run the DB migrations.
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

output "db_endpoint" {
  description = "RDS endpoint (for running migrations via bastion or SSM)"
  value       = aws_db_instance.main.address
}

output "db_secret_arn" {
  description = "Secrets Manager ARN (already set as Lambda env var automatically)"
  value       = aws_secretsmanager_secret.db.arn
}
