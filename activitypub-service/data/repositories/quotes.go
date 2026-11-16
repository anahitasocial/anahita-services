package repositories

import (
	"database/sql"
	"errors"
	"time"
)

// Quotes between servers (roadmap row 156, FEP-044f): what a quote here of
// a post elsewhere asked of its author, and the stamps given here to posts
// elsewhere that quote one here.

// The states of a quote here waiting on another server's answer.
const (
	QuotePending  = "pending"
	QuoteAccepted = "accepted"
	QuoteRejected = "rejected"
)

// QuoteRequest is a quote here of a post on another server, and what its
// author answered.
type QuoteRequest struct {
	NodeID          int64
	RequestID       string
	QuotedURI       string
	QuotedAuthorURI string
	State           string
	StampURI        string
}

// QuoteStamp is a stamp given here: the person here who wrote `QuotedNodeID`
// let `QuoteURI`, by `QuoteAuthorURI`, quote it.
type QuoteStamp struct {
	UUID           string
	QuotedNodeID   int64
	AuthorID       int64
	QuoteURI       string
	QuoteAuthorURI string
}

type QuotesInterface interface {
	// BeginRequest keeps a request about to be sent. False when one was
	// kept already for this note: it is asked once.
	BeginRequest(request *QuoteRequest) (bool, error)
	// RequestOf is what a note here asked, or nil.
	RequestOf(nodeID int64) (*QuoteRequest, error)
	// RequestByID is the request an answer names, or nil.
	RequestByID(requestID string) (*QuoteRequest, error)
	// Answer keeps an answer: accepted, with the stamp; or rejected.
	Answer(nodeID int64, accepted bool, stampURI string) error
	// AddStamp keeps a stamp, or answers with the one kept already for the
	// same quote of the same post.
	AddStamp(stamp *QuoteStamp) (*QuoteStamp, error)
	// Stamp is a stamp by its UUID, or nil.
	Stamp(uuid string) (*QuoteStamp, error)
	// StampFor is the stamp given to a quote of a post here, or nil.
	StampFor(quotedNodeID int64, quoteURI string) (*QuoteStamp, error)
}

type Quotes struct {
	db *sql.DB
}

func NewQuotes(db *sql.DB) *Quotes {
	return &Quotes{db: db}
}

const beginQuoteRequest = `
INSERT IGNORE INTO activitypub_quote_requests
    (node_id, request_id, quoted_uri, quoted_author_uri, state, stamp_uri, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, '', ?, ?)`

func (repo *Quotes) BeginRequest(request *QuoteRequest) (bool, error) {
	now := time.Now().UTC()
	result, err := repo.db.Exec(beginQuoteRequest,
		request.NodeID, request.RequestID, request.QuotedURI, request.QuotedAuthorURI, QuotePending, now, now)
	if err != nil {
		return false, err
	}
	added, err := result.RowsAffected()
	return added > 0, err
}

const readQuoteRequest = `
SELECT node_id, request_id, quoted_uri, quoted_author_uri, state, stamp_uri
FROM activitypub_quote_requests
WHERE `

func (repo *Quotes) readRequest(where string, arg any) (*QuoteRequest, error) {
	request := &QuoteRequest{}
	err := repo.db.QueryRow(readQuoteRequest+where, arg).Scan(
		&request.NodeID, &request.RequestID, &request.QuotedURI, &request.QuotedAuthorURI,
		&request.State, &request.StampURI)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (repo *Quotes) RequestOf(nodeID int64) (*QuoteRequest, error) {
	return repo.readRequest("node_id = ?", nodeID)
}

func (repo *Quotes) RequestByID(requestID string) (*QuoteRequest, error) {
	return repo.readRequest("request_id = ?", requestID)
}

const answerQuoteRequest = `
UPDATE activitypub_quote_requests
SET state = ?,
    stamp_uri = ?,
    updated_at = ?
WHERE node_id = ?`

func (repo *Quotes) Answer(nodeID int64, accepted bool, stampURI string) error {
	state := QuoteRejected
	if accepted {
		state = QuoteAccepted
	} else {
		stampURI = ""
	}
	_, err := repo.db.Exec(answerQuoteRequest, state, stampURI, time.Now().UTC(), nodeID)
	return err
}

const addQuoteStamp = `
INSERT IGNORE INTO activitypub_quote_stamps
    (uuid, quoted_node_id, author_id, quote_uri, quote_author_uri, created_at)
VALUES (?, ?, ?, ?, ?, ?)`

func (repo *Quotes) AddStamp(stamp *QuoteStamp) (*QuoteStamp, error) {
	if _, err := repo.db.Exec(addQuoteStamp,
		stamp.UUID, stamp.QuotedNodeID, stamp.AuthorID, stamp.QuoteURI, stamp.QuoteAuthorURI, time.Now().UTC()); err != nil {
		return nil, err
	}
	return repo.StampFor(stamp.QuotedNodeID, stamp.QuoteURI)
}

const readQuoteStamp = `
SELECT uuid, quoted_node_id, author_id, quote_uri, quote_author_uri
FROM activitypub_quote_stamps
WHERE `

func (repo *Quotes) readStamp(where string, args ...any) (*QuoteStamp, error) {
	stamp := &QuoteStamp{}
	err := repo.db.QueryRow(readQuoteStamp+where, args...).Scan(
		&stamp.UUID, &stamp.QuotedNodeID, &stamp.AuthorID, &stamp.QuoteURI, &stamp.QuoteAuthorURI)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return stamp, nil
}

func (repo *Quotes) Stamp(uuid string) (*QuoteStamp, error) {
	return repo.readStamp("uuid = ?", uuid)
}

func (repo *Quotes) StampFor(quotedNodeID int64, quoteURI string) (*QuoteStamp, error) {
	return repo.readStamp("quoted_node_id = ? AND quote_uri = ?", quotedNodeID, quoteURI)
}
