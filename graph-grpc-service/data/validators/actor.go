package validators

import (
	"anahita-libs/constants"
	"anahita-libs/data/entities"
	"errors"
)

func NewActorValidator() *ActorValidator {
	return &ActorValidator{}
}

type ActorValidator struct{}

func (v *ActorValidator) baseValidation(node *entities.AnahitaNode) error {
	if node.Type == "" {
		return errors.New("type is required")
	}

	if node.Service == "" {
		return errors.New("service is required")
	}

	// NO NAME CHECK. A display name is optional.
	//
	// It was required, which made signup unable to create an account without
	// one — and asking for a display name at the door is exactly what was
	// removed, because neither Mastodon nor Bluesky does it and the profile
	// belongs in onboarding.
	//
	// An empty name is also what both federation protocols expect to handle:
	// an ActivityPub consumer falls back from `name` to `preferredUsername`,
	// ATProto from `displayName` to the handle. The web app does the same, in
	// getActorName.
	//
	// The ALIAS is what makes an actor addressable, and that is still
	// required below — a node with no handle has no URL, no WebFinger answer
	// and nothing to fall back to.
	if node.Alias == "" {
		return errors.New("alias is required")
	}

	// No body check. A bio is optional, and requiring one here rejected every
	// edit to a profile that had none — 717 of 823 people — including the
	// access-only edit, which does not touch the body at all. Changing your
	// privacy setting failed with a 500 unless you had written a bio first.
	//
	// Alias and access below are genuinely required: a node with no handle
	// cannot be addressed at all, and one with no access has no answer to
	// "who may see this". A name is neither — it is a label on something
	// already addressable.

	if node.Name != "" && len(node.Name) > 255 {
		return errors.New("name exceeds maximum length of 255 characters")
	}

	if node.Alias != "" && len(node.Alias) > 255 {
		return errors.New("alias exceeds maximum length of 255 characters")
	}

	if node.Access == "" {
		return errors.New("access is required")
	}

	return nil
}

func (v *ActorValidator) ValidateEdit(node *entities.AnahitaNode) error {
	if node.ID <= 0 {
		return errors.New("invalid node ID for note edit")
	}

	if err := v.baseValidation(node); err != nil {
		return err
	}

	if node.ModifiedBy <= 0 {
		return errors.New("invalid modifier ID for note edit")
	}

	return nil
}

func (v *ActorValidator) ValidateAdd(node *entities.AnahitaNode) error {
	if err := v.baseValidation(node); err != nil {
		return err
	}

	// A person has no creator; every other actor does.
	//
	// A group or an event is made BY somebody, and recording who is worth
	// requiring. A person node is the somebody: at signup there is no other
	// party, and no id to name even in principle, because the id being
	// validated is the one the insert is about to assign. Invite-accept is the
	// same — the invitation came from an administrator, but the account is the
	// invitee's own.
	//
	// This check never ran on that path before. Signup wrote `nodes` with a
	// direct INSERT from auth-service, which reached no validator at all and
	// left created_by NULL. Routing signup through ActorService is what
	// surfaced it, and the honest answer is that the requirement does not
	// apply to a person rather than that signup should invent a creator.
	if node.Type != constants.NodeActorPersonEntityType && node.CreatedBy <= 0 {
		return errors.New("an actor must record who created it")
	}

	return nil
}
