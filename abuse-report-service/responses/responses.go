package responses

import (
	an_handlers "anahita-libs/handlers"
	"time"

	"abusereport/data/entities"
	"abusereport/data/reasons"

	libs_entities "anahita-libs/data/entities"
	libs_responses "anahita-libs/responses"
)

// Reason is one entry of the list a reporter chooses from, in their language.
type Reason struct {
	// What it is called in a report.
	Key string `json:"key"`
	// What it is called, in the language asked for.
	Label string `json:"label"`
	// What it covers.
	Description string `json:"description,omitempty"`
	// Whether the reporter must say more in their own words.
	RequiresDetails bool `json:"requires_details"`
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

// CaseSummary is how many cases wait to be decided.
type CaseSummary struct {
	// How many are open.
	Open uint64 `json:"open"`
}

// ReasonCount is how many reports in a case gave one reason.
type ReasonCount struct {
	// The reason.
	Key string `json:"key"`
	// What it is called.
	Label string `json:"label"`
	// How many reports gave it.
	Count int `json:"count"`
}

// Case is what an administrator sees of a case in a list.
type Case struct {
	// Its id.
	ID int64 `json:"id"`
	// `open`, `actioned` or `dismissed`.
	Status string `json:"status"`
	// How many people reported it.
	ReportCount int `json:"report_count"`

	// The id of what was reported.
	TargetID int64 `json:"target_id"`
	// What kind of node it is.
	TargetType string `json:"target_type"`
	// Target is the reported node as it is now, or absent when it is gone.
	Target *libs_responses.NodeItem `json:"target,omitempty"`
	// Whether what was reported is gone.
	TargetGone bool `json:"target_gone"`
	// The profile it is on.
	TargetOwner *libs_responses.NodeItem `json:"target_owner,omitempty"`
	// Who wrote it, when it is a reply one of a website's administrators made
	// as the website. Sent only here, to those who handle reports.
	WrittenBy *libs_responses.NodeItem `json:"written_by,omitempty"`

	// The reasons given, with how many gave each.
	Reasons []ReasonCount `json:"reasons"`

	// When it was first reported.
	FirstReportedAt time.Time `json:"first_reported_at"`
	// When it was last reported.
	LastReportedAt time.Time `json:"last_reported_at"`

	// Who decided it.
	DecidedBy *libs_responses.NodeItem `json:"decided_by,omitempty"`
	// When it was decided.
	DecidedAt *time.Time `json:"decided_at,omitempty"`
	// What was done.
	DecisionAction string `json:"decision_action,omitempty"`
	// What the administrator noted.
	DecisionNote string `json:"decision_note,omitempty"`
}

// Report is one person's report, as an administrator sees it.
type Report struct {
	// Its id.
	ID int64 `json:"id"`
	// Who reported it.
	Reporter *libs_responses.NodeItem `json:"reporter,omitempty"`
	// Their id.
	ReporterID int64 `json:"reporter_id"`
	// The reason given.
	ReasonKey string `json:"reason_key"`
	// What it is called.
	ReasonLabel string `json:"reason_label"`
	// What the reporter added.
	Details string `json:"details,omitempty"`
	// With reason_key "rule": the rule broken, by its title, and whose rule
	// it is, 0 for the site's.
	Rule          string `json:"rule,omitempty"`
	RuleProfileID int64  `json:"rule_profile_id,omitempty"`
	// The group or event whose rule it is.
	RuleProfile *libs_responses.NodeItem `json:"rule_profile,omitempty"`
	// When it was made.
	CreatedAt time.Time `json:"created_at"`
	// When it was last changed: a second report by the same person updates it.
	UpdatedAt time.Time `json:"updated_at"`
}

// ProfileCase is a case as a group's or an event's administrators see it
// (roadmap row 102, part 6b): what was reported and the reports against
// its rules, never who made them, nor the site's decision notes.
type ProfileCase struct {
	// Its id.
	ID int64 `json:"id"`
	// `open`, `actioned` or `dismissed`.
	Status string `json:"status"`
	// The id of what was reported.
	TargetID int64 `json:"target_id"`
	// What kind of node it is.
	TargetType string `json:"target_type"`
	// Target is the reported node as it is now, or absent when it is gone.
	Target *libs_responses.NodeItem `json:"target,omitempty"`
	// Whether what was reported is gone.
	TargetGone bool `json:"target_gone"`
	// When it was last reported.
	LastReportedAt time.Time `json:"last_reported_at"`
	// The reports against the profile's rules, oldest first.
	Reports []ProfileReport `json:"reports"`
}

// ProfileReport is one report against a profile's rules, with nobody's name.
type ProfileReport struct {
	// The rule broken, by its title.
	Rule string `json:"rule"`
	// What the reporter added.
	Details string `json:"details,omitempty"`
	// When it was made.
	CreatedAt time.Time `json:"created_at"`
}

// ProfileCase maps a case and its reports against the profile's rules.
func (m *Mapper) ProfileCase(c *entities.Case, reports []*entities.Report) ProfileCase {
	mapped := ProfileCase{
		ID:             c.ID,
		Status:         c.Status,
		TargetID:       c.TargetID,
		TargetType:     c.TargetType,
		Target:         m.node(c.TargetID),
		TargetGone:     m.node(c.TargetID) == nil,
		LastReportedAt: c.LastReportedAt,
		Reports:        []ProfileReport{},
	}
	for _, report := range reports {
		if report.CaseID == c.ID {
			mapped.Reports = append(mapped.Reports, ProfileReport{
				Rule:      report.RuleTitle,
				Details:   report.Details,
				CreatedAt: report.CreatedAt,
			})
		}
	}
	return mapped
}

// CaseDetail is a case with its reports and the target's earlier cases.
type CaseDetail struct {
	Case
	// Each person's report.
	Reports []Report `json:"reports"`
	// The earlier cases about the same thing.
	Previous []Case `json:"previous"`
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

// writer is who wrote a reply made as a website, or nil.
func (m *Mapper) writer(targetID int64) *libs_responses.NodeItem {
	target, ok := m.Nodes[targetID]
	if !ok || target == nil {
		return nil
	}
	return m.node(an_handlers.WrittenBy(target.Meta))
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
		WrittenBy:       m.writer(c.TargetID),
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
		ID:            report.ID,
		Reporter:      m.node(report.ReporterID),
		ReporterID:    report.ReporterID,
		ReasonKey:     report.ReasonKey,
		ReasonLabel:   m.label(report.ReasonKey),
		Details:       report.Details,
		Rule:          report.RuleTitle,
		RuleProfileID: report.RuleProfileID,
		RuleProfile:   m.node(report.RuleProfileID),
		CreatedAt:     report.CreatedAt,
		UpdatedAt:     report.UpdatedAt,
	}
}
