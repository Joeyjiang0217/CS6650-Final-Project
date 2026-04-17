locals {
  name_prefix = "/ecs/${var.project_name}/${var.environment}"
}

resource "aws_cloudwatch_log_group" "this" {
  for_each = toset(var.log_group_names)

  name              = "${local.name_prefix}/${each.value}"
  retention_in_days = var.retention_in_days

  tags = {
    Name = "${var.project_name}-${var.environment}-${each.value}-logs"
  }
}