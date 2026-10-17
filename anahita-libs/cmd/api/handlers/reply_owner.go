package handlers

import (
	"encoding/json"

	"anahita-libs/cmd/api/permissions"
	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

// WrittenByMetaKey is where a reply made as a Pulse keeps who wrote it: one
// of its administrators. Never sent with the reply; for the installation's
// moderators, when it is reported (docs/plans/pulse.md).
const WrittenByMetaKey = "written_by"

// OwnReply gives a new reply its owner and author: whoever writes it, as a
// reply belongs to its writer wherever the thread is; or, under a Pulse's
// own post for one of its administrators, the Pulse, so that the site
// answers its readers as itself. Who wrote it is then kept in its meta.
//
// Called after anything else is put in the reply's meta.
func OwnReply(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode, viewer *entities.Viewer) {
	reply.ModifiedBy = viewer.ID

	if pulse := permissions.RepliesAs(thread); pulse != nil {
		reply.OwnerID = pulse.ID
		reply.OwnerType = pulse.Type
		reply.CreatedBy = pulse.ID
		reply.Meta = WithWrittenBy(reply.Meta, viewer.ID)
		return
	}

	reply.OwnerID = viewer.ID
	reply.OwnerType = constants.NodeActorPersonEntityType
	reply.CreatedBy = viewer.ID
}

// WithWrittenBy keeps, beside what a post's meta holds, which administrator
// made it as a Pulse: one of its replies, or one of its articles, made by
// pulse-worker as its first administrator.
func WithWrittenBy(meta json.RawMessage, personID int64) json.RawMessage {
	fields := map[string]json.RawMessage{}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &fields)
		if fields == nil {
			fields = map[string]json.RawMessage{}
		}
	}
	fields[WrittenByMetaKey], _ = json.Marshal(personID)
	out, err := json.Marshal(fields)
	if err != nil {
		return meta
	}
	return out
}

// WrittenBy is the administrator who wrote a reply made as a Pulse, or 0.
func WrittenBy(meta json.RawMessage) int64 {
	var fields struct {
		WrittenBy int64 `json:"written_by"`
	}
	if len(meta) == 0 || json.Unmarshal(meta, &fields) != nil {
		return 0
	}
	return fields.WrittenBy
}
