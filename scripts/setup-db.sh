#!/bin/bash

# Setup database through ECS task
# This script runs migration SQL through a running ECS task

set -e

REGION="us-west-2"
CLUSTER="chatroom-dev-cluster"
DB_HOST="chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com"
DB_USER="admin"
DB_PASSWORD="change-me-in-real-usage"
DB_NAME="chatroom"

echo "========================================="
echo "Database Setup via ECS Task"
echo "========================================="
echo ""

# Get running gateway task
echo "📦 Finding running gateway task..."
TASK_ARN=$(aws ecs list-tasks \
  --cluster "$CLUSTER" \
  --service-name chatroom-dev-gateway \
  --desired-status RUNNING \
  --region "$REGION" \
  --query 'taskArns[0]' \
  --output text)

if [ -z "$TASK_ARN" ] || [ "$TASK_ARN" == "None" ]; then
  echo "❌ Error: No running gateway tasks found"
  exit 1
fi

TASK_ID=$(basename "$TASK_ARN")
echo "✅ Found task: $TASK_ID"
echo ""

# Create combined SQL with proper formatting
echo "📦 Preparing migration SQL..."
cat > /tmp/db-setup.sql << 'EOSQL'
CREATE DATABASE IF NOT EXISTS chatroom;
USE chatroom;

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

echo "✅ SQL prepared at /tmp/db-setup.sql"
echo ""

echo "📦 Installing mysql client in container (if not already installed)..."
aws ecs execute-command \
  --cluster "$CLUSTER" \
  --task "$TASK_ID" \
  --container gateway \
  --region "$REGION" \
  --interactive \
  --command "sh -c 'which mysql > /dev/null 2>&1 || (apk add --no-cache mysql-client && echo \"mysql client installed\")'" \
  2>/dev/null || echo "Note: mysql client may already be installed or not available via apk"

echo ""
echo "========================================="
echo "Ready to run migrations!"
echo "========================================="
echo ""
echo "Option 1: Run automatically (requires Session Manager plugin)"
echo ""
echo "Install plugin from:"
echo "https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html"
echo ""
echo "Then run this command:"
echo ""
echo "cat /tmp/db-setup.sql | aws ecs execute-command \\"
echo "  --cluster $CLUSTER \\"
echo "  --task $TASK_ID \\"
echo "  --container gateway \\"
echo "  --region $REGION \\"
echo "  --interactive \\"
echo "  --command \"mysql -h $DB_HOST -u $DB_USER -p'$DB_PASSWORD' $DB_NAME\""
echo ""
echo "========================================="
echo ""
echo "Option 2: Run manually (easier)"
echo ""
echo "1. Connect to the container:"
echo ""
echo "aws ecs execute-command \\"
echo "  --cluster $CLUSTER \\"
echo "  --task $TASK_ID \\"
echo "  --container gateway \\"
echo "  --region $REGION \\"
echo "  --interactive \\"
echo "  --command \"/bin/sh\""
echo ""
echo "2. In the container, run:"
echo ""
echo "cat > /tmp/setup.sql << 'EOF'"
cat /tmp/db-setup.sql
echo "EOF"
echo ""
echo "mysql -h $DB_HOST -u $DB_USER -p'$DB_PASSWORD' $DB_NAME < /tmp/setup.sql"
echo ""
echo "========================================="
