# Verified domains: prove you own a website, and use it as your handle

*Roadmap item 26, planned 2026-10-02. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
On Bluesky a person can use their own domain as their handle (`@rastinmehr.com`) by adding a DNS record that proves they control it. We want the same on Anahita: a person proves they own a domain, their profile shows it as verified, and where a network supports it the domain becomes their handle.

This is why `nodes.verified` was kept when the unused legacy columns were dropped ([done/remove-unused-node-columns.md](done/remove-unused-node-columns.md)).

## Does it work with Mastodon? Partly, by a different mechanism
The two networks verify different things, so one proof on our side feeds two different results.

| | Bluesky / AT Protocol | Mastodon / ActivityPub |
| --- | --- | --- |
| What is verified | The **handle**: the domain *is* the handle | A **link** on the profile: the green tick beside a website |
| How it is proved | A DNS TXT record `_atproto.<domain>` holding the account's DID, or the file `/.well-known/atproto-did` on the domain | The website links back to the profile with `rel="me"`. Mastodon fetches the page and checks |
| What others see | `@rastinmehr.com` as the handle, everywhere on Bluesky | The handle stays `@rastin@anahita-instance`; the website shows as a verified link on the profile |
| Domain as handle | Yes | **No.** A person can make `@me@rastinmehr.com` *findable* by serving a WebFinger file on their domain that points at their real account, but Mastodon then shows the real handle, not the domain |

So: on Bluesky the domain becomes the handle. On Mastodon the handle can't change, but the same domain shows as a verified link, which is the verification Mastodon users recognise. Search by `@me@domain` can be made to work with one small file on the person's site.

## What exists
- `nodes.verified` (boolean) is stored and returned on actors (`anahita-libs/responses/actor.go:17`), and an actor edit can set it (`graph-grpc-service/cmd/server/handlers/actor_fields.go:40`). Nothing sets it from a real check and the web app never shows it.
- `website_url` is one validated http(s) link on an actor profile.
- Nothing verifies domains, and there is no DNS lookup code.
- Federation (item 8) is not built yet. Its AT Protocol phase gives each opted-in person a handle `alias.DOMAIN` on our PDS.

## Decisions
- **One proof, three results.** A person verifies a domain once; Anahita shows the badge, Bluesky gets the handle, Mastodon gets the verified link.
- **Two ways to prove, either is enough:**
  1. **DNS:** a TXT record. If the person has opted into Bluesky, the record is `_atproto.<domain>` with their DID, so the same record also makes the domain their Bluesky handle. Otherwise `_anahita.<domain>` with a token.
  2. **A link back:** their site links to their Anahita profile with `rel="me"`. This is the same thing Mastodon checks, so one link satisfies both.
- **`verified` stops being settable through an actor edit.** Only the verification check sets it.
- **Verification can lapse.** Domains are re-checked, and a domain that stops proving ownership loses the badge.

