package federation

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	images "anahita-libs/utils"
)

// Keeper copies the pictures of people and posts from other servers into
// this installation's storage, whichever protocol they came by.
type Keeper struct {
	Nodes      services.GraphNodeGRPCInterface
	Files      services.FilesGRPCInterface
	PhotoFiles PhotoFiles
	Pictures   Pictures
	// Whether a server is one this installation refuses. Nil for none.
	Refused func(host string) bool
}

// Picture is one picture of a post from elsewhere: where it is, and what
// its author said it shows.
type Picture struct {
	URL string
	Alt string
}

func (k *Keeper) refused(address string) bool {
	if k.Refused == nil {
		return false
	}
	parsed, err := url.Parse(address)
	if err != nil {
		return false
	}
	return k.Refused(strings.ToLower(parsed.Hostname()))
}

func cutRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}

// The pictures of somebody on another server: their avatar, and the wide
// one across the top of their profile.
//
// Each is copied here, once, and shown from this installation's own storage
// like anybody's avatar. It is never shown from where it came from: a
// picture loaded straight from another server tells that server who looked
// at whom, and when, for every reader. The same rule as the picture on a
// link's card.
//
// What is copied is not what was sent: it is decoded, checked to be a
// picture and nothing else, and written again at this installation's own
// sizes, as an avatar uploaded here is.

const (
	// The most a picture may weigh as it comes, and the most pixels it may
	// say it has.
	avatarMaxBytes  = 5 << 20
	avatarMaxPixels = 40_000_000
	// The most one stored size may weigh: what the file service takes in
	// one message.
	avatarMaxStored = 4*1024*1024 - 64*1024
)

// picture is one of the two: where its stored name is kept on a node, and
// the sizes it is stored at.
type picture struct {
	name     string
	filename func(node *entities.AnahitaNode) string
	rename   func(node *entities.AnahitaNode, filename string)
	sizes    func(node *entities.AnahitaNode) (map[string]entities.AnahitaNodeImage, error)
}

var (
	avatarPicture = picture{
		name:     "avatar",
		filename: func(node *entities.AnahitaNode) string { return node.AvatarFilename },
		rename:   func(node *entities.AnahitaNode, filename string) { node.AvatarFilename = filename },
		sizes: func(node *entities.AnahitaNode) (map[string]entities.AnahitaNodeImage, error) {
			return node.GetAvatars()
		},
	}
	coverPicture = picture{
		name:     "cover",
		filename: func(node *entities.AnahitaNode) string { return node.CoverFilename },
		rename:   func(node *entities.AnahitaNode, filename string) { node.CoverFilename = filename },
		sizes: func(node *entities.AnahitaNode) (map[string]entities.AnahitaNodeImage, error) {
			return node.GetCovers()
		},
	}
)

// Pictures is what fetches a picture from another server.
type Pictures interface {
	Bytes(ctx context.Context, address string, max int64) ([]byte, string, error)
}

// CopyAvatar fetches somebody's picture and keeps it as their avatar here.
func (k *Keeper) CopyAvatar(nodeID int64, address string) {
	k.copyPicture(nodeID, address, avatarPicture)
}

// CopyCover fetches the wide picture across the top of somebody's profile
// and keeps it as their cover here.
func (k *Keeper) CopyCover(nodeID int64, address string) {
	k.copyPicture(nodeID, address, coverPicture)
}

