output "api_url" {
  description = "HTTPS URL for the API through the public Application Load Balancer."
  value       = "https://${aws_lb.api.dns_name}"
}

output "alb_dns_name" {
  description = "DNS name of the public Application Load Balancer."
  value       = aws_lb.api.dns_name
}

output "ecr_repository_url" {
  description = "ECR repository URL for the Cards API image."
  value       = aws_ecr_repository.api.repository_url
}

output "ecs_cluster_name" {
  description = "ECS cluster name for the Cards API service."
  value       = aws_ecs_cluster.main.name
}

output "ecs_service_name" {
  description = "ECS service name for the Cards API."
  value       = local.name
}

output "private_subnet_ids" {
  description = "Private subnet IDs for ECS tasks."
  value       = [for subnet in aws_subnet.private : subnet.id]
}

output "rds_endpoint" {
  description = "Private RDS endpoint."
  value       = aws_db_instance.cards.address
}

output "pan_hash_salt_secret_arn" {
  description = "ARN of the Secrets Manager secret used by the API."
  value       = aws_secretsmanager_secret.pan_hash_salt.arn
}

output "cloudwatch_log_group" {
  description = "CloudWatch Logs group receiving API stdout/stderr."
  value       = aws_cloudwatch_log_group.api.name
}
