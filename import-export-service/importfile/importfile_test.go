package importfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"importexport/data/repositories"
)

func summary(lines []*repositories.Line) []string {
	out := []string{}
	for _, l := range lines {
		out = append(out, fmt.Sprintf("%s %s%s hide=%v", l.Kind, l.Address, l.URI, l.HideNotifications))
	}
	return out
}

// THE RULE THIS GUARDS (roadmap row 124): a file is read as Mastodon reads
// it: by its first header, or with none by its name or what the person
// said it is; Misskey's lists of addresses too; lists and blocked servers,
// which Anahita has not, are said to be so.
func TestACSVIsKnownAsMastodonKnowsIt(t *testing.T) {
	cases := []struct {
		name, kind, body string
		want             []string
		err              error
	}{
		{"following_accounts.csv", "", "Account address,Show boosts,Notify on new posts,Languages\nnaomi@rocinante.example,true,false,\n@amos@rocinante.example,true,false,en\n",
			[]string{"follow naomi@rocinante.example hide=true", "follow amos@rocinante.example hide=true"}, nil},
		{"muted_accounts.csv", "", "Account address,Hide notifications\nmoss@reynholm.example,false\nroy@reynholm.example,true\n",
			[]string{"mute moss@reynholm.example hide=false", "mute roy@reynholm.example hide=true"}, nil},
		{"blocked_accounts.csv", "", "spam@bad.example\n", []string{"block spam@bad.example hide=true"}, nil},
		{"bookmarks.csv", "", "https://rocinante.example/notes/1\n", []string{"save https://rocinante.example/notes/1 hide=true"}, nil},
		{"anything.csv", "", "#uri\nat://did:plc:x/app.bsky.feed.post/1\n", []string{"save at://did:plc:x/app.bsky.feed.post/1 hide=true"}, nil},
		// Misskey's: one address a line, with its @.
		{"export.csv", "follow", "@rose@tardis.example\n@rose@tardis.example\n", []string{"follow rose@tardis.example hide=true"}, nil},
		{"export.csv", "", "@rose@tardis.example\n", nil, ErrKindUnknown},
		{"followers.csv", "", "rose@tardis.example\n", nil, ErrKindUnknown},
		{"lists.csv", "", "List name,Account address\nFriends,rose@tardis.example\n", nil, ErrNotSupported},
		{"blocked_domains.csv", "", "#domain\nbad.example\n", nil, ErrNotSupported},
		{"empty.csv", "", "  \n", nil, ErrEmpty},
	}
	for _, c := range cases {
		source, lines, err := Read(c.name, []byte(c.body), c.kind)
		if !errors.Is(err, c.err) {
			t.Errorf("%s: %v, want %v", c.name, err, c.err)
			continue
		}
		if c.err != nil {
			continue
		}
		if source != SourceMastodon {
			t.Errorf("%s: source %q", c.name, source)
		}
		if got := summary(lines); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s:\n%v\nwant\n%v", c.name, got, c.want)
		}
	}
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	for name, body := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// THE RULE THIS GUARDS: another Anahita's download is read by its data/
// files, which say the most: a mute's length and whether it hides
// notifications, a group followed by its page; its mastodon/ folder is
// read only when there is no data/.
func TestADownloadIsReadByItsData(t *testing.T) {
	data := zipOf(t, map[string]string{
		"data/profile.json":               `{"format": 2}`,
		"data/following.json":             `[{"kind":"person","alias":"naomi","address":"naomi@old.example","uri":"https://old.example/ap/users/1"},{"kind":"group","name":"Rocinante crew","url":"https://old.example/groups/5-crew"}]`,
		"data/blocked.json":               `[{"kind":"person","alias":"spam@bad.example"}]`,
		"data/muted.json":                 `[{"kind":"person","address":"moss@reynholm.example","hide_notifications":false,"ends_at":"2026-11-15T12:00:00Z"}]`,
		"data/saved.json":                 `[{"kind":"note","uri":"https://old.example/ap/notes/abc"}]`,
		"data/followers.json":             `[{"kind":"person","address":"roy@reynholm.example"}]`,
		"mastodon/following_accounts.csv": "Account address\nnobody@else.example\n",
	})
	source, lines, err := Read("anahita-naomi.zip", data, "")
	if err != nil {
		t.Fatal(err)
	}
	if source != SourceAnahita {
		t.Errorf("source %q", source)
	}
	want := []string{
		"follow naomi@old.examplehttps://old.example/ap/users/1 hide=true",
		"follow  hide=true",
		"block spam@bad.example hide=true",
		"mute moss@reynholm.example hide=false",
		"save https://old.example/ap/notes/abc hide=true",
	}
	if got := summary(lines); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("\n%v\nwant\n%v", got, want)
	}
	if lines[1].URL != "https://old.example/groups/5-crew" {
		t.Errorf("the group: %+v", lines[1])
	}
	if lines[3].EndsAt == nil || lines[3].EndsAt.Format("2006-01-02") != "2026-11-15" {
		t.Errorf("the mute's end: %+v", lines[3].EndsAt)
	}

	// No data/: Mastodon's files are read.
	_, lines, err = Read("x.zip", zipOf(t, map[string]string{
		"mastodon/following_accounts.csv": "Account address,Show boosts\nrose@tardis.example,true\n",
		"mastodon/blocked_accounts.csv":   "spam@bad.example\n",
	}), "")
	if err != nil || len(lines) != 2 {
		t.Errorf("mastodon/ alone: %v %v", summary(lines), err)
	}
}

// THE RULE THIS GUARDS: no more than Mastodon takes.
func TestTheLimits(t *testing.T) {
	var many strings.Builder
	for i := 0; i <= MaxLines; i++ {
		fmt.Fprintf(&many, "p%d@tardis.example\n", i)
	}
	if _, _, err := Read("following.csv", []byte(many.String()), ""); !errors.Is(err, ErrTooMany) {
		t.Errorf("too many: %v", err)
	}
	if _, _, err := Read("x.zip", []byte("PK\x03\x04 not a zip"), ""); !errors.Is(err, ErrUnreadable) {
		t.Errorf("a broken zip: %v", err)
	}
}
