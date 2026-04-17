# 部署问题排查和解决方案

## 问题总结

### 初始症状
```bash
curl -X POST "http://chatroom-dev-alb-xxx.us-west-2.elb.amazonaws.com/api/sessions/group" \
  -H "Content-Type: application/json" \
  -d '{"creator_id": "u1", "name": "demo-group", "member_ids": ["u1", "u2", "u3"]}'

# 返回错误
{"error":"failed to create group session"}
```

### 排查过程

#### 1. Service Connect 配置问题 ✅ 已解决

**问题**: ECS 服务的 `serviceConnectConfiguration` 显示为 `null`

**原因**: Terraform 状态和 AWS 实际状态不同步

**解决方案**: 强制重新创建 ECS 服务
```bash
cd terraform/environments/dev
terraform apply \
  -replace='module.chatservice_service.aws_ecs_service.this' \
  -replace='module.messagestorage_service.aws_ecs_service.this' \
  -replace='module.messagetransmit_service.aws_ecs_service.this' \
  -replace='module.gateway_service.aws_ecs_service.this' \
  -auto-approve
```

**验证**: 检查 ECS 任务是否包含 Service Connect 容器
```bash
TASK_ARN=$(aws ecs list-tasks \
  --cluster chatroom-dev-cluster \
  --service-name chatroom-dev-chatservice \
  --region us-west-2 \
  --query 'taskArns[0]' --output text)

aws ecs describe-tasks \
  --cluster chatroom-dev-cluster \
  --tasks $TASK_ARN \
  --region us-west-2 \
  --query 'tasks[0].containers[*].name'

# 应该看到: ["ecs-service-connect-xxxxx", "chatservice"]
```

#### 2. 数据库表缺失问题 ⚠️ 待解决

**问题**: 
```
[ChatService] CreateGroupChatSession repo error: 
insert chat session: Error 1146 (42S02): Table 'chatroom.chat_sessions' doesn't exist
```

**原因**: 数据库迁移未执行，缺少必要的表结构

**解决方案**: 运行数据库迁移

## 解决方案：运行数据库迁移

### 方法 1: 使用 Python 脚本（推荐，如果可以从本地访问 RDS）

```bash
# 1. 安装 pymysql（如果还没有）
pip install pymysql

# 2. 设置数据库密码
export DB_PASSWORD='your-database-password'

# 3. 运行迁移
python3 scripts/run_migrations.py
```

### 方法 2: 使用 mysql 客户端

如果您的本地机器可以访问 RDS（通过 VPN 或 SSH 隧道）：

```bash
# 1. 设置密码
export DB_PASSWORD='your-database-password'

# 2. 运行迁移脚本
./scripts/run-migrations.sh
```

### 方法 3: 通过 ECS 任务执行（如果 RDS 不可公开访问）

```bash
# 1. 获取运行中的任务
TASK_ARN=$(aws ecs list-tasks \
  --cluster chatroom-dev-cluster \
  --service-name chatroom-dev-gateway \
  --region us-west-2 \
  --query 'taskArns[0]' --output text)

TASK_ID=$(basename $TASK_ARN)

# 2. 安装 Session Manager 插件（首次需要）
# https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html

# 3. 连接到 ECS 任务
aws ecs execute-command \
  --cluster chatroom-dev-cluster \
  --task $TASK_ID \
  --container gateway \
  --region us-west-2 \
  --interactive \
  --command "/bin/sh"

# 4. 在容器内执行（如果使用 Alpine）
apk add mysql-client

# 5. 运行迁移
mysql -h chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com \
  -P 3306 \
  -u admin \
  -p'YOUR_PASSWORD' \
  chatroom < /path/to/migrations/combined-all.sql
```

### 方法 4: 使用 AWS Systems Manager Parameter Store（最安全）

```bash
# 1. 将迁移 SQL 上传到 S3
aws s3 cp migrations/combined-all.sql s3://your-bucket/migrations/

# 2. 创建 ECS 任务定义来运行迁移（一次性任务）
# 3. 使用 ECS Run Task 执行迁移
```

