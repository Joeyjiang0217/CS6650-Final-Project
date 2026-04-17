variable "project_name" {
  description = "Project name prefix"
  type        = string
  default     = "chatroom"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
}

variable "availability_zones" {
  description = "Two AZs for this environment"
  type        = list(string)
}

variable "public_subnet_cidrs" {
  description = "CIDRs for public subnets"
  type        = list(string)
}

variable "private_app_subnet_cidrs" {
  description = "CIDRs for private app subnets"
  type        = list(string)
}

variable "private_data_subnet_cidrs" {
  description = "CIDRs for private data subnets"
  type        = list(string)
}

variable "gateway_port" {
  description = "Gateway HTTP port"
  type        = number
  default     = 8080
}

variable "grpc_service_ports" {
  description = "Ports for internal gRPC services"
  type        = list(number)
  default     = [50051, 50052, 50053]
}

variable "mysql_port" {
  description = "MySQL port"
  type        = number
  default     = 3306
}

variable "redis_port" {
  description = "Redis port"
  type        = number
  default     = 6379
}

variable "image_tags" {
  description = "Image tags for each service"
  type        = map(string)

  default = {
    gateway         = "v1"
    chatservice     = "v1"
    messagestorage  = "v1"
    messagetransmit = "v1"
  }
}

variable "use_image_digest" {
  description = "If true, resolve each image tag to an ECR digest and use repo@digest instead of repo:tag"
  type        = bool
  default     = false
}

variable "db_name" {
  type    = string
  default = "chatroom"
}

variable "db_username" {
  type    = string
  default = "admin"
}

variable "db_password" {
  type      = string
  sensitive = true
}

variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "db_allocated_storage" {
  type    = number
  default = 20
}

variable "db_engine_version" {
  type    = string
  default = "8.0"
}

variable "redis_node_type" {
  type    = string
  default = "cache.t4g.micro"
}

variable "redis_engine_version" {
  type    = string
  default = "7.1"
}

variable "aws_account_id" {
  description = "AWS account ID used to construct ECR image URLs"
  type        = string
}