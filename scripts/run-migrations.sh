#!/bin/bash

# Run database migrations on RDS
# This script connects to RDS through a bastion ECS task

set -e

REGION="us-west-2"
DB_HOST="chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com"
DB_PORT="3306"
DB_NAME="chatroom"
DB_USER="admin"
MIGRATIONS_DIR="migrations"

echo "========================================="
echo "Database Migration Script"
echo "========================================="
echo ""
echo "Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"
echo "Migrations directory: ${MIGRATIONS_DIR}"
echo ""

# Check if password is provided
if [ -z "$DB_PASSWORD" ]; then
  echo "❌ Error: DB_PASSWORD environment variable is required"
  echo ""
  echo "Usage:"
  echo "  export DB_PASSWORD='your-password'"
  echo "  ./scripts/run-migrations.sh"
  exit 1
fi

# Check if mysql client is available
if ! command -v mysql &> /dev/null; then
  echo "❌ Error: mysql client is not installed"
  echo ""
  echo "Install on macOS:"
  echo "  brew install mysql-client"
  echo "  export PATH=\"/opt/homebrew/opt/mysql-client/bin:\$PATH\""
  exit 1
fi

# Run migrations
echo "📦 Running migrations..."
echo ""

for migration_file in $(ls ${MIGRATIONS_DIR}/*.sql | sort); do
  echo "▶️  Applying migration: $(basename $migration_file)"

  mysql \
    --host="${DB_HOST}" \
    --port="${DB_PORT}" \
    --user="${DB_USER}" \
    --password="${DB_PASSWORD}" \
    --database="${DB_NAME}" \
    < "$migration_file"

  if [ $? -eq 0 ]; then
    echo "   ✅ Success"
  else
    echo "   ❌ Failed"
    exit 1
  fi
  echo ""
done

echo "========================================="
echo "✅ All migrations completed successfully"
echo "========================================="