## 验证修复

### 1. 检查数据库表

```bash
# 通过 mysql 客户端
mysql -h chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com \
  -u admin -p -e "USE chatroom; SHOW TABLES;"

# 应该看到:
# - users
# - relations
# - chat_sessions
# - chat_session_members
# - messages
```

### 2. 测试 API

```bash
# 测试创建群组会话
curl -X POST "http://chatroom-dev-alb-xxx.us-west-2.elb.amazonaws.com/api/sessions/group" \
  -H "Content-Type: application/json" \
  -d '{
    "creator_id": "u1",
    "name": "demo-group",
    "member_ids": ["u1", "u2", "u3"]
  }'

# 应该返回成功响应，包含 session 和 members 信息
```

### 3. 检查服务日志

```bash
# Gateway 日志
aws logs tail /ecs/chatroom/dev/gateway \
  --region us-west-2 \
  --since 5m \
  --follow

# ChatService 日志
aws logs tail /ecs/chatroom/dev/chatservice \
  --region us-west-2 \
  --since 5m \
  --follow
```

## 数据库迁移文件说明

### 表结构

1. **users** - 用户信息
   - user_id: 用户唯一标识
   - nickname: 昵称
   - avatar_url: 头像 URL

2. **relations** - 用户关系（好友）
   - user_id, friend_id: 好友关系对

3. **chat_sessions** - 聊天会话
   - session_id: 会话唯一标识
   - name: 会话名称
   - type: direct（单聊）或 group（群聊）
   - creator_id: 创建者
   - last_seq: 最后消息序列号

4. **chat_session_members** - 会话成员
   - session_id, user_id: 成员关系
   - role: owner（拥有者）或 member（成员）
   - last_read_seq: 最后已读序列号

5. **messages** - 消息记录
   - message_id: 消息唯一标识
   - session_id: 所属会话
   - session_seq: 会话内序列号
   - sender_id: 发送者
   - content: 消息内容

## 常见问题

### Q1: RDS 密码在哪里？

密码在 Terraform 变量中设置，检查：
- `terraform/environments/dev/terraform.tfvars`
- 或者在部署时通过命令行传递的变量

### Q2: 如何访问私有 RDS？

选项：
1. 使用 VPN 连接到 VPC
2. 创建 SSH 隧道通过跳板机
3. 临时将 RDS 设置为 publicly accessible（不推荐）
4. 使用 ECS Exec 从运行的容器内执行

### Q3: 迁移是否幂等？

是的，所有表创建语句使用 `CREATE TABLE IF NOT EXISTS`，可以安全地重复运行。

### Q4: 如何回滚迁移？

目前没有自动回滚机制。如需回滚，需要手动删除表或恢复数据库快照。

## 预防措施

### 1. 将迁移集成到部署流程

在 Terraform 部署后自动运行迁移：

```bash
# 在 terraform apply 后添加
terraform apply && ./scripts/run-migrations.sh
```

### 2. 使用 Lambda 函数运行迁移

创建一个 Lambda 函数，在 RDS 实例创建后自动运行迁移。

### 3. 使用 RDS Proxy

考虑使用 RDS Proxy 来管理数据库连接，简化访问控制。

### 4. 添加健康检查

在应用启动时检查必要的表是否存在，如果不存在则记录警告或失败。

## 相关文件

- 迁移文件: `migrations/*.sql`
- 合并迁移: `migrations/combined-all.sql`
- Python 脚本: `scripts/run_migrations.py`
- Bash 脚本: `scripts/run-migrations.sh`
- ECS 脚本: `scripts/run-migrations-via-ecs.sh`
- Service Connect 修复: `docs/service-connect-fix.md`

## 联系信息

如有问题，请检查：
- CloudWatch 日志组
- ECS 服务事件
- RDS 事件日志
