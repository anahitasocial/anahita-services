package responses

import (
	"testing"

	"abusereport/data/entities"

	libs_entities "anahita-libs/data/entities"
)

// A reply made as a website says, to those who handle reports, which of its
// administrators wrote it.
func TestACaseSaysWhoWroteAWebsitesReply(t *testing.T) {
	reply := libs_entities.NewAnahitaNode()
	reply.ID, reply.Type = 20, "node.medium.text-service.note.v1"
	reply.Meta = []byte(`{"written_by": 7}`)
	ana := libs_entities.NewAnahitaNode()
	ana.ID, ana.Type, ana.Name = 7, "node.actor.person-service.person.v1", "Ana"

	m := &Mapper{Nodes: map[int64]*libs_entities.AnahitaNode{20: reply, 7: ana}}
	got := m.Case(&entities.Case{ID: 1, TargetID: 20}, nil)
	if got.WrittenBy == nil || got.WrittenBy.ID != 7 {
		t.Errorf("written by %+v", got.WrittenBy)
	}

	reply.Meta = nil
	if m.Case(&entities.Case{ID: 1, TargetID: 20}, nil).WrittenBy != nil {
		t.Error("a reply of somebody's own says it was written by another")
	}
}
