variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "cluster_name_override" {
  description = "Optional explicit cluster name"
  type        = string
  default     = ""
}

variable "service_connect_namespace_arn" {
  description = "Optional Service Connect namespace ARN"
  type        = string
  default     = null
}