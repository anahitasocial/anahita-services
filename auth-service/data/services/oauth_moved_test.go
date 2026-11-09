package services

import (
	"testing"

	"auth/data/repositories"
)

type fakeMoved map[int64]bool

func (f fakeMoved) HasMoved(id int64) (bool, error) { return f[id], nil }

// THE RULE THIS GUARDS (roadmap row 124, part 3): somebody who moved their
// account away is minted tokens that post, like, follow and tag nothing;
// anybody else, and they once they move back, the scopes they asked for.
func TestAMovedAccountsTokensCannotPostOrFollow(t *testing.T) {
	s := &OAuthService{Repos: &repositories.Repositories{Moved: fakeMoved{7: true}}}
	asked := "openid profile actors:read actors:write media:write likes:write socialgraph:write taxonomy:write account:write"

	if got := s.withheldIfMoved(7, asked); got != "openid profile actors:read actors:write account:write" {
		t.Errorf("moved: %q", got)
	}
	if got := s.withheldIfMoved(8, asked); got != asked {
		t.Errorf("not moved: %q", got)
	}
}
