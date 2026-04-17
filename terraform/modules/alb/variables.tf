variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "public_subnet_ids" {
  type = list(string)
}

variable "alb_security_group_id" {
  type = string
}

variable "gateway_port" {
  type = number
}

variable "health_check_path" {
  type    = string
  default = "/health"
}

variable "health_check_matcher" {
  type    = string
  default = "200"
}

variable "internal" {
  type    = bool
  default = false
}