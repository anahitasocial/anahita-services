// Package importfile reads what a person uploads to bring their lists in
// (anahita-web roadmap row 124): another Anahita's download, as a zip, or a
// single CSV, Mastodon's or Misskey's. What it reads is a list of lines,
// each one person to follow, block or mute, or one post to save; nothing is
// done here.
//
// A CSV is known as Mastodon knows one (its app/models/form/import.rb): by
// its first header, or, with none, by its name or by what the person said
// it is.
package importfile

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"importexport/data/repositories"
)

// The limits, Mastodon's own: at most so many lines, and so large a file.
const (
	MaxLines = 20000
	MaxBytes = 20 << 20
)

// Where a file came from.
const (
	SourceAnahita  = "anahita"
	SourceMastodon = "mastodon"
)

// Why a file could not be read, in a word the web app turns into a sentence.
var (
	ErrUnreadable = errors.New("unreadable")
	ErrEmpty      = errors.New("empty")
	ErrTooLarge   = errors.New("too_large")
	ErrTooMany    = errors.New("too_many_lines")
	// A CSV with no header and no name to tell what it is: the person is
	// asked which kind it is.
	ErrKindUnknown = errors.New("kind_unknown")
	// Lists and blocked servers, which Anahita does not have yet.
	ErrNotSupported = errors.New("not_supported")
)

// Read reads a file: its name as uploaded, its bytes, and, for a CSV with
// no header, the kind the person said it is ("" when they did not).
func Read(name string, data []byte, kind string) (string, []*repositories.Line, error) {
	if len(data) > MaxBytes {
		return "", nil, ErrTooLarge
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", nil, ErrEmpty
	}

	var (
		source string
		lines  []*repositories.Line
		err    error
	)
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		source, lines, err = readZip(data)
	} else {
		source = SourceMastodon
		lines, err = readCSV(name, data, kind)
	}
	if err != nil {
		return "", nil, err
	}

	lines = distinct(lines)
	if len(lines) == 0 {
		return "", nil, ErrEmpty
	}
	if len(lines) > MaxLines {
		return "", nil, ErrTooMany
	}
	return source, lines, nil
}

// readZip reads a download: another Anahita's data/ files, which say the
// most, or, failing them, Mastodon's files in its mastodon/ folder.
func readZip(data []byte) (string, []*repositories.Line, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, ErrUnreadable
	}
	files := map[string]*zip.File{}
	for _, f := range z.File {
		files[strings.TrimPrefix(f.Name, "./")] = f
	}

	if _, ok := files["data/profile.json"]; ok {
		lines := []*repositories.Line{}
		for _, list := range []struct {
			file string
			kind string
		}{
			{"data/following.json", repositories.KindFollow},
			{"data/blocked.json", repositories.KindBlock},
			{"data/muted.json", repositories.KindMute},
			{"data/saved.json", repositories.KindSave},
		} {
			f, ok := files[list.file]
			if !ok {
				continue
			}
			read, err := readRefs(f, list.kind)
			if err != nil {
				return "", nil, err
			}
			lines = append(lines, read...)
		}
		return SourceAnahita, lines, nil
	}

	lines := []*repositories.Line{}
	found := false
	for name, f := range files {
		if path.Ext(name) != ".csv" {
			continue
		}
		body, err := open(f)
		if err != nil {
			return "", nil, err
		}
		read, err := readCSV(path.Base(name), body, "")
		if errors.Is(err, ErrKindUnknown) || errors.Is(err, ErrNotSupported) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		found = true
		lines = append(lines, read...)
	}
	if !found {
		return "", nil, ErrUnreadable
	}
	return SourceMastodon, lines, nil
}

func open(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > MaxBytes {
		return nil, ErrTooLarge
	}
	r, err := f.Open()
	if err != nil {
		return nil, ErrUnreadable
	}
	defer r.Close()
	body, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, ErrUnreadable
	}
	if len(body) > MaxBytes {
		return nil, ErrTooLarge
	}
	return body, nil
}

// ref is an entry of a download's lists (import-export-service/archive).
type ref struct {
	Kind              string     `json:"kind"`
	Name              string     `json:"name"`
	Alias             string     `json:"alias"`
	URL               string     `json:"url"`
	Address           string     `json:"address"`
	URI               string     `json:"uri"`
	HideNotifications *bool      `json:"hide_notifications"`
	EndsAt            *time.Time `json:"ends_at"`
}

