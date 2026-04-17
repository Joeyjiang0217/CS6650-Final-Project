module "network" {
  source = "../../modules/network"

  project_name              = var.project_name
  environment               = var.environment
  vpc_cidr                  = var.vpc_cidr
  availability_zones        = var.availability_zones
  public_subnet_cidrs       = var.public_subnet_cidrs
  private_app_subnet_cidrs  = var.private_app_subnet_cidrs
  private_data_subnet_cidrs = var.private_data_subnet_cidrs
}

module "security" {
  source = "../../modules/security"

  project_name       = var.project_name
  environment        = var.environment
  vpc_id             = module.network.vpc_id
  gateway_port       = var.gateway_port
  grpc_service_ports = var.grpc_service_ports
  mysql_port         = var.mysql_port
  redis_port         = var.redis_port
}

resource "aws_service_discovery_private_dns_namespace" "service_connect" {
  name        = "${var.project_name}-${var.environment}.local"
  vpc         = module.network.vpc_id
  description = "Private DNS namespace for ${var.project_name}-${var.environment}"
}

module "ecr" {
  source = "../../modules/ecr"

  project_name = var.project_name
  environment  = var.environment

  repository_names = [
    "gateway",
    "chatservice",
    "messagestorage",
    "messagetransmit",
  ]
}

module "logs" {
  source = "../../modules/logs"

  project_name = var.project_name
  environment  = var.environment

  log_group_names = [
    "gateway",
    "chatservice",
    "messagestorage",
    "messagetransmit",
  ]

  retention_in_days = 30
}

module "iam" {
  source = "../../modules/iam"

  project_name = var.project_name
  environment  = var.environment
}

module "ecs_cluster" {
  source = "../../modules/ecs_cluster"

  project_name                  = var.project_name
  environment                   = var.environment
  service_connect_namespace_arn = aws_service_discovery_private_dns_namespace.service_connect.arn
}

module "alb" {
  source = "../../modules/alb"

  project_name          = var.project_name
  environment           = var.environment
  vpc_id                = module.network.vpc_id
  public_subnet_ids     = module.network.public_subnet_ids
  alb_security_group_id = module.security.alb_security_group_id
  gateway_port          = var.gateway_port
  health_check_path     = "/health"
}

module "rds_mysql" {
  source = "../../modules/rds_mysql"

  project_name            = var.project_name
  environment             = var.environment
  private_data_subnet_ids = module.network.private_data_subnet_ids
  rds_security_group_id   = module.security.rds_security_group_id

  db_name           = var.db_name
  username          = var.db_username
  password          = var.db_password
  instance_class    = var.db_instance_class
  allocated_storage = var.db_allocated_storage
  engine_version    = var.db_engine_version

  multi_az            = false
  skip_final_snapshot = true
  publicly_accessible = false
}

module "elasticache_redis" {
  source = "../../modules/elasticache_redis"

  project_name            = var.project_name
  environment             = var.environment
  private_data_subnet_ids = module.network.private_data_subnet_ids
  redis_security_group_id = module.security.redis_security_group_id

  node_type                  = var.redis_node_type
  engine_version             = var.redis_engine_version
  num_cache_clusters         = 1
  multi_az_enabled           = false
  automatic_failover_enabled = false
  at_rest_encryption_enabled = false
  transit_encryption_enabled = false
}

locals {
  ecr_registry = "${var.aws_account_id}.dkr.ecr.${var.aws_region}.amazonaws.com"

  ecr_repository_names = {
    gateway         = "${var.project_name}-${var.environment}-gateway"
    chatservice     = "${var.project_name}-${var.environment}-chatservice"
    messagestorage  = "${var.project_name}-${var.environment}-messagestorage"
    messagetransmit = "${var.project_name}-${var.environment}-messagetransmit"
  }

  ecr_repository_urls = {
    for k, v in local.ecr_repository_names :
    k => "${local.ecr_registry}/${v}"
  }
}

data "aws_ecr_image" "chatservice" {
  count = var.use_image_digest ? 1 : 0

  repository_name = local.ecr_repository_names["chatservice"]
  image_tag       = var.image_tags["chatservice"]
}

data "aws_ecr_image" "messagestorage" {
  count = var.use_image_digest ? 1 : 0

  repository_name = local.ecr_repository_names["messagestorage"]
  image_tag       = var.image_tags["messagestorage"]
}