## Design
**Data**
- `nodes.verified` stays as the flag on the actor.
- New table `verified_domains`: `actor_id`, `domain`, `method` (`dns` | `rel_me`), `token`, `status` (`pending` | `verified` | `lapsed`), `verified_at`, `checked_at`, `last_error`. Unique on `domain` among `verified` rows, so one domain verifies one actor.
- Groups can verify a domain too (an organisation's site).

**Checking**
- **DNS:** look up the TXT record with Go's resolver, against two public resolvers, so a stale local cache doesn't decide.
- **Link back:** fetch the page with the SSRF-safe fetcher from the link preview service ([link-previews.md](link-previews.md): no private addresses, a size cap, a timeout) and look for `<a>` or `<link>` with `rel="me"` whose `href` is the actor's canonical profile URL.
- **Re-checks:** the sweeper re-checks verified domains weekly. Three failures in a row mark the domain `lapsed`, clear `verified`, and notify the person. A lapsed domain can be verified again.
- **Names that could mislead:** domains are stored in punycode and shown in Unicode only when every label is in one script, the usual defence against look-alike domains.
- **Rate limit** on verification attempts (the shared limiter).

**Endpoints** (person-service and group-service, sharing a handler in `anahita-libs`)

| Endpoint | Purpose |
| --- | --- |
| `POST /people/:id/domains` `{ domain }` | Start: returns the DNS record to add and the link to place |
| `POST /people/:id/domains/:domain/verify` | Check now |
| `GET /people/:id/domains` | The person's domains and their state |
| `DELETE /people/:id/domains/:domain` | Remove |

Starting and removing require step-up, since a verified domain is part of identity.

**What a verified domain does**
- **On Anahita:** a verified mark and the domain on the profile and beside the name in feeds. The actor response gains `verified_domain`.
- **Bluesky (item 8, AT phase):** atproto-service sets the domain as the handle on the PDS, once the `_atproto` record resolves to the person's DID. If the domain lapses, the handle falls back to `alias.DOMAIN`.
- **Mastodon and the rest of ActivityPub (item 8, ActivityPub phase):**
  - the actor JSON lists the website as a profile field (a `PropertyValue` attachment), which is how Mastodon shows profile links;
  - the profile page's HTML carries `<link rel="me" href="https://domain/">`, and the person's site links back with `rel="me"`. Mastodon then shows the link as verified. This has to be in the HTML the server returns, so it comes from the web shell ([open-graph.md](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/open-graph.md), item 10), not from the React app;
  - `alsoKnownAs` on the actor lists `https://domain/`;
  - **optional, for search by domain:** the settings page offers a ready-made WebFinger file for the person to host at `https://domain/.well-known/webfinger`, so `@me@domain` finds their account from any fediverse server. Their handle shown on Mastodon is still the Anahita one.
- **Remote accounts:** for Bluesky accounts whose handle is a domain, and Mastodon accounts with verified links, show the same mark. Bluesky handles are verified by the network already; for Mastodon links we run our own link-back check, since Mastodon doesn't send its verdict.

## Web (anahita-web)
- **Settings → Account → "Verify a domain":** enter the domain, then a two-tab instruction card (DNS record, or link on your site) with copy buttons, a "Check now" button, and the state (pending, verified, lapsed with the reason).
- The verified mark and domain on the profile header, actor cards and feed items, with a tooltip saying what was verified and when.
- When Bluesky is connected: "Use this domain as my Bluesky handle", on by default.
- The WebFinger file download, with a one-line explanation of what it does and doesn't change.
- i18n in en-GB and fr-FR.

## Estimate

| Part | Time |
| --- | --- |
| `verified_domains` table, DNS and link-back checks, endpoints, step-up, rate limit, tests | 5–6 h |
| Sweeper re-checks, lapse and notification; `verified` no longer settable by edit | 2 h |
| Web: settings flow, verified mark on profiles, cards and feeds | 3–4 h |
| Bluesky handle switch (atproto-service) | 2–3 h |
| ActivityPub profile field, `alsoKnownAs`, `rel="me"` in the web shell, WebFinger file | 2–3 h |
| **Total** | **about 14–18 h, 4–5 sessions** |

The first three rows (the Anahita part) can ship before federation exists.

## Verification
- A TXT record with the right value verifies; a wrong value, or a record on another domain, does not. Removing the record lapses the domain after three weekly checks and clears the mark.
- A page with `rel="me"` pointing at the profile verifies; the same link without `rel="me"`, or pointing at another profile, does not.
- A domain verified by one actor can't be verified by another.
- Verification attempts against `localhost`, a private address or a redirect to one are refused.
- `verified` can't be set through `PATCH /people/:id`.
- With Bluesky connected, the handle on bsky.app becomes the domain, and reverts when the domain lapses.
- On a Mastodon server, the Anahita profile shows the website with the verified tick once the site links back.

## Dependencies
- **Hard for the federation parts:** item 8 (AT Protocol phase for the handle, ActivityPub phase for the profile field) and item 10 (the web shell, for `rel="me"` in served HTML).
- **Reuses:** item 10's SSRF-safe fetcher, the shared rate limiter (item 4b), step-up.
- **The Anahita-only part** (prove a domain, show the mark) depends on none of them.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `registration.md`** (or the planned `federation.md`): what a verified domain is, the two ways to prove one, how it lapses, and what it does on Bluesky and on Mastodon.
- **services `configuration.md`**: the re-check interval and the resolvers used.
- **web `architecture.md`**: the verification flow and where the mark appears.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-02 |
| Implementation | not started |
