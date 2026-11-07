package handlers

import (
	"log"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"graph/requests"
)

// markThreads tells a list which of its notes open a chain of their
// writer's replies, how long each is and which note comes second, so that
// the list can show it as a thread (anahita-web roadmap row 126). Two
// queries for a page: the chains, and the second notes. A list that cannot
// be told is shown as it was, with no thread.
func markThreads(env *Env, nodes []*entities.AnahitaNode) {
	var ids []int64
	for _, node := range nodes {
		if node != nil && node.Type == constants.NodeMediumNoteEntityType && node.ParentID == 0 && !node.IsRemote() {
			ids = append(ids, node.ID)
		}
	}
	if len(ids) == 0 {
		return
	}

	threads, err := env.Repos.Nodes.Threads(ids)
	if err != nil {
		log.Println("threads:", err)
		return
	}
	if len(threads) == 0 {
		return
	}

	nextIDs := make([]int64, 0, len(threads))
	for _, thread := range threads {
		if thread.NextID > 0 {
			nextIDs = append(nextIDs, thread.NextID)
		}
	}
	next := map[int64]*entities.AnahitaNode{}
	if len(nextIDs) > 0 {
		read, err := env.Repos.Nodes.BatchRead(&requests.NodeBatchReadRequest{IDs: nextIDs})
		if err != nil {
			log.Println("threads: the second notes:", err)
			return
		}
		for _, node := range read {
			next[node.ID] = node
		}
	}

	for _, node := range nodes {
		thread, ok := threads[node.ID]
		if node == nil || !ok {
			continue
		}
		node.ThreadCount = thread.Count
		node.ThreadNext = next[thread.NextID]
	}
}
