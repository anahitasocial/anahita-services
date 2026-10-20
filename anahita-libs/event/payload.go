package event

type EventPayload struct {
	Name             string `json:"name"`
	SourceService    string `json:"source_service"`
	OwnerID          int64  `json:"owner_id"`
	OwnerType        string `json:"owner_type"`
	SubjectID        int64  `json:"subject_id"`
	SubjectType      string `json:"subject_type"`
	ObjectID         int64  `json:"object_id"`
	ObjectType       string `json:"object_type"`
	TargetID         int64  `json:"target_id"`
	TargetType       string `json:"target_type"`
	ViewerID         int64  `json:"viewer_id"`
	ViewerPersonType string `json:"viewer_user_type"`
	// Part is where a post is in a thread written in one go (roadmap row
	// 101): 1 for the first, 2 and on for the rest, 0 for a post made on its
	// own. Who is told of a thread is told of its first part.
	Part int `json:"part,omitempty"`
}
