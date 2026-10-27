package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"anahita-libs/permissions"
)

func threadFixture(viewerID int64) (*permissions.ReplyAccessContext, *entities.AnahitaNode) {
	owner := entities.NewAnahitaNode()
	owner.ID = 1
	owner.Type = constants.NodeActorPersonEntityType
	owner.Access = "public"
	// As the actor read returns a profile: with who may like and comment
	// on what is posted there, the defaults merged in.
	_ = owner.SetFeatures([]*entities.AnahitaNodeFeature{{
		Service: "text-service",
		Enabled: true,
		AddPermissions: []entities.AnahitaNodeAddPermission{
			{Entity: "like", Access: "registered"},
			{Entity: "comment", Access: "registered"},
		},
	}})

	root := entities.NewAnahitaNode()
	root.ID = 10
	root.Type = "node.medium.text-service.note.v1"
	root.Service = "text-service"
	root.Access = "public"
	root.OwnerID = owner.ID
	root.Owner = owner
	root.CreatedBy = 1
	root.CommentStatus = true

	reply := entities.NewAnahitaNode()
	reply.ID = 20
	reply.Type = root.Type
	reply.Service = root.Service
	reply.Body = "an answer"
	reply.OwnerID = 2
	reply.CreatedBy = 2
	reply.ParentID = root.ID
	reply.RootID = root.ID

	viewer := entities.NewViewer()
	viewer.ID = viewerID
	viewer.Authenticated = viewerID > 0
	viewer.PersonType = constants.PersonTypeRegistered
	viewer.Scopes = []string{constants.OAuthScopeMediaWrite, constants.OAuthScopeLikesWrite}

	return &permissions.ReplyAccessContext{
		Viewer:             viewer,
		Root:               root,
		RootOwnerViewerRel: &entities.ActorRelation{},
	}, reply
}

// What a thread sends for one reply, as the web app receives it.
func sentFor(t *testing.T, thread *permissions.ReplyAccessContext, reply *entities.AnahitaNode) map[string]any {
	t.Helper()

	setReplyFlags(thread, reply)
	blankRemoved(reply)

	encoded, err := json.Marshal(threadResponse([]*entities.AnahitaNode{reply}))
	if err != nil {
		t.Fatal(err)
	}

	var sent []map[string]any
	if err := json.Unmarshal(encoded, &sent); err != nil {
		t.Fatal(err)
	}
	return sent[0]
}

// The web app draws a control on a reply only when the reply says the viewer
// may use it. The answers have to be in what is sent: left out, nobody is
// offered edit or delete on their own reply, which is what happened when a
// thread was sent as a list of posts.
func TestAThreadSendsWhatTheViewerMayDoWithEachReply(t *testing.T) {
	cases := []struct {
		who    string
		viewer int64
		want   map[string]bool
	}{
		{"the author of the reply", 2,
			map[string]bool{"edit": true, "delete": true, "hide": false, "comment": true, "like": true}},
		{"the author of the post", 1,
			map[string]bool{"edit": false, "delete": true, "hide": true, "comment": true, "like": true}},
		{"somebody else", 3,
			map[string]bool{"edit": false, "delete": false, "hide": false, "comment": true, "like": true}},
		{"a visitor", 0,
			map[string]bool{"edit": false, "delete": false, "hide": false, "comment": false, "like": false}},
	}

	for _, c := range cases {
		t.Run(c.who, func(t *testing.T) {
			thread, reply := threadFixture(c.viewer)
			sent := sentFor(t, thread, reply)

			authorized, ok := sent["authorized"].(map[string]any)
			if !ok {
				t.Fatalf("no authorized in what was sent: %v", sent)
			}
			for action, want := range c.want {
				got, present := authorized[action]
				if !present {
					t.Errorf("%s is not answered at all", action)
					continue
				}
				if got != want {
					t.Errorf("%s = %v, want %v", action, got, want)
				}
			}

			// And where it sits in the thread, which is how it is drawn
			// under what it answers.
			if sent["parent_id"] != float64(10) || sent["root_id"] != float64(10) {
				t.Errorf("parent %v, root %v", sent["parent_id"], sent["root_id"])
			}
		})
	}
}

// A removed reply is sent as a placeholder: no text, no name, and nothing
// that can be done to it, whoever is asking.
func TestARemovedReplyIsSentEmptyAndOfferingNothing(t *testing.T) {
	thread, reply := threadFixture(2)
	when := time.Now()
	reply.DeletedAt = &when
	reply.Author.ID = 2
	reply.Author.Name = "Bob"

	sent := sentFor(t, thread, reply)

	if sent["deleted"] != true {
		t.Errorf("not marked as removed: %v", sent)
	}
	if _, has := sent["body"]; has {
		t.Errorf("a removed reply was sent with its text: %v", sent["body"])
	}
	if _, has := sent["author"]; has {
		t.Errorf("a removed reply was sent with its author: %v", sent["author"])
	}

	authorized, _ := sent["authorized"].(map[string]any)
	for _, action := range []string{"edit", "delete", "hide", "comment", "like"} {
		if authorized[action] == true {
			t.Errorf("a removed reply offers %s", action)
		}
	}
}
