package handlers

import (
	"log"
	"net/http"
	"time"

	libs_responses "anahita-libs/responses"

	"auth/data/repositories"
	"auth/requests"

	"github.com/labstack/echo/v4"
)

const (
	accountsDefaultLimit = 50

	accountDateLayout = "2006-01-02"
)

type accountsHandler struct {
	*Env
}

func newAccountsHandler(env *Env) *accountsHandler {
	return &accountsHandler{
		env,
	}
}

type accountResponse struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Alias      string `json:"alias"`
	Name       string `json:"name"`
	PersonType string `json:"person_type,omitempty"`
	State      string `json:"state"`
	Tier       string `json:"tier"`

	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`

	// People only. Pointers, so that a group carries neither rather than
	// carrying "false".
	EmailVerified *bool `json:"email_verified,omitempty"`
	Onboarded     *bool `json:"onboarded,omitempty"`

	ContentCount  int64 `json:"content_count"`
	FollowerCount int64 `json:"follower_count"`
	LeaderCount   int64 `json:"leader_count"`

	// Groups only.
	AdminCount *int64 `json:"admin_count,omitempty"`
}

// Browse lists the site's people or groups for cleanup, least recently active
// first.
//
//	GET /accounts?kind=person|group
//
// Filters, all optional: q, tier (empty, dormant, active), state (active,
// disabled, archived, deleted), last_active_before and created_before
// (YYYY-MM-DD), content_max, and for people email_verified=false and
// onboarded=false, for groups has_admin=false. sort is last_active_at,
// created_at or name; dir is asc or desc; limit is at most 500, which is as
// many accounts as one purge request may name.
//
// Super administrators only. Purged accounts are not listed.
func (h *accountsHandler) Browse(c echo.Context) error {
	viewer := getViewer(c)

	if err := h.Perms.Account.CanBrowse(viewer); err != nil {
		log.Println("accounts browse:", err)
		return c.NoContent(http.StatusForbidden)
	}

	req := &requests.AccountBrowseRequest{}
	if err := c.Bind(req); err != nil {
		log.Println("accounts browse:", err)
		return c.NoContent(http.StatusBadRequest)
	}
	if err := c.Validate(req); err != nil {
		log.Println("accounts browse:", err)
		return c.NoContent(http.StatusBadRequest)
	}

	filter := &repositories.AccountFilter{
		Kind:          repositories.AccountKindPerson,
		Search:        req.Search,
		Tier:          req.Tier,
		State:         req.State,
		ContentMax:    -1,
		DormantBefore: time.Now().AddDate(0, 0, -h.Config.GetAccountDormantDays()),
		Sort:          req.Sort,
		Descending:    req.Direction == "desc",
		Limit:         req.Limit,
		Offset:        req.Offset,
	}
	if filter.Limit == 0 {
		filter.Limit = accountsDefaultLimit
	}
	if req.ContentMax != nil {
		filter.ContentMax = *req.ContentMax
	}

	// The validator has already accepted both as dates.
	if req.LastActiveBefore != "" {
		day, _ := time.Parse(accountDateLayout, req.LastActiveBefore)
		filter.LastActiveBefore = &day
	}
	if req.CreatedBefore != "" {
		day, _ := time.Parse(accountDateLayout, req.CreatedBefore)
		filter.CreatedBefore = &day
	}

	// Each kind has filters that mean nothing for the other. They are
	// ignored rather than refused: the client keeps one filter bar for both.
	if req.Kind == repositories.AccountKindGroup {
		filter.Kind = repositories.AccountKindGroup
		filter.OnlyWithoutAdmin = req.HasAdmin == "false"
	} else {
		filter.OnlyUnverified = req.EmailVerified == "false"
		filter.OnlyNotOnboarded = req.Onboarded == "false"
	}

	found, err := h.Repos.Accounts.Browse(filter)
	if err != nil {
		log.Println("accounts browse:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	total, err := h.Repos.Accounts.Count(filter)
	if err != nil {
		log.Println("accounts browse: count failed:", err)
		return c.NoContent(http.StatusInternalServerError)
	}

	items := make([]accountResponse, 0, len(found))
	for _, account := range found {
		items = append(items, mapAccount(filter.Kind, account))
	}

	type Response struct {
		Data       []accountResponse         `json:"data"`
		Pagination libs_responses.Pagination `json:"pagination"`
	}

	return c.JSON(http.StatusOK, &Response{
		Data: items,
		Pagination: libs_responses.Pagination{
			Limit:  filter.Limit,
			Offset: filter.Offset,
			Total:  total,
		},
	})
}

func mapAccount(kind string, account *repositories.Account) accountResponse {
	item := accountResponse{
		ID:            account.ID,
		Kind:          kind,
		Alias:         account.Alias,
		Name:          account.Name,
		PersonType:    account.PersonType,
		State:         account.State,
		Tier:          account.Tier,
		CreatedAt:     account.CreatedAt,
		LastActiveAt:  account.LastActiveAt,
		ContentCount:  account.ContentCount,
		FollowerCount: account.FollowerCount,
		LeaderCount:   account.LeaderCount,
	}

	if kind == repositories.AccountKindGroup {
		item.AdminCount = &account.AdminCount
	} else {
		item.EmailVerified = &account.EmailVerified
		item.Onboarded = &account.Onboarded
	}

	return item
}
