# Asking to follow on the fediverse (roadmap row 130)

Asked for by the user on 2026-11-07: "some profiles on Mastodon require
request to follow, but on Anahita, their profile shows only Follow. Is there
a way to detect a Request to follow on Mastodon and display it correctly on
Anahita?" Planned the same day. **Done the same day:** built as below, with
one change to the plan: the column the rules already read,
`allow_follow_request`, carries it, not a new one; tried against a real
Mastodon account on mastodon.world from the development stack, asked,
waiting, and accepted.

## Contents

1. [Settled](#settled)
2. [What there is now](#what-there-is-now)
3. [The change](#the-change)
4. [Bluesky](#bluesky)
5. [Out of scope](#out-of-scope)
6. [Phases](#phases)
7. [Documentation](#documentation)

## Settled

1. **A profile elsewhere that approves its followers is asked, not
   followed.** Following it makes a request to follow here, the edge local
   profiles already use (`edge.actor.follow-request.v1`), not a follow.
   It becomes a follow when their server accepts it.
2. **The button says "Request to follow",** as it does for a profile here
   that is seen only by its followers, and as Mastodon says it. Once asked,
   it reads "Requested", and pressing it again takes the request back. The
   same labels, already in every language, for both (the user, 2026-11-07).
3. **Only the fediverse.** Bluesky has no requests to follow.

## What there is now

1. **Their setting is read and kept.** A profile's
   `manuallyApprovesFollowers` is kept as `locked` in the activitypub
   service's own record of them (`remote/actor.go`,
   `data/repositories/actors.go`), and returned by the lookup. The graph does
   not know it, and a profile's page does not show it.
2. **The follow is made at once, and settled later**
   ([federation.md](../federation.md), "Following from here"). Pressing Follow
   makes the follow edge; `events.go` hears `edge.follow.*.added` and sends
   `Follow`, recorded as `pending`; `Accept` marks it `accepted`, and
   `Reject` takes the follow away (`inbox.go`, `answered`).
3. **So, until they answer, it is wrong in three ways:** the button says
   Following, they are counted among the profile's followers, and nothing
   says that nothing will come until they accept. Mastodon often never
   answers a request it ignores, so it can stay that way for good.
4. **Requests to follow already work between local profiles**
   (`socialgraph-service` `follow_request.go`; the graph's
   `socialgraph_follow.go`): a request edge, not counted among followers;
   accepting it removes it and makes the follow; events for each;
   `is_follow_requested_by_viewer` on the profile; "Requested" in the web
   app (`ControlFollowRequest`).

## The change

1. **The graph knows a profile is locked.** Kept on the remote profile's
   node when the activitypub service saves them (`keep`), and refreshed with
   them, in `allow_follow_request`: built so, where a column of its own
   (`approves_followers`) was planned. For a profile here it says that it
   may be asked; for one from elsewhere, which is public, that it must be;
   either way asking is how it is followed, and the rules that read it
   needed no change but one: following such a profile elsewhere outright is
   refused (`permissions.CanFollow`).
2. **Following a locked profile makes a request.** socialgraph-service's
   follow, for a remote target that approves its followers, adds the request
   edge instead of the follow; `is_follow_requested_by_viewer` is then true.
3. **The activitypub service follows requests too.** It hears
   `edge.actor.follow-request.*.added` for a remote target and sends
   `Follow`; `.deleted`, not followed by an accept, sends `Undo`.
4. **Their answer settles it.** `Accept` turns the request into the follow,
   through the graph's accept (`AcceptFollowRequest`), so counts and events
   are as for a local one, and nobody here is notified of a follow they
   made. `Reject` removes the request.
5. **Follows already pending** on a site, when this is deployed, are turned
   into requests by a migration (`20261122000012`): an outgoing follow still
   `pending`, to a profile that approves its followers, becomes the request
   edge, and the counts are made again. Rehearsed in a transaction on the
   development database, rolled back, then run.
6. **The sweep** (`missed.go`) asks again about a request whose news the bus
   lost, and does not take a request still waiting for a follow that is
   gone.
7. **A profile that changes its mind** is read again as it is today: one
   that stops approving its followers is followed outright next time; a
   request waiting for one that has stopped is accepted by their server in
   any case.
8. **The web app:** "Request to follow", then "Requested", in the Follow
   control itself (`controls/Follow.jsx`), so that it keeps its sizes and its
   place in menus, with the words local requests use (`actor:limited.request`
   and `actor:limited.requested`), for a profile elsewhere that approves its
   followers; nothing new to translate. Profiles and lists say
   `is_follow_requested_by_viewer`.

## Bluesky

Following on Bluesky is a record anybody may write; there is nothing to
approve. Nothing changes there.

## Out of scope

1. **Approving followers from elsewhere** for a profile here. A public
   profile is followed by anybody, here or elsewhere
   ([federation.md](../federation.md), "Nobody on the fediverse approves their
   followers").
2. **Requests that wait for ever.** A request is kept until it is answered
   or taken back, as Mastodon keeps it.

## Phases

1. **Services:** the column, the follow to a locked profile as a request,
   the activitypub service sending for requests and settling them, the
   migration; tested, and tried in the lab against GoToSocial with a profile
   that approves its followers: one session.
2. **Web:** "Request to follow" and "Requested" for those profiles; tried at
   both sizes against the lab: half a session.
3. **banu.io:** the migration rehearsed on a copy, then deployed; a pending
   follow there checked before and after.

## Documentation

1. anahita-services: [federation.md](../federation.md) "Following from here",
   [`docs/permissions.md`](../../permissions.md#following), the API reference
   for the profile's new field, `CHANGELOG.md`.
2. anahita-web: `docs/` where a profile's follow button is described,
   `CHANGELOG.md`.
