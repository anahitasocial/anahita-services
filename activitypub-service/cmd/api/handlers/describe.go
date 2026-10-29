package handlers

// What the routes take, for the API reference (cmd/api/routes.go). The
// handlers read these values themselves, with c.Param and c.QueryParam by
// the same names; nothing binds into these structs.

// WebFingerRequest asks who an account is.
type WebFingerRequest struct {
	// `acct:ana@example.org`, or the address of somebody's actor.
	Resource string `query:"resource"`
}

// ActorRef names somebody here on the fediverse.
type ActorRef struct {
	// Their UUID, as other servers know them.
	UUID string `param:"uuid"`
}

// NoteRef names a post here, as other servers know it.
type NoteRef struct {
	// The post's UUID.
	UUID string `param:"uuid"`
}

// NotePictureRef names one picture of a photo post.
type NotePictureRef struct {
	// The post's UUID.
	UUID string `param:"uuid"`
	// Which picture, from 0.
	Position int `param:"position"`
}

// LookupRequest asks for somebody on another server.
type LookupRequest struct {
	// Their handle (`@alice@mastodon.example`) or the address of their
	// profile there.
	Q string `query:"q"`
}

// PostPicturesRequest names a post from another server.
type PostPicturesRequest struct {
	// The post's id here.
	ID int64 `param:"id"`
}

// SettingsQuery asks whose settings: the viewer's, or, for a site
// administrator, somebody else's, to read and not to change.
type SettingsQuery struct {
	// The person's id, or a website's for one of its administrators. The
	// viewer when left out.
	PersonID int64 `query:"person_id"`
}

// BlocksBrowse asks for a page of the servers that are refused.
type BlocksBrowse struct {
	// Words the name or the reason holds.
	Q string `query:"q"`
	// How many to pass over.
	Start int `query:"start"`
	// The most to send: 50 unless asked, 200 at most.
	Limit int `query:"limit"`
}

// DomainRef names a refused server.
type DomainRef struct {
	// Its name: `spam.example`.
	Domain string `param:"domain"`
}

// RefusedAccountRef names a refused account, by its number on the list.
type RefusedAccountRef struct {
	// Its number on the list.
	ID int64 `param:"id"`
}
