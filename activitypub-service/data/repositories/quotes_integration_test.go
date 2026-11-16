package repositories

import "testing"

// Quotes between servers against a real MySQL: see missedDB.
func TestQuotesKeepRequestsAndStamps(t *testing.T) {
	db := missedDB(t)
	for _, stmt := range []string{`DELETE FROM activitypub_quote_requests`, `DELETE FROM activitypub_quote_stamps`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	quotes := NewQuotes(db)

	request := &QuoteRequest{NodeID: 7, RequestID: "https://h.example/ap/notes/u/quote", QuotedURI: "https://m.example/s/1", QuotedAuthorURI: "https://m.example/u/a"}
	if begun, err := quotes.BeginRequest(request); err != nil || !begun {
		t.Fatalf("begun %v: %v", begun, err)
	}
	if again, err := quotes.BeginRequest(request); err != nil || again {
		t.Errorf("asked twice: %v %v", again, err)
	}
	if read, err := quotes.RequestByID(request.RequestID); err != nil || read == nil || read.State != QuotePending || read.NodeID != 7 {
		t.Fatalf("read %+v: %v", read, err)
	}
	if err := quotes.Answer(7, true, "https://m.example/stamps/1"); err != nil {
		t.Fatal(err)
	}
	if read, _ := quotes.RequestOf(7); read == nil || read.State != QuoteAccepted || read.StampURI != "https://m.example/stamps/1" {
		t.Errorf("after the Accept: %+v", read)
	}

	stamp := &QuoteStamp{UUID: "11111111-2222-3333-4444-555555555555", QuotedNodeID: 9, AuthorID: 3, QuoteURI: "https://m.example/s/2", QuoteAuthorURI: "https://m.example/u/a"}
	given, err := quotes.AddStamp(stamp)
	if err != nil || given == nil || given.UUID != stamp.UUID {
		t.Fatalf("given %+v: %v", given, err)
	}
	// The same quote of the same post: the stamp it was given.
	again := *stamp
	again.UUID = "66666666-7777-8888-9999-000000000000"
	if same, err := quotes.AddStamp(&again); err != nil || same == nil || same.UUID != stamp.UUID {
		t.Errorf("given twice: %+v %v", same, err)
	}
	if read, _ := quotes.Stamp(stamp.UUID); read == nil || read.QuoteURI != stamp.QuoteURI {
		t.Errorf("read %+v", read)
	}
}
