package event

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
)

// A handler's failure is said, and a panic in it is caught: the listener
// goes on to the next event either way.
func TestHandleSaysWhatWentWrong(t *testing.T) {
	var out bytes.Buffer
	log.SetOutput(&out)
	defer log.SetOutput(os.Stderr)

	payload := &EventPayload{Name: "edge.follow.person.person.added.v1"}

	handle(func(*EventPayload) error { return errors.New("no such profile") }, payload)
	if !strings.Contains(out.String(), "edge.follow.person.person.added.v1: no such profile") {
		t.Fatalf("the error was not logged: %q", out.String())
	}

	out.Reset()
	handle(func(*EventPayload) error { panic("Khan") }, payload)
	if !strings.Contains(out.String(), "the handler panicked: Khan") {
		t.Fatalf("the panic was not caught and logged: %q", out.String())
	}

	out.Reset()
	handle(func(*EventPayload) error { return nil }, payload)
	if out.Len() != 0 {
		t.Fatalf("a handler that succeeded logged %q", out.String())
	}
}
