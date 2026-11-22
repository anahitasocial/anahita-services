package entities

import "testing"

// A picture's name with no extension, which no picture stored here has, is
// no file, and not a crash of whatever reads the node.
func TestAPictureNameWithNoExtensionIsNoFile(t *testing.T) {
	node := NewAnahitaNode()
	node.AvatarFilename, node.CoverFilename = "abc", "def"
	if got := node.GetAvatarFile("small"); got != "" {
		t.Errorf("profile photo: %q", got)
	}
	if got := node.GetCoverFile("large"); got != "" {
		t.Errorf("cover photo: %q", got)
	}

	node.AvatarFilename, node.CoverFilename = "abc.jpg", "def.png"
	if node.GetAvatarFile("small") != "abc_small.jpg" || node.GetCoverFile("large") != "def_large.png" {
		t.Errorf("named as stored: %q %q", node.GetAvatarFile("small"), node.GetCoverFile("large"))
	}
}
