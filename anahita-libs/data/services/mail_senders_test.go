package services

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// mailSenders are the only services that may send email: auth-service, for
// accounts, invitations, signups and a group's deletion, and the mail
// service itself. Notifications are never emailed (the user, 2026-10-19);
// the services that make them must not be able to (docs/notifications.md).
var mailSenders = []string{"auth-service", "mail-grpc-service"}

var usesMail = regexp.MustCompile(`anahita-libs/proto/mailpb|\bNewMailGRPC\b|\bMailGRPCInterface\b`)

func TestOnlyAuthServiceSendsMail(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "notification-service")); err != nil {
		t.Skipf("the services are not beside anahita-libs here: %v", err)
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if d.Name() == "vendor" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
		if top == "anahita-libs" {
			return nil
		}
		for _, sender := range mailSenders {
			if top == sender {
				return nil
			}
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if usesMail.Match(body) {
			t.Errorf("%s uses the mail client: only %s may send email", rel, strings.Join(mailSenders, " and "))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
