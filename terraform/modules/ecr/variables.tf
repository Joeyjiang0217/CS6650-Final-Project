variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "repository_names" {
  description = "Repository suffix names, for example: [gateway, chatservice]"
  type        = list(string)
}

variable "image_tag_mutability" {
  description = "MUTABLE or IMMUTABLE"
  type        = string
  default     = "MUTABLE"
}

variable "scan_on_push" {
  description = "Whether to enable ECR image scanning on push"
  type        = bool
  default     = true
}