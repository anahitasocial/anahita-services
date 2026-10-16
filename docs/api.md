# The API

What is true of every route of Anahita's API. What each route takes and sends
is in the reference beside this page, [`docs/api/`](api/), made from the code.

## Contents

- [The reference](#the-reference)
- [Reading and writing](#reading-and-writing)
- [Lists](#lists)
- [What the viewer may do](#what-the-viewer-may-do)
- [Signing in](#signing-in)
- [Refusals](#refusals)
- [Services an installation may not run](#services-an-installation-may-not-run)
- [Describing a route](#describing-a-route)

## The reference

[`docs/api/`](api/) holds an [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0)
document for each service whose routes are described (`hashtag-service.json`),
and `anahita.json`, all of them in one. Any OpenAPI tool reads them: a
reference viewer, a client generator, a test tool, an agent asked to call the
API. Services are added as their routes are described; until then, a
service's `cmd/api/routes.go` is where its routes are.

They are made from the code, not written by hand, and `make test` fails when
they are out of date (see [Describing a route](#describing-a-route)). Each
operation says:

- who may call it, in words and as `x-viewer`: `anyone` (signed in or not),
  `signed_in`, `admin` (a site administrator, or a super administrator) or
  `super_admin`, with the OAuth scope its token must carry, and `x-step-up`
  where the person must just have proved who they are again;
- what it takes: values in the path, the query and the body, with what each
  may be. A rule JSON Schema has no word for is kept as it is written in the
  code, as `x-validate` (`hashtag` is a hashtag's own rule);
- what it sends, and what it refuses, with why.

The documents describe the wire, as a client sees it. The web app turns what
it is sent into camelCase and back (`src/api` in anahita-web), so its own code
is no guide to the names.

## Reading and writing

- **Names are snake_case:** `created_at`, `likes_count`.
- **Ids are numbers.** A person is also found by their username
  (`/people/ana`), and `GET /actors/<alias>` says what kind of profile an
  alias is, with its id.
- **Times** are RFC 3339 in UTC, and their names end in `_at`. A date with no
  time ends in `_on`.
- **A field that is empty is often left out** rather than sent empty: the
  reference marks as required only what is always there. A list with nothing
  in it may be `null`.
- **Bodies** are JSON, except where a file is sent, which is
  `multipart/form-data`.

## Lists

A route that lists is paged by two values in the query:

| Value | Means |
| --- | --- |
| `start` | How many to pass over. 0 unless asked |
| `limit` | The most to send. 20 unless asked |

and answers with what is on the page and where it is:

```json
{
  "data": [ … ],
  "pagination": { "offset": 40, "limit": 20, "total": 113 }
}
```

`data` is `null` when the page is empty. A few lists are not paged and send an
array alone (the hashtags used most lately, for one); the reference says which.

**Custom feeds are paged by what came before**, since counting one would mean
reading all of it: `GET /feeds/<id>/items` takes `cursor` instead of `start`,
and answers with `pagination.next_cursor` and `pagination.has_more`, and no
total ([feeds.md](feeds.md)). The home feed and a profile's are paged by
`start`, as other lists are.

## What the viewer may do

A post, a reply or a profile is sent with `authorized`: what the viewer may do
with it, as the server's own rules answer, so that a client offers only what
will work. `edit`, `delete` and `administration` are always there; the rest
are there when they were worked out for this answer, and their absence is not
a no:

| Field | On | Says whether the viewer may |
| --- | --- | --- |
| `comment` | a post, a reply | reply to it |
| `like` | a post, a reply | like it |
| `quote` | a post | quote it |
| `pin` | a post | pin it to the profile it is on |
| `hide` | a reply | hide it from its thread, or show it again |
| `block_from_owner` | a post or a reply in a group or an event | block its writer from there, as the group |
| `download` | a photo or video post | download what it holds |
| `add_follower`, `invite` | a profile | add a follower, invite people to follow it |
| `composers`, `audiences` | a profile; a post | what may be posted there, and who it may be shown to |

Who may do what, and why, is in [permissions.md](permissions.md).

## Signing in

A route for somebody signed in takes an access token from auth-service:
`Authorization: Bearer <token>`. How a person signs in is in
[signing-in.md](signing-in.md); how a client is registered, and the scopes a
token carries, in [oauth.md](oauth.md). A route for anyone also reads a token
when there is one: the viewer decides what is seen.

## Refusals

| Status | Means here |
| --- | --- |
| `400` | The request is not well formed, or a value in it is not one it may be |
| `401` | No token, or one that has expired |
| `403` | The viewer may not do this, or may not see it |
| `404` | There is no such thing |
| `409` | It clashes with something there already: a name that is taken |
| `410` | It was there and has been removed |
| `422` | Well formed, and not something that can be done to it: pinning a reply |
| `429` | Too many requests of this kind lately; try again later |
| `501` | Registered, and not built |

Most refusals have no body. Some say why, as `{ "error": "<reason>" }`, and the
reasons are listed with the route. What one route answers for something the
viewer may not see is not always what another does: each route's refusals are
in the reference.

## Services an installation may not run

Some services are optional: recommendations, abuse reports, link previews,
video, data export, and the two for other networks
([architecture.md](architecture.md)). Where one is not run, its routes are
not there, and a client takes a `404`, or a `501` or above from the gateway,
for there being nothing, not for a failure, as the web app does. What an installation speaks
to other networks is in its NodeInfo (`/.well-known/nodeinfo`,
[federation.md](federation.md)).

## Describing a route

A route is described where it is registered, through `anahita-libs/apidoc`,
which registers it with echo and keeps its description, so its path is
written once:

```go
api := doc.On(e)
hashtags := api.Group("hashtags")
hashtags.GET("/:alias", r.handler.Hashtag.Read, r.setViewer).
    Summary("One hashtag").
    Viewer(apidoc.Anyone).
    Takes(requests.NewHashtagRequest().Read).
    Sends(http.StatusOK, responses.HashtagNodeItem{}).
    Refuses(http.StatusNotFound, "there is no such hashtag")
```

- **`Takes`** is the struct the handler binds, as the handler makes it: its
  `param`, `query`, `header`, `json` and `form` tags say where each value
  comes from, its `validate` tags what it may be, and a value set before
  binding is the default.
- **`Sends`** is what the handler answers with. A list's answer is
  `responses.List[T]`, the one shape every list has.
- **The first paragraph of the comment on each field is its description,**
  read from the source, so nothing is worded twice. As in Go's own
  documentation, the first paragraph says what the field is, for whoever
  calls the API; what follows it (why it is never left out, what broke
  before) is for whoever works on the code, and stays out of the reference.
  A field with no comment has no description.
- `SendsNothing`, `Describe`, `Scope`, `StepUp` and `NotBuilt` say the rest.
  `400`, `401` and `403` are added where a route of its kind can answer them.

A named struct of anahita-libs is described once and named as it is
(`NodeItem`); one of a service is named with its module
(`hashtag.HashtagNodeItem`).

Two checks run in `make test`, from a test beside each described service's
routes (`cmd/api/apidoc_test.go`) and one in anahita-libs for `anahita.json`:

1. **Every route registered is described, and every description is of a
   route.** A route added with echo directly, past apidoc, fails by name.
2. **What is committed in `docs/api/` is what the code makes now.** After
   changing a route, or a struct it takes or sends, run `make api_docs` and
   commit what it writes with the change.

A service is added to `make api_docs` when its routes are described.
