# Following from another server (roadmap row 134)

Asked for by the user on 2026-11-07, and confirmed on 2026-11-22: "I should
be able to follow someone on any other ActivityPub instance and they should
be able to follow anyone on an Anahita instance without having to copy
their handle, put it in the search box and then follow their locally
cached profile."

## Status

| # | What | Status |
| --- | --- | --- |
| 1 | WebFinger lists where people here act on something from elsewhere: OStatus `subscribe`, FEP-3b86 `Object` and `Follow` | ⬜ |
| 2 | `/authorize_interaction?uri=` in the web app: whoever or whatever is named, looked up and opened, after signing in if need be | ⬜ |
| 3 | The lookup takes a post's address too, keeping a public post from elsewhere as a repost's is kept | ⬜ |
| 4 | Somebody from elsewhere on a page here: their handle, their server's own page to follow, like, reply or repost from | ⬜ |
| 5 | The docs; tried with Mastodon's and Pixelfed's own pages where possible; changelogs | ⬜ |

## Contents

1. [What other servers publish](#what-other-servers-publish)
2. [Somebody here, on a page elsewhere](#somebody-here-on-a-page-elsewhere)
3. [Somebody from elsewhere, on a page here](#somebody-from-elsewhere-on-a-page-here)
4. [What stays as it is](#what-stays-as-it-is)
5. [Out of scope](#out-of-scope)
6. [Documentation](#documentation)

## What other servers publish

Checked on 2026-11-22 in their WebFinger answers:

| | OStatus `subscribe` | FEP-3b86 intents |
| --- | --- | --- |
| mastodon.social | `/authorize_interaction?uri={uri}` | `Object` (the same page, `{object}`), `Create` (`/share?text={content}`) |
| pixelfed.social | `/authorize_interaction?uri={uri}` | none |
| Anahita | none | none |

FEP-3b86 is a draft, and it advises: its intents first, then the
`subscribe` template, then nothing.

## Somebody here, on a page elsewhere

They press Follow on a Mastodon or Pixelfed page while signed out there,
give `banu.io` or their whole handle, and their server sends them home.

1. **WebFinger** (activitypub-service, `discovery.go`) adds, for every
   person who federates, three links with one template,
   `{web}/authorize_interaction?uri={uri}`:
   1. `http://ostatus.org/schema/1.0/subscribe`, with `{uri}`, as Mastodon
      and Pixelfed have it;
   2. `https://w3id.org/fep/3b86/Object`, with `{object}`;
   3. `https://w3id.org/fep/3b86/Follow`, with `{object}`.
2. **`/authorize_interaction?uri=`** is a page of the web app, the path
   Mastodon and Pixelfed use (the user, 2026-11-22). `uri` is an address or
   a handle, `acct:` or not.
   1. Signed in, it asks the lookup (`api.federation.lookup`), then opens
      what is found: a profile, with + Follow at hand, or a post, with Like,
      Reply and Repost at hand. Something that cannot be found says so
      in a line, with the address, and a way to search.
   2. Signed out, it keeps where it was going, has them sign in, and goes
      on there. The sign-in now returns to a page kept this way
      (`OAuthCallback.jsx`); today it always goes home.
   3. An address of this site is opened as it is.
3. **The lookup** (`GET /activitypub/lookup?q=`) takes a post's address too:
   asked first as a profile, then as a post through `fetchNote`, the same
   rule a reposted note is kept by (public, not a reply, written by
   somebody on the same server). Answered with what it found and its kind.

## Somebody from elsewhere, on a page here

They read a profile or a post here, not signed in, and have an account on
Mastodon, Pixelfed or another server.

1. **Where it is offered** (settled by the user on 2026-11-23):
   1. On a profile that federates, the **+ Follow** a visitor sees opens a
      small dialog: their handle, for their own server, or Sign in, for an
      account here.
   2. On a post that went out, under it, one quiet line: "Reply, like or
      repost from your own server", opening the same dialog.
2. **The dialog** asks for a handle, `@name@server`, and remembers it in the
   browser for next time, nowhere else. Its main action is "Continue".
3. **Where it sends them**, worked out by activitypub-service:
   `GET /activitypub/intent?handle=…&node_id=…`. It reads their server's
   WebFinger, as the lookup reads anybody's, and answers with an address:
   1. their `Follow` intent for a profile, their `Object` intent for a post;
   2. else their `subscribe` template, with the profile's or the post's
      address;
   3. else nothing, said plainly: "Your server does not take you there;
      search for … on it instead", with the address to copy.
   A server this site refuses is answered the same as one that does not
   answer. Rate-limited, as the lookup is.
4. **What may be offered:** a person, group or channel only once it
   federates (it has an address on the fediverse); a post only once it
   went out. Anything else shows no dialog.

## What stays as it is

1. Follows, likes, replies and reposts themselves: they are ActivityPub's,
   built already. This only takes people to the button.
2. A profile's or a post's access: nothing here is shown to anybody who
   may not see it already.
3. Bluesky: it has nothing of the kind.

## Out of scope

1. FEP-3b86 `Create`, sharing a page here into a server's composer: the
   share dialog keeps its own way (`/share` guessed for Mastodon).
2. Groups followed from elsewhere: roadmap row 73.

## Documentation

1. anahita-services: `docs/federation.md` (WebFinger, the lookup, the
   intent), `docs/features.md` ("Interact from another server": yes), the
   API reference, `CHANGELOG.md`.
2. anahita-web: `docs/architecture.md` (routes, the dialog), `CHANGELOG.md`.
