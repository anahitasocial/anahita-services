package handlers

import (
	"log"
	"math"
	"strings"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
	"anahita-libs/federation"

	"github.com/gosimple/slug"

	"activitypub/ap"
)

// Locations from the fediverse (roadmap row 82, part 4; the user,
// 2026-10-17 and 2026-10-19). A post kept from another server is tagged
// with the places it names, Pixelfed's `location` above all: each is matched
// to a location here, or one is made, so that it shows among that
// location's posts, in trending and nearby. Only on an installation that
// takes them (FEDERATION_LOCATIONS, on unless `off`). Nobody's own position
// is read: `Arrive` and other check-ins are not (row 94).

const (
	// A location of the same name this near is the same one: Pixelfed's
	// places are mostly cities, whose point differs from one server to the
	// next.
	sameNameWithin = 25000 // metres
	// Any location this near is the same one, whatever it is called.
	samePointWithin = 100 // metres
	// A point made into a location here is kept to four decimals, about
	// 11 m: as near as a place need be, and no nearer than the post said.
	madePointDecimals = 4
	// The most locations read near a point, to find one of the same name.
	nearbyLimit = 20
	// The longest name a location may have.
	locationNameMax = 100
)

// LocationTags reads, adds and takes off a post's location tags.
type LocationTags interface {
	Browse(req *services.GraphEdgeBrowseRequest) ([]*entities.AnahitaEdge, error)
	Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error)
	Delete(id int64) error
}

// locate tags a post kept from elsewhere with the places it names, and, as
// it is edited there, takes off the ones it no longer names.
func (h *Handlers) locate(post *entities.AnahitaNode, note *ap.ReceivedNote, authorID int64) {
	if post == nil || post.ID == 0 || h.Tags == nil || !h.TakesLocations {
		return
	}
	places := federation.ReadPlaces(note.Location, note.Tag)
	want := []int64{}
	for _, place := range places {
		if id := h.locationFor(place, authorID); id > 0 {
			want = append(want, id)
		}
	}
	h.retag(post, want, authorID)
}

// locationFor is the location here a place is: the one of the same name
// within 25 km, else any within 100 m, else one made for it, put down to the
// post's author as a hashtag is to whoever first used it. 0 when none could
// be found or made.
func (h *Handlers) locationFor(place federation.Place, authorID int64) int64 {
	viewer := entities.NewViewer()
	viewer.PersonType = constants.PersonTypeSuperAdmin

	near, err := h.Nodes.Browse(&services.GraphNodeBrowseRequest{
		Types:           []string{constants.NodeBaseLocationEntityType},
		NearbyLatitude:  place.Latitude,
		NearbyLongitude: place.Longitude,
		NearbyRadius:    sameNameWithin,
		Limit:           nearbyLimit,
		Viewer:          viewer,
	})
	if err != nil {
		log.Printf("place %q: finding locations near it: %v", place.Name, err)
		return 0
	}
	name := strings.TrimSpace(place.Name)
	for _, location := range near {
		if name != "" && strings.EqualFold(strings.TrimSpace(location.Name), name) {
			return location.ID
		}
	}
	for _, location := range near {
		if distanceMetres(place.Latitude, place.Longitude, location.GeoLatitude, location.GeoLongitude) <= samePointWithin {
			return location.ID
		}
	}

	// None: one is made, which needs a name to be found by.
	if name == "" {
		return 0
	}
	if letters := []rune(name); len(letters) > locationNameMax {
		name = string(letters[:locationNameMax])
	}
	node := entities.NewAnahitaNode()
	node.Type = constants.NodeBaseLocationEntityType
	node.Service = "location-service"
	node.Enabled = true
	node.Access = constants.AnahitaAccessPublic
	node.Name = name
	node.Alias = slug.Make(name)
	node.GeoLatitude = rounded(place.Latitude, madePointDecimals)
	node.GeoLongitude = rounded(place.Longitude, madePointDecimals)
	node.CreatedBy = authorID
	node.ModifiedBy = authorID
	made, err := h.Nodes.Add(node)
	if err != nil || made == nil {
		log.Printf("place %q: making a location of it: %v", place.Name, err)
		return 0
	}
	return made.ID
}

// retag makes a post's location tags these, and only these.
func (h *Handlers) retag(post *entities.AnahitaNode, want []int64, by int64) {
	have, err := h.Tags.Browse(&services.GraphEdgeBrowseRequest{
		NodeBID: post.ID,
		Types:   []string{constants.EdgeTagLocationType},
		Limit:   100,
	})
	if err != nil {
		log.Printf("post %d: reading its locations: %v", post.ID, err)
		return
	}
	wanted := map[int64]bool{}
	for _, id := range want {
		wanted[id] = true
	}
	tagged := map[int64]bool{}
	for _, edge := range have {
		if edge == nil {
			continue
		}
		if !wanted[edge.NodeAID] {
			if err := h.Tags.Delete(edge.ID); err != nil {
				log.Printf("post %d: taking off location %d: %v", post.ID, edge.NodeAID, err)
			}
			continue
		}
		tagged[edge.NodeAID] = true
	}
	for _, id := range want {
		if tagged[id] {
			continue
		}
		tagged[id] = true
		edge := entities.NewAnahitaEdge()
		edge.Type = constants.EdgeTagLocationType
		edge.NodeAID = id
		edge.NodeAType = constants.NodeBaseLocationEntityType
		edge.NodeBID = post.ID
		edge.NodeBType = post.Type
		edge.CreatedBy = by
		edge.ModifiedBy = by
		if _, err := h.Tags.Add(edge); err != nil {
			log.Printf("post %d: tagging it with location %d: %v", post.ID, id, err)
		}
	}
}

func rounded(value float64, decimals int) float64 {
	scale := math.Pow(10, float64(decimals))
	return math.Round(value*scale) / scale
}

// distanceMetres between two points on the Earth, as the graph reckons it.
func distanceMetres(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371000
	radians := func(degrees float64) float64 { return degrees * math.Pi / 180 }
	dLat := radians(lat2 - lat1)
	dLng := radians(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(radians(lat1))*math.Cos(radians(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
