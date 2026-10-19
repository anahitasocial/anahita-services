package services

import "testing"

// One connection for each address, however often it is asked for: a new
// one for every call was left open each time.
func TestOneConnectionForEachAddress(t *testing.T) {
	first, err := sharedConnection("mail-grpc-service:50001")
	if err != nil {
		t.Fatal(err)
	}
	again, err := sharedConnection("mail-grpc-service:50001")
	if err != nil {
		t.Fatal(err)
	}
	other, err := sharedConnection("graph-grpc-service:50001")
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Error("asked twice, made two connections")
	}
	if first == other {
		t.Error("two addresses share a connection")
	}
}
