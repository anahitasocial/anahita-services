package handlers

import (
	"context"

	"anahita-libs/proto/graphpb"
)

// The gRPC calls under the names they had until v1.0.17, before they were
// given BREAD names (docs/plans/done/bread-names.md). A service still running
// the release before calls these during a rolling deploy, so they are
// served for one release, each the same as its new name, and removed in
// the release after.

// ListQuotes is BrowseQuotes.
func (h *mediumHandler) ListQuotes(ctx context.Context, req *graphpb.QuotesBrowseRequest) (*graphpb.QuotesBrowseResponse, error) {
	return h.BrowseQuotes(ctx, req)
}

// ListSaved is BrowseSaved.
func (h *mediumHandler) ListSaved(ctx context.Context, req *graphpb.SavedBrowseRequest) (*graphpb.SavedBrowseResponse, error) {
	return h.BrowseSaved(ctx, req)
}

// ListCitedBy is BrowseCitedBy.
func (h *mediumHandler) ListCitedBy(ctx context.Context, req *graphpb.CitedByBrowseRequest) (*graphpb.CitedByBrowseResponse, error) {
	return h.BrowseCitedBy(ctx, req)
}

// ListAuthored is BrowseAuthored.
func (h *mediumHandler) ListAuthored(ctx context.Context, req *graphpb.AuthoredBrowseRequest) (*graphpb.AuthoredBrowseResponse, error) {
	return h.BrowseAuthored(ctx, req)
}

// List is Browse.
func (h *eventGrpcHandler) List(ctx context.Context, req *graphpb.EventBrowseRequest) (*graphpb.EventBrowseResponse, error) {
	return h.Browse(ctx, req)
}
