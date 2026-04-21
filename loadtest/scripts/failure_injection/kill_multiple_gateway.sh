#!/bin/bash

# 故障注入：停止多个Gateway任务

echo "========================================="
echo "  多重故障注入：停止2个Gateway任务"
echo "========================================="
echo ""

# 1. 列出所有Gateway任务
echo "正在获取Gateway任务列��..."
TASKS=$(aws ecs list-tasks \
  --cluster chatroom-dev-cluster \
  --service-name chatroom-dev-gateway \
  --region us-west-2 \
  --query 'taskArns' \
  --output json)

TASK_COUNT=$(echo "$TASKS" | jq 'length')

echo "找到 $TASK_COUNT 个运���中的Gateway任务"
echo ""

if [ "$TASK_COUNT" -lt 2 ]; then
  echo "⚠️  警告：只有 $TASK_COUNT 个任务，无法停止2个"
  echo "建议先扩展服务到至少3个实例"
  exit 1
fi

# 获取前两个任务
TASK1=$(echo "$TASKS" | jq -r '.[0]')
TASK2=$(echo "$TASKS" | jq -r '.[1]')

echo "将要停止的任务："
echo "  1. $(basename $TASK1)"
echo "  2. $(basename $TASK2)"
echo ""

read -p "确认要停止这2个��务吗？这会造成严重的服务中断！(y/N): " confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "已取消"
  exit 0
fi

# 停止第一个任务
echo ""
echo "正在停止第1个任务..."
aws ecs stop-task \
  --cluster chatroom-dev-cluster \
  --task "$TASK1" \
  --reason "Load testing - multiple failure injection (1/2)" \
  --region us-west-2 \
  --output text > /dev/null

echo "✅ 第1个任务已停止"

# 等待3秒
echo "等待3秒..."
sleep 3

# 停止第二个任务
echo "正在停止第2个任务..."
aws ecs stop-task \
  --cluster chatroom-dev-cluster \
  --task "$TASK2" \
  --reason "Load testing - multiple failure injection (2/2)" \
  --region us-west-2 \
  --output text > /dev/null

echo "✅ 第2个任务已停止"
echo ""
echo "⚠️  严重故障已注入！观察系统恢复时间和影响范围"
echo ""

# 显示当前状态
echo "当前服务状态："
aws ecs describe-services \
  --cluster chatroom-dev-cluster \
  --services chatroom-dev-gateway \
  --region us-west-2 \
  --query 'services[0].[runningCount,desiredCount]' \
  --output table
