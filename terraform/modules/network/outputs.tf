output "vpc_id" {
  value = aws_vpc.this.id
}

output "vpc_cidr_block" {
  value = aws_vpc.this.cidr_block
}

output "internet_gateway_id" {
  value = aws_internet_gateway.this.id
}

output "public_subnet_ids" {
  value = [for s in aws_subnet.public : s.id]
}

output "private_app_subnet_ids" {
  value = [for s in aws_subnet.private_app : s.id]
}

output "private_data_subnet_ids" {
  value = [for s in aws_subnet.private_data : s.id]
}

output "public_route_table_id" {
  value = aws_route_table.public.id
}

output "private_app_route_table_ids" {
  value = [for rt in aws_route_table.private_app : rt.id]
}

output "private_data_route_table_ids" {
  value = [for rt in aws_route_table.private_data : rt.id]
}