package handlers

import (
	"encoding/json"
	"testing"

	"activitypub/ap"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/data/services"
)

// The graph, as far as locations go: what is near a point, and what is
// made.
type placeNodes struct {
	services.GraphNodeGRPCInterface
	locations []*entities.AnahitaNode
	made      []*entities.AnahitaNode
	nextID    int64
}

func (f *placeNodes) Browse(req *services.GraphNodeBrowseRequest) ([]*entities.AnahitaNode, error) {
	found := []*entities.AnahitaNode{}
	for _, location := range f.locations {
		if distanceMetres(req.NearbyLatitude, req.NearbyLongitude, location.GeoLatitude, location.GeoLongitude) <= float64(req.NearbyRadius) {
			found = append(found, location)
		}
	}
	return found, nil
}

func (f *placeNodes) Add(node *entities.AnahitaNode) (*entities.AnahitaNode, error) {
	f.nextID++
	node.ID = f.nextID
	f.made = append(f.made, node)
	f.locations = append(f.locations, node)
	return node, nil
}

// A post's location tags.
type placeTags struct {
	edges  []*entities.AnahitaEdge
	nextID int64
}

func (f *placeTags) Browse(req *services.GraphEdgeBrowseRequest) ([]*entities.AnahitaEdge, error) {
	found := []*entities.AnahitaEdge{}
	for _, edge := range f.edges {
		if edge.NodeBID == req.NodeBID {
			found = append(found, edge)
		}
	}
	return found, nil
}

func (f *placeTags) Add(edge *entities.AnahitaEdge) (*entities.AnahitaEdge, error) {
	f.nextID++
	edge.ID = f.nextID
	f.edges = append(f.edges, edge)
	return edge, nil
}

func (f *placeTags) Delete(id int64) error {
	kept := []*entities.AnahitaEdge{}
	for _, edge := range f.edges {
		if edge.ID != id {
			kept = append(kept, edge)
		}
	}
	f.edges = kept
	return nil
}

func (f *placeTags) taggedWith(postID int64) []int64 {
	ids := []int64{}
	for _, edge := range f.edges {
		if edge.NodeBID == postID {
			ids = append(ids, edge.NodeAID)
		}
	}
	return ids
}

func kept(id int64) *entities.AnahitaNode {
	post := entities.NewAnahitaNode()
	post.ID = id
	post.Type = constants.NodeMediumPhotoEntityType
	return post
}

func noteWith(location, tag string) *ap.ReceivedNote {
	note := &ap.ReceivedNote{}
	if location != "" {
		note.Location = json.RawMessage(location)
	}
	if tag != "" {
		note.Tag = json.RawMessage(tag)
	}
	return note
}

// THE RULE THIS GUARDS (roadmap row 82, part 4): a post from elsewhere is
// tagged with the places it names, each matched to a location here (the
// same name within 25 km, else any within 100 m) or made, put down to the
// post's author; an edit there takes off what it no longer names; and
// nothing is read where the installation takes no locations.
func TestAPostFromElsewhereIsTaggedWithItsPlaces(t *testing.T) {
	vancouver := location(1, "Vancouver", "vancouver", 49.2827, -123.1207)
	beach := location(2, "English Bay Beach", "english-bay-beach", 49.2869, -123.143)
	nodes := &placeNodes{locations: []*entities.AnahitaNode{vancouver, beach}, nextID: 100}
	tags := &placeTags{}
	h := &Handlers{Env: &Env{Nodes: nodes, Tags: tags, TakesLocations: true}}

	// Pixelfed's city, a few km from ours: the same name, the same one.
	// A point 30 m from the beach, called something else: the beach. A
	// place nowhere near anything here: made.
	note := noteWith(
		`{"type":"Place","name":"Vancouver","latitude":49.25,"longitude":-123.10}`,
		`[{"type":"Place","name":"Sunset Beach","latitude":49.2871,"longitude":-123.1427},
		  {"type":"Place","name":"Promenade","latitude":10.12345678,"longitude":20.98765432}]`)
	h.locate(kept(500), note, 77)

	got := tags.taggedWith(500)
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 101 {
		t.Fatalf("tagged with %v; want Vancouver, the beach and one made", got)
	}
	made := nodes.made[0]
	if made.Name != "Promenade" || made.Alias != "promenade" || made.CreatedBy != 77 ||
		made.GeoLatitude != 10.1235 || made.GeoLongitude != 20.9877 || made.Type != constants.NodeBaseLocationEntityType {
		t.Errorf("made %+v", made)
	}

	// Edited there: it names only the beach now.
	h.locate(kept(500), noteWith(`{"type":"Place","name":"English Bay Beach","latitude":49.2869,"longitude":-123.143}`, ""), 77)
	if got := tags.taggedWith(500); len(got) != 1 || got[0] != 2 {
		t.Errorf("after the edit, tagged with %v; want the beach alone", got)
	}

	// A place with no name and nothing near it is not made.
	h.locate(kept(501), noteWith(`{"type":"Place","latitude":-40,"longitude":170}`, ""), 77)
	if got := tags.taggedWith(501); len(got) != 0 {
		t.Errorf("tagged with %v; a nameless place nowhere is no location", got)
	}

	// Where the installation takes no locations, nothing is read.
	h.TakesLocations = false
	h.locate(kept(502), noteWith(`{"type":"Place","name":"Vancouver","latitude":49.2827,"longitude":-123.1207}`, ""), 77)
	if got := tags.taggedWith(502); len(got) != 0 {
		t.Errorf("tagged with %v where locations are not taken", got)
	}
}
