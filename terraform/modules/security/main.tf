locals {
  name_prefix = "${var.project_name}-${var.environment}"
}

resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-alb-sg"
  description = "ALB security group"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name_prefix}-alb-sg"
  }
}

resource "aws_security_group" "gateway" {
  name        = "${local.name_prefix}-gateway-sg"
  description = "Gateway service security group"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name_prefix}-gateway-sg"
  }
}

resource "aws_security_group" "internal_service" {
  name        = "${local.name_prefix}-internal-service-sg"
  description = "Internal gRPC services security group"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name_prefix}-internal-service-sg"
  }
}

resource "aws_security_group" "rds" {
  name        = "${local.name_prefix}-rds-sg"
  description = "RDS MySQL security group"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name_prefix}-rds-sg"
  }
}

resource "aws_security_group" "redis" {
  name        = "${local.name_prefix}-redis-sg"
  description = "Redis security group"
  vpc_id      = var.vpc_id

  tags = {
    Name = "${local.name_prefix}-redis-sg"
  }
}

# ALB ingress from internet
resource "aws_vpc_security_group_ingress_rule" "alb_http" {
  security_group_id = aws_security_group.alb.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  to_port           = 80
  ip_protocol       = "tcp"
  description       = "Allow HTTP from internet"
}

resource "aws_vpc_security_group_ingress_rule" "alb_https" {
  security_group_id = aws_security_group.alb.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
  description       = "Allow HTTPS from internet"
}

resource "aws_vpc_security_group_egress_rule" "alb_all_out" {
  security_group_id = aws_security_group.alb.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "Allow all outbound"
}

# Gateway ingress only from ALB
resource "aws_vpc_security_group_ingress_rule" "gateway_from_alb" {
  security_group_id            = aws_security_group.gateway.id
  referenced_security_group_id = aws_security_group.alb.id
  from_port                    = var.gateway_port
  to_port                      = var.gateway_port
  ip_protocol                  = "tcp"
  description                  = "Allow gateway traffic from ALB"
}

resource "aws_vpc_security_group_egress_rule" "gateway_all_out" {
  security_group_id = aws_security_group.gateway.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "Allow all outbound"
}

# Internal services ingress from gateway for each gRPC port
resource "aws_vpc_security_group_ingress_rule" "internal_from_gateway" {
  for_each = toset([for p in var.grpc_service_ports : tostring(p)])

  security_group_id            = aws_security_group.internal_service.id
  referenced_security_group_id = aws_security_group.gateway.id
  from_port                    = tonumber(each.value)
  to_port                      = tonumber(each.value)
  ip_protocol                  = "tcp"
  description                  = "Allow gateway to internal gRPC services"
}

# Internal services ingress from themselves for service-to-service calls
resource "aws_vpc_security_group_ingress_rule" "internal_from_internal" {
  for_each = toset([for p in var.grpc_service_ports : tostring(p)])

  security_group_id            = aws_security_group.internal_service.id
  referenced_security_group_id = aws_security_group.internal_service.id
  from_port                    = tonumber(each.value)
  to_port                      = tonumber(each.value)
  ip_protocol                  = "tcp"
  description                  = "Allow internal service to internal service gRPC"
}

resource "aws_vpc_security_group_egress_rule" "internal_all_out" {
  security_group_id = aws_security_group.internal_service.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "Allow all outbound"
}

# RDS ingress only from internal services
resource "aws_vpc_security_group_ingress_rule" "rds_from_internal" {
  security_group_id            = aws_security_group.rds.id
  referenced_security_group_id = aws_security_group.internal_service.id
  from_port                    = var.mysql_port
  to_port                      = var.mysql_port
  ip_protocol                  = "tcp"
  description                  = "Allow MySQL from internal services"
}

resource "aws_vpc_security_group_egress_rule" "rds_all_out" {
  security_group_id = aws_security_group.rds.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "Allow all outbound"
}

# Redis ingress only from gateway
resource "aws_vpc_security_group_ingress_rule" "redis_from_gateway" {
  security_group_id            = aws_security_group.redis.id
  referenced_security_group_id = aws_security_group.gateway.id
  from_port                    = var.redis_port
  to_port                      = var.redis_port
  ip_protocol                  = "tcp"
  description                  = "Allow Redis from gateway"
}

resource "aws_vpc_security_group_egress_rule" "redis_all_out" {
  security_group_id = aws_security_group.redis.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "Allow all outbound"
}