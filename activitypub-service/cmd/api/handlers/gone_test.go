package handlers

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A note deleted with its thread is not there at all, and graph-grpc-service
// says so only in the database's words. It is taken back, as one kept as
// deleted is; any other failure is tried again.
func TestGoneIsANodeThatIsNotThere(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{status.Error(codes.Unknown, "sql: no rows in result set"), true},
		{status.Error(codes.Unavailable, "connection refused"), false},
		{errors.New("timeout"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := gone(c.err); got != c.want {
			t.Errorf("gone(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
