#!/bin/bash

# 故障注入：关闭一个Gateway任务

echo "========================================="
echo "  故障注入：关闭Gateway任务"
echo "========================================="
echo ""

# 1. 列出所有Gateway任务
echo "正在获取Gateway任务列表..."
TASKS=$(aws ecs list-tasks \
  --cluster chatroom-dev-cluster \
  --service-name chatroom-dev-gateway \
  --region us-west-2 \
  --query 'taskArns[0]' \
  --output text)

if [ "$TASKS" == "None" ] || [ -z "$TASKS" ]; then
  echo "❌ 没有找到运行中的Gateway任务"
  exit 1
fi

echo "找到任务: $TASKS"
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
read -p "确认要停止这个任务吗？(y/N): " confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "已取消"
  exit 0
fi

# 3. 停止任务
echo ""
echo "正在停止任务..."
aws ecs stop-task \
  --cluster chatroom-dev-cluster \
  --task "$TASKS" \
  --reason "Load testing - failure injection experiment" \
  --region us-west-2 \
  --query 'task.[taskArn,lastStatus,stoppedReason]' \
  --output table

echo ""
echo "✅ 任务已停止"
echo ""
echo "ECS会自动启动新任务来替换它"
echo "观察你的实验4脚本输出，查看恢复时间和影响"
