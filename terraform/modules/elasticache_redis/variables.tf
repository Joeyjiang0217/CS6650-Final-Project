variable "project_name" {
  type = string
}

variable "environment" {
  type = string
}

variable "private_data_subnet_ids" {
  type = list(string)
}

variable "redis_security_group_id" {
  type = string
}

variable "node_type" {
  type    = string
  default = "cache.t4g.micro"
}

variable "engine_version" {
  type    = string
  default = "7.1"
}

variable "num_cache_clusters" {
  type    = number
  default = 1
}

variable "multi_az_enabled" {
  type    = bool
  default = false
}

variable "automatic_failover_enabled" {
  type    = bool
  default = false
}

variable "at_rest_encryption_enabled" {
  type    = bool
  default = false
}

variable "transit_encryption_enabled" {
  type    = bool
  default = false
}