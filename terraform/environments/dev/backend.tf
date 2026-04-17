# For now, use local state by default.
# Later, when deploying to AWS seriously, replace this with an S3 backend.
#
# terraform {
#   backend "s3" {
#     bucket         = "your-terraform-state-bucket"
#     key            = "chatroom/dev/terraform.tfstate"
#     region         = "us-west-2"
#     dynamodb_table = "your-terraform-locks"
#     encrypt        = true
#   }
# }