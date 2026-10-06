package constants

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scopeCheckFuncs are the methods whose argument must always be a scope
// constant. Both live on entities.Viewer; RequireScope additionally has
// a middleware namesake, which this catches too — that one takes a
// constant as well.
var scopeCheckFuncs = map[string]bool{
	"HasScope":     true,
	"RequireScope": true,
}

// TestNoScopeStringLiterals is the guard that makes the scope
// vocabulary enforceable rather than aspirational.
//
// The fleet once carried three generations of scope names at the same
// time — a JSON file, this package, and dozens of hand-typed string
// literals in permission files — because nothing connected them. Since
// HasScope is a plain slices.Contains, a literal that matches no
// constant fails closed and silently: the gate returns "insufficient
// scope" forever, for everyone, with nothing in the logs naming the
// cause. That is exactly how every notification endpoint 403'd for
// months.
//
// The compiler catches a mistyped *constant*. Only this test catches a
// reintroduced *literal*.
func TestNoScopeStringLiterals(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	var offenders []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			// A file this package can't parse isn't this test's problem;
			// the build will surface it.
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !scopeCheckFuncs[sel.Sel.Name] {
				return true
			}
			for _, arg := range call.Args {
				lit, ok := arg.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel+":"+
					fset.Position(lit.Pos()).String()[len(fset.Position(lit.Pos()).Filename)+1:]+
					" "+sel.Sel.Name+"("+lit.Value+")")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	for _, o := range offenders {
		t.Errorf("scope string literal — use a constants.OAuthScope* constant instead: %s", o)
	}
}

// TestScopeDefinitionsAreWellFormed keeps the registry internally
// consistent. A key that disagrees with its Scope field would make
// ScopeDescriptions and the discovery document disagree; an empty
// Description silently degrades the consent screen to a blank bullet.
func TestScopeDefinitionsAreWellFormed(t *testing.T) {
	const want = 18
	if got := len(OAuthScopeDefinitions); got != want {
		t.Errorf("registry has %d scopes, want %d", got, want)
	}

	for key, def := range OAuthScopeDefinitions {
		if def.Scope != key {
			t.Errorf("scope %q has mismatched Scope field %q", key, def.Scope)
		}
		if strings.TrimSpace(def.Description) == "" {
			t.Errorf("scope %q has an empty Description; it would render as a blank consent bullet", key)
		}
		if !OAuthScopeExists(key) {
			t.Errorf("scope %q is in the map but OAuthScopeExists says otherwise", key)
		}
	}

	if OAuthScopeExists("definitely:not:a:scope") {
		t.Error("OAuthScopeExists accepted an unknown scope")
	}
}

// TestOnlyOAuthScopesAreSuperAdminOnly pins the one deliberate
// privilege escalation in the catalog. AdminOnly is intentionally
// unused — gating a write scope on admin would break self-service, and
// role authority belongs in the per-resource permission checks.
func TestOnlyOAuthScopesAreSuperAdminOnly(t *testing.T) {
	for key, def := range OAuthScopeDefinitions {
		if def.AdminOnly {
			t.Errorf("scope %q is AdminOnly; role authority belongs in handler permissions, not the scope catalog", key)
		}
		wantSuper := key == OAuthScopeOAuthRead || key == OAuthScopeOAuthWrite
		if def.SuperAdminOnly != wantSuper {
			t.Errorf("scope %q SuperAdminOnly = %v, want %v", key, def.SuperAdminOnly, wantSuper)
		}
	}
}

// repoRoot walks up from the test's working directory to the directory
// holding go.work, which is the workspace root covering every module.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.work above the test directory")
		}
		dir = parent
	}
}
