package handlers

import (
	an_handlers "anahita-libs/cmd/api/handlers"
	"anahita-libs/cmd/api/permissions"
	"anahita-libs/data/entities"
	"anahita-libs/responses"
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

// Quote posts: a note that carries another post under its own words. See
// permissions/quote.go for the rule, and docs/permissions.md, Quotes.
//
// A quote is made with the note (POST /notes/:owner_id/ with quote_id) and
// read with it: every note that quotes something is sent what it quotes, as
// the reader may see it, by setQuotes below. This file has what is left:
// asking whether a note may quote a post, and taking a post out of a quote.

// mayQuote asks whether the viewer may quote a post. It answers with the
// reason when they may not, or with a status when the post cannot be read at
// all: not found, which is also what a post they may not see answers.
func (h *noteHandler) mayQuote(quotedID int64, viewer *entities.Viewer) (bool, string, int) {
	// Read as the viewer: a post they cannot see is not found.
	quoted, err := h.Srv.MediaGRPC.Read(quotedID, viewer)
	if err != nil {
		log.Println(err)
		return false, permissions.QuoteRefusedUnavailable, 0
	}

	// The owner as the actor read returns it, which has its access.
	owner, err := h.Srv.ActorsGRPC.Read(quoted.OwnerID, viewer)
	if err != nil {
		log.Println(err)
		return false, permissions.QuoteRefusedUnavailable, 0
	}
	quoted.Owner = owner

	// The author's own setting is what a post with none falls back on. On
	// the author's own profile the owner is the author.
	authorID := quoted.GetAuthorID()
	authorPolicy := owner.QuotePolicy
	if authorID != quoted.OwnerID {
		author, err := h.Srv.ActorsGRPC.Read(authorID, viewer)
		if err != nil {
			// An author the viewer cannot read, somebody who has blocked
			// them for one, is not somebody they quote.
			log.Println(err)
			return false, permissions.QuoteRefusedUnavailable, 0
		}
		authorPolicy = author.QuotePolicy
	}

	rels, err := h.Srv.SocialgraphGRPC.Browse([]int64{quoted.OwnerID, authorID}, viewer.ID)
	if err != nil {
		log.Println(err)
		return false, "", http.StatusInternalServerError
	}

	allowed, reason := permissions.CanQuote(&permissions.QuoteAccessContext{
		Viewer:          viewer,
		Post:            quoted,
		AuthorPolicy:    authorPolicy,
		OwnerViewerRel:  rels[quoted.OwnerID],
		AuthorViewerRel: rels[authorID],
	})

	return allowed, reason, 0
}

// setQuotes gives the notes that quote a post that post, as this viewer may
// see it, with its images made ready.
func (h *mediumHandler) setQuotes(nodes []*entities.AnahitaNode, viewer *entities.Viewer) {
	if quoted := an_handlers.SetQuotes(h.Srv.MediaGRPC, nodes, viewer); len(quoted) > 0 {
		batchPresignPaths(h.Env, quoted)
	}
}

// DetachQuote takes the quoted post out of a note that quotes it. The note
// stays, saying that the post was removed by its author.
//
// DELETE /notes/:id/quote, where :id is the note that quotes.
//
// For the author of the QUOTED post, and site administrators: it is how
// somebody quoted keeps the say over where their post appears. It cannot be
// undone, by them or by the note's author.
func (h *noteHandler) DetachQuote(c echo.Context) error {
	viewer := getViewer(c)

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	note, err := h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}
	if note.QuoteID <= 0 {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "not_a_quote"})
	}

	// The quoted post itself, not as the viewer: who wrote it is all that is
	// asked of it, and its author can always be told that much.
	quoted, err := h.Srv.GraphNodeGRPC.Read(note.QuoteID)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusNotFound)
	}

	if !permissions.CanDetachQuote(viewer, quoted) {
		return c.NoContent(http.StatusForbidden)
	}

	if err := h.Srv.MediaGRPC.DetachQuote(note.ID, viewer); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	note, err = h.Srv.MediaGRPC.Read(id, viewer)
	if err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	if err = h.onAfterFetched(c, note); err != nil {
		log.Println(err)
		return c.NoContent(http.StatusInternalServerError)
	}

	batchPresignPaths(h.Env, []*entities.AnahitaNode{note})

	mediaResp := &responses.MediumNodeResponse{}
	return c.JSON(http.StatusOK, mediaResp.MapMediumNode(note))
}
