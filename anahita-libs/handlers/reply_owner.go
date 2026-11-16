package handlers

import (
	"encoding/json"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
)

// WrittenByMetaKey is where a reply made as a channel keeps who wrote it: one
// of its administrators. Never sent with the reply; for the installation's
// moderators, when it is reported (docs/plans/done/channel.md).
const WrittenByMetaKey = "written_by"

// OwnReply gives a new reply its owner and author: whoever writes it, as a
// reply belongs to its writer wherever the thread is; or, under a channel's
// own post for one of its administrators, the channel, so that the site
// answers its readers as itself. Who wrote it is then kept in its meta.
//
// Called after anything else is put in the reply's meta.
func OwnReply(thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode, viewer *entities.Viewer) {
	reply.ModifiedBy = viewer.ID

	if channel := permissions.RepliesAs(thread); channel != nil {
		reply.OwnerID = channel.ID
		reply.OwnerType = channel.Type
		reply.CreatedBy = channel.ID
		reply.Meta = WithWrittenBy(reply.Meta, viewer.ID)
		return
	}

	reply.OwnerID = viewer.ID
	reply.OwnerType = constants.NodeActorPersonEntityType
	reply.CreatedBy = viewer.ID
}

// WithWrittenBy keeps, beside what a post's meta holds, which administrator
// made it as a channel: one of its replies, or one of its articles, made by
// Channel-worker as its first administrator.
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

// WrittenBy is the administrator who wrote a reply made as a channel, or 0.
func WrittenBy(meta json.RawMessage) int64 {
	var fields struct {
		WrittenBy int64 `json:"written_by"`
	}
	if len(meta) == 0 || json.Unmarshal(meta, &fields) != nil {
		return 0
	}
	return fields.WrittenBy
}

// OwnPost makes a new post on a channel the channel's own (roadmap row 147):
// its admins write as it, so the channel is its author, shown and sent as
// such, and the admin who wrote it is kept in its meta. A post anywhere
// else, and a reply, are left as they are: a reply is OwnReply's.
func OwnPost(post *entities.AnahitaNode, writerID int64) {
	if post == nil || post.OwnerType != constants.NodeActorChannelEntityType || post.ParentID > 0 || post.OwnerID <= 0 {
		return
	}
	post.CreatedBy = post.OwnerID
	post.ModifiedBy = writerID
	post.Meta = WithWrittenBy(post.Meta, writerID)
}
