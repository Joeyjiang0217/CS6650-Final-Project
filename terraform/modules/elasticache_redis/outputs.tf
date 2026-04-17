output "replication_group_arn" {
  value = aws_elasticache_replication_group.this.arn
}

output "replication_group_id" {
  value = aws_elasticache_replication_group.this.replication_group_id
}

output "primary_endpoint_address" {
  value = aws_elasticache_replication_group.this.primary_endpoint_address
}

output "reader_endpoint_address" {
  value = aws_elasticache_replication_group.this.reader_endpoint_address
}

output "port" {
  value = aws_elasticache_replication_group.this.port
}

output "subnet_group_name" {
  value = aws_elasticache_subnet_group.this.name
}