package constants

const (
	ServiceName = "abuse-report-service"

	// A case is open until an administrator decides it. "Actioned" means
	// something was done about what was reported; "dismissed" means the
	// report was looked at and nothing needed doing.
	CaseStatusOpen      = "open"
	CaseStatusActioned  = "actioned"
	CaseStatusDismissed = "dismissed"

	// What was done, recorded with the decision. The service does none of
	// these itself: the administrator's client calls the service that owns
	// the thing, then records here what it did.
	ActionNone     = "none"
	ActionDeleted  = "deleted"
	ActionDisabled = "disabled"
	ActionArchived = "archived"
	ActionBlocked  = "blocked"
	ActionOther    = "other"

	// DetailsMaxLength is how much a reporter may write. Enough to explain,
	// short enough that a queue of reports can be read.
	DetailsMaxLength = 1000

	NoteMaxLength = 2000

	// ReportsPerHour is how many reports one person may file in an hour.
	// Far more than anybody reporting in good faith needs, and few enough
	// that the queue cannot be flooded from one account.
	ReportsPerHour = 20

	RateLimitScopeReport = "abuse_report_reporter"

	// AddressRetentionDays is how long a report keeps the network address it
	// came from. Long enough to look into a burst of false reports, and no
	// longer: an address is personal data with no use after that.
	AddressRetentionDays = 90

	DefaultLanguage = "en-GB"

	DefaultBrowseLimit = 20
	MaxBrowseLimit     = 100
)

// CaseStatuses are the values a case's status may hold.
var CaseStatuses = []string{CaseStatusOpen, CaseStatusActioned, CaseStatusDismissed}

// Actions are the values a decision's action may hold.
var Actions = []string{
	ActionNone, ActionDeleted, ActionDisabled, ActionArchived, ActionBlocked, ActionOther,
}
