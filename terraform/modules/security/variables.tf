variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "gateway_port" {
  type = number
}

variable "grpc_service_ports" {
  type = list(number)
}

variable "mysql_port" {
  type = number
}

variable "redis_port" {
  type = number
}