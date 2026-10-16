package handlers

import (
	"net/http"

	"anahita-libs/proto/graphpb"

	"github.com/labstack/echo/v4"
)

// What person-service and group-service answer for a purge. Both call the
// same RPCs and must say the same things, so the answers are written once.

// PurgeRefusal is why one profile cannot be purged.
type PurgeRefusal struct {
	// The profile's id.
	ID int64 `json:"id"`
	// Why not.
	Reason string `json:"reason"`
}

// PurgeRefused refuses a purge: nothing was queued.
type PurgeRefused struct {
	// Always `purge_refused`.
	Error string `json:"error"`
	// Each profile that cannot be purged, and why.
	Refused []PurgeRefusal `json:"refused"`
}

// PurgeQueued says what a request to purge did.
type PurgeQueued struct {
	// What to ask about it by.
	BatchID string `json:"batch_id"`
	// How many were queued.
	Queued int64 `json:"queued"`
	// How many were purged or queued already.
	Skipped int64 `json:"skipped"`
}

// PurgeItem is how far one profile of a purge has got.
type PurgeItem struct {
	// The profile's id.
	ID int64 `json:"id"`
	// What it was called.
	Alias string `json:"alias"`
	// `queued`, `done` or `failed`.
	Status string `json:"status"`
	// Why it failed.
	Error string `json:"error,omitempty"`
}

// PurgeProgress is how far a purge request has got.
type PurgeProgress struct {
	// What it is asked about by.
	BatchID string `json:"batch_id"`
	// How many it names.
	Total int64 `json:"total"`
	// How many are waiting.
	Queued int64 `json:"queued"`
	// How many are purged.
	Done int64 `json:"done"`
	// How many failed.
	Failed int64 `json:"failed"`
	// Each profile.
	Items []PurgeItem `json:"items"`
}

// RespondPurgeRequested answers a request to purge.
//
//	409  one or more of the actors cannot be purged; nothing was queued
//	202  queued, with the batch id to ask about
//	200  nothing to do: every one was already purged or already queued
func RespondPurgeRequested(c echo.Context, result *graphpb.ActorPurgeBatchResponse) error {
	if len(result.Refused) > 0 {
		refused := make([]PurgeRefusal, 0, len(result.Refused))
		for _, refusal := range result.Refused {
			refused = append(refused, PurgeRefusal{ID: refusal.Id, Reason: refusal.Reason})
		}

		return c.JSON(http.StatusConflict, PurgeRefused{Error: "purge_refused", Refused: refused})
	}

	status := http.StatusAccepted
	if result.Queued == 0 {
		status = http.StatusOK
	}

	return c.JSON(status, PurgeQueued{BatchID: result.BatchId, Queued: result.Queued, Skipped: result.Skipped})
}

// RespondPurgeProgress answers how far a purge request has got, or 404 for a
// batch nobody was given.
func RespondPurgeProgress(c echo.Context, batch *graphpb.ActorPurgeBatchReadResponse) error {
	if batch.Total == 0 {
		return c.NoContent(http.StatusNotFound)
	}

	items := make([]PurgeItem, 0, len(batch.Items))
	for _, item := range batch.Items {
		items = append(items, PurgeItem{
			ID:     item.Id,
			Alias:  item.Alias,
			Status: item.Status,
			Error:  item.Error,
		})
	}

	return c.JSON(http.StatusOK, PurgeProgress{
		BatchID: batch.BatchId,
		Total:   batch.Total,
		Queued:  batch.Queued,
		Done:    batch.Done,
		Failed:  batch.Failed,
		Items:   items,
	})
}