data "aws_ecr_image" "messagetransmit" {
  count = var.use_image_digest ? 1 : 0

  repository_name = local.ecr_repository_names["messagetransmit"]
  image_tag       = var.image_tags["messagetransmit"]
}

data "aws_ecr_image" "gateway" {
  count = var.use_image_digest ? 1 : 0

  repository_name = local.ecr_repository_names["gateway"]
  image_tag       = var.image_tags["gateway"]
}

locals {
  chatservice_image     = var.use_image_digest ? "${local.ecr_repository_urls["chatservice"]}@${data.aws_ecr_image.chatservice[0].image_digest}" : "${local.ecr_repository_urls["chatservice"]}:${var.image_tags["chatservice"]}"
  messagestorage_image  = var.use_image_digest ? "${local.ecr_repository_urls["messagestorage"]}@${data.aws_ecr_image.messagestorage[0].image_digest}" : "${local.ecr_repository_urls["messagestorage"]}:${var.image_tags["messagestorage"]}"
  messagetransmit_image = var.use_image_digest ? "${local.ecr_repository_urls["messagetransmit"]}@${data.aws_ecr_image.messagetransmit[0].image_digest}" : "${local.ecr_repository_urls["messagetransmit"]}:${var.image_tags["messagetransmit"]}"
  gateway_image         = var.use_image_digest ? "${local.ecr_repository_urls["gateway"]}@${data.aws_ecr_image.gateway[0].image_digest}" : "${local.ecr_repository_urls["gateway"]}:${var.image_tags["gateway"]}"
}

module "chatservice_service" {
  source = "../../modules/ecs_service"

  project_name = var.project_name
  environment  = var.environment
  cluster_arn  = module.ecs_cluster.cluster_arn

  service_name   = "chatservice"
  container_name = "chatservice"
  image          = local.chatservice_image

  task_cpu    = 256
  task_memory = 512

  container_port = 50051
  port_name      = "grpc-chatservice"
  app_protocol   = "grpc"

  desired_count      = 1
  subnet_ids         = module.network.private_app_subnet_ids
  security_group_ids = [module.security.internal_service_security_group_id]
  assign_public_ip   = false

  task_execution_role_arn = module.iam.ecs_task_execution_role_arn
  task_role_arn           = module.iam.ecs_task_role_arn
  log_group_name          = module.logs.log_group_names["chatservice"]
  aws_region              = var.aws_region

  environment_variables = {
    CHATSERVICE_ADDR                      = ":50051"
    CHATSERVICE_DB_HOST                   = module.rds_mysql.db_endpoint
    CHATSERVICE_DB_PORT                   = tostring(module.rds_mysql.db_port)
    CHATSERVICE_DB_USER                   = var.db_username
    CHATSERVICE_DB_PASSWORD               = var.db_password
    CHATSERVICE_DB_NAME                   = var.db_name
    CHATSERVICE_DB_MAX_OPEN_CONNS         = "20"
    CHATSERVICE_DB_MAX_IDLE_CONNS         = "10"
    CHATSERVICE_DB_CONN_MAX_LIFETIME_MIN  = "30"
    CHATSERVICE_DB_CONN_MAX_IDLE_TIME_MIN = "5"
  }

  enable_service_connect         = true
  service_connect_namespace_arn  = aws_service_discovery_private_dns_namespace.service_connect.arn
  service_connect_server_enabled = true
  service_connect_discovery_name = "chatservice"
  service_connect_dns_name       = "chatservice"
}

module "messagestorage_service" {
  source = "../../modules/ecs_service"

  project_name = var.project_name
  environment  = var.environment
  cluster_arn  = module.ecs_cluster.cluster_arn

  service_name   = "messagestorage"
  container_name = "messagestorage"
  image          = local.messagestorage_image

  task_cpu    = 256
  task_memory = 512

  container_port = 50052
  port_name      = "grpc-messagestorage"
  app_protocol   = "grpc"

  desired_count      = 1
  subnet_ids         = module.network.private_app_subnet_ids
  security_group_ids = [module.security.internal_service_security_group_id]
  assign_public_ip   = false

  task_execution_role_arn = module.iam.ecs_task_execution_role_arn
  task_role_arn           = module.iam.ecs_task_role_arn
  log_group_name          = module.logs.log_group_names["messagestorage"]
  aws_region              = var.aws_region

