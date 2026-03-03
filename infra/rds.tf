# ---------------------------------------------------------------------------
# RDS PostgreSQL — db.t4g.micro (~$13/month), private subnets only
# ---------------------------------------------------------------------------

resource "aws_db_subnet_group" "main" {
  name       = "${var.app_name}-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id
  tags       = { Name = "${var.app_name}-db-subnet-group" }
}

resource "aws_db_instance" "main" {
  identifier        = "${var.app_name}-postgres"
  engine            = "postgres"
  engine_version    = "16"
  instance_class    = "db.t4g.micro"
  allocated_storage = 20
  storage_type      = "gp3"
  storage_encrypted = true

  db_name  = "chatterchat"
  username = "chatterchat"
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false
  multi_az               = false # Single-AZ is fine for personal projects

  backup_retention_period = 7
  skip_final_snapshot     = true # Set to false in production
  deletion_protection     = false

  tags = { Name = "${var.app_name}-postgres" }
}
