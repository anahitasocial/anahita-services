package entities

import "time"

// Case is one reported node and what was decided about it. Everybody who
// reports the same node while its case is open joins that case.
type Case struct {
	ID            int64
	TargetID      int64
	TargetType    string
	TargetOwnerID int64 // zero when the target has no owner: a hashtag, a place
	Status        string
	ReportCount   int

	FirstReportedAt time.Time
	LastReportedAt  time.Time

	DecidedBy      int64
	DecidedAt      *time.Time
	DecisionAction string
	DecisionNote   string

	TargetGoneAt *time.Time
}

// Report is one person's complaint inside a case.
type Report struct {
	ID         int64
	CaseID     int64
	ReporterID int64
	ReasonKey  string
	Details    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	CreatedIP  string
}

// ReasonCount is how many reports in a case gave one reason.
type ReasonCount struct {
	CaseID    int64
	ReasonKey string
	Count     int
}
