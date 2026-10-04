package repositories

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// One name for timestamps.
//
// A column holding a date with a time ends in `_at`: created_at, updated_at,
// starts_at. A column holding a calendar date with no time ends in `_on`.
//
// The schema used to carry three conventions — `_on` and `_date` from the PHP
// application, a `start_time`, and `_at` for everything written since — and a
// reader had to remember which table was which. They were made one on
// 2026-10-04, and this is what keeps it that way: it reads the schema a fresh
// installation is built from and fails on a column that does not follow.
//
// It lives here because this is the service the schema's two largest tables
// belong to, and a Go test runs with everything else; the schema itself is
// migration-job's, which has no tests of its own.
func TestTimestampColumnsEndInAt(t *testing.T) {
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "migration-job", "schema", "init.sql"))
	if err != nil {
		t.Fatalf("reading the schema: %v", err)
	}

	problems := misnamedTimeColumns(string(schema))
	for _, problem := range problems {
		t.Error(problem)
	}

	// The check must be looking at something. A schema that moved, or a
	// pattern that stopped matching, would otherwise pass by finding nothing.
	if found := len(timeColumn.FindAllString(string(schema), -1)); found < 40 {
		t.Errorf("found only %d date and time columns in the schema; has it moved, "+
			"or has the pattern stopped matching?", found)
	}
}

// timeColumn matches a column definition of a date or time type, capturing
// the name and the type: "  `created_at` datetime NOT NULL".
var timeColumn = regexp.MustCompile("(?mi)^\\s*`(\\w+)`\\s+(datetime|timestamp|date)\\b")

func misnamedTimeColumns(schema string) []string {
	var problems []string

	for _, match := range timeColumn.FindAllStringSubmatch(schema, -1) {
		name, kind := match[1], strings.ToLower(match[2])

		switch {
		case kind == "date" && !strings.HasSuffix(name, "_on"):
			problems = append(problems, "`"+name+"` is a date with no time, and should end in _on")
		case kind != "date" && !strings.HasSuffix(name, "_at"):
			problems = append(problems, "`"+name+"` is a "+kind+", and should end in _at")
		}
	}

	return problems
}

// The guard itself, on names it must and must not object to.
func TestTimestampGuardObjectsToTheOldNames(t *testing.T) {
	schema := "CREATE TABLE example (\n" +
		"  `id` bigint unsigned NOT NULL,\n" +
		"  `created_at` datetime NOT NULL,\n" +
		"  `born_on` date DEFAULT NULL,\n" +
		"  `modified_on` datetime DEFAULT NULL,\n" +
		"  `start_date` datetime DEFAULT NULL,\n" +
		"  `seen` timestamp NULL,\n" +
		"  `died_at` date DEFAULT NULL,\n" +
		"  `date_format` varchar(20) DEFAULT NULL\n" +
		")"

	problems := strings.Join(misnamedTimeColumns(schema), "\n")

	for _, wrong := range []string{"modified_on", "start_date", "seen", "died_at"} {
		if !strings.Contains(problems, "`"+wrong+"`") {
			t.Errorf("the guard let `%s` through", wrong)
		}
	}
	for _, right := range []string{"created_at", "born_on", "date_format", "`id`"} {
		if strings.Contains(problems, right) {
			t.Errorf("the guard objected to %s:\n%s", right, problems)
		}
	}
}
