variable "aws_region" {
  description = "AWS region where the stack will be created."
  type        = string
  default     = "us-east-2"
}

variable "project_name" {
  description = "Short name used to name AWS resources."
  type        = string
  default     = "cards-api"
}

variable "environment" {
  description = "Deployment environment."
  type        = string
  default     = "dev"
}

variable "vpc_cidr" {
  description = "IPv4 range for the VPC."
  type        = string
  default     = "10.42.0.0/16"
}

variable "container_image" {
  description = "Full ECR image URI. Leave empty for the first apply; set it after pushing the image."
  type        = string
  default     = ""
}

variable "desired_count" {
  description = "Number of Fargate tasks. The test requires at least two."
  type        = number
  default     = 2

  validation {
    condition     = var.desired_count >= 2
    error_message = "desired_count must be at least 2 to satisfy the test requirements."
  }
}

variable "task_cpu" {
  description = "Fargate task CPU units."
  type        = number
  default     = 256
}

variable "task_memory" {
  description = "Fargate task memory in MiB."
  type        = number
  default     = 512
}

variable "db_instance_class" {
  description = "RDS instance class for the development database."
  type        = string
  default     = "db.t4g.micro"
}

variable "db_name" {
  description = "Database name created by RDS."
  type        = string
  default     = "cards_api"
}

variable "db_username" {
  description = "RDS master username. The master password is managed by RDS and Secrets Manager."
  type        = string
  default     = "cards_api_admin"
}

variable "db_allocated_storage" {
  description = "Initial RDS storage in GiB."
  type        = number
  default     = 20
}

variable "db_max_allocated_storage" {
  description = "Maximum autoscaled RDS storage in GiB."
  type        = number
  default     = 50
}

variable "db_deletion_protection" {
  description = "Protect the database from accidental deletion. Keep false only for disposable environments."
  type        = bool
  default     = false
}

variable "log_retention_days" {
  description = "Retention period for application logs in CloudWatch Logs."
  type        = number
  default     = 14
}

variable "single_nat_gateway" {
  description = "Use one NAT gateway to reduce development cost; this is not AZ-resilient egress."
  type        = bool
  default     = true
}
