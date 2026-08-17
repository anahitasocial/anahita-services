#!/bin/bash
set -e  # Exit immediately if a command fails

# Set Goose environment variables
export GOOSE_DRIVER=mysql
export GOOSE_DBSTRING="$MYSQL_USER:$MYSQL_PASSWORD@tcp($MYSQL_HOST:3306)/$MYSQL_DATABASE?parseTime=true"

echo "Checking database migration status..."

# Wait for MySQL to be ready
echo "Waiting for MySQL to be available..."
until mysql -h "$MYSQL_HOST" -P 3306 -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "SELECT 1" &>/dev/null; do
  sleep 2
  echo "Still waiting for MySQL..."
done

echo "MySQL is ready. Proceeding with migration..."

# Check if the database is empty (no tables at all)
TABLE_COUNT=$(mysql -h "$MYSQL_HOST" -P 3306 -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -D "$MYSQL_DATABASE" -se "
SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$MYSQL_DATABASE';")

TABLE_COUNT=${TABLE_COUNT//[^0-9]/}  # Trim non-numeric chars

if [[ -z "$TABLE_COUNT" || "$TABLE_COUNT" -eq 0 ]]; then
    echo "No tables found. Running initial schema setup..."
    mysql -h "$MYSQL_HOST" -P 3306 -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" < ./schema/init.sql
    echo "Initial schema setup completed."
    exit 0
fi

# Check if legacy tables exist
LEGACY_TABLES=$(mysql -h "$MYSQL_HOST" -P 3306 -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -D "$MYSQL_DATABASE" -se "
SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema = '$MYSQL_DATABASE'
AND (table_name LIKE '%_edges' OR table_name LIKE '%_nodes');")

LEGACY_TABLES=${LEGACY_TABLES//[^0-9]/}

if [[ -n "$LEGACY_TABLES" && "$LEGACY_TABLES" -gt 0 ]]; then
    echo "Legacy tables detected. Running legacy schema upgrade..."
    mysql -h "$MYSQL_HOST" -P 3306 -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" < ./legacy/upgrade.sql
    echo "Legacy schema upgrade completed."
    exit 0
fi

# If there are tables but no legacy tables, proceed with Goose migrations
echo "Tables found but no legacy tables. Proceeding with Goose migrations..."
goose -dir ./migrations up
echo "Goose migrations completed."
