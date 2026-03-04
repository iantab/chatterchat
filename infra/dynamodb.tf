# ---------------------------------------------------------------------------
# DynamoDB — 4 tables (PAY_PER_REQUEST billing, no VPC needed)
# ---------------------------------------------------------------------------

# ---- Users ----

resource "aws_dynamodb_table" "users" {
  name         = "${var.app_name}-users"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "cognito_sub"

  attribute {
    name = "cognito_sub"
    type = "S"
  }

  tags = { Name = "${var.app_name}-users" }
}

# ---- Rooms ----

resource "aws_dynamodb_table" "rooms" {
  name         = "${var.app_name}-rooms"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }

  attribute {
    name = "name"
    type = "S"
  }

  global_secondary_index {
    name            = "name-index"
    hash_key        = "name"
    projection_type = "ALL"
  }

  tags = { Name = "${var.app_name}-rooms" }
}

# ---- Messages ----

resource "aws_dynamodb_table" "messages" {
  name         = "${var.app_name}-messages"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "room_id"
  range_key    = "ts_id"

  attribute {
    name = "room_id"
    type = "S"
  }

  attribute {
    name = "ts_id"
    type = "S"
  }

  tags = { Name = "${var.app_name}-messages" }
}

# ---- Connections ----

resource "aws_dynamodb_table" "connections" {
  name         = "${var.app_name}-connections"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "connection_id"

  attribute {
    name = "connection_id"
    type = "S"
  }

  attribute {
    name = "room_id"
    type = "S"
  }

  global_secondary_index {
    name            = "room-index"
    hash_key        = "room_id"
    projection_type = "ALL"
  }

  tags = { Name = "${var.app_name}-connections" }
}

# ---------------------------------------------------------------------------
# Seed default rooms
# ---------------------------------------------------------------------------

resource "aws_dynamodb_table_item" "room_general" {
  table_name = aws_dynamodb_table.rooms.name
  hash_key   = aws_dynamodb_table.rooms.hash_key

  item = jsonencode({
    id         = { S = "00000000-0000-0000-0000-000000000001" }
    name       = { S = "General" }
    created_at = { S = "2025-01-01T00:00:00Z" }
  })
}

resource "aws_dynamodb_table_item" "room_random" {
  table_name = aws_dynamodb_table.rooms.name
  hash_key   = aws_dynamodb_table.rooms.hash_key

  item = jsonencode({
    id         = { S = "00000000-0000-0000-0000-000000000002" }
    name       = { S = "Random" }
    created_at = { S = "2025-01-01T00:00:00Z" }
  })
}
