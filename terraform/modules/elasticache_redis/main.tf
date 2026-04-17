locals {
  name_prefix = "${var.project_name}-${var.environment}"
}

resource "aws_elasticache_subnet_group" "this" {
  name       = "${local.name_prefix}-redis-subnet-group"
  subnet_ids = var.private_data_subnet_ids

  tags = {
    Name = "${local.name_prefix}-redis-subnet-group"
  }
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id       = "${var.project_name}-${var.environment}-redis"
  description                = "${local.name_prefix} redis replication group"

  engine                     = "redis"
  engine_version             = var.engine_version
  node_type                  = var.node_type
  port                       = 6379

  subnet_group_name          = aws_elasticache_subnet_group.this.name
  security_group_ids         = [var.redis_security_group_id]

  num_cache_clusters         = var.num_cache_clusters
  multi_az_enabled           = var.multi_az_enabled
  automatic_failover_enabled = var.automatic_failover_enabled

  at_rest_encryption_enabled = var.at_rest_encryption_enabled
  transit_encryption_enabled = var.transit_encryption_enabled

  tags = {
    Name = "${local.name_prefix}-redis"
  }
}