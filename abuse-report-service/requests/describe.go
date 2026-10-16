package requests

// What the routes that read their values one by one take, for the API
// reference (cmd/api/routes.go). The handlers read these with c.QueryParam
// and c.Param by the same names; nothing binds into these structs.

// ReasonsBrowse asks for the reasons a thing may be reported for.
type ReasonsBrowse struct {
	// Only the reasons for this kind of node: `node.medium.text-service.note.v1`
	// and so on. All of them when left out.
	Type string `query:"type"`
	// The language to word them in. Accept-Language when left out.
	Lang string `query:"lang"`
}

// CasesBrowse asks for a page of cases.
type CasesBrowse struct {
	// Which cases: `open` (the default), `actioned` or `dismissed`.
	Status string `query:"status" validate:"omitempty,oneof=open actioned dismissed"`
	// Only cases with a report that gave this reason, by its key.
	Reason string `query:"reason" validate:"max=64"`
	// How many to pass over. Named `offset` here, where other lists say
	// `start`.
	Offset uint64 `query:"offset"`
	// The most to send: 20 unless asked, 100 at most.
	Limit uint64 `query:"limit"`
	// The language to word the reasons in. Accept-Language when left out.
	Lang string `query:"lang"`
}

// CaseRead names a case.
type CaseRead struct {
	// The case's id.
	ID int64 `param:"id"`
	// The language to word the reasons in. Accept-Language when left out.
	Lang string `query:"lang"`
}

// CaseReopen names a case to reopen.
type CaseReopen struct {
	// The case's id.
	ID int64 `param:"id"`
}
