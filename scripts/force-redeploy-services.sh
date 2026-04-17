#!/bin/bash

# Force redeploy all ECS services to apply Service Connect configuration
# This script forces a new deployment without changing the task definition

set -e

REGION="us-west-2"
CLUSTER="chatroom-dev-cluster"

SERVICES=(
  "chatroom-dev-chatservice"
  "chatroom-dev-messagestorage"
  "chatroom-dev-messagetransmit"
  "chatroom-dev-gateway"
)

echo "========================================="
echo "Force Redeploying ECS Services"
echo "Cluster: $CLUSTER"
echo "Region: $REGION"
echo "========================================="
echo ""

for SERVICE in "${SERVICES[@]}"; do
  echo "📦 Force deploying service: $SERVICE"

  aws ecs update-service \
    --cluster "$CLUSTER" \
    --service "$SERVICE" \
    --force-new-deployment \
    --region "$REGION" \
    --no-cli-pager \
    --query 'service.{name:serviceName,status:status,desired:desiredCount,running:runningCount}' \
    --output table

  echo ""
done

echo "========================================="
echo "✅ All services redeployment triggered"
echo "========================================="
echo ""
echo "Monitor deployment progress with:"
echo "  watch -n 5 'aws ecs describe-services --cluster $CLUSTER --services ${SERVICES[@]} --region $REGION --query \"services[].{name:serviceName,desired:desiredCount,running:runningCount,pending:pendingCount}\" --output table'"
echo ""
echo "Or check individual service:"
echo "  aws ecs describe-services --cluster $CLUSTER --service chatroom-dev-gateway --region $REGION --query 'services[0].deployments' --output table"
