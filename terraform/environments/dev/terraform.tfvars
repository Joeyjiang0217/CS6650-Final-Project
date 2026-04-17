project_name = "chatroom"
environment  = "dev"
aws_region   = "us-west-2"

vpc_cidr = "10.10.0.0/16"

availability_zones = [
  "us-west-2a",
  "us-west-2b",
]

public_subnet_cidrs = [
  "10.10.0.0/24",
  "10.10.1.0/24",
]

private_app_subnet_cidrs = [
  "10.10.10.0/24",
  "10.10.11.0/24",
]

private_data_subnet_cidrs = [
  "10.10.20.0/24",
  "10.10.21.0/24",
]

image_tags = {
  gateway         = "v1"
  chatservice     = "v1"
  messagestorage  = "v1"
  messagetransmit = "v1"
}

use_image_digest = false

db_password = "change-me-in-real-usage"

aws_account_id = "627195602448"