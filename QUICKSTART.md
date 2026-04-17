# 快速开始 - 修复部署问题

## 📋 问题状态

### ✅ 已解决
- **Service Connect 配置**: 已成功启用，服务之间可以通过 DNS 名称通信
- **ECS 服务**: 所有服务已重新创建并正常运行

### ⚠️ 待解决
- **数据库迁移**: 需要运行迁移脚本创建数据库表

## 🚀 立即修复

### 步骤 1: 运行数据库迁移

**方法 A: 使用 Python 脚本（最简单）**

```bash
# 1. 安装依赖（如果需要）
pip install pymysql

# 2. 设置数据库密码（从 Terraform 变量获取）
export DB_PASSWORD='your-rds-password'

# 3. 运行迁移
python3 scripts/run_migrations.py
```

**方法 B: 通过 ECS 任务（如果 RDS 不可公开访问）**

查看详细说明: `docs/deployment-troubleshooting.md`

### 步骤 2: 验证修复

```bash
# 测试 API
curl -X POST "http://chatroom-dev-alb-1901196816.us-west-2.elb.amazonaws.com/api/sessions/group" \
  -H "Content-Type: application/json" \
  -d '{
    "creator_id": "u1",
    "name": "demo-group",
    "member_ids": ["u1", "u2", "u3"]
  }'

# 应该返回成功响应！
```

## 📚 相关文档

- **详细排查过程**: `docs/deployment-troubleshooting.md`
- **Service Connect 修复**: `docs/service-connect-fix.md`
- **数据库迁移**: `migrations/combined-all.sql`

## 🔧 可用脚本

- `scripts/run_migrations.py` - Python 迁移脚本
- `scripts/run-migrations.sh` - Bash 迁移脚本
- `scripts/run-migrations-via-ecs.sh` - ECS 执行脚本
- `scripts/force-redeploy-services.sh` - 强制重新部署服务

## 🎯 关键信息

**数据库连接信息**:
- Host: `chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com`
- Port: `3306`
- Database: `chatroom`
- User: `admin`
- Password: （在 Terraform 变量中）

**ALB 地址**:
```
http://chatroom-dev-alb-1901196816.us-west-2.elb.amazonaws.com
```

**ECS Cluster**:
```
chatroom-dev-cluster
```

## ✨ 下一步

完成数据库迁移后，您的应用应该完全正常工作！

测试以下功能：
1. ✅ 创建群组会话
2. ✅ 发送消息
3. ✅ 获取会话列表
4. ✅ WebSocket 连接

## 🐛 如果还有问题

1. 检查 CloudWatch 日志：
   ```bash
   aws logs tail /ecs/chatroom/dev/gateway --region us-west-2 --follow
   aws logs tail /ecs/chatroom/dev/chatservice --region us-west-2 --follow
   ```

2. 检查服务状态：
   ```bash
   aws ecs describe-services \
     --cluster chatroom-dev-cluster \
     --services chatroom-dev-gateway chatroom-dev-chatservice \
     --region us-west-2
   ```

3. 查看详细文档：`docs/deployment-troubleshooting.md`
