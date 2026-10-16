package handlers

import (
	"anahita-libs/responses"
	"context"
	"time"

	"atproto/bsky"
	"atproto/data/repositories"

	"anahita-libs/crypt"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/event"
	"anahita-libs/federation"
	an_mw "anahita-libs/middleware"
	"anahita-libs/ratelimit"
	"anahita-libs/site"

	"github.com/labstack/echo/v4"
	"net/http"
)

// Reading Bluesky.
//
// Somebody here looks up an account on Bluesky and follows it, the way
// they follow anybody. Its posts are then kept here as they are written,
// and are in the feed of whoever follows it.
//
// Nothing is written to Bluesky by any of this. The account followed is
// not told, and nobody here has a name there: being on Bluesky needs a
// server of the installation's own, which is the second part of the work
// (docs/plans/federation.md). Reading needs only a way out to the
// internet.

// Bluesky is the network's public reader.
type Bluesky interface {
	Profile(ctx context.Context, actor string) (*bsky.Profile, error)
	AuthorFeed(ctx context.Context, did string, limit int) ([]bsky.FeedItem, error)
	PostAt(ctx context.Context, uri string) (*bsky.Post, error)
	Following(ctx context.Context, did, cursor string) ([]bsky.Profile, string, error)
}

// Refused is the servers this installation will have nothing to do with.
type Refused interface {
	IsBlocked(domain string) bool
}

// Env is what the handlers work with.
type Env struct {
	Site    site.Addresses
	Nodes   services.GraphNodeGRPCInterface
	Actors  repositories.ActorsInterface
	Bluesky Bluesky
	Blocks  Refused
	// For writing to Bluesky as an account somebody connected: who
	// connected what, what was written, what does the writing, what the
	// kept tokens are encrypted with, where what waits to be written is
	// kept, and where an account's server is looked up. Without all of
	// them nothing is written.
	Accounts  repositories.AccountsInterface
	Sent      repositories.SentInterface
	Writer    Writer
	Crypt     crypt.Crypt
	Queue     Enqueuer
	Directory string
	// How long a photo post's pictures are waited for before writing it
	// is left to be tried again: it is said to be made a moment before
	// its pictures are listed.
	PicturesWait time.Duration
	// Where a post of somebody's own, made on Bluesky with the account
	// they connected, is made here as theirs. Nil where that is not done.
	Media services.MediaGRPCInterface
	// For keeping what people on Bluesky do to what was written from
	// here: likes, follows, and who has blocked whom. Nil where that is
	// not read.
	Likes       services.LikesGRPCInterface
	FollowEdges services.SocialgraphFollowGRPCInterface
	Relations   services.SocialgraphGRPCInterface
	// What is kept here of what people on Bluesky did to somebody here,
	// and what asks Bluesky whether each is still so: for finding what
	// was taken back. Nil where that is not looked for.
	Kept   repositories.KeptInterface
	Verify Verified
	// Reposts in the graph, and which post each one heard was of. Nil
	// where reposts are not kept.
	Reposts       services.RepostGRPCInterface
	RepostRecords repositories.RepostsInterface
	// What copies pictures into this installation's storage.
	Keeper *federation.Keeper
	// Where a video post's file is read from, at an address only the
	// services reach, to be sent to Bluesky; and what reads it. Without
	// Transfers a video post goes as a card that leads to it.
	Transfers services.FileTransfersInterface
	Fetch     *http.Client
	// What finds the card of a link. Nil for none.
	Cards federation.Cards
	// Announce says on the bus that a post was kept or is gone. Nil when
	// there is no bus to say it on.
	Announce func(payload *event.EventPayload, name string) error
	// Later runs what need not hold up an answer. Nil runs it at once.
	Later func(func())
	// Changed is called when who is followed on Bluesky may have changed,
	// for the listener to be told whose posts are wanted.
	Changed func()
	Limiter ratelimit.Counter
	// Where pictures are fetched from: "https://cdn.bsky.app".
	CDN string
	// The time, for a test to say what it is.
	Now func() time.Time
	// How long a profile that has just changed is left before it is read
	// again: the reader of the network hears of the change as this does.
	Settle time.Duration
}

type Handlers struct {
	*Env
	tokens tokens
}

func NewHandlers(env *Env) *Handlers {
	return &Handlers{Env: env}
}

func getViewer(c echo.Context) *entities.Viewer {
	return an_mw.GetViewer(c)
}

func refuse(c echo.Context, status int, code string) error {
	return c.JSON(status, responses.Refusal{Error: code})
}

// What everybody on Bluesky is kept as coming from: the site people read
// it on, which is where "see it there" leads. Their own servers are many
// and are nobody's business here.
const originDomain = "bsky.app"

// later runs what need not hold up an answer.
func (h *Handlers) later(work func()) {
	if h.Later != nil {
		h.Later(work)
		return
	}
	work()
}

func (h *Handlers) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now().UTC()
}

func (h *Handlers) changed() {
	if h.Changed != nil {
		h.Changed()
	}
}

// refused says whether an account is one this installation will have
// nothing to do with: Bluesky as a whole is refused, or a name its handle
// is under.
func (h *Handlers) refused(handle string) bool {
	if h.Blocks == nil {
		return false
	}
	return h.Blocks.IsBlocked(originDomain) || (handle != "" && h.Blocks.IsBlocked(handle))
}

func cutRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}
