package handlers

import (
	"context"
	"log"
	"time"

	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	images "anahita-libs/utils"
)

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

// copyAvatar fetches somebody's picture and keeps it as their avatar here.
func (h *Handlers) copyAvatar(nodeID int64, address string) {
	h.copyPicture(nodeID, address, avatarPicture)
}

// copyPicture fetches one of somebody's pictures and keeps it here.
// Whatever goes wrong, they are left as they were: with the picture they
// had, or none.
func (h *Handlers) copyPicture(nodeID int64, address string, which picture) {
	if h.Pictures == nil || h.Files == nil || address == "" {
		return
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(address)) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	node, err := h.Nodes.Read(nodeID)
	if err != nil || node == nil || !node.IsRemote() {
		return
	}

	raw, _, err := h.Pictures.Bytes(ctx, address, avatarMaxBytes)
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
			_, err = h.Files.Add(add)
		}
		if err != nil {
			log.Printf("the picture of %s was not stored: %v", node.Alias, err)
			h.deleteFiles(stored)
			return
		}
		stored = append(stored, size.Url)
	}

	// Named on the node only now, when every size is there to be shown.
	update := entities.NewAnahitaNode()
	update.Type, update.URI, update.OriginDomain = node.Type, node.URI, node.OriginDomain
	update.Name, update.Alias, update.Body, update.WebsiteURL = node.Name, node.Alias, node.Body, node.WebsiteURL
	which.rename(update, filename)
	if _, _, err := h.Nodes.UpsertRemote(update); err != nil {
		log.Printf("the picture of %s was stored and not named: %v", node.Alias, err)
		h.deleteFiles(stored)
		return
	}

	var old []string
	for _, size := range previous {
		old = append(old, size.Url)
	}
	h.deleteFiles(old)
}

// deleteAvatar removes what is stored of somebody's pictures, both of them.
func (h *Handlers) deleteAvatar(node *entities.AnahitaNode) {
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
		h.deleteFiles(stored)
	}
}

func (h *Handlers) deleteFiles(paths []string) {
	if h.Files == nil {
		return
	}
	for _, path := range paths {
		remove := services.NewFileRequest().Delete
		remove.Path = path
		if err := h.Files.Delete(remove); err != nil {
			log.Printf("deleting %s failed: %v", path, err)
		}
	}
}
