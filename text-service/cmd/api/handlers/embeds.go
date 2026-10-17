package handlers

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/embeds"
	"anahita-libs/linkpreviews"
	"anahita-libs/site"
	"log"
	"strings"
)

// What an article or a topic shows within its text: an address on a line of
// its own (package embeds). A post of this installation is drawn by the
// reader's browser as it reads it, so nobody sees one they may not; what is
// kept here is the edge to it, a citation. A page elsewhere is drawn by its card, kept
// in the post's meta as a note keeps its one.

// withEmbeddedPages keeps the cards of the pages a text embeds in its meta.
//
// A card already kept for a page still in the text stays as it is, as a
// note's does: a card is the page as it was when the post was written. A
// page whose preview link-preview-service has not read is a link.
func (h *mediumHandler) withEmbeddedPages(meta []byte, body string) []byte {
	found := embeds.Find(body, site.FromEnv().IsOurs)

	kept := map[string]*linkpreviews.Snapshot{}
	for _, snapshot := range linkpreviews.SnapshotsFromMeta(meta) {
		kept[snapshot.URL] = snapshot
	}

	cards := []*linkpreviews.Snapshot{}
	for _, page := range found.Pages {
		if snapshot, ok := kept[page]; ok {
			cards = append(cards, snapshot)
			continue
		}
		if h.Previews == nil {
			continue
		}
		if preview, ok := h.Previews.Get(page); ok && preview.Usable() {
			if snapshot := linkpreviews.NewSnapshot(preview); snapshot != nil {
				cards = append(cards, snapshot)
			}
		}
	}

	return linkpreviews.SnapshotsIntoMeta(meta, cards)
}

// syncEmbeds writes the citations from a post to the posts of this
// installation it embeds, and takes away those it no longer does.
//
// After the post is saved, and never in the way of saving it: a post whose
// edges could not be written is posted all the same, and they are put
// right the next time it is saved.
func (h *mediumHandler) syncEmbeds(medium *entities.AnahitaNode) {
	// A draft cites nothing yet: what it shows is cited when it is
	// published.
	if medium == nil || medium.ID == 0 || medium.Draft || h.Srv.GraphEdgeGRPC == nil {
		return
	}

	found := embeds.Find(medium.Body, site.FromEnv().IsOurs)

	// The posts it embeds, as the graph has them: an address to a post
	// that is not there, or not a post, is no edge.
	wanted := map[int64]*entities.AnahitaNode{}
	if len(found.Posts) > 0 {
		ids := []int64{}
		for _, post := range found.Posts {
			if post.ID != medium.ID {
				ids = append(ids, post.ID)
			}
		}
		if len(ids) > 0 {
			read := services.NewGraphNodeRequest().BatchRead
			read.IDs = ids
			read.Viewer = entities.NewViewer()
			read.AccessCheck = false
			nodes, err := h.Srv.GraphNodeGRPC.BatchRead(read)
			if err != nil {
				log.Printf("embeds of %d: reading the posts: %v", medium.ID, err)
				return
			}
			for _, node := range nodes {
				if node != nil && strings.HasPrefix(node.Type, an_constants.NodeMediumPrefixEntityType) {
					wanted[node.ID] = node
				}
			}
		}
	}

	// A post cites posts. A profile it embeds is named, a mention, and not
	// a citation: actor-service keeps those (the user, 2026-10-17).
	cited := []int64{}
	for _, post := range found.Posts {
		cited = append(cited, post.ID)
	}

	existing, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(&services.GraphEdgeBatchReadByFiltersRequest{
		NodeAID: medium.ID,
		Types:   []string{an_constants.EdgeMediumCitationType},
	})
	if err != nil {
		log.Printf("embeds of %d: reading the edges: %v", medium.ID, err)
		return
	}

	stale := []int64{}
	have := map[int64]bool{}
	for _, edge := range existing {
		if _, ok := wanted[edge.NodeBID]; ok && !have[edge.NodeBID] {
			have[edge.NodeBID] = true
			continue
		}
		stale = append(stale, edge.ID)
	}
	if len(stale) > 0 {
		if err := h.Srv.GraphEdgeGRPC.BatchDelete(stale); err != nil {
			log.Printf("embeds of %d: taking away %v: %v", medium.ID, stale, err)
		}
	}

	missing := []*entities.AnahitaEdge{}
	for _, id := range cited {
		node, ok := wanted[id]
		if !ok || have[id] {
			continue
		}
		have[id] = true
		missing = append(missing, &entities.AnahitaEdge{
			Type:      an_constants.EdgeMediumCitationType,
			NodeAID:   medium.ID,
			NodeAType: medium.Type,
			NodeBID:   node.ID,
			NodeBType: node.Type,
			CreatedBy: medium.ModifiedBy,
		})
	}
	if len(missing) > 0 {
		if _, err := h.Srv.GraphEdgeGRPC.BatchAdd(missing); err != nil {
			log.Printf("embeds of %d: adding: %v", medium.ID, err)
		}
	}
}
