package handlers

import (
	"net/http"

	"anahita-libs/proto/graphpb"

	"github.com/labstack/echo/v4"
)

// What person-service and group-service answer for a purge. Both call the
// same RPCs and must say the same things, so the answers are written once.

type purgeRefusal struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

type purgeItem struct {
	ID     int64  `json:"id"`
	Alias  string `json:"alias"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// RespondPurgeRequested answers a request to purge.
//
//	409  one or more of the actors cannot be purged; nothing was queued
//	202  queued, with the batch id to ask about
//	200  nothing to do: every one was already purged or already queued
func RespondPurgeRequested(c echo.Context, result *graphpb.ActorPurgeBatchResponse) error {
	if len(result.Refused) > 0 {
		refused := make([]purgeRefusal, 0, len(result.Refused))
		for _, refusal := range result.Refused {
			refused = append(refused, purgeRefusal{ID: refusal.Id, Reason: refusal.Reason})
		}

		return c.JSON(http.StatusConflict, map[string]any{
			"error":   "purge_refused",
			"refused": refused,
		})
	}

	status := http.StatusAccepted
	if result.Queued == 0 {
		status = http.StatusOK
	}

	return c.JSON(status, map[string]any{
		"batch_id": result.BatchId,
		"queued":   result.Queued,
		"skipped":  result.Skipped,
	})
}

// RespondPurgeProgress answers how far a purge request has got, or 404 for a
// batch nobody was given.
func RespondPurgeProgress(c echo.Context, batch *graphpb.ActorPurgeBatchReadResponse) error {
	if batch.Total == 0 {
		return c.NoContent(http.StatusNotFound)
	}

	items := make([]purgeItem, 0, len(batch.Items))
	for _, item := range batch.Items {
		items = append(items, purgeItem{
			ID:     item.Id,
			Alias:  item.Alias,
			Status: item.Status,
			Error:  item.Error,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"batch_id": batch.BatchId,
		"total":    batch.Total,
		"queued":   batch.Queued,
		"done":     batch.Done,
		"failed":   batch.Failed,
		"items":    items,
	})
}