  environment_variables = {
    MESSAGESTORAGE_ADDR                      = ":50052"
    MESSAGESTORAGE_DB_HOST                   = module.rds_mysql.db_endpoint
    MESSAGESTORAGE_DB_PORT                   = tostring(module.rds_mysql.db_port)
    MESSAGESTORAGE_DB_USER                   = var.db_username
    MESSAGESTORAGE_DB_PASSWORD               = var.db_password
    MESSAGESTORAGE_DB_NAME                   = var.db_name
    MESSAGESTORAGE_DB_MAX_OPEN_CONNS         = "20"
    MESSAGESTORAGE_DB_MAX_IDLE_CONNS         = "10"
    MESSAGESTORAGE_DB_CONN_MAX_LIFETIME_MIN  = "30"
    MESSAGESTORAGE_DB_CONN_MAX_IDLE_TIME_MIN = "5"
  }

  enable_service_connect         = true
  service_connect_namespace_arn  = aws_service_discovery_private_dns_namespace.service_connect.arn
  service_connect_server_enabled = true
  service_connect_discovery_name = "messagestorage"
  service_connect_dns_name       = "messagestorage"
}

module "messagetransmit_service" {
  source = "../../modules/ecs_service"

  project_name = var.project_name
  environment  = var.environment
  cluster_arn  = module.ecs_cluster.cluster_arn

  service_name   = "messagetransmit"
  container_name = "messagetransmit"
  image          = local.messagetransmit_image

  task_cpu    = 256
  task_memory = 512

  container_port = 50053
  port_name      = "grpc-messagetransmit"
  app_protocol   = "grpc"

  desired_count      = 1
  subnet_ids         = module.network.private_app_subnet_ids
  security_group_ids = [module.security.internal_service_security_group_id]
  assign_public_ip   = false

  task_execution_role_arn = module.iam.ecs_task_execution_role_arn
  task_role_arn           = module.iam.ecs_task_role_arn
  log_group_name          = module.logs.log_group_names["messagetransmit"]
  aws_region              = var.aws_region

  environment_variables = {
    MESSAGETRANSMIT_ADDR               = ":50053"
    CHATSERVICE_TARGET                 = "chatservice:50051"
    MESSAGESTORAGE_TARGET              = "messagestorage:50052"
    MESSAGETRANSMIT_CLIENT_TIMEOUT_SEC = "3"
  }

  enable_service_connect         = true
  service_connect_namespace_arn  = aws_service_discovery_private_dns_namespace.service_connect.arn
  service_connect_server_enabled = true
  service_connect_discovery_name = "messagetransmit"
  service_connect_dns_name       = "messagetransmit"
}

module "gateway_service" {
  source = "../../modules/ecs_service"

  project_name = var.project_name
  environment  = var.environment
  cluster_arn  = module.ecs_cluster.cluster_arn

  service_name   = "gateway"
  container_name = "gateway"
  image          = local.gateway_image

  task_cpu    = 512
  task_memory = 1024

  container_port = 8080
  port_name      = "http-gateway"
  app_protocol   = "http"

  desired_count      = 1
  subnet_ids         = module.network.private_app_subnet_ids
  security_group_ids = [module.security.gateway_security_group_id]
  assign_public_ip   = false

  task_execution_role_arn = module.iam.ecs_task_execution_role_arn
  task_role_arn           = module.iam.ecs_task_role_arn
  log_group_name          = module.logs.log_group_names["gateway"]
  aws_region              = var.aws_region

  environment_variables = {
    GATEWAY_ADDR               = ":8080"
    CHATSERVICE_TARGET         = "chatservice:50051"
    MESSAGESTORAGE_TARGET      = "messagestorage:50052"
    MESSAGETRANSMIT_TARGET     = "messagetransmit:50053"
    GATEWAY_CLIENT_TIMEOUT_SEC = "3"

    REDIS_ADDR           = "${module.elasticache_redis.primary_endpoint_address}:6379"
    REDIS_PASSWORD       = ""
    REDIS_DB             = "0"
    REDIS_ONLINE_TTL_SEC = "7200"
    GATEWAY_INSTANCE_ID  = "gateway-dev"
  }

  enable_load_balancer              = true
  target_group_arn                  = module.alb.gateway_target_group_arn
  health_check_grace_period_seconds = 60

  enable_service_connect         = true
  service_connect_namespace_arn  = aws_service_discovery_private_dns_namespace.service_connect.arn
  service_connect_server_enabled = false
}