func readRefs(f *zip.File, kind string) ([]*repositories.Line, error) {
	body, err := open(f)
	if err != nil {
		return nil, err
	}
	var refs []ref
	if err := json.Unmarshal(body, &refs); err != nil {
		return nil, ErrUnreadable
	}

	lines := make([]*repositories.Line, 0, len(refs))
	for _, r := range refs {
		line := &repositories.Line{
			Kind:              kind,
			Address:           strings.TrimPrefix(r.Address, "@"),
			URI:               r.URI,
			Name:              r.Name,
			URL:               r.URL,
			HideNotifications: true,
		}
		if kind == repositories.KindMute {
			if r.HideNotifications != nil {
				line.HideNotifications = *r.HideNotifications
			}
			if r.EndsAt != nil {
				ends := r.EndsAt.UTC()
				line.EndsAt = &ends
			}
		}
		// A download from before entries said their address (format 1):
		// somebody's address is their alias when it is a whole handle.
		if line.Address == "" && r.Kind == "person" && strings.Contains(r.Alias, "@") {
			line.Address = strings.TrimPrefix(r.Alias, "@")
		}
		// Neither address nor id, nor a page: nothing to go by.
		if line.Address == "" && line.URI == "" && line.URL == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// readCSV reads one CSV: Mastodon's, Misskey's, or any list of addresses.
func readCSV(name string, data []byte, kind string) ([]*repositories.Line, error) {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, ErrUnreadable
	}
	if len(rows) == 0 {
		return nil, ErrEmpty
	}

	// Which kind, and whether the first row is a header.
	header := map[string]int{}
	first := strings.TrimSpace(rows[0][0])
	switch first {
	case "Account address":
		for i, column := range rows[0] {
			header[strings.TrimSpace(column)] = i
		}
		rows = rows[1:]
		switch {
		case has(header, "Hide notifications"):
			kind = repositories.KindMute
		case has(header, "Show boosts"), has(header, "Notify on new posts"):
			kind = repositories.KindFollow
		default:
			kind = firstOf(kind, kindOfName(name))
		}
	case "#uri":
		rows = rows[1:]
		kind = repositories.KindSave
	case "#domain", "List name":
		return nil, ErrNotSupported
	default:
		kind = firstOf(kind, kindOfName(name))
		if kind == "" && looksLikePost(first) {
			kind = repositories.KindSave
		}
	}
	if kind == "" {
		return nil, ErrKindUnknown
	}

	hide, hasHide := header["Hide notifications"]
	lines := make([]*repositories.Line, 0, len(rows))
	for _, row := range rows {
		value := strings.TrimSpace(row[0])
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		line := &repositories.Line{Kind: kind, HideNotifications: true}
		if kind == repositories.KindSave {
			line.URI = value
		} else {
			line.Address = strings.TrimPrefix(value, "@")
		}
		if kind == repositories.KindMute && hasHide && hide < len(row) {
			if hidden, err := strconv.ParseBool(strings.TrimSpace(row[hide])); err == nil {
				line.HideNotifications = hidden
			}
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func has(header map[string]int, column string) bool {
	_, ok := header[column]
	return ok
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// kindOfName is the kind a file's name says, as Mastodon and Misskey name
// theirs: following_accounts.csv, blocked_accounts.csv, muted_accounts.csv,
// bookmarks.csv.
func kindOfName(name string) string {
	name = strings.ToLower(path.Base(name))
	switch {
	// Who follows somebody is not theirs to bring: their followers come by
	// a Move (part 3), each with their own consent.
	case strings.Contains(name, "follower"):
		return ""
	case strings.Contains(name, "follow"):
		return repositories.KindFollow
	case strings.Contains(name, "block"):
		return repositories.KindBlock
	case strings.Contains(name, "mute"), strings.Contains(name, "muting"):
		return repositories.KindMute
	case strings.Contains(name, "bookmark"), strings.Contains(name, "saved"):
		return repositories.KindSave
	}
	return ""
}

func looksLikePost(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "at://")
}

// distinct leaves each person or post once in each kind, as first met.
func distinct(lines []*repositories.Line) []*repositories.Line {
	seen := map[string]bool{}
	kept := lines[:0]
	for _, line := range lines {
		key := line.Kind + "\x00" + strings.ToLower(firstOf(line.Address, line.URI, line.URL))
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, line)
	}
	return kept
}
