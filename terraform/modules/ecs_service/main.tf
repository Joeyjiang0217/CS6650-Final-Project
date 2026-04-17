locals {
  name_prefix = "${var.project_name}-${var.environment}"

  container_environment = [
    for k, v in var.environment_variables : {
      name  = k
      value = v
    }
  ]

  port_mapping = merge(
    {
      containerPort = var.container_port
      hostPort      = var.container_port
      name          = var.port_name
    },
    var.app_protocol != null ? {
      appProtocol = var.app_protocol
    } : {}
  )

  container_definition = merge(
    {
      name      = var.container_name
      image     = var.image
      essential = true

      portMappings = [local.port_mapping]

      environment = local.container_environment

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = var.log_group_name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = var.container_name
        }
      }
    },
    var.command != null ? {
      command = var.command
    } : {},
    var.entrypoint != null ? {
      entryPoint = var.entrypoint
    } : {}
  )
}

resource "aws_ecs_task_definition" "this" {
  family                   = "${local.name_prefix}-${var.service_name}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]

  cpu    = tostring(var.task_cpu)
  memory = tostring(var.task_memory)

  execution_role_arn = var.task_execution_role_arn
  task_role_arn      = var.task_role_arn

  container_definitions = jsonencode([local.container_definition])

  tags = {
    Name = "${local.name_prefix}-${var.service_name}-taskdef"
  }
}

resource "aws_ecs_service" "this" {
  name            = "${local.name_prefix}-${var.service_name}"
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  platform_version        = "LATEST"
  enable_execute_command  = var.enable_execute_command
  health_check_grace_period_seconds = var.enable_load_balancer ? var.health_check_grace_period_seconds : null

  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = var.assign_public_ip
  }

  dynamic "load_balancer" {
    for_each = var.enable_load_balancer && var.target_group_arn != null ? [1] : []

    content {
      target_group_arn = var.target_group_arn
      container_name   = var.container_name
      container_port   = var.container_port
    }
  }

  dynamic "service_connect_configuration" {
    for_each = var.enable_service_connect ? [1] : []

    content {
      enabled   = true
      namespace = var.service_connect_namespace_arn

      dynamic "service" {
        for_each = var.service_connect_server_enabled ? [1] : []

        content {
          port_name      = var.port_name
          discovery_name = coalesce(var.service_connect_discovery_name, var.service_name)

          client_alias {
            dns_name = coalesce(var.service_connect_dns_name, var.service_name)
            port     = var.container_port
          }
        }
      }
    }
  }

  tags = {
    Name = "${local.name_prefix}-${var.service_name}"
  }
}