package notifiers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Every link in an email sent by this service is meant to land somebody inside
// the application. There are two getters that look like they give you that
// address and only one of them does.
//
//	GetFrontendURL          the web app. What a person clicking a link wants.
//	GetCompleteDomainName   the public marketing site — "https://www." plus
//	                        the domain. A different origin in production.
//
// Four notifiers used the second one. In development it resolves to
// https://www.localhost:3000, which is not a host; in production it lands on
// the marketing site rather than the account the mail is about. And because
// staging resolves both to the same address, a wrong choice passes every
// staging test and breaks in production only — which is precisely why this is
// a test rather than a comment.
//
// Nothing in this package should reach for the marketing site. If a genuine
// marketing link is ever wanted here, this test is the place to record why.

func TestNotifierLinksPointAtTheApplication(t *testing.T) {
	// Parsed rather than grepped, so the comments that explain this trap do
	// not trip the test that guards it.
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "GetCompleteDomainName" {
					return true
				}

				t.Errorf("%s:%d calls GetCompleteDomainName, which is the "+
					"marketing site and not the app. A person clicking that "+
					"link lands somewhere that cannot show them the thing the "+
					"email is about — and in development it is not even a real "+
					"host. Use GetFrontendURL.",
					name, fset.Position(call.Pos()).Line)

				return true
			})
		}
	}
}

// The routes these emails link to have to exist in the web app. They live in
// another repository, so nothing here can check them — this list is the
// record of what was verified by hand, and the reason each one is what it is.
//
//	/dashboard              welcome mail. Was /downloads, a route Anahita has
//	                        never had.
//	/support                TOTP lockout, recovery codes spent, email changed.
//	                        Public, because the people it is sent to cannot
//	                        sign in. Did not exist until it was added.
//	/people/{id}            the new member, for the admin broadcast.
//	/people/{alias}/settings/security   step-up lockout.
//
// If a link here changes, check the route still exists before shipping it. A
// 404 in an email is invisible to everybody except the person who needed it.
func TestNotifierRoutesAreRecorded(t *testing.T) {
	t.Log("routes linked from this package are listed in the comment above")
}
