package permissions

import "anahita-libs/data/entities"

// VisibilityFilter answers which of some nodes a viewer may see: the gate
// every listing goes through, asked about ids. The graph's FilterVisible is
// one (anahita-libs/data/services.GraphNodeGRPCInterface): the same rules as
// CanRead, as SQL, for many nodes at once.
type VisibilityFilter interface {
	FilterVisible(ids []int64, viewer *entities.Viewer) ([]int64, error)
}

// CanReadByID reports whether the viewer may read one node, asked by its id
// where CanRead is asked with the node loaded. False when the answer cannot
// be had.
func CanReadByID(filter VisibilityFilter, id int64, viewer *entities.Viewer) bool {
	if id <= 0 {
		return false
	}

	readable, err := CanReadEach(filter, []int64{id}, viewer)
	if err != nil {
		return false
	}
	return readable[id]
}

// CanReadEach answers, of many nodes at once, which the viewer may read:
// true for each they may. Asked of the filter once, whatever the number.
func CanReadEach(filter VisibilityFilter, ids []int64, viewer *entities.Viewer) (map[int64]bool, error) {
	readable := map[int64]bool{}

	asked := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			asked = append(asked, id)
		}
	}
	if len(asked) == 0 {
		return readable, nil
	}

	visible, err := filter.FilterVisible(asked, viewer)
	if err != nil {
		return nil, err
	}
	for _, id := range visible {
		readable[id] = true
	}
	return readable, nil
}
