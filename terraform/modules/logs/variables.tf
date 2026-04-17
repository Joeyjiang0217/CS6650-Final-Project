variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "log_group_names" {
  description = "Logical service names, for example: [gateway, chatservice]"
  type        = list(string)
}

variable "retention_in_days" {
  description = "CloudWatch log retention"
  type        = number
  default     = 30
}