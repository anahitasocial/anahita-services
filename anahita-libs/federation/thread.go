package federation

import (
	"time"

	"anahita-libs/data/entities"
)

// threadTogether is how far apart in time two notes of a thread written in
// one go can be: they are made in one transaction, a moment apart at most.
const threadTogether = time.Minute

// WrittenTogether says whether `parent` is the note before `reply` in a
// thread written in one go (roadmap row 101): its writer's own, here, and
// made with it. The parts of such a thread are heard of in no set order, so
// a reply heard of first sends the part before it first; an older post of
// the writer's, which never went out, is left as it is.
func WrittenTogether(parent, reply *entities.AnahitaNode) bool {
	if parent == nil || reply == nil || parent.ID == 0 || parent.IsRemote() || reply.IsRemote() {
		return false
	}
	if parent.ID != reply.ParentID || parent.CreatedBy <= 0 || parent.CreatedBy != reply.CreatedBy {
		return false
	}
	apart := reply.CreatedAt.Sub(parent.CreatedAt)
	return apart >= 0 && apart <= threadTogether
}
