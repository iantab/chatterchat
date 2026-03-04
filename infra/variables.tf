variable "aws_region" {
  description = "AWS region to deploy into (e.g. us-east-1)"
  type        = string
  default     = "us-east-1"
}

variable "app_name" {
  description = "Short name used as a prefix on all resources"
  type        = string
  default     = "chatterchat"
}

variable "cognito_domain_prefix" {
  description = <<-EOT
    Globally-unique prefix for the Cognito hosted-UI domain.
    Your login page will be at: https://<prefix>.auth.<region>.amazoncognito.com
    Use lowercase letters, numbers, and hyphens only. Must be globally unique across all AWS accounts.
    Example: "chatterchat-yourname-2024"
  EOT
  type = string
}
