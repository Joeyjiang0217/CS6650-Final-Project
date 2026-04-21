#!/bin/bash

# 故障注入：停止一个MessageStorage任务

echo "========================================="
echo "  故障注入：停止MessageStorage任务"
echo "========================================="
echo ""
echo "⚠️  注意：MessageStorage负责消息存储和查询"
echo "   停止后将影响：消息存储、��史查询、未读消息"
echo ""

# 1. 列出所有MessageStorage任务
echo "正在获取MessageStorage任务列表..."
TASKS=$(aws ecs list-tasks \
  --cluster chatroom-dev-cluster \
  --service-name chatroom-dev-messagestorage \
  --region us-west-2 \
  --query 'taskArns[0]' \
  --output text)

if [ "$TASKS" == "None" ] || [ -z "$TASKS" ]; then
  echo "❌ 没有找到运行中的MessageStorage任务"
  exit 1
fi

echo "找到任务: $(basename $TASKS)"
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
read -p "确认要停止这个MessageStorage任务吗？(y/N): " confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "已取消"
  exit 0
fi

# 3. 停止任务
echo ""
echo "正在停止MessageStorage任务..."
aws ecs stop-task \
  --cluster chatroom-dev-cluster \
  --task "$TASKS" \
  --reason "Load testing - MessageStorage failure injection" \
  --region us-west-2 \
  --query 'task.[taskArn,lastStatus,stoppedReason]' \
  --output table

echo ""
echo "✅ MessageStorage任务已停止"
echo ""
echo "预期影响："
echo "  ❌ 无法存储新消息"
echo "  ❌ 无法查询历史消息"
echo "  ❌ 无法查询未读消息"
echo "  ⚠️  WebSocket推送可能还能工作（内存中）"
echo "  ⚠️  MessageTransmit会调用失败"
echo ""
echo "ECS会自动启动新任务来替换它"
