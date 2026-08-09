output "aws_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "aws_region" {
  value = var.aws_region
}

output "cluster_name" {
  value = aws_eks_cluster.main.name
}

output "cluster_endpoint" {
  value = aws_eks_cluster.main.endpoint
}

output "ecr_api_url" {
  value = aws_ecr_repository.api.repository_url
}

output "ecr_web_url" {
  value = aws_ecr_repository.web.repository_url
}

output "rds_endpoint" {
  value = aws_db_instance.main.address
}

output "rds_port" {
  value = aws_db_instance.main.port
}

output "rds_username" {
  value = aws_db_instance.main.username
}

output "rds_password" {
  value     = random_password.db.result
  sensitive = true
}

output "rds_database_url" {
  value     = "postgres://farewatch:${random_password.db.result}@${aws_db_instance.main.address}:5432/farewatch?sslmode=require"
  sensitive = true
}

output "redis_endpoint" {
  value = aws_elasticache_cluster.main.cache_nodes[0].address
}

output "redis_url" {
  value = "redis://${aws_elasticache_cluster.main.cache_nodes[0].address}:6379/0"
}

output "alb_controller_role_arn" {
  value = aws_iam_role.alb_controller.arn
}

output "vpc_id" {
  value = aws_vpc.main.id
}
