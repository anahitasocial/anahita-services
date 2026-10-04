package requests

// ReportAdd is one person reporting one node.
type ReportAdd struct {
	TargetID  int64  `json:"target_id" validate:"required,min=1"`
	ReasonKey string `json:"reason_key" validate:"required,max=64"`
	Details   string `json:"details" validate:"max=1000"`
}

// CaseResolve closes a case. Status is "actioned" or "dismissed"; Action says
// what was done.
type CaseResolve struct {
	Status string `json:"status" validate:"required,oneof=actioned dismissed"`
	Action string `json:"action" validate:"omitempty,oneof=none deleted disabled archived blocked other"`
	Note   string `json:"note" validate:"max=2000"`
}
