# ---------------------------------------------------------------------------
# Bastion — a tiny EC2 instance used only to run DB migrations.
# No SSH port is open. Access is via AWS Systems Manager (SSM) Session Manager.
# After running migrations you can terminate this instance to save money.
# ---------------------------------------------------------------------------

# Find the latest Amazon Linux 2023 ARM64 AMI
data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]
  filter {
    name   = "name"
    values = ["al2023-ami-*-arm64"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# IAM role so the bastion can use SSM Session Manager (no SSH needed)
resource "aws_iam_role" "bastion" {
  name = "${var.app_name}-bastion-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "bastion_ssm" {
  role       = aws_iam_role.bastion.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

# Allow bastion to read the DB secret (to get the password for psql)
resource "aws_iam_role_policy" "bastion_read_secret" {
  name = "read-db-secret"
  role = aws_iam_role.bastion.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action   = "secretsmanager:GetSecretValue"
      Effect   = "Allow"
      Resource = aws_secretsmanager_secret.db.arn
    }]
  })
}

resource "aws_iam_instance_profile" "bastion" {
  name = "${var.app_name}-bastion-profile"
  role = aws_iam_role.bastion.name
}

# Security group for the bastion — outbound only
resource "aws_security_group" "bastion" {
  name        = "${var.app_name}-bastion-sg"
  description = "Bastion (SSM only, no inbound)"
  vpc_id      = aws_vpc.main.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-bastion-sg" }
}

# Allow bastion to connect to RDS
resource "aws_security_group_rule" "rds_allow_bastion" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.bastion.id
  security_group_id        = aws_security_group.rds.id
}

resource "aws_instance" "bastion" {
  ami                    = data.aws_ami.al2023.id
  instance_type          = "t4g.nano" # ~$1.50/month; terminate after migrations
  iam_instance_profile   = aws_iam_instance_profile.bastion.name
  subnet_id              = aws_subnet.public[0].id
  vpc_security_group_ids = [aws_security_group.bastion.id]

  # Install postgresql client on boot
  user_data = <<-EOF
    #!/bin/bash
    dnf install -y postgresql15
  EOF

  tags = { Name = "${var.app_name}-bastion" }
}

output "bastion_instance_id" {
  description = "Use this to start an SSM session: aws ssm start-session --target <id>"
  value       = aws_instance.bastion.id
}
