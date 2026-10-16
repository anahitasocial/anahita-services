package validators

import (
	"testing"

	"anahita-libs/data/entities"
)

// A post needs something to show: a title, words, or a file. Pictures or a
// video alone are enough, as for a reply with pictures and nothing said.
func TestAMediumNeedsSomethingToShow(t *testing.T) {
	post := func(name, body, filename string) *entities.AnahitaNode {
		node := entities.NewAnahitaNode()
		node.Type = "node.medium.photo-service.photo.v1"
		node.Service = "photo-service"
		node.OwnerID, node.OwnerType, node.CreatedBy = 2, "node.actor.person-service.person.v1", 2
		node.Name, node.Body, node.Filename = name, body, filename
		return node
	}

	video := func() *entities.AnahitaNode {
		node := post("", "", "")
		node.Type, node.Service, node.Mimetype = "node.medium.video-service.video.v1", "video-service", "video/mp4"
		return node
	}

	for name, c := range map[string]struct {
		node *entities.AnahitaNode
		ok   bool
	}{
		"a title":         {post("Sunset", "", ""), true},
		"words":           {post("", "Look at this", ""), true},
		"pictures alone":  {post("", "", "a1b2c3.jpg"), true},
		"nothing to show": {post("", "", ""), false},
		"a video alone":   {video(), true},
	} {
		err := NewMediumValidator().ValidateAdd(c.node)
		if (err == nil) != c.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
}
