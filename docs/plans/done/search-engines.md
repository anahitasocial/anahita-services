# Search engine indexing (roadmap row 174, done 2026-11-22)

Asked for by the user on 2026-11-22, from FediDB's list ("Hide from search
engines"), consent first: search engines are asked not to index a person's
profile and posts unless they say otherwise. Nothing is hidden: who may see
them is the profile's privacy, unchanged (the user, 2026-11-22). The
default is for new people only (the user, 2026-11-22): a profile made
before this keeps what it has today, and may be indexed.

## Status

| # | What | Status |
| --- | --- | --- |
| 1 | `nodes.search_engines` stored, new people made with it off, edited like `graph_access` | ✅ 2026-11-22: migration `20261122000026`, `PATCH /…/:id/search-engines` for every kind |
| 2 | web-shell: `noindex` on the pages of a profile that said no, and on its posts | ✅ 2026-11-22: a `robots` tag and `X-Robots-Tag`; copies from other servers always |
| 3 | Settings › Privacy, and the settings of groups, events and channels, in every language base | ✅ 2026-11-22: a Search engines card under the followers card |
| 4 | The docs; tried at both sizes; changelogs | ✅ 2026-11-22 |

### Notes

1. Tried on the dev stack as Worf at both sizes: the switch saves
   (`search_engines` 1 to 0 and back in MySQL), and switching Reposts on
   and off between leaves it as it was. web-shell is not run on the dev
   stack: its tags and header are tried by its tests
   (`TestSearchEnginesAreAskedNotToIndexWhenAProfileSaysNo`,
   `TestCopiesFromOtherServersAreNotIndexed`).
2. A post's author and owner come with only their names, so web-shell
   reads both profiles for their choice; what it reads is kept a minute
   with the page.
3. An edit that carries no value writes 0: a slip asks search engines not
   to index, never the other way round.

## Contents

1. [The setting](#the-setting)
2. [What it changes](#what-it-changes)
3. [What exists](#what-exists)
4. [Out of scope](#out-of-scope)
5. [Documentation](#documentation)

## The setting

1. **One switch, under Settings › Privacy:** "Search engines", labelled
   "Let search engines index this profile", with the line under it:
   "Search engines such as Google may list this profile and its posts.
   Off, they are asked not to. Who can see them stays as set above." Off for somebody who signs up from now on; on for everybody
   already here, as today.
2. **Groups, events and channels** have the same switch in their own
   settings, for their admins. It is on for a new one: they are made to be
   found (row 121, events anybody can find). Only a person is asked first
   (the user, 2026-11-22).
3. **Stored on the profile,** a column on `nodes`, `search_engines`
   `tinyint(1) NOT NULL DEFAULT 1`, so that every profile there is keeps
   being found, and a person's sign-up writes 0. Read and edited as
   `graph_access` is (row 151): `requests/actor.go`, `responses/actor.go`,
   graph-grpc-service's `actor_fields.go` and `nodes_edit.go`, and
   actor-service's routes for people, groups, events and channels.
4. **It hides nothing** (the user, 2026-11-22): `noindex` asks search
   engines not to index a page. Who may see it is the profile's privacy,
   unchanged.

## What it changes

| Page | When | What it carries |
| --- | --- | --- |
| A profile, at `/@alias`, `/people/:alias`, `/groups/:id` and the rest, and its tabs | The profile said no | `<meta name="robots" content="noindex">` and the header `X-Robots-Tag: noindex` |
| A post, at `/notes/:id` and the rest | Its author or the profile it is on said no | The same |
| A profile or a post from another server, shown here | Always | The same: its home is the page to be found, not our copy |
| Hashtags, locations, feeds, search | Never | Unchanged |

1. **web-shell writes it,** as it writes a page's Open Graph tags
   (`shell/pages.go`, `shell/template.go`): `Subject` gains whether the
   page may be indexed, from the node it already reads, and the header is
   set beside the page. A search engine asks web-shell for every address,
   so this is enough without the web app.
2. **Pages that list posts stay as they are** (the user, 2026-11-22). A hashtag's page, a
   location's or a public feed may show a post from somebody who said no,
   as Mastodon's tag pages do; a search engine that runs the page's script
   may keep its words with the list's address. Leaving those posts out of
   every list shown to somebody not signed in is a larger change, weighed
   if it is asked for.

## What exists

1. **The fediverse's own search is a choice already:** "Include me in
   search" in the Federated tab sends Mastodon's `indexable`
   (activitypub-service, `PUT /activitypub/settings`, see
   [Federation](../../federation.md#listed-and-found)). It stays its own
   choice: another server's search and a search engine are different
   things, and a person may want one and not the other.
2. **A members-only site** carries `noindex` on every page already
   (`routes/MembersOnlyGate.jsx` in the web app): nothing changes there.
3. **There is no sitemap,** so there is nothing to take a profile out of.

## Out of scope

1. Bluesky. A person's Bluesky account is their own, connected to Anahita;
   whether Bluesky's apps show it to people not signed in is set in
   Bluesky.
2. Taking pages out of a search engine that has them already: `noindex`
   is read when the page is next visited, which is the search engine's
   own time.

## Documentation

1. anahita-services: `docs/profiles.md` and `docs/permissions.md` (the
   setting), `docs/web-shell.md` or wherever the shell's tags are
   described, `docs/features.md` ("Hide from search engines": yes), the
   API reference (`search_engines` on profiles), `CHANGELOG.md`.
2. anahita-web: `docs/architecture.md` (Settings › Privacy), `CHANGELOG.md`.
