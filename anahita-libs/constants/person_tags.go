package constants

// A person in a picture or a video (roadmap row 103): not a mention, which
// says "this is for you", but "this person is in it". An edge from the
// person to the post, of its own kind, made by the post's author.
const EdgeTagPersonType = "edge.tag.person.v1"

// A tag's meta says what it is. Pending, the person tagged has not yet
// approved it: shown to nobody but them and the post's author, and nothing
// reads it as a tag. Approved, by them or because they let anybody tag them:
// everything that reads tags reads only these.
const (
	TagPending  = "pending"
	TagApproved = "approved"
)

// TagMaxPerPost is the most people one post may be tagged with.
const TagMaxPerPost = 20

// TagPermission is a person's choice of who may tag them, beside who may
// mention them, under socialgraph-service.
const (
	TagPermission        = "tag"
	TagPermissionService = "socialgraph-service"
)

// The three choices. Anybody signed in tags them at once; each tag waits for
// their approval, which is everybody's choice until they make another (the
// user, 2026-10-19); or nobody may, the person themselves aside, which is
// `admins` as for mentions.
const (
	TagAccessAnybody = AnahitaAccessRegistered
	TagAccessApprove = "approve"
	TagAccessNobody  = "admins"
	TagAccessDefault = TagAccessApprove
)
