# ---------------------------------------------------------------------------
# Cognito User Pool — handles sign-up/sign-in with hosted UI + PKCE
# ---------------------------------------------------------------------------

resource "aws_cognito_user_pool" "main" {
  name = var.app_name

  # Allow users to sign in with their email address
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  password_policy {
    minimum_length    = 8
    require_lowercase = true
    require_numbers   = true
    require_symbols   = false
    require_uppercase = true
  }

  # Email verification
  verification_message_template {
    default_email_option = "CONFIRM_WITH_CODE"
  }

  schema {
    name                = "email"
    attribute_data_type = "String"
    required            = true
    mutable             = true
  }

  tags = { Name = var.app_name }
}

# ---- App Client (no secret = PKCE-compatible) ----

resource "aws_cognito_user_pool_client" "main" {
  name         = "${var.app_name}-client"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret = false # Required for PKCE from a browser

  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  allowed_oauth_flows_user_pool_client = true
  supported_identity_providers         = ["COGNITO"]

  # After login, Cognito redirects here with the auth code
  callback_urls = ["https://${aws_cloudfront_distribution.frontend.domain_name}/chat.html"]
  # After logout, redirect here
  logout_urls = ["https://${aws_cloudfront_distribution.frontend.domain_name}/index.html"]

  explicit_auth_flows = [
    "ALLOW_REFRESH_TOKEN_AUTH",
    "ALLOW_USER_SRP_AUTH",
  ]

  token_validity_units {
    access_token  = "hours"
    id_token      = "hours"
    refresh_token = "days"
  }

  access_token_validity  = 1
  id_token_validity      = 1
  refresh_token_validity = 30
}

# ---- Hosted UI domain ----
# Your login page will be: https://<prefix>.auth.<region>.amazoncognito.com/login

resource "aws_cognito_user_pool_domain" "main" {
  domain       = var.cognito_domain_prefix
  user_pool_id = aws_cognito_user_pool.main.id
}
