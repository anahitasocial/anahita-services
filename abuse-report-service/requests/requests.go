package requests

// ReportAdd is one person reporting one node.
type ReportAdd struct {
	// The id of what is reported: a post, a reply, a profile.
	TargetID int64 `json:"target_id" validate:"required,min=1"`
	// Why, by the reason's key (GET /abuse-reports/reasons).
	ReasonKey string `json:"reason_key" validate:"required,max=64"`
	// What the reporter adds in their own words. Needed where the reason
	// says so.
	Details string `json:"details" validate:"max=1000"`
	// With reason_key "rule": the rule broken, by its title as the reporter
	// read it, and whose rule it is: 0 for the site's, or the group or event
	// the reported thing belongs to (roadmap row 102, part 6).
	RuleTitle     string `json:"rule" validate:"max=100"`
	RuleProfileID int64  `json:"rule_profile_id" validate:"min=0"`
}

// CaseResolve closes a case. Status is "actioned" or "dismissed"; Action says
// what was done.
type CaseResolve struct {
	// The case's id.
	ID int64 `param:"id"`
	// `actioned`, something was done; or `dismissed`, nothing needed doing.
	Status string `json:"status" validate:"required,oneof=actioned dismissed"`
	// What was done: `none`, `deleted`, `disabled`, `archived`, `blocked`,
	// `removed_as_spam` or `other`.
	Action string `json:"action" validate:"omitempty,oneof=none deleted disabled archived blocked removed_as_spam other"`
	// What the administrator wants remembered of it.
	Note string `json:"note" validate:"max=2000"`
}
