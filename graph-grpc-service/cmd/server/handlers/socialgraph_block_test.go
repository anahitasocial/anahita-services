package handlers

import (
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// Unblocking removes the block, and nothing else. It removed "the follow
// edge between them" instead, of which there is never one, and so removed
// nothing while saying it had.
func TestUnblockingRemovesTheBlock(t *testing.T) {
	blocker := entities.NewAnahitaNode()
	blocker.ID, blocker.Type = 5, constants.NodeActorPersonEntityType
	blocked := entities.NewAnahitaNode()
	blocked.ID, blocked.Type = 55, constants.NodeActorPersonEntityType

	filter := unblockFilter(blocker, blocked)

	if len(filter.Types) != 1 || filter.Types[0] != constants.EdgeBlockEntityType {
		t.Fatalf("unblocking removes %v", filter.Types)
	}
	// From whoever blocked to whoever was blocked: not the block the other
	// may have of them, which is theirs to take back.
	if filter.NodeAID != 5 || filter.NodeBID != 55 {
		t.Errorf("removes the block from %d to %d", filter.NodeAID, filter.NodeBID)
	}
}
