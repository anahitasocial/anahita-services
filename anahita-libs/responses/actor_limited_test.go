package responses

import (
	"encoding/json"
	"strings"
	"testing"

	"anahita-libs/data/entities"
)

// What a stranger is sent of a profile is its name and picture. Whatever
// else the node carries stays behind.
func TestLimitedActorLeavesTheProfileOut(t *testing.T) {
	node := entities.NewAnahitaNode()
	node.ID = 7
	node.Type = "node.actor.person-service.person.v1"
	node.Name = "Ana"
	node.Alias = "ana"
	node.Body = "SECRET-BODY"
	node.WebsiteURL = "https://secret.example"
	node.Access = "followers"
	node.FollowerCount = 4242
	node.LeaderCount = 4343
	node.AllowFollowRequest = true

	raw, err := json.Marshal(NewLimitedActor(node, true))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)

	for _, leak := range []string{"SECRET-BODY", "secret.example", "4242", "4343", "followers", "body", "access\""} {
		if strings.Contains(got, leak) {
			t.Errorf("%q is in what a stranger is sent: %s", leak, got)
		}
	}
	for _, want := range []string{`"name":"Ana"`, `"alias":"ana"`, `"restricted":true`, `"allow_follow_request":true`, `"is_follow_requested_by_viewer":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s is missing from %s", want, got)
		}
	}

	if NewLimitedActor(nil, false) != nil {
		t.Error("nothing should map to nothing")
	}
}
