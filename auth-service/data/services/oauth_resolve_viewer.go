package services

import (
	"errors"
	"fmt"

	libs_entities "anahita-libs/data/entities"
)

// ResolveViewerByID builds a Viewer from a user ID.
//
// It is intended for flows that do not have a session-scoped Viewer,
// such as the /token authorization-code exchange where only the user ID
// is available from the authorization-code record.
//
// The method reads the user via Repos.Users.ReadByID, maps user fields onto
// a new Viewer, and marks the Viewer as authenticated.
//
// Callers must ensure authorization has already been validated by some
// other mechanism (for example, a valid authorization code).
func (s *OAuthService) ResolveViewerByID(id int64) (*libs_entities.Viewer, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}

	user, err := s.Repos.Users.ReadByID(id)
	if err != nil {
		return nil, fmt.Errorf("resolve user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	viewer := libs_entities.NewViewer()
	viewer.ID = user.ID
	viewer.Type = user.Type
	viewer.Name = user.Name
	viewer.Username = user.Username
	viewer.LastVisitDate = user.LastVisitDate
	viewer.Authenticated = true

	return viewer, nil
}
