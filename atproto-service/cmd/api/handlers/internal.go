package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"

	"anahita-libs/servicecalls"
	"atproto/bsky"
)

// InternalRoutes are what the installation's own services ask of this one,
// on servicecalls.Port, which the gateway never routes (roadmap row 124):
// find somebody on Bluesky, or a post, and keep it. As the profile search
// does, but for no one signed in, and with no limit of a person's: the
// caller (import-export-worker) paces itself.
func (h *Handlers) InternalRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+servicecalls.PeoplePath, h.internalPerson)
	mux.HandleFunc("GET "+servicecalls.PostsPath, h.internalPost)
	return mux
}

// internalPerson finds somebody by their handle, alice.bsky.social, and
// keeps them; somebody here who connected that account is found as
// themselves.
func (h *Handlers) internalPerson(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*bsky.Timeout)
	defer cancel()

	person, code := h.lookup(ctx, r.URL.Query().Get("address"))
	if person == nil {
		servicecalls.Answer(w, statusOfLookup(code), 0, code)
		return
	}
	servicecalls.Answer(w, http.StatusOK, person.ID, "")
}

// internalPost finds a post by its at:// address and keeps it, with
// whoever wrote it: one kept already is found as it is. Only what a repost
// may bring: a post that begins a thread.
func (h *Handlers) internalPost(w http.ResponseWriter, r *http.Request) {
	uri := strings.TrimSpace(r.URL.Query().Get("uri"))
	if !strings.HasPrefix(uri, "at://") {
		servicecalls.Answer(w, http.StatusNotFound, 0, errLookupQuery)
		return
	}
	if kept, err := h.Nodes.ReadByURI(uri); err == nil && kept != nil && kept.ID != 0 {
		servicecalls.Answer(w, http.StatusOK, kept.ID, "")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*bsky.Timeout)
	defer cancel()

	node, err := h.fetchPost(ctx, uri)
	if err != nil {
		log.Println("internal post:", uri, ":", err)
		servicecalls.Answer(w, http.StatusBadGateway, 0, errLookupFailed)
		return
	}
	if node == nil {
		servicecalls.Answer(w, http.StatusNotFound, 0, errLookupNobody)
		return
	}
	servicecalls.Answer(w, http.StatusOK, node.ID, "")
}

// statusOfLookup is how a lookup's refusal is answered.
func statusOfLookup(code string) int {
	switch code {
	case errLookupBlocked:
		return http.StatusForbidden
	case errLookupFailed:
		return http.StatusBadGateway
	}
	return http.StatusNotFound
}
