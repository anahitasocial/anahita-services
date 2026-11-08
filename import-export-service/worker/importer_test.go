package worker

import (
	"context"
	"testing"
	"time"

	"anahita-libs/data/entities"

	"importexport/data/repositories"
)

// fakeImports keeps one import's lines in order.
type fakeImports struct {
	repositories.ImportsInterface
	person   int64
	lines    []*repositories.Line
	finished map[int64][2]string
	closed   bool
}

func (f *fakeImports) NextLine() (*repositories.Line, error) {
	for _, line := range f.lines {
		if line.Status == repositories.LinePending {
			line.Status = repositories.LineWorking
			return line, nil
		}
	}
	return nil, nil
}

func (f *fakeImports) Read(id string) (*repositories.Import, error) {
	return &repositories.Import{ID: id, PersonID: f.person}, nil
}

func (f *fakeImports) Finish(id int64, status, reason string) error {
	f.finished[id] = [2]string{status, reason}
	for _, line := range f.lines {
		if line.ID == id {
			line.Status = status
		}
	}
	return nil
}

func (f *fakeImports) FinishImports() error {
	f.closed = true
	return nil
}

// fakeActs finds people and posts by a table, and does what is asked.
type fakeActs struct {
	people map[string]int64
	posts  map[string]int64
	did    []string
}

func (f *fakeActs) FindPerson(_ context.Context, _ *entities.Viewer, line *repositories.Line) (*entities.AnahitaNode, string) {
	if id, ok := f.people[line.Address]; ok {
		return &entities.AnahitaNode{ID: id, Type: "node.actor.activitypub-service.person.v1"}, ""
	}
	return nil, ReasonNotFound
}

func (f *fakeActs) FindPost(_ context.Context, _ *entities.Viewer, line *repositories.Line) (*entities.AnahitaNode, string) {
	if id, ok := f.posts[line.URI]; ok {
		return &entities.AnahitaNode{ID: id}, ""
	}
	return nil, ReasonUnreachable
}

func (f *fakeActs) act(what string, target *entities.AnahitaNode) (string, string) {
	f.did = append(f.did, what)
	return repositories.LineDone, ""
}

func (f *fakeActs) Follow(_ *entities.Viewer, t *entities.AnahitaNode) (string, string) {
	return f.act("follow", t)
}
func (f *fakeActs) Block(_ *entities.Viewer, t *entities.AnahitaNode) (string, string) {
	return f.act("block", t)
}
func (f *fakeActs) Mute(_ *entities.Viewer, t *entities.AnahitaNode, hide bool, _ *time.Time) (string, string) {
	if hide {
		return f.act("mute hiding", t)
	}
	return f.act("mute", t)
}
func (f *fakeActs) Save(_ *entities.Viewer, p *entities.AnahitaNode) (string, string) {
	return f.act("save", p)
}

// THE RULE THIS GUARDS (roadmap row 124): each line is done as its person,
// once; what cannot be found, a group from another Anahita, and a mute that
// has ended are said to be so and not tried; and with nothing left, the
// imports are closed.
func TestTheImporterGoesThroughTheLines(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	past, later := now.Add(-time.Hour), now.Add(time.Hour)
	line := func(id int64, kind, address, uri string) *repositories.Line {
		return &repositories.Line{ID: id, ImportID: "i", Kind: kind, Address: address, URI: uri,
			HideNotifications: true, Status: repositories.LinePending}
	}
	lines := []*repositories.Line{
		line(1, repositories.KindFollow, "naomi@rocinante.example", ""),
		line(2, repositories.KindFollow, "nobody@rocinante.example", ""),
		line(3, repositories.KindFollow, "", ""),
		line(4, repositories.KindBlock, "spam@bad.example", ""),
		line(5, repositories.KindMute, "moss@reynholm.example", ""),
		line(6, repositories.KindMute, "roy@reynholm.example", ""),
		line(7, repositories.KindSave, "", "https://rocinante.example/notes/1"),
		line(8, repositories.KindSave, "", "https://away.example/notes/2"),
	}
	lines[2].URL = "https://old.example/groups/5-crew"
	lines[4].EndsAt = &later
	lines[5].EndsAt = &past

	imports := &fakeImports{person: 9, lines: lines, finished: map[int64][2]string{}}
	acts := &fakeActs{
		people: map[string]int64{"naomi@rocinante.example": 20, "spam@bad.example": 21, "moss@reynholm.example": 22, "roy@reynholm.example": 23},
		posts:  map[string]int64{"https://rocinante.example/notes/1": 30},
	}
	im := &Importer{Imports: imports, Acts: acts, Now: func() time.Time { return now }}

	people := map[string]int64{}
	for {
		more, err := im.Next(context.Background(), people)
		if err != nil {
			t.Fatal(err)
		}
		if !more {
			break
		}
	}

	want := map[int64][2]string{
		1: {repositories.LineDone, ""},
		2: {repositories.LineFailed, ReasonNotFound},
		3: {repositories.LineFailed, ReasonNotOnFediverse},
		4: {repositories.LineDone, ""},
		5: {repositories.LineDone, ""},
		6: {repositories.LineAlready, ReasonEnded},
		7: {repositories.LineDone, ""},
		8: {repositories.LineFailed, ReasonUnreachable},
	}
	for id, w := range want {
		if imports.finished[id] != w {
			t.Errorf("line %d: %v, want %v", id, imports.finished[id], w)
		}
	}
	if got := len(acts.did); got != 4 || acts.did[2] != "mute hiding" {
		t.Errorf("done: %v", acts.did)
	}
	if !imports.closed {
		t.Errorf("the imports were not closed with nothing left")
	}
}

func TestHandleOf(t *testing.T) {
	for in, want := range map[string][2]string{
		"@naomi@Rocinante.example": {"naomi", "rocinante.example"},
		"sky.bsky.social":          {"", "sky.bsky.social"},
	} {
		if user, domain := handleOf(in); [2]string{user, domain} != want {
			t.Errorf("%s: %s %s", in, user, domain)
		}
	}
}
