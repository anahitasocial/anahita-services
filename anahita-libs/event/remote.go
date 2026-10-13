package event

// What is said on the bus when a post from another server is kept here.
//
// It is not said as a post written here is (node.medium.…): everything
// that listens for those would then treat it as one. Whoever is named in
// it by "@name" would be told they were mentioned, though the name is
// somebody's on another server; the people who follow its author would be
// notified as if by somebody here; and this installation would try to send
// it out again. So it has a name of its own, and only what means to act on
// a post from elsewhere listens for it: the hashtag service, which gives
// it its hashtags as it gives them to any post.
const (
	// RemoteMediaTopic is every such event, to listen for.
	RemoteMediaTopic = "federation.medium.*.*.*.v1"

	remoteNoteAdded  = "federation.medium.text-service.note.added.v1"
	remoteNoteEdited = "federation.medium.text-service.note.edited.v1"

	// RemoteNoteDeleted says a note kept from another server is gone from
	// here, because it was taken away where it was written.
	RemoteNoteDeleted = "federation.medium.text-service.note.deleted.v1"
)

// A repost by somebody on another server, of something here, having been
// taken back there. The subject is who reposted, the object what was
// reposted: the repost itself is gone by the time this is said.
const (
	RemoteRepostTopic   = "federation.repost.*.*.*.v1"
	RemoteRepostDeleted = "federation.repost.repost.note.deleted.v1"
)

// RemoteNoteEvent is the name to say it by: kept for the first time, or
// kept again because it was changed where it was written.
func RemoteNoteEvent(created bool) string {
	if created {
		return remoteNoteAdded
	}
	return remoteNoteEdited
}
