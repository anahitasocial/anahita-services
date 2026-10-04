package responses

import (
	"time"

	"abusereport/data/entities"
	"abusereport/data/reasons"

	libs_entities "anahita-libs/data/entities"
	libs_responses "anahita-libs/responses"
)

// Reason is one entry of the list a reporter chooses from, in their language.
type Reason struct {
	Key             string `json:"key"`
	Label           string `json:"label"`
	Description     string `json:"description,omitempty"`
	RequiresDetails bool   `json:"requires_details"`
}

func MapReasons(list []reasons.Reason, language string) []Reason {
	mapped := make([]Reason, 0, len(list))
	for _, reason := range list {
		mapped = append(mapped, Reason{
			Key:             reason.Key,
			Label:           reason.LabelIn(language),
			Description:     reason.DescriptionIn(language),
			RequiresDetails: reason.RequiresDetails,
		})
	}
	return mapped
}

// ReasonCount is how many reports in a case gave one reason.
type ReasonCount struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// Case is what an administrator sees of a case in a list.
type Case struct {
	ID          int64  `json:"id"`
	Status      string `json:"status"`
	ReportCount int    `json:"report_count"`

	TargetID   int64  `json:"target_id"`
	TargetType string `json:"target_type"`
	// Target is the reported node as it is now, or absent when it is gone.
	Target      *libs_responses.NodeItem `json:"target,omitempty"`
	TargetGone  bool                     `json:"target_gone"`
	TargetOwner *libs_responses.NodeItem `json:"target_owner,omitempty"`

	Reasons []ReasonCount `json:"reasons"`

	FirstReportedAt time.Time `json:"first_reported_at"`
	LastReportedAt  time.Time `json:"last_reported_at"`

	DecidedBy      *libs_responses.NodeItem `json:"decided_by,omitempty"`
	DecidedAt      *time.Time               `json:"decided_at,omitempty"`
	DecisionAction string                   `json:"decision_action,omitempty"`
	DecisionNote   string                   `json:"decision_note,omitempty"`
}

// Report is one person's report, as an administrator sees it.
type Report struct {
	ID          int64                    `json:"id"`
	Reporter    *libs_responses.NodeItem `json:"reporter,omitempty"`
	ReporterID  int64                    `json:"reporter_id"`
	ReasonKey   string                   `json:"reason_key"`
	ReasonLabel string                   `json:"reason_label"`
	Details     string                   `json:"details,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

// CaseDetail is a case with its reports and the target's earlier cases.
type CaseDetail struct {
	Case
	Reports  []Report `json:"reports"`
	Previous []Case   `json:"previous"`
}

// Mapper turns stored cases into responses, given the nodes they refer to
// and the reasons list for labels.
type Mapper struct {
	Nodes     map[int64]*libs_entities.AnahitaNode
	Catalogue *reasons.Catalogue
	Language  string
}

func (m *Mapper) node(id int64) *libs_responses.NodeItem {
	if id == 0 {
		return nil
	}
	node, ok := m.Nodes[id]
	if !ok || node == nil {
		return nil
	}
	return (&libs_responses.NodeResponse{}).MapNodeItem(node)
}

// label is the reason's label, or its key when the installation has since
// removed the reason from its list.
func (m *Mapper) label(key string) string {
	if reason, ok := m.Catalogue.Find(key); ok {
		return reason.LabelIn(m.Language)
	}
	return key
}

func (m *Mapper) Case(c *entities.Case, counts []entities.ReasonCount) Case {
	target := m.node(c.TargetID)

	mapped := Case{
		ID:              c.ID,
		Status:          c.Status,
		ReportCount:     c.ReportCount,
		TargetID:        c.TargetID,
		TargetType:      c.TargetType,
		Target:          target,
		TargetGone:      target == nil,
		TargetOwner:     m.node(c.TargetOwnerID),
		Reasons:         []ReasonCount{},
		FirstReportedAt: c.FirstReportedAt,
		LastReportedAt:  c.LastReportedAt,
		DecidedBy:       m.node(c.DecidedBy),
		DecidedAt:       c.DecidedAt,
		DecisionAction:  c.DecisionAction,
		DecisionNote:    c.DecisionNote,
	}

	for _, count := range counts {
		if count.CaseID == c.ID {
			mapped.Reasons = append(mapped.Reasons, ReasonCount{
				Key:   count.ReasonKey,
				Label: m.label(count.ReasonKey),
				Count: count.Count,
			})
		}
	}

	return mapped
}

func (m *Mapper) Report(report *entities.Report) Report {
	return Report{
		ID:          report.ID,
		Reporter:    m.node(report.ReporterID),
		ReporterID:  report.ReporterID,
		ReasonKey:   report.ReasonKey,
		ReasonLabel: m.label(report.ReasonKey),
		Details:     report.Details,
		CreatedAt:   report.CreatedAt,
		UpdatedAt:   report.UpdatedAt,
	}
}
