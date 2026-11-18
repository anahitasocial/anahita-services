package main

import (
	"net/http"

	"text/cmd/api/handlers"
	"text/requests"

	"anahita-libs/apidoc"
	an_mw "anahita-libs/middleware"
	"anahita-libs/photos"
	"anahita-libs/responses"
)

// --- Notes ---
// Short posts, and every reply. Reading them is public, and the caller is
// known when a token is sent (r.setViewer); everything else needs a token
// (r.authorize).
func (r *Routes) notesRoutes(api *apidoc.Router, d postDocs) {
	media, write := d.media, d.write

	// The requests a note takes where they differ from other posts'.
	notesReq := requests.NewNoteRequest()

	notes := api.Group("notes")

	notes.GET("/:owner_id/", r.handler.Note.Browse, r.setViewer).With(d.browse("note"))

	notes.GET("/:id", r.handler.Note.Read, r.setViewer).With(d.read("note"))

	notes.PATCH("/:id", r.handler.Note.Edit, r.authorize).
		With(d.edit("note", notesReq.Edit)).
		Describe("A reply is changed by its author alone.").
		Refuses(http.StatusGone, "a reply that was removed")

	notes.DELETE("/:id", r.handler.Note.Delete, r.authorize).With(d.remove("note"))

	notes.POST("/:owner_id/", r.handler.Note.Add, r.authorize).
		With(d.add("note", notesReq.Add)).
		Describe("With `quote_id`, a quote: the post is carried under the note's words, as its "+
			"author's quote policy lets. With `link_preview_url`, the card of a link the writer "+
			"was shown; what the card says is what this installation read, never what is sent.").
		Sends(http.StatusForbidden, responses.Refusal{}, "`quote_not_allowed`, with `reason`: the "+
			"post may not be quoted by the viewer")

	// --- Replies and threads ---
	// A reply is a note that answers a post or another reply, so it is
	// read, edited and removed at /notes/:id like any note. These routes are
	// what only a thread has, and the post answered can be of any kind.
	// Reading a thread is public (r.setViewer), but for members only where
	// visitors are given a preview (an_mw.MembersInPreview): a preview shows
	// a post and how much was said under it, not what was said. Everything
	// else needs a token (r.authorize).
	notes.GET("/:id/thread", r.handler.Note.Thread, r.setViewer, an_mw.MembersInPreview()).
		Summary("The replies under a post").
		Describe("Of a post of any kind, as the viewer may see them, with whether they may reply. "+
			"On an installation in preview, only for somebody signed in.").
		Viewer(apidoc.Anyone).
		Takes(media.Read).
		Sends(http.StatusOK, handlers.Thread{}).
		Refuses(http.StatusUnauthorized,
			"the installation is in preview and the viewer is not signed in").
		Refuses(http.StatusForbidden, "the viewer may not see the post").
		Refuses(http.StatusNotFound, "there is no such post").
		Sends(http.StatusConflict, handlers.NotARoot{},
			"it is a reply: its thread is the one of `root_id`")

	notes.POST("/:id/replies", r.handler.Note.AddReply, r.authorize).
		Summary("Reply").
		Describe("A note in the thread of the post or the reply `id`, owned by whoever writes it, "+
			"seen by whoever may see the post at the top of the thread. Who may reply is that "+
			"post's to say.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.ReplyAddRequest{}).
		Sends(http.StatusCreated, responses.MediumNode{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`; or not well "+
			"formed, without a body").
		Refuses(http.StatusForbidden, "the viewer may not reply here").
		Refuses(http.StatusNotFound, "there is no such post the viewer may see").
		Refuses(http.StatusGone, "it answers a reply that was removed")

	// A thread written in one go (roadmap row 101): the notes are all made,
	// or none; a note with pictures is a photo post in it (row 143).
	notes.POST("/:owner_id/thread", r.handler.Note.AddThread, r.authorize).
		Summary("Post a thread").
		Describe("Up to ten notes, made together in order, all or none: the first a post on the "+
			"profile `owner_id`, with what a post takes, and each after it a reply to the one "+
			"before, written by the viewer. Each is checked as a note is; one with `uploads` is a "+
			"photo post, its pictures checked as a photo post's, uploaded for the profile it is "+
			"on: `owner_id` for the first, the viewer for the rest. They go to other networks in "+
			"order, and whoever is told of the thread is told of its first note.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.NoteThreadAddRequest{}).
		Sends(http.StatusCreated, handlers.ThreadAdded{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`, "+
			"`reply_access_invalid`, `quote_policy_invalid`; or not well formed, or more than ten "+
			"notes, a note without words or pictures").
		Refuses(http.StatusForbidden, "the viewer may not post here, or a picture was uploaded by "+
			"somebody else or for another profile (`upload_not_yours`)").
		Refuses(http.StatusNotFound,
			"there is no such profile, or no such upload (`file_not_found`)").
		Sends(http.StatusUnprocessableEntity, responses.AudienceRefusal{}, "`access_not_allowed`, "+
			"with the audiences it may have; or, as a photo post's pictures are refused, "+
			"`too_many_files`, `file_repeated` or `alt_text_too_long`, with `max`")

	notes.POST("/:id/replies/thread", r.handler.Note.AddReplyThread, r.authorize).
		Summary("Reply with a thread").
		Describe("Up to ten notes, made together in order, all or none: the first a reply to the "+
			"post or the reply `id`, and each after it a reply to the one before. Each is checked "+
			"as a reply is; one with `uploads` is a photo post, its pictures uploaded for the "+
			"viewer. They go to other networks in order, and whoever is told of the thread is "+
			"told of its first note.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.ReplyThreadAddRequest{}).
		Sends(http.StatusCreated, handlers.ThreadAdded{}).
		Sends(http.StatusBadRequest, responses.Refusal{}, "`language_invalid`; or not well "+
			"formed, or more than ten notes, a note without words or pictures").
		Refuses(http.StatusForbidden, "the viewer may not reply here, or a picture was uploaded "+
			"by somebody else (`upload_not_yours`)").
		Refuses(http.StatusNotFound,
			"there is no such post the viewer may see, or no such upload (`file_not_found`)").
		Refuses(http.StatusGone, "it answers a reply that was removed").
		Sends(http.StatusUnprocessableEntity, photos.FilesRefusal{}, "`too_many_files`, "+
			"`file_repeated` or `alt_text_too_long`, with `max`")

	notes.PATCH("/:id/hidden", r.handler.Note.HideReply, r.authorize).
		Summary("Hide a reply, or show it again").
		Describe("From everybody but its writer and whoever looks after the thread, with "+
			"everything said under it. It is not deleted, and its writer is not told. A reply "+
			"from Bluesky is hidden there too, when the post went there (federation.md).").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(&handlers.ReplyHideRequest{}).
		Sends(http.StatusOK, handlers.HiddenAnswer{}).
		Refuses(http.StatusForbidden, "the viewer does not look after the thread").
		Refuses(http.StatusNotFound, "there is no such reply")

	// --- Quotes and citations ---
	// Taking one's post out of a quote needs a token (r.authorize). Reading
	// who quoted or cited a post is public (r.setViewer), and, like a
	// thread, not part of a preview (an_mw.MembersInPreview).
	notes.DELETE("/:id/quote", r.handler.Note.DetachQuote, r.authorize).
		Summary("Take one's post out of a quote").
		Describe("For the author of the quoted post. The note stays, saying the post was removed "+
			"by its author. It cannot be undone.").
		Viewer(apidoc.SignedIn).
		Scope(write).
		Takes(media.Read).
		Sends(http.StatusOK, responses.MediumNode{}).
		Refuses(http.StatusForbidden, "the viewer is not the quoted post's author").
		Refuses(http.StatusNotFound, "there is no such note the viewer may see").
		Sends(http.StatusUnprocessableEntity, responses.Refusal{}, "`not_a_quote`")

	notes.GET("/:id/quotes", r.handler.Note.Quotes, r.setViewer, an_mw.MembersInPreview()).
		Summary("The notes that quote a post").
		Describe("Those the viewer may see.").
		Viewer(apidoc.Anyone).
		Takes(media.Read).
		Sends(http.StatusOK, responses.Unpaged[*responses.MediumNode]{}).
		Refuses(http.StatusUnauthorized,
			"the installation is in preview and the viewer is not signed in").
		Refuses(http.StatusNotFound, "there is no such post that the viewer may see")

	notes.GET("/:id/cited_by", r.handler.Note.CitedBy, r.setViewer, an_mw.MembersInPreview()).
		Summary("The posts that cite a post").
		Describe("The articles and topics that show it within their text, and the notes that "+
			"quote it, together, newest first: those the viewer may see. A quote its post was "+
			"taken out of is not listed. Its author's own are listed too, though they are not "+
			"counted in its `citation_count`, and quotes are counted in its `quote_count` "+
			"instead.").
		Viewer(apidoc.Anyone).
		Takes(&handlers.CitedByRequest{Limit: 20}).
		Sends(http.StatusOK, responses.List[*responses.MediumNodeItem]{}).
		Refuses(http.StatusUnauthorized,
			"the installation is in preview and the viewer is not signed in").
		Refuses(http.StatusNotFound, "there is no such post that the viewer may see")

	// --- Settings ---
	// A note's settings, as any post's: who may see it, reply to it and quote
	// it, its content warning, and its pin. Each needs a token (r.authorize).
	notes.PATCH("/:id/access", r.handler.Note.EditAccess, r.authorize).With(d.access)

	notes.PATCH("/:id/comment-status", r.handler.Note.EditCommentStatus, r.authorize).
		With(d.commentStatus)

	notes.PATCH("/:id/reply-access", r.handler.Note.EditReplyAccess, r.authorize).
		With(d.replyAccess)

	notes.PATCH("/:id/quote-policy", r.handler.Note.EditQuotePolicy, r.authorize).
		With(d.quotePolicy)

	notes.PATCH("/:id/warning", r.handler.Note.EditWarning, r.authorize).With(d.warning)

	notes.PUT("/:id/pin", r.handler.Note.Pin, r.authorize).
		With(d.pin("Pin it to the profile it is on"))

	notes.DELETE("/:id/pin", r.handler.Note.Unpin, r.authorize).With(d.pin("Take its pin off"))
}
