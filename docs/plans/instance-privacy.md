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

**The server tells the app.** Every person and group response carries
`authorized.audiences`, from `permissions.PostAudiences`, so the composer
offers what will be accepted and does not keep a second copy of the rule.

**Web.** A button beside Post names the current audience and opens a menu
(`containers/media/Composer/Audience.jsx`). `utils/audience.js` reads the
server's list and adds what the server has no opinion on. The choice is sent
with the post.
- In a group, followers are called Members.
- An audience wider than the profile itself is shown disabled, with the
  reason.
- It starts on the last choice for that kind of place (your own profile,
  somebody else's, a group), kept per person in the browser, or else on the
  widest the profile allows.
- `leaders` is accepted by the server on a person's profile and not offered
  when writing: it is not an audience anybody picks.

**The control on an existing post is the same button** (asked for by the user
on 2026-10-04). `controls/medium/Access.jsx` was a bare globe or padlock
opening a list of level names; it now uses the composer's button
(`components/AudienceButton.jsx`), and a post the viewer may edit carries
`authorized.audiences`, the audiences it may be changed to. A post's response
does not say how widely its profile is visible, so on an existing post the
wider-than-the-profile choices are not greyed out as they are when writing.

**Not built yet,** from this section of the plan: filtering by
`MIN_CONTENT_ACCESS`, which comes with that setting, and the fediverse hint,
which comes with federation.

**Not checked.** The picker in a browser, and a post created through the
running stack with each audience. The rule has unit tests on both sides.

### No new public content: `MIN_CONTENT_ACCESS` (2026-10-04)

**One place reads it:** package `anahita-libs/policy`. A value it does not
understand is read as `registered`, the stricter one, and logged.

**Where it is enforced.**
- **Posts:** `permissions.MediumAudiences` leaves `public` out, so creating or
  changing a post to public is refused with 422 `access_not_allowed`, and the
  lists a profile and a post carry (`authorized.audiences`) leave it out too.
  The composer and the control on a post follow without knowing the setting.
- **Profiles and groups:** the change-access handlers in person-service and
  group-service refuse `public` the same way.
- **Defaults:** a post with no audience said, and a new account's profile
  (signup, an approved request, an accepted invitation, the first super
  administrator), get `registered`.
- **What already is public may stay so:** a change-access request that keeps
  the current value is let through, so editing something else about a public
  profile does not fail.

**NodeInfo** carries `minContentAccess`. The web app uses it in one place, the
profile access settings, which has no list from the server to follow.

**Configuration.** `MIN_CONTENT_ACCESS` and `SITE_READ_ACCESS` are now passed
to every service in `docker-compose.yml` and the Kubernetes manifests, as
optional keys of the shared configuration. The second is not read yet.

**Where it differs from the plan.**
- New groups were not changed: a group already starts as `admins`, a draft.
- The "shared validator replacing three `oneof` lists" is the audience rule
  for posts and a two-line check for profiles. The `oneof` tags stay, as the
  first check that the value is a word the API knows.

**Checked.** Unit tests for the policy and for audiences under it. On the
development stack, NodeInfo reports `public` by default, `registered` with the
setting, and `registered` with a misspelt value, with the log line.

**Not checked.** Creating a post or changing a profile with the setting on,
through the running stack with a session.

### Members-only: `SITE_READ_ACCESS=registered` (2026-10-05)

**Enforced in one place.** Every route that serves anything to somebody who is
not signed in goes through `an_mw.SetViewer`, which is the only place an
anonymous viewer is made for a request. With the setting on it answers 401
`members_only` instead. No service's routes changed.

**A guard instead of an allowlist.** `TestEveryRouteNamesItsViewer` in
`anahita-libs/middleware` reads every service's `routes.go` and fails if a
route has neither `Authorize` nor `SetViewer` in front of it. Today only
auth-service has such routes, 44 of them, all ways in (sign in, sign up,
password reset, invitations, OAuth, NodeInfo); they are held to a list in the
test.

**NodeInfo** carries `readAccess`.

**Web.** `routes/MembersOnlyGate.jsx` wraps the route table. On a members-only
site, somebody who is not signed in sees a notice with a Sign in button and
the home page, at whatever address they asked for, and every page carries
`robots: noindex`. The pages that stay open are in `routes/membersOnly.js`.

**Where it differs from the plan.**
- **The SQL gates in graph-grpc were not changed.** The plan had them treat
  viewer 0 as matching nothing, as a second line. But viewer 0 is also what
  the services use for their own reads, with nobody signed in: notifications
  being written from events, hashtags being extracted, accounts being
  created. Making it match nothing would have broken those on exactly the
  installations that turn this on. The guard test is the second line instead.
- **No new middleware and no shared allowlist.** The plan had a
  `RequireMembersOnly` wired into every router beside `SetViewer`, which every
  new service would have to remember. Putting it inside `SetViewer` cannot be
  forgotten.
- The media proxy, the web shell and federation do not exist yet. Their rules
  for this mode stay in this plan for when they do.

**Checked.** On the development stack with the setting on: profiles, lists,
feeds, hashtags, places, followers, photos and search all answer 401 to
somebody not signed in; NodeInfo, the sign-in page and the sign-up page answer
200; NodeInfo reports `readAccess: registered`. With it off again, everything
answers as before.

**Not checked.** A signed-in member on a members-only stack, and the web
app's notice in a browser.

### Converting what is already public (2026-10-05)

**Graph.** `PublicAccessRepo` counts what is public (people, groups, posts
with their revisions) and changes it to `registered` in chunks of 5,000 rows,
then writes one row to a new table, `site_actions`, with who did it and the
counts. Hashtags and places are left. `updated_at` is not touched: nobody
edited the posts. Running it again changes nothing.

**API**, in auth-service beside the accounts list:
- `GET /accounts/public-content` gives the counts and whether public content
  is still allowed.
- `POST /accounts/make-registered` with `{ "confirm": "CONVERT 8675" }` does
  it. Super administrators only, with a proof of identity, and the number must
  be what is public at that moment. 409 `public_content_allowed` unless
  `MIN_CONTENT_ACCESS=registered`.

**Web.** A Privacy tab in Site Settings shows the two settings as the server
has them, and the conversion with its counts, the typed confirmation and the
proof of identity.

**Where it differs from the plan.** It is refused while public content is
still allowed, which the plan did not say. Converting then would be undone by
the next public post, and pressing it by mistake on an open site would hide
the whole of it from visitors.

**Checked.** The change against real MySQL (what changes, what does not, the
record, a second run); the gates in front of it in handler tests; the
migration on the fresh and upgrade paths. On the development stack the two
routes refuse a caller with no token.

**Not checked.** The conversion through the running stack, and the Privacy
tab in a browser.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Composer audience picker | done 2026-10-04, not yet tried in a browser |
| `MIN_CONTENT_ACCESS` | done 2026-10-04 |
| `SITE_READ_ACCESS=registered` | done 2026-10-05 |
| `SITE_READ_ACCESS=preview` | not started |
| Convert public to registered | done 2026-10-05 |
