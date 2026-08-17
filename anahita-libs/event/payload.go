package event

type EventPayload struct {
	Name           string `json:"name"`
	SourceService  string `json:"source_service"`
	OwnerID        int64  `json:"owner_id"`
	OwnerType      string `json:"owner_type"`
	SubjectID      int64  `json:"subject_id"`
	SubjectType    string `json:"subject_type"`
	ObjectID       int64  `json:"object_id"`
	ObjectType     string `json:"object_type"`
	TargetID       int64  `json:"target_id"`
	TargetType     string `json:"target_type"`
	ViewerID       int64  `json:"viewer_id"`
	ViewerUserType string `json:"viewer_user_type"`
}
