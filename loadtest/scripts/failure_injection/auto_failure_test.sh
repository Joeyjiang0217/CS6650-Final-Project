#!/bin/bash

# 自动化故障注入测试
# 在后台运行测试，自动在指定时间注入故障

set -e

LOADTEST_DIR="/Users/huiluo112/Documents/CS6650/homework/final project/CS6650-Final-Project-fresh/loadtest"
cd "$LOADTEST_DIR"

echo "========================================="
echo "  自动化故障注入测试"
echo "========================================="
echo ""
echo "可选的测试场景："
echo "  1. ChatService故障"
echo "  2. MessageStorage故障"
echo "  3. 多个Gateway故障"
echo ""
read -p "选择测试场景 (1/2/3): " choice

case $choice in
  1)
    TEST_NAME="ChatService"
    KILL_SCRIPT="./kill_chatservice.sh"
    USERS=100
    DURATION=300
    ;;
  2)
    TEST_NAME="MessageStorage"
    KILL_SCRIPT="./kill_messagestorage.sh"
    USERS=100
    DURATION=300
    ;;
  3)
    TEST_NAME="Multiple Gateway"
    KILL_SCRIPT="./kill_multiple_gateway.sh"
    USERS=200
    DURATION=360
    ;;
  *)
    echo "无效选择"
    exit 1
    ;;
esac

echo ""
echo "将要运行："
echo "  测试: $TEST_NAME 故障注入"
echo "  用户数: $USERS"
echo "  持续时间: $DURATION 秒"
echo "  故障注入时间: 测试开始后60秒"
echo ""
read -p "确认开始？(y/N): " confirm

if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "已取消"
  exit 0
fi

echo ""
echo "========================================="
echo "  阶段1: 启动负载测试"
echo "========================================="

# 在后台启动测试
python3 experiment4_failure.py --users $USERS --duration $DURATION > experiment4_output.log 2>&1 &
TEST_PID=$!

echo "✅ 测试已启动 (PID: $TEST_PID)"
echo "📝 输出保存到: experiment4_output.log"
echo ""

# 倒计时60秒
echo "========================================="
echo "  等待60秒后注入故障..."
echo "========================================="
for i in {60..1}; do
  if [ $((i % 10)) -eq 0 ] || [ $i -le 5 ]; then
    echo "  倒计时: ${i}秒..."
  fi
  sleep 1
done

echo ""
echo "========================================="
echo "  阶段2: 注入故障"
echo "========================================="

# 注入故障（自动确认）
if [ "$choice" == "3" ]; then
  # 多Gateway需要特殊处理
  echo "正在停止2个Gateway任务..."

  TASKS=$(aws ecs list-tasks \
    --cluster chatroom-dev-cluster \
    --service-name chatroom-dev-gateway \
    --region us-west-2 \
    --query 'taskArns' \
    --output json)

  TASK1=$(echo "$TASKS" | jq -r '.[0]')
  TASK2=$(echo "$TASKS" | jq -r '.[1]')

  aws ecs stop-task \
    --cluster chatroom-dev-cluster \
    --task "$TASK1" \
    --reason "Auto failure injection test (1/2)" \
    --region us-west-2 \
    --output text > /dev/null

  echo "✅ 第1个任务已停止"
  sleep 3

  aws ecs stop-task \
    --cluster chatroom-dev-cluster \
    --task "$TASK2" \
    --reason "Auto failure injection test (2/2)" \
    --region us-west-2 \
    --output text > /dev/null

  echo "✅ 第2个任务已停止"

elif [ "$choice" == "1" ]; then
  # ChatService
  TASK=$(aws ecs list-tasks \
    --cluster chatroom-dev-cluster \
    --service-name chatroom-dev-chatservice \
    --region us-west-2 \
    --query 'taskArns[0]' \
    --output text)

  aws ecs stop-task \
    --cluster chatroom-dev-cluster \
    --task "$TASK" \
    --reason "Auto failure injection test - ChatService" \
    --region us-west-2 \
    --output text > /dev/null

  echo "✅ ChatService任务已停止"

else
  # MessageStorage
  TASK=$(aws ecs list-tasks \
    --cluster chatroom-dev-cluster \
    --service-name chatroom-dev-messagestorage \
    --region us-west-2 \
    --query 'taskArns[0]' \
    --output text)

  aws ecs stop-task \
    --cluster chatroom-dev-cluster \
    --task "$TASK" \
    --reason "Auto failure injection test - MessageStorage" \
    --region us-west-2 \
    --output text > /dev/null

  echo "✅ MessageStorage任务已停止"
fi

echo ""
echo "🔥 故障已注入！观察系统恢复..."
echo ""

# 等待测试完成
echo "========================================="
echo "  阶段3: 等待测试完成"
echo "========================================="
echo "测试还将运行 $((DURATION - 60)) 秒..."
echo "可以实时查看日志: tail -f experiment4_output.log"
echo ""

# 等待测试进程结束
wait $TEST_PID
EXIT_CODE=$?

echo ""
echo "========================================="
echo "  测试完成！"
echo "========================================="

if [ $EXIT_CODE -eq 0 ]; then
  echo "✅ 测试成功完成"
else
  echo "⚠️  测试退出代码: $EXIT_CODE"
fi

echo ""
echo "📊 查看结果："
echo "  - 完整输出: cat experiment4_output.log"
echo "  - 结果文件: ls -lh experiment4_results_*.json"
echo ""

# 显示服务当前状态
echo "当前服务状态："
aws ecs describe-services \
  --cluster chatroom-dev-cluster \
  --services chatroom-dev-gateway chatroom-dev-chatservice chatroom-dev-messagestorage \
  --region us-west-2 \
  --query 'services[*].[serviceName,runningCount,desiredCount]' \
  --output table

echo ""
echo "========================================="
echo "测试已完成！结果已保存。"
echo "========================================="
