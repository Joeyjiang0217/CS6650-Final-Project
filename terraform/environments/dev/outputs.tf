output "vpc_id" {
  value = module.network.vpc_id
}

output "public_subnet_ids" {
  value = module.network.public_subnet_ids
}

output "private_app_subnet_ids" {
  value = module.network.private_app_subnet_ids
}

output "private_data_subnet_ids" {
  value = module.network.private_data_subnet_ids
}

output "alb_security_group_id" {
  value = module.security.alb_security_group_id
}

output "gateway_security_group_id" {
  value = module.security.gateway_security_group_id
}

output "internal_service_security_group_id" {
  value = module.security.internal_service_security_group_id
}

output "rds_security_group_id" {
  value = module.security.rds_security_group_id
}

output "redis_security_group_id" {
  value = module.security.redis_security_group_id
}

output "ecr_repository_urls" {
  value = module.ecr.repository_urls
}

output "log_group_names" {
  value = module.logs.log_group_names
}

output "ecs_task_execution_role_arn" {
  value = module.iam.ecs_task_execution_role_arn
}

output "ecs_task_role_arn" {
  value = module.iam.ecs_task_role_arn
}

output "ecs_cluster_name" {
  value = module.ecs_cluster.cluster_name
}

output "ecs_cluster_arn" {
  value = module.ecs_cluster.cluster_arn
}

output "alb_dns_name" {
  value = module.alb.alb_dns_name
}

output "gateway_target_group_arn" {
  value = module.alb.gateway_target_group_arn
}

output "rds_endpoint" {
  value = module.rds_mysql.db_endpoint
}

output "rds_port" {
  value = module.rds_mysql.db_port
}

output "redis_primary_endpoint" {
  value = module.elasticache_redis.primary_endpoint_address
}

output "redis_port" {
  value = module.elasticache_redis.port
}

output "service_connect_namespace_arn" {
  value = aws_service_discovery_private_dns_namespace.service_connect.arn
}

output "gateway_service_name" {
  value = module.gateway_service.service_name
}

output "chatservice_service_name" {
  value = module.chatservice_service.service_name
}

output "messagestorage_service_name" {
  value = module.messagestorage_service.service_name
}

output "messagetransmit_service_name" {
  value = module.messagetransmit_service.service_name
}