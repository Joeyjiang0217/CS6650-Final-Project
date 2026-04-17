# 🚀 立即运行数据库迁移

## ⚡ 最快速的方法（推荐）

### 方法 1: 使用 Go 迁移工具 + 临时开启公共访问

这是最快的方法，但需要临时修改 RDS 安全组。

```bash
# 步骤 1: 临时允许你的 IP 访问 RDS
# 获取你的公网 IP
MY_IP=$(curl -s https://checkip.amazonaws.com)
echo "Your IP: $MY_IP"

# 添加入站规则到 RDS 安全组
aws ec2 authorize-security-group-ingress \
  --group-id sg-008a9a084f1dbb42e \
  --protocol tcp \
  --port 3306 \
  --cidr $MY_IP/32 \
  --region us-west-2

# 步骤 2: 运行迁移
export DB_PASSWORD='change-me-in-real-usage'
go run cmd/migrate/main.go

# 步骤 3: 移除临时规则（重要！）
aws ec2 revoke-security-group-ingress \
  --group-id sg-008a9a084f1dbb42e \
  --protocol tcp \
  --port 3306 \
  --cidr $MY_IP/32 \
  --region us-west-2
```

### 方法 2: 通过 ECS Exec（最安全）

需要先安装 AWS Session Manager plugin。

#### 2.1 安装 Session Manager Plugin

**macOS:**
```bash
brew install --cask session-manager-plugin
```

**或者手动安装:**
```bash
curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/mac_arm64/sessionmanager-bundle.zip" -o "sessionmanager-bundle.zip"
unzip sessionmanager-bundle.zip
sudo ./sessionmanager-bundle/install -i /usr/local/sessionmanagerplugin -b /usr/local/bin/session-manager-plugin
```

验证安装:
```bash
session-manager-plugin
# 应该看到 "The Session Manager plugin is installed successfully"
```

#### 2.2 连接到 ECS 容器并运行迁移

```bash
# 运行脚本
./scripts/run-migrations-simple.sh
```

或者手动执行:

```bash
# 1. 获取 chatservice 任务 ID
TASK_ARN=$(aws ecs list-tasks \
  --cluster chatroom-dev-cluster \
  --service-name chatroom-dev-chatservice \
  --desired-status RUNNING \
  --region us-west-2 \
  --query 'taskArns[0]' \
  --output text)

TASK_ID=$(basename $TASK_ARN)
echo "Task ID: $TASK_ID"

# 2. 连接到容器
aws ecs execute-command \
  --cluster chatroom-dev-cluster \
  --task $TASK_ID \
  --container chatservice \
  --region us-west-2 \
  --interactive \
  --command "/bin/sh"

# 3. 在容器内运行（复制粘贴以下所有内容）:
apk add --no-cache mysql-client

mysql -h chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com \
  -P 3306 \
  -u admin \
  -p'change-me-in-real-usage' \
  chatroom << 'EOSQL'

CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id VARCHAR(64) NOT NULL,
  nickname VARCHAR(128) NOT NULL,
  avatar_url VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_user_id (user_id),
  KEY idx_users_nickname (nickname)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS relations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id VARCHAR(64) NOT NULL,
  friend_id VARCHAR(64) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_relations_pair (user_id, friend_id),
  KEY idx_relations_user_id (user_id),
  KEY idx_relations_friend_id (friend_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS chat_sessions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  session_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  type ENUM('direct', 'group') NOT NULL,
  creator_id VARCHAR(64) NOT NULL,
  last_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  last_message_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_chat_sessions_session_id (session_id),
  KEY idx_chat_sessions_creator_id (creator_id),
  KEY idx_chat_sessions_last_message_at (last_message_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS chat_session_members (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  session_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  role ENUM('owner', 'member') NOT NULL DEFAULT 'member',
  joined_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_read_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uq_chat_session_members_pair (session_id, user_id),
  KEY idx_chat_session_members_session_id (session_id),
  KEY idx_chat_session_members_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS messages (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  message_id VARCHAR(64) NOT NULL,
  session_id VARCHAR(64) NOT NULL,
  session_seq BIGINT UNSIGNED NOT NULL,
  sender_id VARCHAR(64) NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_messages_message_id (message_id),
  UNIQUE KEY uq_messages_session_seq (session_id, session_seq),
  KEY idx_messages_session_created_at (session_id, created_at),
  KEY idx_messages_session_seq (session_id, session_seq)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

EOSQL

# 看到成功消息后，输入 exit 退出容器
exit
```

## ✅ 验证迁移成功

运行迁移后，测试 API:

```bash
curl -X POST "http://chatroom-dev-alb-1901196816.us-west-2.elb.amazonaws.com/api/sessions/group" \
  -H "Content-Type: application/json" \
  -d '{
    "creator_id": "u1",
    "name": "demo-group",
    "member_ids": ["u1", "u2", "u3"]
  }'
```

✅ 如果返回包含 `"session"` 和 `"members"` 的 JSON，说明成功了！

❌ 如果还是返回错误，检查日志:
```bash
aws logs tail /ecs/chatroom/dev/chatservice --region us-west-2 --since 1m --follow
```

## 📊 检查创建的表

在容器内或通过 mysql 客户端:
```bash
mysql -h chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com \
  -u admin -p'change-me-in-real-usage' \
  -e "USE chatroom; SHOW TABLES;"
```

应该看到:
```
+--------------------+
| Tables_in_chatroom |
+--------------------+
| chat_session_members|
| chat_sessions      |
| messages           |
| relations          |
| users              |
+--------------------+
```

## 🎉 完成！

迁移完成后，你的聊天室应用应该完全正常工作了！

测试所有功能：
- ✅ 创建群组会话
- ✅ 发送消息
- ✅ 获取会话列表
- ✅ WebSocket 实时通信
