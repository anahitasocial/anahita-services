package handlers

import (
	"strings"
	"testing"

	"anahita-libs/constants"
	"anahita-libs/data/entities"
)

func TestSetWarning(t *testing.T) {
	cases := []struct {
		name             string
		post             entities.AnahitaNode
		kind, text       string
		valid, allowed   bool
		wantKind, wantTx string
	}{
		{name: "none is none", kind: "", valid: true, allowed: true},
		{name: "a kind", kind: "graphic", text: "ignored", valid: true, allowed: true, wantKind: "graphic"},
		{name: "other with words", kind: "other", text: "  Spoilers:   season 2 ", valid: true, allowed: true, wantKind: "other", wantTx: "Spoilers: season 2"},
		{name: "other without words", kind: "other", text: "  ", valid: false, allowed: true},
		{name: "other too long", kind: "other", text: strings.Repeat("ج", 101), valid: false, allowed: true},
		{name: "other at the limit", kind: "other", text: strings.Repeat("ج", 100), valid: true, allowed: true, wantKind: "other", wantTx: strings.Repeat("ج", 100)},
		{name: "not a kind", kind: "ai", valid: false, allowed: true},
		{name: "taken off", post: entities.AnahitaNode{Warning: "nudity"}, kind: "", valid: true, allowed: true},
		{name: "a moderator's taken off", post: entities.AnahitaNode{Warning: "nudity", WarningByModerator: true}, kind: "", valid: true, allowed: false, wantKind: "nudity"},
		{name: "a moderator's changed", post: entities.AnahitaNode{Warning: "nudity", WarningByModerator: true}, kind: "adult", valid: true, allowed: false, wantKind: "nudity"},
		{name: "a moderator's kept", post: entities.AnahitaNode{Warning: "nudity", WarningByModerator: true}, kind: "nudity", valid: true, allowed: true, wantKind: "nudity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			post := tc.post
			valid, allowed := SetWarning(&post, tc.kind, tc.text)
			if valid != tc.valid || allowed != tc.allowed {
				t.Fatalf("valid, allowed = %v, %v; want %v, %v", valid, allowed, tc.valid, tc.allowed)
			}
			if valid && allowed || !allowed {
				if post.Warning != tc.wantKind || post.WarningText != tc.wantTx {
					t.Fatalf("warning = %q %q; want %q %q", post.Warning, post.WarningText, tc.wantKind, tc.wantTx)
				}
			}
		})
	}
}

// THE RULE THIS GUARDS: a site administrator warns as a moderator what
// they did not write, and what they give is marked theirs; taken off, the
// mark goes with it.
func TestWarningModerator(t *testing.T) {
	post := &entities.AnahitaNode{CreatedBy: 7}
	admin := entities.NewViewer()
	admin.ID = 9
	admin.PersonType = constants.PersonTypeAdmin
	author := entities.NewViewer()
	author.ID = 7
	author.PersonType = constants.PersonTypeAdmin
	member := entities.NewViewer()
	member.ID = 9
	member.PersonType = constants.PersonTypeRegistered

	if !IsWarningModerator(post, admin) {
		t.Error("an administrator is not a moderator of another's post")
	}
	if IsWarningModerator(post, author) {
		t.Error("an administrator is a moderator of their own post")
	}
	if IsWarningModerator(post, member) || IsWarningModerator(post, nil) {
		t.Error("somebody not an administrator is a moderator")
	}

	if !SetWarningAsModerator(post, "graphic", "") || post.Warning != "graphic" || !post.WarningByModerator {
		t.Errorf("given: %+v", post)
	}
	if !SetWarningAsModerator(post, "", "") || post.Warning != "" || post.WarningByModerator {
		t.Errorf("taken off: %+v", post)
	}
	if SetWarningAsModerator(post, "ai", "") {
		t.Error("a kind that is not one was taken")
	}
}
