#!/usr/bin/env python3
"""Guard against statement-ordering mistakes in legacy/upgrade.sql.

entrypoint.sh runs this one file, top to bottom, against a database migrated
from the PHP app — then exits. It never sees schema/init.sql or the goose
migrations, so every schema change made elsewhere has to be mirrored here by
hand. That hand-mirroring is the risk this checks: that a table is made
before it is used. make schema_test checks the rest, by running the file.

The failure it exists for: `ALTER TABLE tokens ADD COLUMN new_email` was
mirrored in near the top of the file while `CREATE TABLE tokens` sits 170 lines
below it, so every legacy upgrade died with "Table 'anahita.tokens' doesn't
exist" — and only for people migrating from the PHP app, which is the path
least likely to be exercised during development.

Run from the repository root:  python3 migration-job/check_sql_order.py
"""

import re
import sys
from pathlib import Path

UPGRADE = Path(__file__).parent / "legacy" / "upgrade.sql"

# Statements that require the table to already exist.
USES = re.compile(
    r"\b(?:ALTER\s+TABLE|INSERT\s+INTO|UPDATE|DELETE\s+FROM|CREATE\s+INDEX\s+ON)\s+`?(\w+)`?",
    re.I,
)
CREATES = re.compile(r"^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?`?(\w+)`?", re.I)


def main() -> int:
    lines = UPGRADE.read_text().split("\n")

    created: dict[str, int] = {}
    for number, line in enumerate(lines, 1):
        if match := CREATES.match(line):
            created.setdefault(match.group(1).lower(), number)

    problems = []
    for table, create_line in created.items():
        for number, line in enumerate(lines[: create_line - 1], 1):
            statement = line.strip()
            if not statement or statement.startswith("--"):
                continue
            for used in USES.findall(statement):
                if used.lower() == table:
                    problems.append((table, number, create_line, statement[:70]))

    if problems:
        print("legacy/upgrade.sql: table used before it is created\n")
        for table, used_at, made_at, statement in problems:
            print(f"  {table}: used on line {used_at}, created on line {made_at}")
            print(f"    {statement}")
        print(
            "\nA legacy upgrade will fail here with \"Table doesn't exist\". Either move\n"
            "the statement below the CREATE, or fold the column into the CREATE itself."
        )
        return 1

    print(f"legacy/upgrade.sql: {len(created)} tables created, none used before creation")
    return 0


if __name__ == "__main__":
    sys.exit(main())
