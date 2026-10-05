# Members-only instances, no-public-content policy, and the composer privacy picker

*Roadmap item 13, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
Some instances (a school, a company, a private community) want nothing visible to logged-out visitors, or at least no *new* public content. Separately, people should choose who can see a post **while writing it**, not only by editing it afterwards.

**What exists:**
- Site-wide policies are env vars read by auth-service and published to the web through NodeInfo `metadata`. For example `REGISTRATION_MODE` and `INVITES_FROM` (`docs/configuration.md:54-55`, `auth-service/cmd/api/handlers/node_info.go:197-251`). The web app reads them through `readNodeInfo` (`anahita-web/src/actions/app.js:20-26`).
- **Anonymous reads** go through `an_mw.SetViewer`, which sets viewer 0 (`anahita-libs/middleware/oauth.go:69-81`). The SQL access gates admit viewer 0 to `public` and NULL (`graph-grpc-service/data/repositories/access.go:30-33,73-76`).
- **Access values:**
  - media: `public registered followers leaders mutuals admins myself` (`anahita-libs/requests/media.go:71,87`);
  - persons: `public registered followers leaders mutuals myself` (`person.go:70`);
  - groups: `public registered followers admins` (`actor.go:106`).
- **Defaults are `public`:** new persons (`auth-service/cmd/api/handlers/signup.go:338`, `signup_requests.go:188`, `invite_accept.go:333`) and media (`anahita-libs/requests/media.go:33-42`).
- **The composer never sends `access`.** Only `name` and `body` go out (`anahita-web/src/containers/media/Composer/index.jsx:60-64`). The create request accepts `access` (`media.go:87`, `omitempty`), so every new post is public. Access can only be changed afterwards, by editors, through `src/containers/controls/medium/Access.jsx` (`PATCH /{ns}/{id}/access`).

## Two instance settings, which can be combined
Both follow the `REGISTRATION_MODE` pattern: env vars set in the shared config map, read by every service that needs them, and published in NodeInfo `metadata`. They take effect on restart. An admin UI can come later.

**1. `SITE_READ_ACCESS=public|registered`: members-only site ("make everything private")**
- **No data changes, and fully reversible.** Stored access values stay as they are; the rule is applied when reading.
- **API:** a new `an_mw.RequireMembersOnly` middleware in `anahita-libs`, wired into every service's router next to `SetViewer`, returns **401 to anonymous viewers** on every route except an allowlist:
  - the auth-service login, signup, OAuth, password and code routes;
  - `/.well-known/*` and `/nodeinfo/*`;
  - the legal pages;
  - health checks.
  - One shared list in `anahita-libs`, so no service forgets it.
- **Belt and braces in graph-grpc:** when the setting is on, the SQL gates treat viewer 0 as matching nothing (`access.go`, `access_actor.go`, `access_medium.go`, `access_feed.go`, `access_search.go`). Any path that misses the middleware still leaks nothing.
- **Media:** presigned URLs are only handed out by authorised API calls, so they're fine. The stable media proxy (item 8 phase 0.4 / item 9 A) must **require a signed-in session or a short-lived signed URL** in this mode.
- **Web app:**
  - With NodeInfo `metadata.readAccess = registered`, every route except the landing page, `/legal`, `/support` and the OAuth callback goes through `AuthenticatedRoute`.
  - Logged-out visitors see a landing page with the site's name, description and Sign in / Sign up (per `REGISTRATION_MODE`).
  - Add a `robots` `noindex`.
- **Web shell (item 10):** only site-wide Open Graph tags, never per-page ones.
- **Federation (item 8):** federating is incompatible with a members-only site.
  - The protocol services refuse opt-ins, and stop serving actors, objects and WebFinger (404).
  - NodeInfo reports `protocols: []`.
  - Existing opted-in people are told when federation was switched off.
- **Link previews (item 10)** keep working for members.

**1b. `SITE_READ_ACCESS=preview`: logged-out visitors see partial public content (added 2026-09-28)**

A middle value between `public` and `registered`. Logged-out visitors still pass the access gates for **public** content, but the API returns a **preview**, and a "Sign in to read more" prompt does the rest. It's enforced **on the server**: the API is public, so the web app hiding things wouldn't be enough.
- **Where it's applied:** a shared response filter in `anahita-libs/responses`, applied when the viewer is anonymous and the mode is `preview`.
- **What the preview contains:**

  | Content | Anonymous visitors get |
  | --- | --- |
  | Note / topic / article body | The first ~280 characters (or the first paragraph), Markdown stripped, with `truncated: true` |
  | Photos | One small thumbnail per post (the smallest existing size), no originals, no albums |
  | Replies and comments | Hidden; counts only |
  | Profiles | Name, avatar, cover and bio. Follower and following lists are hidden; counts only. |
  | Lists (profile tabs, hashtags, locations, search) | The first page only (about 10 items), no pagination |
  | Media downloads, online event URLs (item 12), attendee lists | Hidden |

