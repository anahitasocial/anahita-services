package entities

import "fmt"

// PhotoFile is one image of a photo post.
//
// A post holds up to constants.PhotoMaxFiles of them, in order. The first is
// the cover, and the node's own Filename is a copy of its name, so anything
// that shows one image for a photo needs to know nothing about these.
type PhotoFile struct {
	NodeID   int64
	Position int
	Filename string
	Mimetype string
	// Of the largest copy kept, turned the right way up. 0 for an image
	// stored before these were recorded.
	Width  int
	Height int
	// What the image shows, for people who cannot see it.
	AltText string
	// Each stored size. Paths until they are presigned, addresses after.
	Portrait map[string]AnahitaNodeImage
}

// PhotoUpload is an image that has been uploaded and is not in a post yet.
type PhotoUpload struct {
	ID int64
	// The profile it was uploaded for. Part of where the image is stored, so
	// an upload can only go into a post on that profile.
	OwnerID int64
	// Who uploaded it. Nobody else can put it in a post.
	CreatedBy int64
	Filename  string
	Mimetype  string
	Width     int
	Height    int
}

// PortraitsFor is every stored size of one image belonging to this node, as
// GetPortraits is for the image the node itself names.
//
// An image is stored under the profile the post is on, not under the post, so
// this needs the node's service, type and owner and nothing else. That is
// what lets an image be stored before its post exists.
func (a *AnahitaNode) PortraitsFor(filename string) (map[string]AnahitaNodeImage, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename is empty")
	}

	named := &AnahitaNode{}
	named.Type = a.Type
	named.Service = a.Service
	named.OwnerID = a.OwnerID
	named.Filename = filename

	return named.GetPortraits()
}

// SetFilePortraits fills in the stored sizes of each of the node's images.
func (a *AnahitaNode) SetFilePortraits() {
	for _, file := range a.Files {
		portraits, err := a.PortraitsFor(file.Filename)
		if err != nil {
			continue
		}
		file.Portrait = portraits
	}
}
