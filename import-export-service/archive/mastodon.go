package archive

import (
	"archive/zip"
	"encoding/csv"
	"strconv"
	"strings"
)

// Mastodon's own files of who is followed, blocked and muted, and what was
// saved (roadmap row 124), in the folder mastodon/. Mastodon, Misskey,
// GoToSocial, Akkoma and the rest import them: the shapes are Mastodon's
// own, from its app/models/export.rb.
//
// Only people on the fediverse are in them, by their handle: somebody on
// Bluesky alone has no address those servers follow, and a group or an
// event here is not on the fediverse. Another Anahita reads data/, which
// has everything.
func (b *Builder) writeMastodon(z *zip.Writer, page *pageData) error {
	following := [][]string{{"Account address", "Show boosts", "Notify on new posts", "Languages"}}
	for _, ref := range fediversePeople(page.Following) {
		following = append(following, []string{ref.Address, "true", "false", ""})
	}

	blocked := [][]string{}
	for _, ref := range fediversePeople(page.Blocked) {
		blocked = append(blocked, []string{ref.Address})
	}

	// A mute that ends is written as one that does not: Mastodon's file
	// has no column for when.
	muted := [][]string{{"Account address", "Hide notifications"}}
	for _, ref := range fediversePeople(page.Muted) {
		hide := ref.HideNotifications == nil || *ref.HideNotifications
		muted = append(muted, []string{ref.Address, strconv.FormatBool(hide)})
	}

	bookmarks := [][]string{}
	for _, ref := range page.Saved {
		if ref.URI != "" {
			bookmarks = append(bookmarks, []string{ref.URI})
		}
	}

	for _, file := range []struct {
		name string
		rows [][]string
	}{
		{"mastodon/following_accounts.csv", following},
		{"mastodon/blocked_accounts.csv", blocked},
		{"mastodon/muted_accounts.csv", muted},
		{"mastodon/bookmarks.csv", bookmarks},
	} {
		if err := writeCSV(z, file.name, file.rows); err != nil {
			return err
		}
	}
	return nil
}

// fediversePeople are the people among refs with a handle another
// fediverse server can follow: user@domain.
func fediversePeople(refs []*Ref) []*Ref {
	people := []*Ref{}
	for _, ref := range refs {
		if ref.Kind == "person" && strings.Contains(ref.Address, "@") {
			people = append(people, ref)
		}
	}
	return people
}

func writeCSV(z *zip.Writer, name string, rows [][]string) error {
	out, err := z.Create(name)
	if err != nil {
		return err
	}
	w := csv.NewWriter(out)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	return w.Error()
}
