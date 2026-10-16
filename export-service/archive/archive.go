// Package archive makes a person's copy of their own data: one zip file
// with everything they wrote, the pictures and videos they posted, who they
// follow and who follows them, and a page to read it all from.
//
// It is written straight into whatever it is given, a file as it stands,
// and never held whole in memory: somebody with years of photographs has an
// archive far larger than the machine making it has to spare.
//
// What is in it:
//
//	index.html          a page to read it from, with nothing fetched from
//	                    anywhere: it works with no connection
//	README.txt          what each file is
//	data/profile.json   the profile
//	data/posts.ndjson   everything written, one post on each line
//	data/following.json, followers.json, blocked.json, liked.json, saved.json
//	actor.json, outbox.json   the same posts as ActivityStreams, the form
//	                    other fediverse software reads
//	media/              the pictures and videos, by the post they are of
package archive

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/videos"
)

// Source is where a person's data is read from.
type Source interface {
	Profile(personID int64) (*entities.AnahitaNode, error)
	// Authored is what they wrote, lowest id first, after an id.
	Authored(personID, afterID int64, limit uint64) ([]*entities.AnahitaNode, error)
	// PhotoFiles are the images of these photo posts.
	PhotoFiles(postIDs []int64) ([]*entities.PhotoFile, error)
	// Connected is a page of the people or posts they are connected to in
	// one way: Following, Followers, Blocked or Liked.
	Connected(personID int64, kind string, offset uint64) ([]*entities.AnahitaNode, error)
	// Saved is a page of the posts they saved.
	Saved(personID int64, offset uint64) ([]*entities.AnahitaNode, error)
}

// Files opens a stored file. ErrNoFile when there is none by that key.
type Files interface {
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

var ErrNoFile = errors.New("there is no such file")

// The ways a person is connected to others and to posts.
const (
	Following = "following"
	Followers = "followers"
	Blocked   = "blocked"
	Liked     = "liked"
)

// PageSize is how many are asked for at a time.
const PageSize = 100

// ErrStopped is the archive given up while it was being made.
var ErrStopped = errors.New("the export was stopped")

// Builder makes archives.
type Builder struct {
	Source Source
	Files  Files
	// The site's address, "https://www.example.org", for the links in the
	// archive back to where each thing is.
	WebURL string
	// The site's name, for the page's heading.
	SiteName string
	// Asked now and then: true to give up.
	Stopped func() bool
	Now     func() time.Time
}

// Summary is what went into an archive.
type Summary struct {
	Posts int
	Files int
	// Files a post names that storage did not have. Left out, and counted,
	// so that one missing picture does not cost somebody their archive.
	Missing int
}

// Post is one thing somebody wrote, as it is in data/posts.ndjson.
type Post struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title,omitempty"`
	Text      string    `json:"text,omitempty"`
	// Who can see it: "public", "followers" and so on. Empty on a reply,
	// which is seen by whoever sees the post it is under.
	Audience string `json:"audience,omitempty"`
	Language string `json:"language,omitempty"`
	// The profile it was posted on, when that is not the author's own.
	PostedOn *Ref `json:"posted_on,omitempty"`
	// What it answers, the post at the top of that thread, what it quotes,
	// and what it reposts.
	InReplyTo int64 `json:"in_reply_to,omitempty"`
	Thread    int64 `json:"thread,omitempty"`
	Quotes    int64 `json:"quotes,omitempty"`
	RepostOf  *Ref  `json:"repost_of,omitempty"`
	// Its pictures and videos, as paths in this archive.
	Media         []string `json:"media,omitempty"`
	AllowDownload bool     `json:"allow_download,omitempty"`
	Likes         int64    `json:"likes"`
	Replies       int64    `json:"replies"`
	Reposts       int64    `json:"reposts"`
}

// Ref names somebody or something else: enough to recognise it and find it.
type Ref struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	// A person's handle.
	Alias string `json:"alias,omitempty"`
	URL   string `json:"url"`
}

// kindOf is the short word for a type: "note", "photo", "person".
func kindOf(nodeType string) string {
	parts := strings.Split(nodeType, ".")
	if len(parts) >= 4 {
		return parts[3]
	}
	return nodeType
}

// urlOf is where something is on the site, as the web app addresses it.
func (b *Builder) urlOf(node *entities.AnahitaNode) string {
	kind := kindOf(node.Type)
	plural := kind + "s"
	switch kind {
	case "person":
		return b.WebURL + "/@" + node.Alias
	case "generator":
		return fmt.Sprintf("%s/feeds/%d", b.WebURL, node.ID)
	case "repost":
		return ""
	}
	return fmt.Sprintf("%s/%s/%d", b.WebURL, plural, node.ID)
}

func (b *Builder) refOf(node *entities.AnahitaNode) *Ref {
	if node == nil || node.ID == 0 {
		return nil
	}
	return &Ref{
		ID:    node.ID,
		Kind:  kindOf(node.Type),
		Name:  node.Name,
		Alias: node.Alias,
		URL:   b.urlOf(node),
	}
}

func (b *Builder) stopped() bool {
	return b.Stopped != nil && b.Stopped()
}

