#!/bin/bash

# Run database migrations via ECS Exec
# This script executes migrations from within an ECS task that has VPC access

set -e

REGION="us-west-2"
CLUSTER="chatroom-dev-cluster"
SERVICE="chatroom-dev-gateway"
DB_HOST="chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com"
DB_PORT="3306"
DB_NAME="chatroom"
DB_USER="admin"

echo "========================================="
echo "Database Migration via ECS Exec"
echo "========================================="
echo ""

# Check if password is provided
if [ -z "$DB_PASSWORD" ]; then
  echo "❌ Error: DB_PASSWORD environment variable is required"
  echo ""
  echo "Usage:"
  echo "  export DB_PASSWORD='your-password'"
  echo "  ./scripts/run-migrations-via-ecs.sh"
  exit 1
fi

# Get running task ARN
echo "📦 Finding running task..."
TASK_ARN=$(aws ecs list-tasks \
  --cluster "$CLUSTER" \
  --service-name "$SERVICE" \
  --desired-status RUNNING \
  --region "$REGION" \
  --query 'taskArns[0]' \
  --output text)

if [ -z "$TASK_ARN" ] || [ "$TASK_ARN" == "None" ]; then
  echo "❌ Error: No running tasks found for service $SERVICE"
  exit 1
fi

TASK_ID=$(basename "$TASK_ARN")
echo "✅ Found task: $TASK_ID"
echo ""

# Function to execute SQL via ECS Exec
execute_migration() {
  local migration_file=$1
  local migration_name=$(basename "$migration_file")

  echo "▶️  Applying migration: $migration_name"

  # Read SQL file and execute via mysql in the container
  # Note: The container must have mysql-client installed
  aws ecs execute-command \
    --cluster "$CLUSTER" \
    --task "$TASK_ID" \
    --container "gateway" \
    --region "$REGION" \
    --interactive \
    --command "/bin/sh -c \"echo '$(cat $migration_file | sed "s/'/'\\\\''/g")' | mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p'$DB_PASSWORD' $DB_NAME\""

  if [ $? -eq 0 ]; then
    echo "   ✅ Success"
  else
    echo "   ❌ Failed"
    return 1
  fi
  echo ""
}

# Note: This approach won't work if mysql client is not in the container
echo "⚠️  Note: This script requires mysql client to be installed in the container"
echo ""
echo "Alternative approach: Create migration SQL and execute manually"
echo ""

# Create combined migration SQL file
echo "📦 Creating combined migration SQL file..."
cat migrations/*.sql > /tmp/combined-migrations.sql

echo "✅ Combined migrations saved to: /tmp/combined-migrations.sql"
echo ""
echo "To apply migrations, run:"
echo ""
echo "1. Install Session Manager plugin:"
echo "   https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html"
echo ""
echo "2. Connect to ECS task:"
echo "   aws ecs execute-command \\"
echo "     --cluster $CLUSTER \\"
echo "     --task $TASK_ID \\"
echo "     --container gateway \\"
echo "     --region $REGION \\"
echo "     --interactive \\"
echo "     --command \"/bin/sh\""
echo ""
echo "3. Inside the container, run:"
echo "   apk add mysql-client  # If using Alpine"
echo "   cat > /tmp/migrations.sql << 'EOF'"
echo "   $(cat /tmp/combined-migrations.sql)"
echo "   EOF"
echo "   mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p'$DB_PASSWORD' $DB_NAME < /tmp/migrations.sql"
echo ""