// copyPicture fetches one of somebody's pictures and keeps it here.
// Whatever goes wrong, they are left as they were: with the picture they
// had, or none.
func (k *Keeper) copyPicture(nodeID int64, address string, which picture) {
	if k.Pictures == nil || k.Files == nil || address == "" {
		return
	}
	if k.refused(address) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	node, err := k.Nodes.Read(nodeID)
	if err != nil || node == nil || !node.IsRemote() {
		return
	}

	raw, _, err := k.Pictures.Bytes(ctx, address, avatarMaxBytes)
	if err != nil {
		log.Printf("the %s of %s was not fetched: %v", which.name, node.Alias, err)
		return
	}
	// Whatever the server said it was, it is looked at.
	content, mimeType, err := images.Accept(raw, avatarMaxBytes, avatarMaxPixels)
	if err != nil {
		log.Printf("the %s of %s is not one that is kept: %v", which.name, node.Alias, err)
		return
	}

	// What they had, to remove once the new one is in place.
	var previous map[string]entities.AnahitaNodeImage
	if which.filename(node) != "" {
		previous, _ = which.sizes(node)
	}

	filename, err := images.GenerateUniqueFilename(mimeType)
	if err != nil {
		return
	}
	which.rename(node, filename)
	sizes, err := which.sizes(node)
	if err != nil {
		return
	}

	widest := 0
	for _, size := range sizes {
		widest = max(widest, size.Size.Width)
	}
	largest, err := images.FitImage(content, widest)
	if err != nil {
		log.Printf("the picture of %s could not be made smaller: %v", node.Alias, err)
		return
	}

	var stored []string
	for _, size := range sizes {
		var resized []byte
		if size.Size.Height > 0 && size.Size.Height == size.Size.Width {
			resized, err = images.SquareImage(largest, size.Size.Width)
		} else {
			resized, err = images.FitImage(largest, size.Size.Width)
		}
		if err == nil && len(resized) > avatarMaxStored {
			err = images.ErrTooLarge
		}
		if err == nil {
			add := services.NewFileRequest().Add
			add.Path, add.Content, add.Mimetype, add.Size = size.Url, resized, mimeType, int64(len(resized))
			_, err = k.Files.Add(add)
		}
		if err != nil {
			log.Printf("the picture of %s was not stored: %v", node.Alias, err)
			k.DeleteFiles(stored)
			return
		}
		stored = append(stored, size.Url)
	}

	// Named on the node only now, when every size is there to be shown.
	update := entities.NewAnahitaNode()
	update.Type, update.URI, update.OriginDomain = node.Type, node.URI, node.OriginDomain
	update.Name, update.Alias, update.Body, update.WebsiteURL = node.Name, node.Alias, node.Body, node.WebsiteURL
	which.rename(update, filename)
	if _, _, err := k.Nodes.UpsertRemote(update); err != nil {
		log.Printf("the picture of %s was stored and not named: %v", node.Alias, err)
		k.DeleteFiles(stored)
		return
	}

	var old []string
	for _, size := range previous {
		old = append(old, size.Url)
	}
	k.DeleteFiles(old)
}

// DeleteActorPictures removes what is stored of somebody's pictures, both of them.
func (k *Keeper) DeleteActorPictures(node *entities.AnahitaNode) {
	if node == nil {
		return
	}
	for _, which := range []picture{avatarPicture, coverPicture} {
		if which.filename(node) == "" {
			continue
		}
		sizes, err := which.sizes(node)
		if err != nil {
			continue
		}
		var stored []string
		for _, size := range sizes {
			stored = append(stored, size.Url)
		}
		k.DeleteFiles(stored)
	}
}

// DeleteFiles removes what is stored at each path.
func (k *Keeper) DeleteFiles(paths []string) {
	if k.Files == nil {
		return
	}
	for _, path := range paths {
		remove := services.NewFileRequest().Delete
		remove.Path = path
		if err := k.Files.Delete(remove); err != nil {
			log.Printf("deleting %s failed: %v", path, err)
		}
	}
}

// The pictures of a post from another server.
//
// A post that arrives with pictures is kept as a photo post, which is what
// a post with pictures is here: its words are the caption, and up to four
// pictures are its images, each with what its author said it shows. So it
// is drawn, opened and stepped through like any photo post, and is in its
// author's photos.
//
// The pictures are copied into this installation's storage, never shown
// from where they came from: a picture loaded straight from another server
// tells that server who looked at what. Each is checked to be a picture
// and nothing else and written out again at this installation's own sizes,
// as an upload is, so nothing a file carries besides its picture is kept.
//
// What its author marked as sensitive is kept without its pictures: there
// is nothing here yet that puts a picture behind a warning, and showing it
// plainly is not this installation's to decide.

const (
	// The most pictures of one post that are kept: what a photo post holds.
	postPicturesMax = 4
	// The limits an uploaded photo is held to (photo-service/constants).
	postPictureMaxBytes  = 10 << 20
	postPictureMaxPixels = 50_000_000
	postPictureMaxWidth  = 2560
	postPictureMaxStored = 4<<20 - 64<<10
)

// PhotoFiles is where a post's list of pictures is kept: the graph.
type PhotoFiles interface {
	BrowsePhotoFiles(nodeIDs []int64) ([]*entities.PhotoFile, error)
	SetPhotoFiles(nodeID int64, files []*entities.PhotoFile, uploadIDs []int64) ([]*entities.PhotoFile, error)
}

