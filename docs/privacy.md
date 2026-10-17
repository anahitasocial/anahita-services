# Privacy

[Documentation](README.md) › Privacy

Who can see what on an installation. There are three layers, and something is
seen only by somebody who gets past all of them:

1. **The installation**: what people who are not signed in may read at all,
   and whether anything new may be public. Set by whoever runs the site.
2. **The profile**: who can see a person's profile or a group. Set by its
   owner or administrators.
3. **The post**: who can see one post. Set by its author.

The routes that set who may see what are in the API reference, each with the audiences it takes: making everything registered-only is [`POST /accounts/make-registered`](api/index.html#postAccountsMakeRegistered).

## Contents

- [The audiences](#the-audiences)
- [Who can see a post](#who-can-see-a-post)
- [Who can see a profile or a group](#who-can-see-a-profile-or-a-group)
- [What the installation allows](#what-the-installation-allows)
  - [Visitors: `SITE_READ_ACCESS`](#visitors-site_read_access)
  - [New content: `MIN_CONTENT_ACCESS`](#new-content-min_content_access)
  - [Using them together](#using-them-together)
  - [Making what is already public members-only](#making-what-is-already-public-members-only)
- [Setting them](#setting-them)
- [Checking them](#checking-them)
- [What the web app does](#what-the-web-app-does)
- [Where it is enforced, and how it is tested](#where-it-is-enforced-and-how-it-is-tested)
- [Federation](#federation)

## The audiences

Posts, profiles and groups all use the same words:

| Audience | Who that is |
| --- | --- |
| `public` | Anybody, signed in or not |
| `registered` | Anybody who is signed in |
| `followers` | People who follow the profile or group |
| `leaders` | People the profile follows. People only |
| `mutuals` | People who follow the profile and are followed back. People only |
| `admins` | The administrators of the group. Groups only |
| `myself` | Only the owner of the profile. People only |

Each one is measured against **the profile the post is on**, not against
whoever wrote it: `followers` are that profile's followers.

The owner of a profile and its administrators always see everything on it, and
an author always sees their own post where they can still see the profile.
Nobody sees anything belonging to somebody they have blocked or been blocked
by. The full rules are in [Permissions](permissions.md).

## Who can see a post

The author chooses when writing it, with the audience button beside Post, and
whoever may edit the post can change it afterwards with the same button.

Which audiences a post may have depends on where it is:

| Where the post is | Audiences |
| --- | --- |
| Your own profile | `public`, `registered`, `followers`, `leaders`, `mutuals`, `myself` |
| Somebody else's profile | `public`, `registered`, `followers`, `leaders`, `mutuals` |
| A group, as one of its followers | `public`, `registered`, `followers` |
| A group, as one of its administrators, or as a site administrator | `public`, `registered`, `followers`, `admins` |

Anything else is refused with 422 `access_not_allowed`, and the answer lists
the audiences that are allowed.

- **No "only me" on somebody else's profile.** It would be a post only its
  author and that person can see, which is a private message. This platform
  does not have those.
- **No "administrators only" from a follower who is not one**, for the same
  reason.
- **A post is never seen more widely than its profile.** On a followers-only
  group a `public` post is still seen only by its followers.
- A post written without saying is `public`, or `registered` where the
  installation allows nothing new to be public.
- Replies have no audience of their own. They are seen by whoever can see the
  post at the top of their thread.

The API: `access` when the post is created (`POST /notes/:owner_id/`, and the
same for articles, topics and photos; for a video, when its upload begins,
`POST /videos/:owner_id/uploads`), and `PATCH /…/:id/access` to change it.
Every person, group and post in a response carries `authorized.audiences`, the
audiences the viewer may choose there, so a client offers only those.

## Who can see a profile or a group

Set on the profile's Access settings.

| | Audiences |
| --- | --- |
| A person | `public`, `registered`, `followers`, `leaders`, `mutuals`, `myself` |
| A group | `public`, `registered`, `followers`, `admins` |

A new account's profile is `public`, or `registered` where the installation
allows nothing new to be public. A new group starts as `admins`, a draft only
its administrators see, until they open it.

Somebody who cannot see a profile cannot see anything on it, whatever each
post says.

## What the installation allows

Two settings, independent of each other.

### Visitors: `SITE_READ_ACCESS`

What somebody who is **not signed in** may read.

| Value | A visitor gets |
| --- | --- |
| `public` (the default) | Everything that is public |
| `preview` | Only the start of what is public |
| `registered` | Nothing. A members-only site |

Signed-in people see exactly the same under all three. Nothing stored changes,
so any of them can be switched to any other: a post that says `public` is
public again the moment the setting is `public`.

**`registered`, the members-only site.** Every route that serves content
answers 401 `members_only` to a visitor: profiles, posts, feeds, replies,
hashtags, places, followers and search. What stays open is what people need to
get in: signing in and up, password reset, accepting an invitation, the OAuth
endpoints and NodeInfo.

**`preview`.** A visitor still reaches what is public, and is sent only the
start of it:

| | What a visitor is sent |
| --- | --- |
| A post's text, and a profile's | The first 280 characters or so, as plain words, cut at a word and ending in "…". The post is marked `truncated: true`. A text that fits is sent as it was written |
| A photo | The first image only, at its smallest size. `files_held_back` says how many more the post has |
| A list | The first 10 items. `pagination.more` is true when there are more, and `pagination.total` is the number sent. Asking for a later page answers 401 `members_only` |
| Replies | Nothing: 401 `members_only`. The count on the post is still there |
| Who follows a profile, and whom it follows | Nothing: 401 `members_only`. The counts on the profile are still there |
| Avatars and covers | As usual |

It is enough for a link shared elsewhere to say what it leads to, while the
rest stays behind signing in.

### New content: `MIN_CONTENT_ACCESS`

The widest audience anything **new** may be given.

| Value | |
| --- | --- |
| `public` (the default) | Anything may be public |
| `registered` | Nothing new may be public |

With `registered`:

- A post can no longer be created public or changed to public, and neither can
  a person's profile or a group. The answer is 422 `access_not_allowed`.
- A post written without saying who can see it is for signed-in people, and so
  is a new account's profile.
- **What is already public stays public**, and may stay so when it is edited.
  The setting governs what is written from now on.

### Using them together

| `SITE_READ_ACCESS` | `MIN_CONTENT_ACCESS` | The site |
| --- | --- | --- |
| `public` | `public` | Open. The default |
| `public` | `registered` | Old public posts stay readable by visitors; nothing new is added to them |
| `preview` | `public` | Visitors see the start of things; people still post publicly, which is what makes a preview worth having |
| `registered` | `public` | Members-only. "Public" then means every signed-in person, and would mean everybody again if the site were opened |
| `registered` | `registered` | Fully private, and it would stay private if the site were opened later |

### Making what is already public members-only

`MIN_CONTENT_ACCESS=registered` does not reach back. A super administrator can
bring the rest into line, once: **Site Settings › Privacy › Make everything
public members-only**.

- It changes every public person, group and post to `registered`. Hashtags and
  places are left: they are nobody's content.
- It asks for the number of things it will change to be typed (`CONVERT 8675`)
  and for a proof of identity.
- It is refused, with 409 `public_content_allowed`, on an installation that
  still allows public content: there it would be undone by the next public
  post.
- It records who did it and how much it changed, in `site_actions`.
- **It cannot be undone.** Nothing records what was public before.

The API: `GET /accounts/public-content` for the counts, and
`POST /accounts/make-registered` with `{ "confirm": "CONVERT 8675" }`.

## Setting them

Both are environment variables read by every service, so they are set in one
place and need a restart.

With docker-compose, in `.env`:

~~~sh
SITE_READ_ACCESS=registered
MIN_CONTENT_ACCESS=registered
~~~

then `make up`.

On Kubernetes, in the overlay's `config.env`, then apply it and restart the
deployments. See [Deploying](deploying.md).

A value that is not understood is treated as `registered`, the strictest, and
each service says so in its log:

~~~
policy: SITE_READ_ACCESS is "privat", which is not "public", "preview" or "registered"; treating it as "registered"
~~~

## Checking them

NodeInfo says what the installation is set to, and answers on every kind of
site:

~~~sh
curl -s https://api.example.org/nodeinfo/2.1 | grep -E 'readAccess|minContentAccess'
~~~

Then, without signing in, ask for something public. `SOMEONE` is any handle
and `5` any person's id:

| Request | Open | Preview | Members-only |
| --- | --- | --- | --- |
| `GET /people/SOMEONE` | 200 | 200 | 401 |
| `GET /notes/5/?limit=50` | 200 | 200, at most 10 items, long ones cut and marked `truncated` | 401 |
| `GET /notes/5/?start=10` | 200 | 401 | 401 |
| `GET /socialgraph/5/followers/` | 200 | 401 | 401 |
| `GET /photos/5/download` (where the post allows it) | 302 | 403 | 401 |
| `GET /oauth/login` | 200 | 200 | 200 |

## What the web app does

- **The audience button** offers what the server says a post may have there.
  A choice wider than the profile is greyed out, with the reason.
- **On a members-only site**, a visitor sees a notice with a Sign in button
  and the home page, in place of whatever they asked for and at the same
  address. The search box and the People, Groups, Hashtags and Places menu
  entries are not shown. Pages carry `robots: noindex`.
- **On a preview site**, a visitor sees a notice above each page, "Sign in to
  read more" under a post that was cut, and a sign-in prompt where a post's
  replies or a profile's list of followers would be. The app does not ask the
  server for those.
- **Where nothing new may be public**, "Public" is not offered.
- **Site Settings › Privacy**, for super administrators, shows the two
  settings as the server has them.

## Where it is enforced, and how it is tested

Each rule lives in one place, with tests that run without a database.

| Rule | Where | Tests |
| --- | --- | --- |
| Reading the two settings, and treating a bad value as the strictest | `anahita-libs/policy` | `policy_test.go` |
| Which audiences a post may have | `permissions.MediumAudiences` in `anahita-libs/cmd/api/permissions/audience.go` | `audience_test.go` |
| Who can see a post, a profile or a group | `SetMediumAccessQuery` and `SetActorAccessQuery` in `graph-grpc-service/data/repositories/` (`access_medium.go`, `access_actor.go`, `access.go`) | `access_test.go` |
| Refusing a visitor on a members-only site | `SetViewer` in `anahita-libs/middleware/oauth.go` | `members_only_test.go`, `signed_in_test.go` |
| Cutting what a visitor is sent on a preview site | `anahita-libs/middleware/preview.go` | `preview_test.go` |
| Making everything public members-only | `nodes_public_access.go` in graph-grpc-service, which records it in table `site_actions`; the endpoint in auth-service (`public_content.go`) | `nodes_public_access_test.go`, `public_content_test.go` |
| What NodeInfo reports | auth-service | `node_info_privacy_test.go` |
| Sending nothing out on a preview or members-only site | `siteFederates` in `activitypub-service/cmd/api/handlers/handlers.go`; `canWrite` in `atproto-service/cmd/api/handlers/settings.go` | `handlers_test.go` (activitypub-service), `out_test.go` (atproto-service) |

Three tests are guards. They read the code and fail when something new goes
around a rule:

- **`TestEveryRouteNamesItsViewer`** reads every service's routes and fails if
  one has neither `Authorize` nor `SetViewer` in front of it. Such a route
  would serve visitors on a members-only site.
- **`TestTheRoutesAPreviewLeavesOutSaySo`** checks that replies and the
  follower lists are marked as left out of a preview.
- **`TestEveryHandlerThatSetsAnAudienceChecksIt`** checks that every handler
  that creates a post, changes its audience, changes a profile's access or
  creates an account asks the rule first.

`signed_in_test.go` is the other half of both restrictions: somebody who is
signed in is never refused and never sent a preview.

The services' own reads, with nobody signed in, are not restricted: writing a
notification, extracting hashtags, creating an account. They do not come
through `SetViewer`.

## Federation

- Only `public` posts of people who chose to take part are sent to other
  servers: on the fediverse, people who turned federation on; on Bluesky,
  people who connected their own account. Only people whose profile
  anybody may see take part at all.
- With `MIN_CONTENT_ACCESS=registered`, nothing new is public, so nothing
  new is sent.
- With `SITE_READ_ACCESS` set to `preview` or `registered`, the installation
  sends nothing to the fediverse or to Bluesky, and connecting a Bluesky
  account is not offered. On a preview site, sending out would give away
  the whole of what the preview holds back. Reading from elsewhere still
  works.
- What came from elsewhere is copied in, so a reader's browser never asks
  another server for it. See [Federation](federation.md#who-owns-what).
