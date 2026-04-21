#!/bin/bash

# 快速检查系统是否准备好进行故障测试

echo "========================================="
echo "  系统状态检查"
echo "========================================="
echo ""

# 1. 检查ECS服务状态
echo "1️⃣  ECS服务状态："
aws ecs describe-services \
  --cluster chatroom-dev-cluster \
  --services chatroom-dev-gateway chatroom-dev-chatservice chatroom-dev-messagestorage chatroom-dev-messagetransmit \
  --region us-west-2 \
  --query 'services[*].[serviceName,runningCount,desiredCount,deployments[0].rolloutState]' \
  --output table

echo ""

# 2. 检查ALB健康状态
echo "2️⃣  ALB健康检查："
ALB_ARN=$(aws elbv2 describe-load-balancers \
  --region us-west-2 \
  --query 'LoadBalancers[?contains(LoadBalancerName, `chatroom-dev`)].LoadBalancerArn' \
  --output text)

TG_ARN=$(aws elbv2 describe-target-groups \
  --load-balancer-arn "$ALB_ARN" \
  --region us-west-2 \
  --query 'TargetGroups[0].TargetGroupArn' \
  --output text)

aws elbv2 describe-target-health \
  --target-group-arn "$TG_ARN" \
  --region us-west-2 \
  --query 'TargetHealthDescriptions[*].[Target.Id,TargetHealth.State]' \
  --output table

echo ""

# 3. 测试HTTP连接
echo "3️⃣  HTTP健康测试："
ALB_DNS=$(aws elbv2 describe-load-balancers \
  --region us-west-2 \
  --query 'LoadBalancers[?contains(LoadBalancerName, `chatroom-dev`)].DNSName' \
  --output text)

HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://$ALB_DNS/health")

if [ "$HTTP_STATUS" == "200" ]; then
  echo "✅ HTTP健康检查通过 (200 OK)"
else
  echo "❌ HTTP健康检查失败 (HTTP $HTTP_STATUS)"
fi

echo ""

# 4. 检查最近的错误
echo "4️⃣  最近的错误日志（最近5分钟）："
ERROR_COUNT=$(aws logs filter-log-events \
  --log-group-name /ecs/chatroom/dev/gateway \
  --filter-pattern "ERROR" \
  --start-time $(($(date +%s) - 300))000 \
  --region us-west-2 \
  --query 'events' \
  --output json | jq 'length')

echo "Gateway错误数: $ERROR_COUNT"

echo ""
echo "========================================="
echo "  系统状态总结"
echo "========================================="

# 判断是否可以开始测试
READY=true

# 检查Gateway实例数
GW_COUNT=$(aws ecs describe-services \
  --cluster chatroom-dev-cluster \
  --services chatroom-dev-gateway \
  --region us-west-2 \
  --query 'services[0].runningCount' \
  --output text)

if [ "$GW_COUNT" -lt 1 ]; then
  echo "❌ Gateway实例数不足 ($GW_COUNT < 1)"
  READY=false
else
  echo "✅ Gateway实例数: $GW_COUNT"
fi

if [ "$HTTP_STATUS" != "200" ]; then
  echo "❌ HTTP健康检查失败"
  READY=false
else
  echo "✅ HTTP健康检查通过"
fi

echo ""

if [ "$READY" = true ]; then
  echo "🎉 系统准备就绪，可以开始故障测试！"
  echo ""
  echo "运行自动化测试："
  echo "  ./auto_failure_test.sh"
  echo ""
  echo "或手动测试："
  echo "  python3 experiment4_failure.py --users 100 --duration 300"
else
  echo "⚠️  系统未准备好，请等待服务稳定后再测试"
fi

echo "========================================="