- **Web app:** truncated items end with a fade and "Sign in to read more", and list ends show "Sign in to see more". It keys off the response's `truncated` flag and NodeInfo `metadata.readAccess = preview`.
- **Web shell (item 10):** per-page Open Graph tags work, using the preview text and thumbnail, so shared links look good while the full content stays behind sign-in. This is a nice reason to pick this mode.
- **Federation (item 8):** federating would send the full content out, which defeats the purpose. The protocol services behave as in members-only mode (off), and the Fediverse settings say why.
- **Estimate:** about 4–5 h extra (the response filter and its tests, the web prompts).

**2. `MIN_CONTENT_ACCESS=public|registered`: no new public content ("from now on, registered and up")**
- **Writes reject `public`** with 422 `access_not_allowed` on:
  - media create and `PATCH /…/access`;
  - person and group access edits;
  - new-account creation.
  
  A shared validator in `anahita-libs/requests` replaces the three hard-coded `oneof` lists, using one ordered list of access levels filtered by the setting.
- **Defaults become `registered`:** the signup, invite-accept and signup-request person creation, new groups, and media defaults.
- **Existing public content stays public** (decided 2026-09-28). A super-admin can run a one-time **"Convert all public content to registered-only"** action:
  - An admin Settings action (item 4a area) with a typed confirmation and step-up, backed by an idempotent graph-grpc batch update (`access='public'` → `registered` on actors and media).
  - It writes an audit row and can't be undone automatically.
- **Web:** the Public option disappears from every access menu (`src/constants/access.js`, filtered by NodeInfo `metadata.minContentAccess`) and from the composer picker below.
- **Federation:** only public posts federate, so with this setting nothing new goes out. The Fediverse settings section says so.

**Combining them:** `SITE_READ_ACCESS=registered` + `MIN_CONTENT_ACCESS=registered` is the fully private instance. The second alone keeps old public content visible but adds no more.

## Privacy picker in the composer (added 2026-09-28)
- **Web** (`src/containers/media/Composer/`):
  - Add an **audience picker** next to Post, a small button with an icon and label ("Public", "Registered", "Followers", "Mutuals", "Only me", or "Admins" for groups). It reuses the menu from `controls/medium/Access.jsx` and the labels in `languages/*/access.js`.
  - Send `access` with the create request by adding it to the fields in `Composer/index.jsx:60-64`.
- **Options depend on where you post:**
  - On your own profile: public, registered, followers, mutuals, only me.
  - On a group: public, registered, followers (members), admins.
  - Filtered by `MIN_CONTENT_ACCESS`.
  - Options wider than the profile or group itself are shown disabled with a hint. For example, on a followers-only group, "Public" isn't offered, because the owner's access limits the post anyway.
- **Default:** the person's last choice for that kind of place (remembered per person in `localStorage`, with try/catch). Otherwise `public`, or `registered` under the setting. Always clamped to what's allowed.
- **Replies** (item 7) have no picker; they inherit the thread's audience.
- **Federation hint:** when the person has opted into item 8, a non-public choice shows "This post won't be shared with the fediverse".
- **Server:** the create handlers validate `access` with the shared validator (above) and the owner type (`admins` only on groups, `mutuals` / `myself` only on people). Add tests for both.

## Estimate

| Part | Time |
| --- | --- |
| `SITE_READ_ACCESS`: shared middleware + allowlist in every service, graph-grpc gates, NodeInfo field, media proxy rule | 5–6 h |
| `SITE_READ_ACCESS`: web route gating + landing page, web-shell and federation behaviour | 3–4 h |
| `MIN_CONTENT_ACCESS`: shared access validator, defaults, NodeInfo field, web menus | 3–4 h |
| One-time "convert public → registered" admin action (step-up, audit) | 2–3 h |
| Composer audience picker (web) + server validation by owner type + tests | 3–4 h |
| `SITE_READ_ACCESS=preview`: response filter, web prompts, OG from previews | 4–5 h |
| Docs (`docs/configuration.md`, `docs/permissions.md`) | 1 h |
| **Total** | **about 21–27 h, 5–7 sessions** |

