// Package scoring turns what the signals found into suggestions, best
// first, each with the reasons it is one.
//
// It reads nothing and knows nothing of the database: given the same finds
// it gives the same answer, so that an answer can be explained.
package scoring

import (
	"sort"
	"time"

	"recommendation/constants"
)

// Hit is one profile found by one signal: how strongly, and the ids of what
// it was found through (the leaders who follow it, the hashtags shared).
type Hit struct {
	ID    int64
	Count int
	IDs   []int64
}

// Reason is why a profile is suggested, as it is sent.
type Reason struct {
	Type  string  `json:"type"`
	Count int     `json:"count"`
	IDs   []int64 `json:"ids,omitempty"`

	weight float64
}

// Suggestion is one profile, with what it scored and why.
type Suggestion struct {
	ActorID int64    `json:"actor_id"`
	Score   float64  `json:"score"`
	Reasons []Reason `json:"reasons"`
}

// Rank puts the profiles found in order. `found` is each signal's hits by
// the reason's name, and `lastActive` when each profile was last visited or
// posted to. A profile with no date is taken as dormant.
func Rank(found map[string][]Hit, lastActive map[int64]time.Time, now time.Time) []Suggestion {
	byActor := map[int64]*Suggestion{}

	for _, reason := range constants.ReasonOrder {
		weight := constants.Weights[reason]

		for _, hit := range found[reason] {
			if hit.ID <= 0 || hit.Count <= 0 {
				continue
			}

			points := float64(hit.Count) * weight.Each
			if points > weight.Most {
				points = weight.Most
			}

			suggestion, ok := byActor[hit.ID]
			if !ok {
				suggestion = &Suggestion{ActorID: hit.ID}
				byActor[hit.ID] = suggestion
			}

			ids := hit.IDs
			if len(ids) > constants.MaxIDsInAReason {
				ids = ids[:constants.MaxIDsInAReason]
			}

			suggestion.Score += points
			suggestion.Reasons = append(suggestion.Reasons, Reason{
				Type:   reason,
				Count:  hit.Count,
				IDs:    ids,
				weight: points,
			})
		}
	}

	ranked := make([]Suggestion, 0, len(byActor))
	for _, suggestion := range byActor {
		suggestion.Score *= activity(lastActive[suggestion.ActorID], now)
		if suggestion.Score <= 0 {
			continue
		}

		// The reason that counted most leads. The order they were added
		// in, most personal first, settles a tie.
		sort.SliceStable(suggestion.Reasons, func(i, j int) bool {
			return suggestion.Reasons[i].weight > suggestion.Reasons[j].weight
		})

		ranked = append(ranked, *suggestion)
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].ActorID < ranked[j].ActorID
	})

	return ranked
}

// activity is how much a profile's score is kept by, for how lately it was
// last alive.
func activity(last time.Time, now time.Time) float64 {
	if last.IsZero() {
		return constants.DormantFactor
	}

	quiet := now.Sub(last)

	switch {
	case quiet <= constants.ActiveWithin:
		return 1
	case quiet <= constants.DormantAfter:
		return constants.QuietFactor
	default:
		return constants.DormantFactor
	}
}
