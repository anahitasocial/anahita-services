// Package constants holds what the recommendation service is tuned by.
//
// Constants, and not settings: they are the service's own judgement of what
// makes a good suggestion, and an installation that wants another judgement
// runs another service behind the same contract. See docs/recommendations.md.
package constants

import "time"

// The kinds of actor that are suggested, as a request names them, and the
// types they stand for. Events are not suggested: an event is answered, and
// found through its group or by being invited.
const (
	KindPerson = "person"
	KindGroup  = "group"

	PersonType = "node.actor.person-service.person.v1"
	GroupType  = "node.actor.group-service.group.v1"
)

// Where a suggestion is asked for from. Somebody who has just joined
// follows nobody, so what is asked for there leaves out everything that is
// worked out from who they follow.
const (
	ContextHome       = "home"
	ContextOnboarding = "onboarding"
)

// Why a profile is suggested, as the answer names it.
const (
	ReasonFollowsYou         = "follows_you"
	ReasonFollowedByLeaders  = "followed_by_leaders"
	ReasonInviter            = "inviter"
	ReasonSharedInterest     = "shared_interest"
	ReasonYouEngaged         = "you_engaged"
	ReasonPopular            = "popular"
	ReasonFeatured           = "featured"
	MaxIDsInAReason          = 3
	MaxCandidatesFromASignal = 200
)

// How much each reason counts for, and the most it may count: a profile ten
// of your leaders follow is not ten times the suggestion one of five is.
type Weight struct {
	Each float64
	Most float64
}

var Weights = map[string]Weight{
	ReasonFollowsYou:        {Each: 4, Most: 4},
	ReasonFollowedByLeaders: {Each: 3, Most: 15},
	ReasonInviter:           {Each: 3, Most: 3},
	ReasonSharedInterest:    {Each: 2, Most: 8},
	ReasonYouEngaged:        {Each: 2, Most: 8},
	ReasonPopular:           {Each: 0.5, Most: 0.5},
	ReasonFeatured:          {Each: 0.5, Most: 0.5},
}

// The order reasons are given in when two count the same: the more
// personal first.
var ReasonOrder = []string{
	ReasonFollowsYou,
	ReasonFollowedByLeaders,
	ReasonInviter,
	ReasonYouEngaged,
	ReasonSharedInterest,
	ReasonFeatured,
	ReasonPopular,
}

// How far back each signal looks.
const (
	InterestWindow = 365 * 24 * time.Hour
	EngagedWindow  = 90 * 24 * time.Hour
	PopularWindow  = 30 * 24 * time.Hour
)

// A profile nobody has been on or posted to for a year is not suggested,
// and one quiet for three months counts for half.
const (
	ActiveWithin  = 90 * 24 * time.Hour
	DormantAfter  = 365 * 24 * time.Hour
	QuietFactor   = 0.5
	DormantFactor = 0
)

// How many suggestions an answer holds, and how many of the best are put to
// the graph to ask which of them the viewer may see.
const (
	DefaultLimit = 10
	MaxLimit     = 50
	GateSize     = 100
)
