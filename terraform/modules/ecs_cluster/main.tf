locals {
  cluster_name = var.cluster_name_override != "" ? var.cluster_name_override : "${var.project_name}-${var.environment}-cluster"
}

resource "aws_ecs_cluster" "this" {
  name = local.cluster_name

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  dynamic "service_connect_defaults" {
    for_each = var.service_connect_namespace_arn != null ? [1] : []

    content {
      namespace = var.service_connect_namespace_arn
    }
  }

  tags = {
    Name = local.cluster_name
  }
}