package requests

import (
	"anahita-libs/constants"
	"anahita-libs/policy"
)

type MediumRequest struct {
	Browse            *MediumBrowseRequest
	Read              *MediumReadRequest
	Edit              *MediumEditRequest
	EditAccess        *MediumEditAccessRequest
	EditCommentStatus *MediumEditCommentStatusRequest
	Add               *MediumAddRequest
	Delete            *MediumDeleteRequest
}

func NewMediumRequest() *MediumRequest {
	return &MediumRequest{
		Browse: &MediumBrowseRequest{
			Offset:    0,
			Limit:     constants.RequestLimitDefault,
			Direction: "desc",
			Ordering:  "created_at",
			OwnerID:   0,
		},
		Read: &MediumReadRequest{
			ID: 0,
		},
		Edit: &MediumEditRequest{
			ID:      0,
			Title:   "",
			Content: "",
		},
		EditAccess: &MediumEditAccessRequest{
			ID:     0,
			Access: policy.MinContentAccess(),
		},
		EditCommentStatus: &MediumEditCommentStatusRequest{
			ID:     0,
			Status: false,
		},
		Add: &MediumAddRequest{
			// What a post gets when the client does not say: public, or
			// registered where the installation allows nothing public.
			Access: policy.MinContentAccess(),
		},
		Delete: &MediumDeleteRequest{},
	}
}

type MediumBrowseRequest struct {
	// How many to pass over.
	Offset uint64 `query:"start"`
	// The most to send.
	Limit uint64 `query:"limit"`
	// What they are ordered by.
	Ordering string `query:"ordering" validate:"max=15,omitempty,oneof=updated_at created_at enabled"`
	// Which way the order runs.
	Direction string `query:"dir" validate:"max=4,omitempty,oneof=asc desc"`
	// Disabled posts as well, for whoever may see them.
	Disabled bool `query:"disabled" validate:"omitempty"`
	// The profile whose posts are listed.
	//
	// OwnerID is the profile whose posts are listed, and is required. There is
	// no list of every post on the site: with the owner optional, and readable
	// from ?oid= as well as the path, leaving it out returned exactly that.
	// Discovery is the job of feeds, search, hashtags and profiles.
	OwnerID int64 `param:"owner_id" validate:"required,min=1"`
	// The viewer's own media library instead: all their photo or video posts
	// on their own profile, with those they put up from inside the article
	// editor, which no other list holds. The owner is then the viewer,
	// whatever the path says.
	Library bool `query:"library" validate:"omitempty"`
	// The viewer's own drafts on this profile instead, articles or topics
	// still being written, which no other list holds.
	Drafts bool `query:"drafts" validate:"omitempty"`
}

type MediumReadRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
}

type MediumEditRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Its title.
	Title string `form:"title" validate:"required,max=100"`
	// Its words, as Markdown.
	Content string `form:"content" validate:"required,max=8000"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
}

type MediumEditAccessRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Who may see it: `public`, `registered`, `followers`, `leaders`,
	// `mutuals`, `admins` or `myself` (permissions.md). The widest an
	// installation allows when left out.
	Access string `form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
}

// MediumEditCommentStatusRequest opens or closes a medium to comments. Status
// binds from "1"/"0" as well as "true"/"false", which is what the web client
// sends.
type MediumEditCommentStatusRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Whether it takes replies.
	Status bool `form:"status" validate:"omitempty"`
}

type MediumAddRequest struct {
	// The profile it is posted on.
	OwnerID int64 `param:"owner_id" validate:"required,numeric"`
	// Its title.
	Title string `form:"title" validate:"required,max=100"`
	// Its words, as Markdown.
	Content string `form:"content" validate:"required,max=8000"`
	// Who may see it, as for changing it. The widest the installation allows
	// when left out.
	Access string `form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
	// The language it is written in, as a BCP 47 tag: "en", "fr-CA". Chosen
	// by the author; left out, the post has none. See package language.
	Language string `form:"language" validate:"omitempty,max=12"`
	// Who may reply: "anyone", "nobody", or any of "followers", "following"
	// and "mentioned" joined by commas. Left out, anyone the profile lets.
	// See constants.NormalizeReplyAccess.
	ReplyAccess string `json:"reply_access" form:"reply_access" validate:"omitempty,max=64"`
	// Who may quote it: "anyone", "followers" or "nobody". Left out, the
	// author's own setting applies. See constants.EffectiveQuotePolicy.
	QuotePolicy string `json:"quote_policy" form:"quote_policy" validate:"omitempty,max=16"`
	// Begun as a draft: an article or a topic, seen by its author alone
	// until it is published (POST /<kind>/<id>/publish). Not read of
	// anything else.
	Draft bool `form:"draft" json:"draft"`
}

// MediumEditQuotePolicyRequest changes who may quote a post.
type MediumEditQuotePolicyRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Who may quote it: `anyone`, `followers` or `nobody`.
	QuotePolicy string `json:"quote_policy" form:"quote_policy" validate:"required,max=16"`
}

// MediumEditReplyAccessRequest changes who may reply to a post.
type MediumEditReplyAccessRequest struct {
	// The post's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Who may reply: `anyone`, `nobody`, or any of `followers`, `following`
	// and `mentioned` joined by commas.
	ReplyAccess string `json:"reply_access" form:"reply_access" validate:"required,max=64"`
}

type MediumDeleteRequest struct {
	// The post's id.
	ID int64 `param:"id"`
}

// MediumPublishRequest publishes a draft: an article or a topic still being
// written becomes a post from now, seen by whoever its audience says, and is
// announced as one made now.
type MediumPublishRequest struct {
	// The draft's id.
	ID int64 `param:"id" validate:"required,numeric"`
	// Who may see it once it is published. Left out, the audience it was
	// saved with.
	Access string `json:"access" form:"access" validate:"omitempty,oneof=public registered followers leaders mutuals admins myself"`
}
