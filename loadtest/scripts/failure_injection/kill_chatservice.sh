#!/bin/bash

# 故障注入：停止一个ChatService任务

echo "========================================="
echo "  故障注入：停止ChatService任务"
echo "========================================="
echo ""
echo "⚠️  注意：ChatService负责会话管理和成员验证"
echo "   停止后将影响：创建群聊、发送消息验证、查询会话"
echo ""

# 1. 列出所有ChatService任务
echo "正在获取ChatService任务列表..."
TASKS=$(aws ecs list-tasks \
  --cluster chatroom-dev-cluster \
  --service-name chatroom-dev-chatservice \
  --region us-west-2 \
  --query 'taskArns[0]' \
  --output text)

if [ "$TASKS" == "None" ] || [ -z "$TASKS" ]; then
  echo "❌ 没有找到运行中的ChatService任务"
  exit 1
fi

echo "找��任务: $(basename $TASKS)"
echo ""

# 2. 显示任务信息
echo "任务详情："
aws ecs describe-tasks \
  --cluster chatroom-dev-cluster \
  --tasks "$TASKS" \
  --region us-west-2 \
  --query 'tasks[0].[taskArn,lastStatus,healthStatus]' \
  --output table

echo ""
read -p "确认要停止这个ChatService任务吗？(y/N): " confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "已取消"
  exit 0
fi

# 3. 停止任务
echo ""
echo "正在停止ChatService任务..."
aws ecs stop-task \
  --cluster chatroom-dev-cluster \
  --task "$TASKS" \
  --reason "Load testing - ChatService failure injection" \
  --region us-west-2 \
  --query 'task.[taskArn,lastStatus,stoppedReason]' \
  --output table

echo ""
echo "✅ ChatService任务已停止"
echo ""
echo "预期影响："
echo "  ❌ 无法创建���群聊"
echo "  ❌ 无法验证发送者身份"
echo "  ❌ 无法查询会话列表"
echo "  ⚠️  已建立的WebSocket连接不受影响"
echo ""
echo "ECS会自动启动新任务来替换它"
