package handlers

import (
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// kind is one sort of identity image: an avatar or a cover.
//
// Everything that differs between the two is here, so that nothing else in
// this service has to ask which one it is dealing with. A third sort would
// be a third value of this and two routes.
type kind struct {
	// What the graph calls it when asked to take one off a node.
	name string
	// The form field the file arrives in. Each keeps the name it had when it
	// was a service of its own, so no client changes.
	field string
	// Whether a node of this sort may have one.
	allowed func(node *entities.AnahitaNode) bool
	// The stored name the node has now, and how to give it another.
	filename    func(node *entities.AnahitaNode) string
	setFilename func(node *entities.AnahitaNode, filename string)
	// Every stored size of the image the node names, by size.
	sizes func(node *entities.AnahitaNode) (map[string]entities.AnahitaNodeImage, error)
	// The same, as addresses a browser can load.
	presigned func(files services.FilesGRPCInterface, node *entities.AnahitaNode) map[string]entities.AnahitaNodeImage
}

// An avatar is the picture that stands for an actor: a person, a group.
// Only an actor has one.
var avatar = kind{
	name:  "avatar",
	field: "avatar",
	allowed: func(node *entities.AnahitaNode) bool {
		return node.IsActor()
	},
	filename: func(node *entities.AnahitaNode) string {
		return node.AvatarFilename
	},
	setFilename: func(node *entities.AnahitaNode, filename string) {
		node.AvatarFilename = filename
	},
	sizes: func(node *entities.AnahitaNode) (map[string]entities.AnahitaNodeImage, error) {
		return node.GetAvatars()
	},
	presigned: func(files services.FilesGRPCInterface, node *entities.AnahitaNode) map[string]entities.AnahitaNodeImage {
		services.PresignAvatar(files, node)
		return node.Avatar
	},
}

// A cover is the wide picture across the top of a profile or of a post. An
// actor can have one, and so can a post.
var cover = kind{
	name:  "cover",
	field: "cover",
	allowed: func(node *entities.AnahitaNode) bool {
		return node.IsActor() || node.IsMedium()
	},
	filename: func(node *entities.AnahitaNode) string {
		return node.CoverFilename
	},
	setFilename: func(node *entities.AnahitaNode, filename string) {
		node.CoverFilename = filename
	},
	sizes: func(node *entities.AnahitaNode) (map[string]entities.AnahitaNodeImage, error) {
		return node.GetCovers()
	},
	presigned: func(files services.FilesGRPCInterface, node *entities.AnahitaNode) map[string]entities.AnahitaNodeImage {
		services.PresignCover(files, node)
		return node.Cover
	},
}