// CanKeepPictures says whether there is what it takes to keep a post's
// pictures at all.
func (k *Keeper) CanKeepPictures() bool {
	return k.Pictures != nil && k.Files != nil && k.PhotoFiles != nil
}

// PostPicturesMax is the most pictures of one post that are kept.
const PostPicturesMax = postPicturesMax

// CopyPostPictures fetches a post's pictures and keeps them as its images.
// Whatever goes wrong with one, the others are kept; with all of them, the
// post is left with its words.
func (k *Keeper) CopyPostPictures(post *entities.AnahitaNode, pictures []Picture) {
	if post == nil || post.ID == 0 || post.Type != constants.NodeMediumPhotoEntityType || len(pictures) == 0 {
		return
	}
	// Kept already: a post is told of more than once, and its pictures do
	// not change.
	if have, err := k.PhotoFiles.BrowsePhotoFiles([]int64{post.ID}); err != nil || len(have) > 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var (
		files  []*entities.PhotoFile
		stored []string
	)
	for _, picture := range pictures {
		if k.refused(picture.URL) {
			continue
		}
		file, paths := k.copyPostPicture(ctx, post, picture)
		if file == nil {
			continue
		}
		files = append(files, file)
		stored = append(stored, paths...)
	}
	if len(files) == 0 {
		return
	}

	if _, err := k.PhotoFiles.SetPhotoFiles(post.ID, files, nil); err != nil {
		log.Printf("the pictures of %s were stored and not listed: %v", post.URI, err)
		k.DeleteFiles(stored)
	}
}

// copyPostPicture keeps one picture at every size a photo is kept at, and
// answers with what to list it as and where it was stored.
func (k *Keeper) copyPostPicture(ctx context.Context, post *entities.AnahitaNode, picture Picture) (*entities.PhotoFile, []string) {
	raw, _, err := k.Pictures.Bytes(ctx, picture.URL, postPictureMaxBytes)
	if err != nil {
		log.Printf("a picture of %s was not fetched: %v", post.URI, err)
		return nil, nil
	}
	// Whatever the server said it was, it is looked at.
	content, mimeType, err := images.Accept(raw, postPictureMaxBytes, postPictureMaxPixels)
	if err != nil {
		log.Printf("a picture of %s is not one that is kept: %v", post.URI, err)
		return nil, nil
	}

	filename, err := images.GenerateUniqueFilename(mimeType)
	if err != nil {
		return nil, nil
	}
	sizes, err := post.PortraitsFor(filename)
	if err != nil {
		return nil, nil
	}

	largest, err := images.FitImage(content, postPictureMaxWidth)
	if err != nil {
		return nil, nil
	}
	width, height, err := images.Dimensions(largest)
	if err != nil {
		return nil, nil
	}

	var stored []string
	for name, size := range sizes {
		var resized []byte
		switch name {
		case entities.PortraitSizeSquare:
			resized, err = images.SquareImage(largest, size.Size.Width)
		case entities.PortraitSizeOriginal:
			resized = largest
		default:
			resized, err = images.FitImage(largest, size.Size.Width)
		}
		if err == nil && len(resized) > postPictureMaxStored {
			err = images.ErrTooLarge
		}
		if err == nil {
			add := services.NewFileRequest().Add
			add.Path, add.Content, add.Mimetype, add.Size = size.Url, resized, mimeType, int64(len(resized))
			_, err = k.Files.Add(add)
		}
		if err != nil {
			log.Printf("a picture of %s was not stored: %v", post.URI, err)
			k.DeleteFiles(stored)
			return nil, nil
		}
		stored = append(stored, size.Url)
	}

	return &entities.PhotoFile{
		Filename: filename,
		Mimetype: mimeType,
		Width:    width,
		Height:   height,
		AltText:  cutRunes(picture.Alt, 1500),
	}, stored
}

// DeletePostPictures removes what is stored of a post's pictures, once the
// post itself is gone. `was` is the post as it was, and `files` its list.
func (k *Keeper) DeletePostPictures(was *entities.AnahitaNode, files []*entities.PhotoFile) {
	var stored []string
	for _, file := range files {
		sizes, err := was.PortraitsFor(file.Filename)
		if err != nil {
			continue
		}
		for _, size := range sizes {
			stored = append(stored, size.Url)
		}
	}
	k.DeleteFiles(stored)
}
