#!/bin/bash
set -e  # Exit immediately if a command fails

# Set Goose environment variables
export GOOSE_DRIVER=mysql
export GOOSE_DBSTRING="$MYSQL_USER:$MYSQL_PASSWORD@tcp($MYSQL_HOST:3306)/$MYSQL_DATABASE?parseTime=true"

# stamp_migrations records every migration as already applied.
#
# schema/init.sql and legacy/upgrade.sql both produce a schema that already
# INCLUDES everything under ./migrations — they are kept in step by hand. Both
# branches used to exit here without telling goose that, so the next start of
# the same database found tables, found no legacy tables, and ran every
# migration from zero against a schema that already had them.
#
# Every migration written so far carries an information-schema guard and
# survives that, but the guards are a seatbelt: the fix is to not drive at the
# wall. A migration added without one would fail the container on restart.
#
# Goose has no baseline or stamp command, so the version rows are written
# directly. The table definition matches the one goose creates for MySQL; the
# WHERE NOT EXISTS makes this safe to run against a table that already has rows.
stamp_migrations() {
    echo "Recording the schema as current so goose does not replay it..."

    mysql -h "$MYSQL_HOST" -P 3306 -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" <<'SQL'
CREATE TABLE IF NOT EXISTS goose_db_version (
    id serial NOT NULL,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp NULL DEFAULT now(),
    PRIMARY KEY(id)
);
SQL

    # Version 0 is goose's own baseline row, then one per migration file. The
    # version id is the numeric prefix of the filename, which is how goose
    # identifies a migration.
    local versions="0"
    for file in ./migrations/*.sql; do
        [ -e "$file" ] || continue
        versions="$versions $(basename "$file" | cut -d_ -f1)"
    done

    for version in $versions; do
        mysql -h "$MYSQL_HOST" -P 3306 -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e "
            INSERT INTO goose_db_version (version_id, is_applied)
            SELECT $version, 1 FROM DUAL
            WHERE NOT EXISTS (
                SELECT 1 FROM goose_db_version WHERE version_id = $version
            );"
    done

    echo "Recorded $(echo "$versions" | wc -w | tr -d ' ') migration versions."
}

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
    stamp_migrations
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
    stamp_migrations
    exit 0
fi

# If there are tables but no legacy tables, proceed with Goose migrations
echo "Tables found but no legacy tables. Proceeding with Goose migrations..."
goose -dir ./migrations up
echo "Goose migrations completed."
