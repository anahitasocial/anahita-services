package handlers

import (
	an_constants "anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"hashtag/constants"
	"log"
	"slices"
	"strings"
)

func (h *hashtagHandler) HandleEvents(payload *event.EventPayload) error {
	log.Println("Hashtag event received", payload.Name)

	if strings.Contains(payload.Name, ".added.") || strings.Contains(payload.Name, ".edited.") {
		if err := h.updateHashtags(payload); err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

func (h *hashtagHandler) updateHashtags(payload *event.EventPayload) error {
	viewer := entities.NewViewer()
	viewer.ID = payload.ViewerID
	viewer.PersonType = payload.ViewerPersonType

	node, err := h.Srv.GraphNodeGRPC.Read(payload.ObjectID)
	if err != nil {
		log.Println(err)
		return err
	}

	// log.Println("Updating hashtags for", node.ID)

	aliases := CleanStringArray(node.Hashtags)
	if len(aliases) == 0 {
		// Delete all existing hashtag relations
		currentHashtagIDs, err := h.getCurrentHashtagIDs(node)
		if err != nil {
			log.Printf("Failed to get current hashtag IDs for node %d: %v", node.ID, err)
			return err
		}

		for _, hashtagID := range currentHashtagIDs {
			if err := h.Srv.GraphEdgeGRPC.BatchDeleteByFilters(&services.GraphEdgeBatchDeleteByFiltersRequest{
				NodeAID:   hashtagID,
				NodeAType: an_constants.NodeBaseHashtagEntityType,
				NodeBID:   node.ID,
				NodeBType: node.Type,
				Types:     []string{an_constants.EdgeTagHashtagType},
			}); err != nil {
				log.Printf("Failed to delete hashtag relation for hashtag ID %d and node %d: %v", hashtagID, node.ID, err)
				return err
			}
		}

		return nil
	}

	// 1) Bulk-read existing hashtags by aliases
	req := services.NewGraphNodeRequest().BatchReadByAliases
	req.Aliases = aliases
	req.NodeType = an_constants.NodeBaseHashtagEntityType
	req.AccessCheck = false
	existingHashtags, err := h.Srv.GraphNodeGRPC.BatchReadByAliases(req)
	if err != nil {
		log.Printf("Failed to batch read existing hashtags by aliases %v: %v", aliases, err)
		return err
	}

	existingHashtagMap := make(map[string]*entities.AnahitaNode)
	for _, hashtag := range existingHashtags {
		existingHashtagMap[hashtag.Alias] = hashtag
	}

	// 2) Create missing hashtags
	var missing []*entities.AnahitaNode
	for _, alias := range aliases {
		if _, found := existingHashtagMap[alias]; !found {
			hashtag := entities.NewAnahitaNode()
			hashtag.Type = an_constants.NodeBaseHashtagEntityType
			hashtag.Service = constants.HashtagService
			hashtag.Name = alias
			hashtag.Alias = alias
			hashtag.Author.ID = viewer.ID
			missing = append(missing, hashtag)
		}
	}

	if len(missing) > 0 {
		created, err := h.Srv.GraphNodeGRPC.BatchAdd(missing)
		if err != nil {
			log.Printf("Failed to batch create missing hashtags: %v", err)
			return err
		}

		for _, hashtag := range created {
			existingHashtagMap[hashtag.Alias] = hashtag
		}
	}

	// 3) Build final set of target hashtag IDs
	newIDs := make(map[int64]struct{}, len(existingHashtagMap))
	for _, h := range existingHashtagMap {
		newIDs[h.ID] = struct{}{}
	}

	// 4) Get existing relations
	currentIDs, err := h.getCurrentHashtagIDs(node)
	if err != nil {
		return err
	}

	// 5) Create new relations
	for id := range newIDs {
		found := slices.Contains(currentIDs, id)
		if !found {
			edge := entities.NewAnahitaEdge()
			edge.Type = an_constants.EdgeTagHashtagType
			edge.NodeAID = id
			edge.NodeAType = an_constants.NodeBaseHashtagEntityType
			edge.NodeBID = node.ID
			edge.NodeBType = node.Type
			edge.CreatedBy = viewer.ID
			if _, err := h.Srv.GraphEdgeGRPC.Add(edge); err != nil {
				log.Printf("Failed to create hashtag relation for hashtag ID %d and node %d: %v", id, node.ID, err)
				return err
			}
		}
	}

	// 6) Delete obsolete relations
	for _, currentID := range currentIDs {
		if _, found := newIDs[currentID]; !found {
			if err := h.Srv.GraphEdgeGRPC.BatchDeleteByFilters(&services.GraphEdgeBatchDeleteByFiltersRequest{
				NodeAID:   currentID,
				NodeAType: an_constants.NodeBaseHashtagEntityType,
				NodeBID:   node.ID,
				NodeBType: node.Type,
				Types:     []string{an_constants.EdgeTagHashtagType},
			}); err != nil {
				log.Printf("Failed to delete obsolete hashtag relation for hashtag ID %d and node %d: %v", currentID, node.ID, err)
				return err
			}
		}
	}

	return nil
}

func (h *hashtagHandler) getCurrentHashtagIDs(node *entities.AnahitaNode) ([]int64, error) {
	req := services.NewGraphEdgeRequest().BatchReadByFilters
	req.NodeBID = node.ID
	req.Types = []string{an_constants.EdgeTagHashtagType}

	edges, err := h.Srv.GraphEdgeGRPC.BatchReadByFilters(req)
	if err != nil {
		log.Printf("Failed to batch read hashtag edges for node %d: %v", node.ID, err)
		return nil, err
	}

	var hashtagIDs []int64
	for _, edge := range edges {
		hashtagIDs = append(hashtagIDs, edge.NodeAID)
	}

	return hashtagIDs, nil
}
