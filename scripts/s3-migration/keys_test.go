package main

import "testing"

func TestWhereEachFileGoes(t *testing.T) {
	for oldKey, want := range map[string]string{
		// From the layout the platform was built on.
		"assets/v2/avatar-service/person/5/8944e519_large.jpg": "assets/v1/identity-image-service/avatars/person/5/8944e519_large.jpg",
		"assets/v2/avatar-service/group/9/abc_square.png":      "assets/v1/identity-image-service/avatars/group/9/abc_square.png",
		"assets/v2/cover-service/person/5/cover_large.jpg":     "assets/v1/identity-image-service/covers/person/5/cover_large.jpg",
		"assets/v2/cover-service/article/170190/c_medium.jpg":  "assets/v1/identity-image-service/covers/article/170190/c_medium.jpg",
		"assets/v2/photo-service/photo/5/26d4_original.jpg":    "assets/v1/photo-service/photo/5/26d4_original.jpg",
		"assets/v2/photo-service/photo/5/a b (1)_large.jpg":    "assets/v1/photo-service/photo/5/a b (1)_large.jpg",

		// From the PHP application's.
		"assets/public/n5/com_people/avatars/large8944e519.jpg":              "assets/v1/identity-image-service/avatars/person/5/8944e519_large.jpg",
		"assets/public/n9/com_groups/avatars/squareabc.png":                  "assets/v1/identity-image-service/avatars/group/9/abc_square.png",
		"assets/public/n5/com_socialengine/avatars/mediumold.jpg":            "assets/v1/identity-image-service/avatars/person/5/old_medium.jpg",
		"assets/public/n5/com_people/covers/largecover.jpg":                  "assets/v1/identity-image-service/covers/person/5/cover_large.jpg",
		"assets/public/n170190/com_articles/com_articles/covers/mediumc.jpg": "assets/v1/identity-image-service/covers/article/170190/c_medium.jpg",
		"assets/public/n5/com_photos/26d4_original.jpg":                      "assets/v1/photo-service/photo/5/26d4_original.jpg",
	} {
		got, err := NewKey(oldKey)
		if err != nil || got != want {
			t.Errorf("%s\n   went to %q (%v)\n   want    %q", oldKey, got, err, want)
		}
	}
}

// A person's avatar kept twice by the PHP application: the com_people one is
// the current, and wins whichever comes first.
func TestTheCurrentAvatarIsChosen(t *testing.T) {
	people := "assets/public/n106/com_people/avatars/largef09.jpg"
	leftover := "assets/public/n106/com_socialengine/avatars/largef09.jpg"
	a, _ := NewKey(people)
	b, _ := NewKey(leftover)
	if a != b {
		t.Fatalf("expected one new key, got %q and %q", a, b)
	}
	if !Prefer(people, leftover) || Prefer(leftover, people) {
		t.Error("the com_people avatar should be chosen over the com_socialengine one")
	}
	if Prefer("assets/public/n1/com_people/avatars/b.jpg", "assets/public/n1/com_people/avatars/a.jpg") {
		t.Error("between two of the same kind, the first by key")
	}
}

func TestWhatIsNotAPictureIsLeftWhereItIs(t *testing.T) {
	for _, oldKey := range []string{
		"assets/v2/",
		"assets/v2/avatar-service/person/5/",
		"assets/v2/../secrets/key.pem",
		"assets/v1/photo-service/photo/5/x_large.jpg", // there already
		"assets/public/n5/com_files/report.pdf",
		"assets/public/n5/",
		"videos/5/abc/720.mp4",
		"exports/5/abc.zip",
		"",
	} {
		if got, err := NewKey(oldKey); err == nil {
			t.Errorf("%q was given a new place: %q", oldKey, got)
		}
	}
}

func TestACopySourceKeepsItsSlashes(t *testing.T) {
	if got := escapeKey("assets/v2/photo-service/photo/5/a b (1)+é_large.jpg"); got != "assets/v2/photo-service/photo/5/a%20b%20%281%29%2B%C3%A9_large.jpg" {
		t.Errorf("got %q", got)
	}
}
