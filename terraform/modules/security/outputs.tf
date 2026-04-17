output "alb_security_group_id" {
  value = aws_security_group.alb.id
}

output "gateway_security_group_id" {
  value = aws_security_group.gateway.id
}

output "internal_service_security_group_id" {
  value = aws_security_group.internal_service.id
}

output "rds_security_group_id" {
  value = aws_security_group.rds.id
}

output "redis_security_group_id" {
  value = aws_security_group.redis.id
}