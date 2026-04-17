#!/usr/bin/env python3
"""
Database migration script for chatroom application.
Connects to RDS and runs all migration SQL files.
"""

import os
import sys
import glob
import pymysql
from pathlib import Path

# Database configuration
DB_HOST = "chatroom-dev-mysql.cjias2iok297.us-west-2.rds.amazonaws.com"
DB_PORT = 3306
DB_NAME = "chatroom"
DB_USER = "admin"
DB_PASSWORD = os.environ.get("DB_PASSWORD", "")

def run_migrations():
    """Run all SQL migration files"""

    if not DB_PASSWORD:
        print("❌ Error: DB_PASSWORD environment variable is required")
        print("")
        print("Usage:")
        print("  export DB_PASSWORD='your-password'")
        print("  python3 scripts/run_migrations.py")
        sys.exit(1)

    print("=" * 50)
    print("Database Migration Script")
    print("=" * 50)
    print(f"\nDatabase: {DB_HOST}:{DB_PORT}/{DB_NAME}")

    # Connect to database
    try:
        print("\n📦 Connecting to database...")
        connection = pymysql.connect(
            host=DB_HOST,
            port=DB_PORT,
            user=DB_USER,
            password=DB_PASSWORD,
            database=DB_NAME,
            charset='utf8mb4',
            cursorclass=pymysql.cursors.DictCursor
        )
        print("✅ Connected successfully")
    except Exception as e:
        print(f"❌ Failed to connect: {e}")
        sys.exit(1)

    try:
        # Get migration files
        migrations_dir = Path(__file__).parent.parent / "migrations"
        migration_files = sorted(glob.glob(str(migrations_dir / "*.sql")))

        # Filter out combined file
        migration_files = [f for f in migration_files if not f.endswith("combined-all.sql")]

        print(f"\n📦 Found {len(migration_files)} migration files")
        print("")

        # Run each migration
        with connection.cursor() as cursor:
            for migration_file in migration_files:
                migration_name = Path(migration_file).name
                print(f"▶️  Applying migration: {migration_name}")

                # Read and execute SQL file
                with open(migration_file, 'r') as f:
                    sql = f.read()

                try:
                    cursor.execute(sql)
                    connection.commit()
                    print("   ✅ Success")
                except Exception as e:
                    print(f"   ❌ Failed: {e}")
                    connection.rollback()
                    raise

                print("")

        print("=" * 50)
        print("✅ All migrations completed successfully")
        print("=" * 50)

    except Exception as e:
        print(f"\n❌ Error during migration: {e}")
        sys.exit(1)

    finally:
        connection.close()

if __name__ == "__main__":
    try:
        import pymysql
    except ImportError:
        print("❌ Error: pymysql is not installed")
        print("")
        print("Install it with:")
        print("  pip install pymysql")
        sys.exit(1)

    run_migrations()
