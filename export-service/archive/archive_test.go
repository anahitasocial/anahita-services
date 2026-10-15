package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/videos"
)

// An archive, made of a small account held in memory, opened again and
// read. What the graph hands over as "what this person wrote" is the
// graph's to get right, and is tested there.

type fakeSource struct {
	profile   *entities.AnahitaNode
	authored  []*entities.AnahitaNode
	files     []*entities.PhotoFile
	connected map[string][]*entities.AnahitaNode
	saved     []*entities.AnahitaNode
	pages     int
}

func (f *fakeSource) Profile(id int64) (*entities.AnahitaNode, error) { return f.profile, nil }

func (f *fakeSource) Authored(id, after int64, limit uint64) ([]*entities.AnahitaNode, error) {
	f.pages++
	var page []*entities.AnahitaNode
	for _, node := range f.authored {
		if node.ID > after && uint64(len(page)) < 2 {
			page = append(page, node)
		}
	}
	return page, nil
}

func (f *fakeSource) PhotoFiles(ids []int64) ([]*entities.PhotoFile, error) { return f.files, nil }

func (f *fakeSource) Connected(id int64, kind string, offset uint64) ([]*entities.AnahitaNode, error) {
	if offset > 0 {
		return nil, nil
	}
	return f.connected[kind], nil
}

func (f *fakeSource) Saved(id int64, offset uint64) ([]*entities.AnahitaNode, error) {
	if offset > 0 {
		return nil, nil
	}
	return f.saved, nil
}

type fakeFiles map[string]string

func (f fakeFiles) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	body, ok := f[key]
	if !ok {
		return nil, ErrNoFile
	}
	return io.NopCloser(strings.NewReader(body)), nil
}

func node(id int64, kind, service string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = id
	n.Type = "node.medium." + service + "." + kind + ".v1"
	n.Service = service
	n.CreatedBy = 5
	n.OwnerID = 5
	n.CreatedAt = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	n.UpdatedAt = n.CreatedAt
	return n
}

func person(id int64, name, alias string) *entities.AnahitaNode {
	n := entities.NewAnahitaNode()
	n.ID = id
	n.Type = constants.NodeActorPersonEntityType
	n.Name = name
	n.Alias = alias
	return n
}

func account() (*fakeSource, fakeFiles) {
	ana := person(5, `Ana <b>"the grower"</b>`, "ana")
	ana.Body = "I grow beans."
	ana.Avatar = map[string]entities.AnahitaNodeImage{entities.AvatarSizeLarge: {Url: "avatars/5/ana_large.jpg"}}

	public := node(10, "note", "text-service")
	public.Body = "The heap is <script>alert(1)</script> warm"
	public.Access = "public"

	private := node(11, "note", "text-service")
	private.Body = "For me alone"
	private.Access = "myself"

	photo := node(12, "photo", "photo-service")
	photo.Type = constants.NodeMediumPhotoEntityType
	photo.Name = "Sunset"
	photo.Access = "followers"
	photo.Filename = "abc.jpg"
	group := entities.NewAnahitaNode()
	group.ID, group.Type, group.Name = 9, "node.actor.group-service.group.v1", "Growers"
	photo.Owner = group
	photo.OwnerID = 9

	reply := node(13, "note", "text-service")
	reply.Body = "Agreed"
	reply.ParentID, reply.RootID = 77, 70

	video := node(14, "video", "video-service")
	video.Type = videos.NodeType
	video.Name = "A walk"
	video.Access = "public"
	video.Meta, _ = videos.IntoMeta(nil, &videos.Video{
		Renditions: []videos.Rendition{{Height: 720, Key: "videos/5/x/720.mp4"}, {Height: 360, Key: "videos/5/x/360.mp4"}},
	})

	repost := entities.NewAnahitaNode()
	repost.ID, repost.Type, repost.CreatedBy, repost.ParentID = 15, constants.NodeBaseRepostType, 5, 80
	repost.Access = "public"
	repost.CreatedAt = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	repost.Parent = node(80, "note", "text-service")
	repost.Parent.Name = "Bob's note"

	first, _ := photo.PortraitsFor("abc.jpg")
	second, _ := photo.PortraitsFor("def.jpg")

	source := &fakeSource{
		profile:  ana,
		authored: []*entities.AnahitaNode{public, private, photo, reply, video, repost},
		files: []*entities.PhotoFile{
			{NodeID: 12, Position: 0, Filename: "abc.jpg"},
			{NodeID: 12, Position: 1, Filename: "def.jpg"},
		},
		connected: map[string][]*entities.AnahitaNode{
			Following: {person(6, "Bob", "bob")},
			Followers: {person(6, "Bob", "bob"), person(7, "Carol", "carol")},
			Blocked:   {person(8, "Dan", "dan")},
			Liked:     {node(81, "note", "text-service")},
		},
		saved: []*entities.AnahitaNode{node(82, "article", "text-service")},
	}
	files := fakeFiles{
		"avatars/5/ana_large.jpg":                "the avatar",
		first[entities.PortraitSizeOriginal].Url: "the first photo, whole",
		// Kept before the largest size was: only a smaller copy is there.
		second[entities.PortraitSizeXLarge].Url: "the second photo, smaller",
		"videos/5/x/720.mp4":                    "the video",
	}
	return source, files
}

