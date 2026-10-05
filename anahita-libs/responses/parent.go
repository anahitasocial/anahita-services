package responses

import (
	"anahita-libs/data/entities"
)

func NewParent(node *entities.AnahitaNode) *Parent {
	if node == nil {
		return nil
	}

	dto := &Parent{
		ID:                 node.ID,
		Type:               node.Type,
		Alias:              node.Alias,
		Name:               node.Name,
		Body:               node.Body,
		IsLikedByViewer:    node.IsLikedByViewer,
		IsRepostedByViewer: node.IsRepostedByViewer,
	}

	if node.Cover != nil {
		dto.Cover = NewCover(node.Cover)
	}

	if node.Portrait != nil {
		dto.Portrait = NewPortrait(node.Portrait)
	}

	dto.Files = NewPhotoFiles(node.Files)

	if node.Owner != nil {
		dto.Owner = NewOwner(node.Owner)
	}

	if node.Author != nil {
		dto.Author = NewAuthor(node.Author)
	}

	// A repost's comment and like buttons act on the post it reposts, so
	// the answers ride on the parent.
	dto.Authorized = NewNodeAuthorized(node)

	return dto
}

type Parent struct {
	ID                 int64            `json:"id"`
	Type               string           `json:"type"`
	Alias              string           `json:"alias,omitempty"`
	Name               string           `json:"name,omitempty"`
	Body               string           `json:"body,omitempty"`
	Cover              map[string]Image `json:"cover_urls,omitempty"`
	Portrait           map[string]Image `json:"portrait_urls,omitempty"`
	Files              []*PhotoFile     `json:"files,omitempty"`
	Owner              *Owner           `json:"owner,omitempty"`
	Author             *Author          `json:"author,omitempty"`
	IsLikedByViewer    bool             `json:"is_liked_by_viewer,omitempty"`
	IsRepostedByViewer bool             `json:"is_reposted_by_viewer,omitempty"`
	Authorized         *NodeAuthorized  `json:"authorized,omitempty"`
}
