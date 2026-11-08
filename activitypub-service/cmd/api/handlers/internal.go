package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"

	"activitypub/fetch"
	"anahita-libs/servicecalls"
)

// InternalRoutes are what the installation's own services ask of this one,
// on servicecalls.Port, which the gateway never routes (roadmap row 124):
// find somebody on the fediverse, or a post, and keep it. As the profile
// search does, but for no one signed in, and with no limit of a person's:
// the caller (import-export-worker) paces itself.
func (h *Handlers) InternalRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+servicecalls.PeoplePath, h.internalPerson)
	mux.HandleFunc("GET "+servicecalls.PostsPath, h.internalPost)
	return mux
}

// internalPerson finds somebody by their address, alice@mastodon.example,
// and keeps them.
func (h *Handlers) internalPerson(w http.ResponseWriter, r *http.Request) {
	if ok, reason := h.siteFederates(); !ok {
		servicecalls.Answer(w, http.StatusConflict, 0, reason)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*fetch.Timeout)
	defer cancel()

	actor, code := h.find(ctx, r.URL.Query().Get("address"))
	if actor == nil {
		servicecalls.Answer(w, statusOfLookup(code), 0, code)
		return
	}
	node, err := h.keep(actor)
	if err != nil {
		log.Println("internal person: keeping", actor.ID, ":", err)
		servicecalls.Answer(w, http.StatusBadGateway, 0, errLookupFailed)
		return
	}
	servicecalls.Answer(w, http.StatusOK, node.ID, "")
}

// internalPost finds a post by its ActivityPub id and keeps it, with
// whoever wrote it: one kept already is found as it is. Only what a repost
// may bring: a public post that begins a thread.
func (h *Handlers) internalPost(w http.ResponseWriter, r *http.Request) {
	if ok, reason := h.siteFederates(); !ok {
		servicecalls.Answer(w, http.StatusConflict, 0, reason)
		return
	}

	uri := strings.TrimSpace(r.URL.Query().Get("uri"))
	if !strings.HasPrefix(uri, "https://") {
		servicecalls.Answer(w, http.StatusNotFound, 0, errLookupQuery)
		return
	}
	if kept, err := h.Nodes.ReadByURI(uri); err == nil && kept != nil && kept.ID != 0 {
		servicecalls.Answer(w, http.StatusOK, kept.ID, "")
		return
	}
	if h.Blocks != nil && h.Blocks.IsBlocked(hostOf(uri)) {
		servicecalls.Answer(w, http.StatusForbidden, 0, errLookupBlocked)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*fetch.Timeout)
	defer cancel()

	node, err := h.fetchNote(ctx, uri)
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
