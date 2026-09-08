package repositories

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Each access gate compares one row's access column across every branch, and
// which row that is differs per gate: the search gate asks about `node`, the
// feed gate about `object` — the thing a story is about — and both also ask
// about `owner`.
//
// One branch using a different alias from its neighbours is a copy-paste slip,
// and it does not fail loudly. The feed's "admins" branch asked about
// node.access while the other six asked about object.access, so the admins
// access level simply did not work in feeds: the story row's own access is
// usually empty, FIND_IN_SET against it is NULL, and the branch never matched.
// An admins-only object was hidden from the administrators entitled to it.
//
// Read out of the source because the gates build squirrel expressions that are
// only assembled into SQL alongside a full query, and the question here is
// about the shape of the code rather than of any one statement.
func TestEachAccessGateUsesOneAliasForItsSubjectRow(t *testing.T) {
	files, err := os.ReadDir("../repositories")
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	accessLevel := regexp.MustCompile(`FIND_IN_SET\(\?, ([a-z_]+)\.access\)`)

	for _, f := range files {
		name := f.Name()
		if !strings.HasPrefix(name, "access") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		// Comments describe the fix rather than perform it, and would
		// otherwise be counted as usages.
		body := regexp.MustCompile(`(?m)^\s*//.*$`).ReplaceAll(source, nil)

		counts := map[string]int{}
		for _, m := range accessLevel.FindAllSubmatch(body, -1) {
			counts[string(m[1])]++
		}

		// A gate names its subject row several times — one branch per access
		// level. An alias appearing exactly once alongside others that appear
		// many times is the slip this test exists for.
		for alias, n := range counts {
			if n != 1 {
				continue
			}
			for other, m := range counts {
				if other != alias && m >= 3 {
					t.Errorf("%s: %s.access is compared once while %s.access is "+
						"compared %d times — one branch is asking about a "+
						"different row than its neighbours, and that fails "+
						"quietly rather than erroring",
						name, alias, other, m)
				}
			}
		}
	}
}
