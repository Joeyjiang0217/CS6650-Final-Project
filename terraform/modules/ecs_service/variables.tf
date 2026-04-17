variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "cluster_arn" {
  type = string
}

variable "service_name" {
  type = string
}

variable "container_name" {
  type = string
}

variable "image" {
  type = string
}

variable "task_cpu" {
  type = number
}

variable "task_memory" {
  type = number
}

variable "container_port" {
  type = number
}

variable "port_name" {
  type = string
}

variable "app_protocol" {
  type    = string
  default = null
}

variable "desired_count" {
  type    = number
  default = 1
}

variable "subnet_ids" {
  type = list(string)
}

variable "security_group_ids" {
  type = list(string)
}

variable "assign_public_ip" {
  type    = bool
  default = false
}

variable "task_execution_role_arn" {
  type = string
}

variable "task_role_arn" {
  type = string
}

variable "log_group_name" {
  type = string
}

variable "aws_region" {
  type = string
}

variable "environment_variables" {
  type    = map(string)
  default = {}
}

variable "command" {
  type    = list(string)
  default = null
}

variable "entrypoint" {
  type    = list(string)
  default = null
}

variable "enable_execute_command" {
  type    = bool
  default = true
}

variable "enable_load_balancer" {
  type    = bool
  default = false
}

variable "target_group_arn" {
  type    = string
  default = null
}

variable "health_check_grace_period_seconds" {
  type    = number
  default = 60
}

variable "enable_service_connect" {
  type    = bool
  default = false
}

variable "service_connect_namespace_arn" {
  type    = string
  default = null
}

variable "service_connect_server_enabled" {
  type    = bool
  default = false
}

variable "service_connect_discovery_name" {
  type    = string
  default = null
}

variable "service_connect_dns_name" {
  type    = string
  default = null
}