// Write makes the archive of one person's data into w.
func (b *Builder) Write(ctx context.Context, w io.Writer, personID int64) (*Summary, error) {
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}

	profile, err := b.Source.Profile(personID)
	if err != nil || profile == nil || profile.ID == 0 {
		return nil, fmt.Errorf("the profile could not be read: %v", err)
	}

	z := zip.NewWriter(w)
	summary := &Summary{}
	page := &pageData{
		Site:    b.SiteName,
		Made:    now().UTC(),
		Profile: profile,
		URL:     b.urlOf(profile),
	}

	// The profile's own pictures.
	page.Avatar = b.addFile(ctx, z, summary, "media/profile/avatar", keyOf(profile.Avatar, entities.AvatarSizeLarge))
	page.Cover = b.addFile(ctx, z, summary, "media/profile/cover", keyOf(profile.Cover, entities.CoverSizeLarge))

	if err := writeJSON(z, "data/profile.json", b.profileOf(profile, page)); err != nil {
		return nil, err
	}

	// Everything written, a page at a time: each post to its line of the
	// list, with its files beside it.
	posts, err := z.Create("data/posts.ndjson")
	if err != nil {
		return nil, err
	}
	// Written whole, while the list of posts is being written, into another
	// entry of the zip is not possible: an entry is finished before the
	// next is begun. So the posts are kept for the two files made of them
	// afterwards, without their files, which are written as they are met.
	var kept []*Post
	var pending []pendingMedia

	after := int64(0)
	for {
		if b.stopped() {
			return nil, ErrStopped
		}
		nodes, err := b.Source.Authored(personID, after, PageSize)
		if err != nil {
			return nil, err
		}
		if len(nodes) == 0 {
			break
		}

		files, err := b.photoFiles(nodes)
		if err != nil {
			return nil, err
		}

		for _, node := range nodes {
			after = node.ID
			post, media := b.postOf(node, files[node.ID])
			pending = append(pending, media...)
			kept = append(kept, post)

			line, err := json.Marshal(post)
			if err != nil {
				return nil, err
			}
			if _, err := posts.Write(append(line, '\n')); err != nil {
				return nil, err
			}
			summary.Posts++
		}
	}

	// Their files, now that the list is finished. One that storage does
	// not have is taken off its post's list on the page.
	have := map[string]bool{}
	for _, media := range pending {
		if b.stopped() {
			return nil, ErrStopped
		}
		if b.addFile(ctx, z, summary, strings.TrimSuffix(media.path, path.Ext(media.path)), media.keys...) != "" {
			have[media.path] = true
		}
	}
	for _, post := range kept {
		present := post.Media[:0]
		for _, file := range post.Media {
			if have[file] {
				present = append(present, file)
			}
		}
		post.Media = present
	}
	page.Posts = kept

	for _, list := range []struct {
		kind string
		into *[]*Ref
	}{
		{Following, &page.Following},
		{Followers, &page.Followers},
		{Blocked, &page.Blocked},
		{Liked, &page.Liked},
	} {
		refs, err := b.connected(personID, list.kind)
		if err != nil {
			return nil, err
		}
		*list.into = refs
		if err := writeJSON(z, "data/"+list.kind+".json", refs); err != nil {
			return nil, err
		}
	}

	saved, err := b.saved(personID)
	if err != nil {
		return nil, err
	}
	page.Saved = saved
	if err := writeJSON(z, "data/saved.json", saved); err != nil {
		return nil, err
	}

	if err := writeJSON(z, "actor.json", b.actorOf(profile, page)); err != nil {
		return nil, err
	}
	if err := writeJSON(z, "outbox.json", b.outboxOf(profile, kept)); err != nil {
		return nil, err
	}

	page.Summary = summary
	if err := writePage(z, page); err != nil {
		return nil, err
	}
	if err := writeText(z, "README.txt", readme(page)); err != nil {
		return nil, err
	}

	return summary, z.Close()
}

// pendingMedia is a file to put in the archive: where, and the keys it may
// be under in storage, the best first.
type pendingMedia struct {
	path string
	keys []string
}

func keyOf(images map[string]entities.AnahitaNodeImage, size string) string {
	if image, ok := images[size]; ok {
		return image.Url
	}
	return ""
}

// The sizes of a photo to look for, the largest first: an image stored
// before the largest was kept has only the smaller ones.
var photoSizes = []string{
	entities.PortraitSizeOriginal,
	entities.PortraitSizeXXLarge,
	entities.PortraitSizeXLarge,
	entities.PortraitSizeLarge,
}

func (b *Builder) photoFiles(nodes []*entities.AnahitaNode) (map[int64][]*entities.PhotoFile, error) {
	var ids []int64
	for _, node := range nodes {
		if node.Type == constants.NodeMediumPhotoEntityType {
			ids = append(ids, node.ID)
		}
	}
	byPost := map[int64][]*entities.PhotoFile{}
	if len(ids) == 0 {
		return byPost, nil
	}

	files, err := b.Source.PhotoFiles(ids)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		byPost[file.NodeID] = append(byPost[file.NodeID], file)
	}
	return byPost, nil
}

