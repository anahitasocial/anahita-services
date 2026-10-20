// Package constants holds what the recommendation service is tuned by.
//
// Constants, and not settings: they are the service's own judgement of what
// makes a good suggestion, and an installation that wants another judgement
// runs another service behind the same contract. See docs/recommendations.md.
package constants

import "time"

// The kinds of actor that are suggested, as a request names them, and the
// types they stand for. An event is suggested while it is still to come or
// on, and has not been called off.
const (
	KindPerson = "person"
	KindGroup  = "group"
	KindEvent  = "event"

	PersonType = "node.actor.person-service.person.v1"
	GroupType  = "node.actor.group-service.group.v1"
	EventType  = "node.actor.actor-service.event.v1"
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
	ReasonFollowsYou        = "follows_you"
	ReasonFollowedByLeaders = "followed_by_leaders"
	ReasonInviter           = "inviter"
	ReasonHostedByLeader    = "hosted_by_leader"
	ReasonSharedInterest    = "shared_interest"
	ReasonYouEngaged        = "you_engaged"
	ReasonPopular           = "popular"
	ReasonFeatured          = "featured"
	// In public photos or videos with the viewer: tagged together, or one
	// in a photo the other posted (roadmap row 104).
	ReasonPicturedWithYou = "pictured_with_you"
	// In public photos or videos with people the viewer follows, or in ones
	// they posted. `ids` are those people (roadmap row 104).
	ReasonPicturedWithLeaders = "pictured_with_leaders"
	// Others have cited them: their profile, or posts of theirs, shown
	// within an article or a topic (the user, 2026-10-16).
	ReasonCited = "cited"
	// People the viewer follows have cited them.
	ReasonCitedByLeaders     = "cited_by_leaders"
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
	ReasonHostedByLeader:    {Each: 3, Most: 3},
	ReasonSharedInterest:    {Each: 2, Most: 8},
	ReasonYouEngaged:        {Each: 2, Most: 8},
	ReasonPopular:           {Each: 0.5, Most: 0.5},
	ReasonCited:             {Each: 1, Most: 4},
	ReasonCitedByLeaders:    {Each: 3, Most: 9},
	ReasonFeatured:          {Each: 0.5, Most: 0.5},
	// As an invitation, and as a shared interest (the user, 2026-10-20).
	ReasonPicturedWithYou:     {Each: 3, Most: 6},
	ReasonPicturedWithLeaders: {Each: 2, Most: 8},
}

// The most somebody may set a reason to count for, each time it is found.
// Set to nothing, a reason is not used for them at all.
const MaxWeight = 10

// WeightsFor is the weights to rank for somebody by: the service's own,
// with theirs in place of each they have set. What a reason may count for
// at most keeps its proportion to what it counts for each time.
func WeightsFor(own map[string]float64) map[string]Weight {
	weights := make(map[string]Weight, len(Weights))

	for reason, weight := range Weights {
		each, set := own[reason]
		if set && each >= 0 && each <= MaxWeight && weight.Each > 0 {
			weight = Weight{Each: each, Most: each * weight.Most / weight.Each}
		}
		weights[reason] = weight
	}

	return weights
}

// The order reasons are given in when two count the same: the more
// personal first.
var ReasonOrder = []string{
	ReasonFollowsYou,
	ReasonPicturedWithYou,
	ReasonFollowedByLeaders,
	ReasonCitedByLeaders,
	ReasonPicturedWithLeaders,
	ReasonInviter,
	ReasonHostedByLeader,
	ReasonYouEngaged,
	ReasonSharedInterest,
	ReasonCited,
	ReasonFeatured,
	ReasonPopular,
}

// How far back each signal looks.
const (
	InterestWindow = 365 * 24 * time.Hour
	EngagedWindow  = 90 * 24 * time.Hour
	PopularWindow  = 30 * 24 * time.Hour
)

// A profile quiet for three months counts for half, and one nobody has been
// on or posted to for a year for a tenth: it comes after every profile that
// is alive. That is as far as it goes for a group, which is still there
// when nothing else is. A person quiet for longer than the reader chose
// (ActiveWithinChoices) is left out altogether.
const (
	ActiveWithin  = 90 * 24 * time.Hour
	DormantAfter  = 365 * 24 * time.Hour
	QuietFactor   = 0.5
	DormantFactor = 0.1
)

// How lately somebody must have been here, or posted, to be suggested: the
// reader's to choose, in days, from these. Nothing is no limit, which on an
// installation brought over from an older one may be the only way anybody
// is suggested at all.
var ActiveWithinChoices = []int{30, 90, 180, 365, 730, 1825, 0}

// What it is for somebody who has not chosen: a year.
const ActiveWithinDefault = 365

// IsActiveWithinChoice says whether a number of days is one of them.
func IsActiveWithinChoice(days int) bool {
	for _, choice := range ActiveWithinChoices {
		if choice == days {
			return true
		}
	}
	return false
}

// How many suggestions an answer holds, and how many of the best are put to
// the graph to ask which of them the viewer may see.
const (
	DefaultLimit = 10
	MaxLimit     = 50
	GateSize     = 100
)
