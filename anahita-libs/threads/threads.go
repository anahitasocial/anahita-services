// Package threads is a thread written in one go (anahita-web roadmap rows
// 101 and 143): several notes posted together, in order, each after the
// first a reply to the one before. text-service makes one when it is asked
// to; video-service's worker makes the notes after a GIF, a video or a sound
// once its file is ready, since they wait for it.
package threads

import (
	"encoding/json"
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/handlers"
	"anahita-libs/photos"
)

// Part is one note of a thread: what it says, in what language, and the
// link in it to show a card for, as on a note; or, with pictures, a photo
// post in the thread (roadmap row 143), which needs no words.
type Part struct {
	// Its words, as Markdown. A part with pictures need not have any. Up to
	// 500 characters, a link counted as 23.
	Content string `json:"content" validate:"omitempty,max=8000,post_length"`
	// The language it is written in, as a BCP 47 tag.
	Language string `json:"language" validate:"omitempty,max=12"`
	// The link in it to show a card for. Not on a part with pictures.
	LinkPreviewURL string `json:"link_preview_url" validate:"omitempty,max=2000"`
	// Its pictures, as a photo post's: each uploaded first through
	// POST /photos/:owner_id/uploads, for the profile the part is on, which
	// is the thread's for the first part and the writer's for the rest.
	Uploads []photos.FileRef `json:"uploads" validate:"omitempty,max=20"`
}

// HasPictures is whether a part is a photo post.
func (p Part) HasPictures() bool {
	return len(p.Uploads) > 0
}

// SaysSomething is whether a part has words or pictures.
func (p Part) SaysSomething() bool {
	return strings.TrimSpace(p.Content) != "" || p.HasPictures()
}

// CardURL is the link to show a card for: none on a part with pictures,
// which show in its place.
func (p Part) CardURL() string {
	if p.HasPictures() {
		return ""
	}
	return p.LinkPreviewURL
}

// EachSaysSomething is whether every part has words or pictures.
func EachSaysSomething(parts []Part) bool {
	for _, each := range parts {
		if !each.SaysSomething() {
			return false
		}
	}
	return true
}

// Next is a note after the first: a reply to the one before, written by
// whoever writes the thread. It belongs to them, as any reply does; or, in
// a website's thread written as the website, to the website, as the first
// part does. ok is false for a language that is not one.
//
// The thread's content warning goes on: a thread's is every note's.
func Next(first *entities.AnahitaNode, each Part, writerID int64) (*entities.AnahitaNode, bool) {
	part := entities.NewAnahitaNode()
	part.Type = constants.NodeMediumNoteEntityType
	part.Service = constants.TextService
	part.Body = each.Content
	language, valid := handlers.RequestLanguage(each.Language)
	if !valid {
		return nil, false
	}
	part.Language = language
	part.Enabled = true
	part.CommentStatus = true
	part.Warning = first.Warning
	part.WarningText = first.WarningText
	part.ModifiedBy = writerID
	if first.OwnerType == constants.NodeActorWebsiteEntityType {
		part.OwnerID = first.OwnerID
		part.OwnerType = first.OwnerType
		part.CreatedBy = first.OwnerID
		part.Meta = handlers.WithWrittenBy(part.Meta, writerID)
	} else {
		part.OwnerID = writerID
		part.OwnerType = constants.NodeActorPersonEntityType
		part.CreatedBy = writerID
	}
	return part, true
}

// Readers are what the pictures of a thread are checked with.
type Readers struct {
	Media  services.MediaGRPCInterface
	Actors services.ActorsGRPCInterface
}

// Pictures makes each part with pictures a photo post, its pictures checked
// by the rules a photo post's are (anahita-libs/photos): uploaded by the
// writer, for the profile the part is on, none twice in the thread. It
// answers each one's images, by its place, for the graph to set with the
// thread in one go; or the refusal (photos.Err…), or an error reading.
//
// `allowDownload` is what the writer said of the first part's pictures;
// the rest take what the profile they are on says.
func Pictures(r Readers, parts []*entities.AnahitaNode, all []Part, allowDownload *bool, viewer *entities.Viewer) (map[int]services.ThreadPartFiles, string, error) {
	var refs []photos.FileRef
	for _, each := range all {
		refs = append(refs, each.Uploads...)
	}
	if len(refs) == 0 {
		return nil, "", nil
	}

	// An upload is stored once, and can be one picture of one post.
	seen := map[int64]bool{}
	for _, id := range photos.UploadIDs(refs) {
		if seen[id] {
			return nil, photos.ErrFileRepeated, nil
		}
		seen[id] = true
	}

	uploads, err := r.Media.ReadPhotoUploads(photos.UploadIDs(refs))
	if err != nil {
		return nil, "", err
	}

	var author *entities.AnahitaNode
	files := map[int]services.ThreadPartFiles{}
	for position, each := range all {
		if !each.HasPictures() {
			continue
		}
		part := parts[position]
		plan, refused := photos.PlanFiles(nil, uploads, each.Uploads, viewer.ID, part.OwnerID)
		if refused != "" {
			return nil, refused, nil
		}

		owner := part.Owner
		if owner == nil || owner.ID != part.OwnerID {
			if author == nil || author.ID != part.OwnerID {
				if author, err = r.Actors.Read(part.OwnerID, viewer); err != nil {
					return nil, "", err
				}
			}
			owner = author
		}

		part.Type = constants.NodeMediumPhotoEntityType
		part.Service = constants.PhotoServiceName
		said := (*bool)(nil)
		if position == 0 {
			said = allowDownload
		}
		part.AllowDownload = handlers.DefaultAllowDownload(said, owner)
		// The post names its first image itself, for everything that
		// shows one.
		part.Filename = plan.Files[0].Filename
		part.Mimetype = plan.Files[0].Mimetype
		files[position] = services.ThreadPartFiles{Files: plan.Files, UploadIDs: plan.UploadIDs}
	}
	return files, "", nil
}

// Rest is the parts after a GIF, a video or a sound, as kept with its
// upload until its file is ready (video_uploads.thread_after).
func Rest(raw string) []Part {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var parts []Part
	if err := json.Unmarshal([]byte(raw), &parts); err != nil {
		return nil
	}
	return parts
}