func build(t *testing.T, source *fakeSource, files fakeFiles) (map[string]string, *Summary) {
	t.Helper()

	var buffer bytes.Buffer
	b := &Builder{
		Source:   source,
		Files:    files,
		WebURL:   "https://www.example.org",
		SiteName: "The Allotment",
		Now:      func() time.Time { return time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC) },
	}
	summary, err := b.Write(context.Background(), &buffer, 5)
	if err != nil {
		t.Fatal(err)
	}

	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatalf("what was made is not a zip: %v", err)
	}
	entries := map[string]string{}
	for _, file := range reader.File {
		body, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, _ := io.ReadAll(body)
		body.Close()
		entries[file.Name] = string(content)
	}
	return entries, summary
}

func TestAnArchiveHasEverythingAPersonWroteAndTheirFiles(t *testing.T) {
	source, files := account()
	entries, summary := build(t, source, files)

	for _, name := range []string{
		"index.html", "README.txt", "actor.json", "outbox.json",
		"data/profile.json", "data/posts.ndjson", "data/following.json",
		"data/followers.json", "data/blocked.json", "data/liked.json", "data/saved.json",
	} {
		if _, ok := entries[name]; !ok {
			t.Errorf("%s is not in the archive", name)
		}
	}

	// The files, each where its post says, as it was stored.
	for name, want := range map[string]string{
		"media/profile/avatar.jpg": "the avatar",
		"media/12/1.jpg":           "the first photo, whole",
		"media/12/2.jpg":           "the second photo, smaller",
		"media/14/video.mp4":       "the video",
	} {
		if entries[name] != want {
			t.Errorf("%s holds %q", name, entries[name])
		}
	}
	if summary.Posts != 6 || summary.Files != 4 || summary.Missing != 0 {
		t.Errorf("counted %+v", summary)
	}
	// Read in pages until there was nothing more.
	if source.pages != 4 {
		t.Errorf("asked for %d pages of 2 for 6 posts", source.pages)
	}

	// One post a line, what only its author may see among them.
	lines := strings.Split(strings.TrimSpace(entries["data/posts.ndjson"]), "\n")
	if len(lines) != 6 {
		t.Fatalf("%d lines of posts", len(lines))
	}
	posts := map[int64]*Post{}
	for _, line := range lines {
		post := &Post{}
		if err := json.Unmarshal([]byte(line), post); err != nil {
			t.Fatalf("%v: %s", err, line)
		}
		posts[post.ID] = post
	}
	if posts[11].Text != "For me alone" || posts[11].Audience != "myself" {
		t.Errorf("the private note: %+v", posts[11])
	}
	if posts[12].PostedOn == nil || posts[12].PostedOn.Name != "Growers" || len(posts[12].Media) != 2 {
		t.Errorf("the photo in a group: %+v", posts[12])
	}
	if posts[13].Kind != "reply" || posts[13].InReplyTo != 77 || posts[13].Thread != 70 {
		t.Errorf("the reply: %+v", posts[13])
	}
	if posts[15].Kind != "repost" || posts[15].RepostOf == nil || posts[15].RepostOf.ID != 80 {
		t.Errorf("the repost: %+v", posts[15])
	}
	if posts[10].URL != "https://www.example.org/notes/10" || posts[14].URL != "https://www.example.org/videos/14" {
		t.Errorf("addresses: %q %q", posts[10].URL, posts[14].URL)
	}

	if !strings.Contains(entries["data/blocked.json"], `"alias": "dan"`) || !strings.Contains(entries["data/followers.json"], `"alias": "carol"`) {
		t.Errorf("who is blocked and who follows are not listed")
	}
}