// postOf describes one thing somebody wrote, and lists its files.
func (b *Builder) postOf(node *entities.AnahitaNode, files []*entities.PhotoFile) (*Post, []pendingMedia) {
	post := &Post{
		ID:            node.ID,
		Kind:          kindOf(node.Type),
		URL:           b.urlOf(node),
		CreatedAt:     node.CreatedAt.UTC(),
		UpdatedAt:     node.UpdatedAt.UTC(),
		Title:         node.Name,
		Text:          node.Body,
		Audience:      node.Access,
		Language:      node.Language,
		Quotes:        node.QuoteID,
		AllowDownload: node.AllowDownload,
		Likes:         node.LikesCount,
		Replies:       node.CommentCount,
		Reposts:       node.RepostCount,
	}

	if node.Owner != nil && node.Owner.ID != 0 && node.Owner.ID != node.CreatedBy {
		post.PostedOn = b.refOf(node.Owner)
	}

	switch {
	case node.Type == constants.NodeBaseRepostType:
		post.RepostOf = b.refOf(node.Parent)
		if post.RepostOf == nil && node.ParentID > 0 {
			post.RepostOf = &Ref{ID: node.ParentID, Kind: "post"}
		}
	case node.RootID > 0:
		post.InReplyTo = node.ParentID
		post.Thread = node.RootID
		post.Kind = "reply"
	}

	var media []pendingMedia
	folder := fmt.Sprintf("media/%d/", node.ID)

	switch node.Type {
	case constants.NodeMediumPhotoEntityType:
		names := make([]string, 0, len(files)+1)
		for _, file := range files {
			names = append(names, file.Filename)
		}
		if len(names) == 0 && node.Filename != "" {
			names = append(names, node.Filename)
		}
		for i, name := range names {
			portraits, err := node.PortraitsFor(name)
			if err != nil {
				continue
			}
			var keys []string
			for _, size := range photoSizes {
				if key := keyOf(portraits, size); key != "" {
					keys = append(keys, key)
				}
			}
			if len(keys) == 0 {
				continue
			}
			entry := fmt.Sprintf("%s%d%s", folder, i+1, path.Ext(keys[0]))
			media = append(media, pendingMedia{path: entry, keys: keys})
			post.Media = append(post.Media, entry)
		}
	case videos.NodeType:
		// The largest size it has: the others are the same video, smaller.
		if video := videos.FromMeta(node.Meta); video != nil {
			rendition := video.Renditions[0]
			entry := fmt.Sprintf("%svideo.mp4", folder)
			media = append(media, pendingMedia{path: entry, keys: []string{rendition.Key}})
			post.Media = append(post.Media, entry)
		}
	default:
		// An article's cover.
		if key := keyOf(node.Cover, entities.CoverSizeLarge); key != "" {
			entry := fmt.Sprintf("%scover%s", folder, path.Ext(key))
			media = append(media, pendingMedia{path: entry, keys: []string{key}})
			post.Media = append(post.Media, entry)
		}
	}

	return post, media
}

// addFile copies a stored file into the archive under a name, with the
// ending of whichever of its keys was there. It answers with the path it
// was put at, or "" when storage had none of them.
func (b *Builder) addFile(ctx context.Context, z *zip.Writer, summary *Summary, name string, keys ...string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		body, err := b.Files.Open(ctx, key)
		if err != nil {
			continue
		}

		entry := name + path.Ext(key)
		// Stored as it is: a picture or a video does not get smaller for
		// being compressed again, and it takes a long while to try.
		out, err := z.CreateHeader(&zip.FileHeader{Name: entry, Method: zip.Store})
		if err == nil {
			_, err = io.Copy(out, body)
		}
		body.Close()
		if err != nil {
			// The entry is in the archive, cut short. Nothing can take it
			// out again, and the archive is still good without it.
			summary.Missing++
			return ""
		}
		summary.Files++
		return entry
	}

	hasKey := false
	for _, key := range keys {
		hasKey = hasKey || key != ""
	}
	if hasKey {
		summary.Missing++
	}
	return ""
}

func (b *Builder) connected(personID int64, kind string) ([]*Ref, error) {
	refs := []*Ref{}
	for offset := uint64(0); ; offset += PageSize {
		if b.stopped() {
			return nil, ErrStopped
		}
		nodes, err := b.Source.Connected(personID, kind, offset)
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			if ref := b.refOf(node); ref != nil {
				refs = append(refs, ref)
			}
		}
		if len(nodes) < PageSize {
			return refs, nil
		}
	}
}

func (b *Builder) saved(personID int64) ([]*Ref, error) {
	refs := []*Ref{}
	for offset := uint64(0); ; offset += PageSize {
		if b.stopped() {
			return nil, ErrStopped
		}
		nodes, err := b.Source.Saved(personID, offset)
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			if ref := b.refOf(node); ref != nil {
				refs = append(refs, ref)
			}
		}
		if len(nodes) < PageSize {
			return refs, nil
		}
	}
}

func writeJSON(z *zip.Writer, name string, value any) error {
	out, err := z.Create(name)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeText(z *zip.Writer, name, text string) error {
	out, err := z.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
