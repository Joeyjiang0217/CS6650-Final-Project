# Service Connect 配置问题修复文档

## 问题描述

### 症状
1. API 调用返回错误：`{"error":"failed to create group session"}`
2. ECS 服务的 `serviceConnectConfiguration` 显示为 `null`
3. 服务之间无法通过 DNS 名称（如 `chatservice:50051`）通信

### 根本原因
- Terraform 状态文件显示 Service Connect 已配置
- 但 AWS ECS 实际运行的服务没有 Service Connect 配置
- 这是 Terraform 状态和 AWS 实际状态不同步导致的

## 解决方案

### 方法 1: 强制 Terraform 重新创建服务（推荐）

```bash
cd terraform/environments/dev

terraform apply \
  -replace='module.chatservice_service.aws_ecs_service.this' \
  -replace='module.messagestorage_service.aws_ecs_service.this' \
  -replace='module.messagetransmit_service.aws_ecs_service.this' \
  -replace='module.gateway_service.aws_ecs_service.this' \
  -auto-approve
```

这会：
1. 删除现有的 ECS 服务
2. 使用正确的 Service Connect 配置重新创建服务
3. 同步 Terraform 状态和 AWS 实际状态

### 方法 2: 手动强制重新部署（不推荐）

如果只是想快速重新部署而不重新创建服务：

```bash
./scripts/force-redeploy-services.sh
```

**注意**: 此方法不会修复状态不同步问题。

## 验证修复

### 1. 检查 Service Connect 配置

```bash
aws ecs describe-services \
  --cluster chatroom-dev-cluster \
  --services \
    chatroom-dev-gateway \
    chatroom-dev-chatservice \
    chatroom-dev-messagestorage \
    chatroom-dev-messagetransmit \
  --region us-west-2 \
  --query 'services[].{name:serviceName,serviceConnect:serviceConnectConfiguration.enabled}' \
  --output table
```

应该看到所有服务的 `serviceConnect` 为 `True`，而不是 `null`。

### 2. 检查服务状态

```bash
aws ecs describe-services \
  --cluster chatroom-dev-cluster \
  --services \
    chatroom-dev-gateway \
    chatroom-dev-chatservice \
    chatroom-dev-messagestorage \
    chatroom-dev-messagetransmit \
  --region us-west-2 \
  --query 'services[].{name:serviceName,status:status,desired:desiredCount,running:runningCount}' \
  --output table
```

所有服务应该是 `ACTIVE` 状态，`running` 数量应该等于 `desired` 数量。

### 3. 测试 API

```bash
curl -X POST "http://chatroom-dev-alb-1498760335.us-west-2.elb.amazonaws.com/api/sessions/group" \
  -H "Content-Type: application/json" \
  -d '{
    "creator_id": "u1",
    "name": "demo-group",
    "member_ids": ["u1", "u2", "u3"]
  }'
```

应该返回成功的会话创建响应，而不是错误。

## Service Connect 工作原理

### 配置要点

1. **ECS Cluster 级别**:
   ```hcl
   service_connect_defaults {
     namespace = aws_service_discovery_private_dns_namespace.service_connect.arn
   }
   ```

2. **ECS Service 级别**:
   ```hcl
   service_connect_configuration {
     enabled   = true
     namespace = var.service_connect_namespace_arn

     service {
       port_name      = var.port_name
       discovery_name = var.service_name
       
       client_alias {
         dns_name = var.service_name
         port     = var.container_port
       }
     }
   }
   ```

3. **容器定义**:
   ```hcl
   portMappings = [{
     containerPort = 50051
     name          = "grpc-chatservice"  # 必须与 port_name 匹配
     appProtocol   = "grpc"
   }]
   ```

### DNS 解析

启用 Service Connect 后：
- `chatservice:50051` → 解析到 chatservice 服务的实例
- `messagestorage:50052` → 解析到 messagestorage 服务的实例
- `messagetransmit:50053` → 解析到 messagetransmit 服务的实例

## 预防措施

### 1. 使用 Terraform 管理所有基础设施

避免手动修改 AWS 资源，始终通过 Terraform 进行更改。

### 2. 定期验证状态

定期运行以下命令检查 Terraform 状态是否与实际状态一致：

```bash
terraform plan
```

如果看到 "No changes" 但实际服务有问题，可能需要刷新状态：

```bash
terraform refresh
terraform plan
```

### 3. 监控服务连接

在部署后，始终验证：
- Service Connect 配置已启用
- 服务可以相互通信
- API 端点正常工作

## 相关文件

- Terraform 配置: `terraform/environments/dev/main.tf`
- ECS Service 模块: `terraform/modules/ecs_service/main.tf`
- ECS Cluster 模块: `terraform/modules/ecs_cluster/main.tf`
- 部署脚本: `scripts/force-redeploy-services.sh`

## 时间线

- 2026-04-16: 发现 Service Connect 配置为 null 的问题
- 2026-04-16: 使用 `terraform apply -replace` 强制重新创建服务以修复问题
