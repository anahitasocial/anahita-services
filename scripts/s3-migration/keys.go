package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The v1 layout. The same as anahita-libs/data/entities/assets.go, which
// is what the services read by.
const (
	assetsPrefix   = "assets/v1/"
	identityImages = assetsPrefix + "identity-image-service/"
)

var errNotAPicture = errors.New("not a picture in a place this knows")

// NewKey is where a file goes in the v1 layout, from where it is in either
// of the layouts before it.
func NewKey(oldKey string) (string, error) {
	if strings.HasSuffix(oldKey, "/") {
		return "", errNotAPicture
	}
	if strings.HasPrefix(oldKey, "assets/v2/") {
		return fromV2(oldKey)
	}
	if strings.HasPrefix(oldKey, "assets/public/") {
		return fromV0(oldKey)
	}
	return "", errNotAPicture
}

// fromV2 moves a file from the layout this platform used while it was
// being built. Avatars and covers were in folders named for two services
// that have since become one; everything else keeps its place under the
// new prefix.
func fromV2(oldKey string) (string, error) {
	rest := strings.TrimPrefix(oldKey, "assets/v2/")
	if rest == "" || strings.Contains(rest, "..") {
		return "", errNotAPicture
	}

	switch {
	case strings.HasPrefix(rest, "avatar-service/"):
		return identityImages + "avatars/" + strings.TrimPrefix(rest, "avatar-service/"), nil
	case strings.HasPrefix(rest, "cover-service/"):
		return identityImages + "covers/" + strings.TrimPrefix(rest, "cover-service/"), nil
	}
	return assetsPrefix + rest, nil
}

var (
	v0Avatar = regexp.MustCompile(`^assets/public/n(\d+)/(com_people|com_groups|com_socialengine)/avatars/(original|large|medium|small|square)(.*)$`)
	v0Cover  = regexp.MustCompile(`^assets/public/n(\d+)/(com_people|com_groups|com_articles)(/com_articles)?/covers/(large|medium|original)(.*)$`)
	v0Photo  = regexp.MustCompile(`^assets/public/n(\d+)/com_photos/(.*)_(original|large|xlarge|xxlarge|medium|small|square|thumbnail)\.(.*)$`)
)

// fromV0 moves a file from the PHP application's layout, which named each
// by the node it belonged to and the component that stored it.
func fromV0(oldKey string) (string, error) {
	if found := v0Avatar.FindStringSubmatch(oldKey); found != nil {
		kind := "person"
		if found[2] == "com_groups" {
			kind = "group"
		}
		name, extension := split(found[4])
		return fmt.Sprintf("%savatars/%s/%s/%s_%s.%s", identityImages, kind, found[1], name, found[3], extension), nil
	}

	if found := v0Cover.FindStringSubmatch(oldKey); found != nil {
		kind := "person"
		switch found[2] {
		case "com_groups":
			kind = "group"
		case "com_articles":
			kind = "article"
		}
		name, extension := split(found[5])
		return fmt.Sprintf("%scovers/%s/%s/%s_%s.%s", identityImages, kind, found[1], name, found[4], extension), nil
	}

	if found := v0Photo.FindStringSubmatch(oldKey); found != nil {
		return fmt.Sprintf("%sphoto-service/photo/%s/%s_%s.%s", assetsPrefix, found[1], found[2], found[3], found[4]), nil
	}

	return "", errNotAPicture
}

// Prefer says whether oldKey, of two old files with the same new key, is the
// one to keep. The PHP application could keep a person's avatar twice, under
// com_people and under com_socialengine, where an older version of it left
// one; the com_people one is the current. Between two of the same kind, the
// first by key, so that every run chooses alike.
func Prefer(oldKey, other string) bool {
	leftover := func(key string) bool { return strings.Contains(key, "/com_socialengine/") }
	if leftover(oldKey) != leftover(other) {
		return !leftover(oldKey)
	}
	return oldKey < other
}

// split parts a file's name from its ending.
func split(filename string) (string, string) {
	at := strings.LastIndex(filename, ".")
	if at < 0 {
		return filename, ""
	}
	return filename[:at], filename[at+1:]
}
