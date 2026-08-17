package entities

import "strings"

func (a *AnahitaNode) GetID() int64 {
	return a.ID
}

func (a *AnahitaNode) GetBody() string {
	return a.Body
}

func (a *AnahitaNode) GetType() string {
	return a.Type
}

func (a *AnahitaNode) GetService() string {
	return a.Service
}

func (a *AnahitaNode) GetEntityName() string {
	parts := strings.Split(a.Type, ".")
	if len(parts) > 2 {
		return parts[len(parts)-2]
	}
	return ""
}

func (a *AnahitaNode) GetFilename() string {
	return a.Filename
}

func (a *AnahitaNode) GetCoverFilename() string {
	return a.CoverFilename
}

func (a *AnahitaNode) GetOwnerID() int64 {
	return a.OwnerID
}

// GetOwner
func (a *AnahitaNode) GetOwner() *AnahitaNode {
	return a.Owner
}

// GetParentID
func (a *AnahitaNode) GetParentID() int64 {
	return a.ParentID
}

// GetParent
func (a *AnahitaNode) GetParent() *AnahitaNode {
	return a.Parent
}

// GetAuthorID
func (a *AnahitaNode) GetAuthorID() int64 {
	return a.CreatedBy
}

// GetAuthor
func (a *AnahitaNode) GetAuthor() *AnahitaNode {
	return a.Author
}

// GetEditor
func (a *AnahitaNode) GetEditor() *AnahitaNode {
	return a.Editor
}

// GetEditorID
func (a *AnahitaNode) GetEditorID() int64 {
	return a.ModifiedBy
}
