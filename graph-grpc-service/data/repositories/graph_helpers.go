package repositories

// buildDegreeJoinCondition constructs the SQL join condition for counting degrees
// based on the specified direction (IN, OUT, BOTH).
func (r *graphRepo) buildDegreeJoinCondition(direction string) string {
	switch direction {
	case "IN":
		return "edge.node_b_id = node.id"
	case "OUT":
		return "edge.node_a_id = node.id"
	default: // BOTH
		// Matches any edge attached to the node
		return "edge.node_a_id = node.id OR edge.node_b_id = node.id"
	}
}
