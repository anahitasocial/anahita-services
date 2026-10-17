#!/bin/bash
# Checks that the two ways of building a database give the same schema: an
# empty one from schema/init.sql, and the PHP application's from
# legacy/fixtures/php-schema.sql through legacy/upgrade.sql. Compared column
# by column and index by index, never as one joined string, which MySQL cuts
# off at 1,024 characters.
#
# Runs a throwaway MySQL in Docker and removes it, with its volume, after.
# Run from the repository root:  make schema_test
set -euo pipefail

cd "$(dirname "$0")"
IMAGE="${SCHEMA_TEST_IMAGE:-mysql:9.6}"
NAME="anahita-schema-test-$$"
WORK="$(mktemp -d)"

cleanup() {
    docker rm -fv "$NAME" >/dev/null 2>&1 || true
    rm -rf "$WORK"
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
    echo "check_schema.sh: Docker is not running; the schema check needs it." >&2
    exit 1
fi

docker run -d --name "$NAME" -e MYSQL_ROOT_PASSWORD=check "$IMAGE" >/dev/null
until docker exec "$NAME" mysql -uroot -pcheck -e "SELECT 1" >/dev/null 2>&1; do
    sleep 1
done

run() {
    docker exec -i "$NAME" mysql -uroot -pcheck --default-character-set=utf8mb4 "$@" 2>&1 | grep -v "Using a password" || true
}

run -e "CREATE DATABASE fresh; CREATE DATABASE legacy"
run fresh < schema/init.sql
run legacy < legacy/fixtures/php-schema.sql
run legacy < legacy/upgrade.sql > "$WORK/upgrade.log"
if grep -q "^ERROR" "$WORK/upgrade.log"; then
    echo "legacy/upgrade.sql failed on the PHP schema:" >&2
    grep "^ERROR" "$WORK/upgrade.log" >&2
    exit 1
fi

# One line for every column, every index and every table.
shape() {
    run -N "$1" <<'SQL' | sort
SELECT CONCAT_WS(' | ', 'column', table_name, column_name, column_type, is_nullable,
       IFNULL(column_default, 'NULL'), extra, IFNULL(character_set_name, ''), IFNULL(collation_name, ''))
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name <> 'goose_db_version'
UNION ALL
SELECT CONCAT_WS(' | ', 'index', table_name, index_name, non_unique,
       GROUP_CONCAT(CONCAT(column_name, IFNULL(CONCAT('(', sub_part, ')'), '')) ORDER BY seq_in_index))
FROM information_schema.statistics
WHERE table_schema = DATABASE() AND table_name <> 'goose_db_version'
GROUP BY table_name, index_name, non_unique
UNION ALL
SELECT CONCAT_WS(' | ', 'table', table_name, engine, table_collation)
FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' AND table_name <> 'goose_db_version';
SQL
}

shape fresh > "$WORK/fresh.txt"
shape legacy > "$WORK/legacy.txt"

if ! diff -u "$WORK/fresh.txt" "$WORK/legacy.txt" > "$WORK/diff.txt"; then
    echo "schema/init.sql and legacy/upgrade.sql differ (- fresh, + upgraded):" >&2
    grep '^[-+][a-z]' "$WORK/diff.txt" >&2
    exit 1
fi
echo "check_schema.sh: a fresh schema and an upgraded PHP one are the same, $(wc -l < "$WORK/fresh.txt" | tr -d ' ') columns, indexes and tables."
