#!/bin/bash

# Simple migration runner via chatservice container
set -e

REGION="us-west-2"
CLUSTER="chatroom-dev-cluster"
DB_HOST="chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com"
DB_PORT="3306"
DB_USER="admin"
DB_PASSWORD="change-me-in-real-usage"
DB_NAME="chatroom"

echo "========================================="
echo "Running Database Migrations"
echo "========================================="
echo ""

# Get chatservice task (it already has DB access)
echo "📦 Finding chatservice task..."
TASK_ARN=$(aws ecs list-tasks \
  --cluster "$CLUSTER" \
  --service-name chatroom-dev-chatservice \
  --desired-status RUNNING \
  --region "$REGION" \
  --query 'taskArns[0]' \
  --output text)

if [ -z "$TASK_ARN" ] || [ "$TASK_ARN" == "None" ]; then
  echo "❌ No running chatservice tasks found"
  exit 1
fi

TASK_ID=$(basename "$TASK_ARN")
echo "✅ Found task: $TASK_ID"
echo ""

# Create SQL script
SQL_COMMANDS=$(cat << 'EOSQL'
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
)

echo "📦 Attempting to run migrations..."
echo ""
echo "This requires AWS Session Manager plugin to be installed."
echo "Install from: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html"
echo ""

# Try to run with ECS Exec
echo "$SQL_COMMANDS" | aws ecs execute-command \
  --cluster "$CLUSTER" \
  --task "$TASK_ID" \
  --container chatservice \
  --region "$REGION" \
  --interactive \
  --command "sh -c 'mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p\"$DB_PASSWORD\" $DB_NAME'" \
  2>&1 && {
    echo ""
    echo "✅ Migration completed!"
    exit 0
  } || {
    echo ""
    echo "⚠️  Automatic execution failed (Session Manager plugin may not be installed)"
    echo ""
    echo "========================================="
    echo "Manual Steps:"
    echo "========================================="
    echo ""
    echo "1. Install Session Manager plugin:"
    echo "   https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html"
    echo ""
    echo "2. Connect to container:"
    echo ""
    echo "   aws ecs execute-command \\"
    echo "     --cluster $CLUSTER \\"
    echo "     --task $TASK_ID \\"
    echo "     --container chatservice \\"
    echo "     --region $REGION \\"
    echo "     --interactive \\"
    echo "     --command \"/bin/sh\""
    echo ""
    echo "3. In the container shell, run:"
    echo ""
    echo "   # Install mysql client if needed"
    echo "   apk add --no-cache mysql-client"
    echo ""
    echo "   # Run migration"
    echo "   mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p'$DB_PASSWORD' $DB_NAME << 'EOSQL'"
    echo "$SQL_COMMANDS"
    echo "EOSQL"
    echo ""
  }