The composer picker has no dependencies and can ship first, as a small standalone win (about 1 session).

## Verification
- **Members-only:**
  - A logged-out `curl` of `/people/alice`, `/notes/1`, the feeds and search gives 401. Login, signup, NodeInfo and WebFinger still respond (WebFinger answers 404 because federation is off).
  - A signed-in registered user sees public content as before.
  - A test service route that deliberately skips the middleware still returns nothing to viewer 0, through the graph gate.
  - The web shell serves only site-wide tags, and the logged-out web app shows only the landing page.
- **Preview:**
  - A logged-out `curl` of a long public note returns about 280 characters with `truncated: true`, no replies, and a photo post returns only the thumbnail.
  - A profile's follower list gives 401, and page 2 of a hashtag list gives 401.
  - The same requests signed in return everything.
  - The web shell's `og:description` is the preview text.
- **No-public:**
  - Creating a public note gives 422, and a new signup's profile is `registered`.
  - Access menus have no Public option.
  - Existing public posts stay visible to logged-out visitors until the convert action runs. After it, they're registered-only, and a second run changes nothing.
- **Composer:**
  - A post created as "Followers" is followers-only right away (no later edit needed), and is invisible to a logged-out visitor and to a non-follower.
  - On a group, "Mutuals" isn't offered.
  - The last choice is remembered.
  - With the no-public setting on, the picker defaults to Registered and has no Public option.

## Decisions (item 13)
- Two independent env settings, like `REGISTRATION_MODE`:
  - `SITE_READ_ACCESS=public|preview|registered`: open, partial previews for logged-out visitors, or members-only. No data change.
  - `MIN_CONTENT_ACCESS`: no new public content.
- Existing public content stays public, with an optional one-time super-admin conversion.
- Posts get an audience picker in the composer, defaulting to the person's last choice and clamped to what's allowed.

## Dependencies
- The convert action lives in the item 4a admin area.
- Interacts with items 8 (federation off in members-only mode; nothing non-public federates), 9 and 8 phase 0.4 (the media proxy needs an auth rule), 10 (the web shell uses site-wide tags only) and 7 (replies inherit the thread's audience).
- The composer picker is independent and can go early.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `configuration.md`**: `SITE_READ_ACCESS` (public / preview / registered) and `MIN_CONTENT_ACCESS`, with what each does to federation and Open Graph.
- **services `permissions.md`**: how the instance settings narrow access levels; the composer audience rules per place.
- **services `registration.md`**, *NodeInfo*: the new `readAccess` and `minContentAccess` metadata fields.
- **web `configuration.md`**, *Settings that live on the server*: the same two settings as seen by the app.
- **web `architecture.md`**: the landing page for logged-out visitors, and the composer audience picker.

## What was built

### The composer audience picker (2026-10-04)

**Server.** `permissions.MediumAudiences` in `anahita-libs` says which access
levels a post may have where it is going, and the create and change-access
handlers for notes, articles, topics and photos refuse anything else with 422
`access_not_allowed` and the list that is allowed. Until now any of the seven
levels was accepted on any post.

Two of its rules were not in the plan, and are deliberate:
- **No "only me" on somebody else's profile.** It would be a post only its
  author and that person can see: the private notes removed in item 1.
- **"Administrators only" in a group is for its administrators** and site
  administrators. From an ordinary member it would be a private message to
  the group's administrators.

**Web.** A button beside Post names the current audience and opens a menu
(`containers/media/Composer/Audience.jsx`); the rules for what it offers are
in `utils/audience.js`. The choice is sent with the post.
- In a group, followers are called Members.
- An audience wider than the profile itself is shown disabled, with the
  reason.
- It starts on the last choice for that kind of place (your own profile,
  somebody else's, a group), kept per person in the browser, or else on the
  widest the profile allows.
- `leaders` is accepted by the server on a person's profile and not offered
  when writing: it is not an audience anybody picks.

**Not built yet,** from this section of the plan: filtering by
`MIN_CONTENT_ACCESS`, which comes with that setting, and the fediverse hint,
which comes with federation.

**Not checked.** The picker in a browser, and a post created through the
running stack with each audience. The rule has unit tests on both sides.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Composer audience picker | done 2026-10-04, not yet tried in a browser |
| `MIN_CONTENT_ACCESS` | not started |
| `SITE_READ_ACCESS=registered` | not started |
| `SITE_READ_ACCESS=preview` | not started |
| Convert public to registered | not started |
