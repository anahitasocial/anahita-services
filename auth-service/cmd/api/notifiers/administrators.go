package notifiers

import (
	an_constants "anahita-libs/constants"

	"auth/data/entities"
	"auth/data/repositories"
	"auth/requests"
)

// administratorsPage is how many administrators are read at a time.
const administratorsPage = 100

// administrators is every enabled administrator and super administrator,
// read a page at a time to the end. One page of the default size was read
// before, and on a site with more than twenty of them the oldest were never
// told of a new member or a signup request (roadmap row 96, part 5).
func administrators(repos *repositories.Repositories) ([]*entities.User, error) {
	var all []*entities.User
	for offset := uint64(0); ; offset += administratorsPage {
		req := requests.NewUserRequest().Browse
		req.PersonTypes = []string{
			an_constants.PersonTypeAdmin,
			an_constants.PersonTypeSuperAdmin,
		}
		req.Ordering = "id"
		req.Direction = "asc"
		req.Limit = administratorsPage
		req.Offset = offset

		page, err := repos.Users.Browse(req)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < administratorsPage {
			return all, nil
		}
	}
}
