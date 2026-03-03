# ---------------------------------------------------------------------------
# Secrets Manager — stores DB credentials in the exact JSON format
# that the Go backend's secrets.go expects.
# ---------------------------------------------------------------------------

resource "aws_secretsmanager_secret" "db" {
  name                    = "${var.app_name}/db-credentials"
  description             = "PostgreSQL credentials for ChatterChat"
  recovery_window_in_days = 0 # Allows immediate deletion; set to 7+ in production
}

resource "aws_secretsmanager_secret_version" "db" {
  secret_id = aws_secretsmanager_secret.db.id

  # Must match the dbSecret struct in backend/internal/db/secrets.go
  secret_string = jsonencode({
    username = aws_db_instance.main.username
    password = var.db_password
    host     = aws_db_instance.main.address
    port     = 5432
    dbname   = aws_db_instance.main.db_name
  })
}