func TestThePageShowsSomebodysWordsAsWords(t *testing.T) {
	source, files := account()
	entries, _ := build(t, source, files)
	page := entries["index.html"]

	if strings.Contains(page, "<script>alert(1)</script>") || strings.Contains(page, `<b>"the grower"</b>`) {
		t.Fatalf("a name or a post was written into the page as markup")
	}
	for _, want := range []string{
		"The heap is &lt;script&gt;alert(1)&lt;/script&gt; warm",
		"For me alone",
		`<img src="media/12/1.jpg"`,
		`<video src="media/14/video.mp4"`,
		`<a href="https://www.example.org/@bob">Bob</a>`,
		"Posts (6)", "Followers (2)", "Blocked (1)",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page lacks %s", want)
		}
	}
	// Nothing fetched from anywhere, and nothing that runs.
	for _, never := range []string{"<script", "http://", "//cdn", "@import", "url("} {
		if strings.Contains(page, never) {
			t.Errorf("the page has %q in it", never)
		}
	}
}

func TestTheSamePostsAreThereAsOtherSoftwareReadsThem(t *testing.T) {
	source, files := account()
	entries, _ := build(t, source, files)

	var outbox struct {
		Type  string           `json:"type"`
		Total int              `json:"totalItems"`
		Items []map[string]any `json:"orderedItems"`
	}
	if err := json.Unmarshal([]byte(entries["outbox.json"]), &outbox); err != nil {
		t.Fatal(err)
	}
	if outbox.Type != "OrderedCollection" || outbox.Total != 6 || len(outbox.Items) != 6 {
		t.Fatalf("the outbox: %s %d", outbox.Type, outbox.Total)
	}

	byID := map[string]map[string]any{}
	for _, item := range outbox.Items {
		byID[item["id"].(string)] = item
	}
	public := byID["https://www.example.org/notes/10#create"]
	if public == nil || public["type"] != "Create" || len(public["to"].([]any)) != 1 {
		t.Errorf("the public note: %v", public)
	}
	// What was not public is addressed to nobody.
	private := byID["https://www.example.org/notes/11#create"]
	if private == nil || len(private["to"].([]any)) != 0 {
		t.Errorf("the private note is addressed to %v", private["to"])
	}
	repost := byID["https://www.example.org/@ana#reposts/15"]
	if repost == nil || repost["type"] != "Announce" || repost["object"] != "https://www.example.org/notes/80" {
		t.Errorf("the repost: %v", repost)
	}

	var actor map[string]any
	if err := json.Unmarshal([]byte(entries["actor.json"]), &actor); err != nil {
		t.Fatal(err)
	}
	if actor["type"] != "Person" || actor["preferredUsername"] != "ana" {
		t.Errorf("the actor: %v", actor)
	}
}

func TestAFileStorageDoesNotHaveIsLeftOutAndCounted(t *testing.T) {
	source, files := account()
	delete(files, "videos/5/x/720.mp4")
	entries, summary := build(t, source, files)

	if _, there := entries["media/14/video.mp4"]; there {
		t.Errorf("a file storage did not have is in the archive")
	}
	if summary.Missing != 1 || summary.Files != 3 || summary.Posts != 6 {
		t.Errorf("counted %+v", summary)
	}
	if strings.Contains(entries["index.html"], "media/14/video.mp4") {
		t.Errorf("the page shows a file that is not there")
	}
	if !strings.Contains(entries["README.txt"], "1 files could not be read") {
		t.Errorf("the readme does not say a file was left out")
	}
}

func TestAnExportGivenUpStops(t *testing.T) {
	source, files := account()
	asked := 0
	b := &Builder{
		Source: source,
		Files:  files,
		Stopped: func() bool {
			asked++
			return asked > 1
		},
	}
	if _, err := b.Write(context.Background(), io.Discard, 5); !errors.Is(err, ErrStopped) {
		t.Errorf("went on after being given up: %v", err)
	}

	// Nobody there: nothing made.
	b = &Builder{Source: &fakeSource{}, Files: files}
	if _, err := b.Write(context.Background(), io.Discard, 5); err == nil {
		t.Errorf("an archive was made of nobody")
	}
}
