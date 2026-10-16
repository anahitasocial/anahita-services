# The API reference: every request and answer, made from the code

*Roadmap item 38, planned 2026-10-11. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context

Nothing says what the API takes and sends. The pages under `docs/` explain how a feature works and show a shape here and there (seven of them have a JSON example), but somebody writing a client, a mobile app or a plugin has to read Go to learn a field's name.

**What exists (counted 2026-10-11):**
- **About 308 routes** in 20 services, each registered in a `routes.go` (`hashtags.GET("/:alias", r.handler.Hashtag.Read, r.setViewer)`).
- **What a route takes is a struct,** bound and validated by its handler: `c.Bind(req)`, then `c.Validate(req)`. Its tags already say everything a reference needs: where a value comes from (`json`, `query`, `param`), and what it may be (`validate:"required,min=1"`, `oneof=asc desc`). 48 of them are shared in `anahita-libs/requests`; each service has its own beside its handlers.
- **What a route sends is a struct about two times in three** (`anahita-libs/responses`, and each service's `responses`), with `json` tags and usually a comment on each field. **The third time it is a map written where it is sent** (99 of 313 `c.JSON` calls), mostly errors and small acknowledgements.
- **Nothing connects a route to its two structs** except the body of its handler.
- **No OpenAPI document, no generator, no annotations.**
- **The wire is snake_case.** The web app turns answers to camelCase and bodies back as it sends them (`src/api`), so its own code is no guide to the wire.

## Who reads it (user, 2026-10-11)

People who run their own Anahita site and build a service or a client against it, and ourselves when `anahita-mobile` is written. For the mobile app this is not a convenience: it is the contract the app is built to, and a generated client for it is the likeliest first use. That makes phases A to C worth doing **before the mobile app is begun**.

## Decisions

- **The reference is generated from the code, and a test fails when it is out of date.** The check of the docs on 2026-10-11 found plans listed as not started that were finished, make targets that no longer existed and a test target that had been broken for a week, in pages written with care. Three hundred routes written out by hand would not last a milestone.
- **OpenAPI 3.1 is the format.** It is what every tool reads: reference pages, client generators, test tools, and an agent asked to call the API.
- **One file for each service, and one of them all together,** under `docs/api/`. Committed, so that a change to a shape shows in review as a change to the reference.
- **The prose pages stay.** The reference says what a field is called and what it may hold; `permissions.md` says who may do what and why. Each links to the other and neither repeats it.

## Design

### Saying what a route is

A route is described where it is registered, in a few lines beside it, by a small package `anahita-libs/apidoc`:

```go
hashtags.GET("/:alias", r.handler.Hashtag.Read, r.setViewer)
r.doc.GET("/hashtags/:alias").
    Summary("One hashtag").
    Viewer(apidoc.Anyone).
    Takes(requests.HashtagRead{}).
    Sends(200, responses.Hashtag{}).
    Refuses(404, "no such hashtag, or not one the viewer may see")
```

- **`Viewer`** is one of `Anyone` (read without signing in; the viewer still decides what is seen), `SignedIn`, `Admin`, `SuperAdmin`, with the OAuth scope where one is asked for (`docs/oauth.md`), and `StepUp` where the person must just have proved who they are.
- **`Takes`** reads the struct by reflection: `param` tags become path parameters, `query` tags query parameters, `json` tags the body, and `validate` tags what each may be (`required`, `min`, `max`, `oneof` as an enum, `omitempty`).
- **`Sends`** does the same for an answer. Field comments are read from the source (`go/packages`), so the comment a field already has is its description. No second wording to keep.
- **Refusals** are listed by status with the reason in words. The one shape every refusal has is described once.

### Three checks, in `make test`

1. **Every route has a description and every description a route.** The test starts the service's router without its dependencies, reads `e.Routes()`, and compares. A route added without one fails.
2. **The committed files are what the generator makes now.** `make api_docs` writes them; the test makes them again and compares.
3. **Answers are what the reference says** (last phase): handler tests that already call a route check what came back against its schema, so a field renamed in a struct and missed in a hand-built map is caught.

### Maps become structs

An answer written as a map cannot be described. As each service is done, its maps become structs: one shared shape for a refusal (`{ "error": …, "code": … }`, as most already are), one for "done", and a struct of its own for anything else. Shapes on the wire do not change; where two routes say the same thing two ways, that is recorded and left for a later, deliberate change.

### What is written by hand: `docs/api.md`

One page for what is true of every route, which no generator can know:

- the wire is snake_case; ids are numbers and, for people, also usernames; dates are RFC 3339 in UTC and their names end in `_at`;
- how a list is paged (`limit`, `offset`, and the cursor the feeds use) and the envelope a list comes in;
- the `authorized` object that says what the viewer may do with what was sent;
- how to sign in as a client (a pointer to `oauth.md`), and what `401`, `403`, `404`, `409`, `422` and `429` mean here: above all that **`404` is also the answer for something the viewer may not see**;
- what is optional: the services an installation may not run, and how a client tells (NodeInfo).

### Reading it

- **The files themselves,** for tools and agents.
- **A page for people,** made from the combined file as static HTML at build time, with its script kept in the repository. Nothing fetched from somebody else's server when the page is opened (item 9's rule).
- **Through the docs MCP server** ([docs-for-agents.md](docs-for-agents.md)): asking for one route's shapes without loading three hundred.

## Considered and not chosen

- **Annotations in comments (swaggo).** Nothing checks a comment against the code beside it, so it drifts exactly as hand-written pages do; and it writes OpenAPI 2.
- **Writing the YAML by hand.** See Decisions.
- **Reading the handlers' source to find what each binds and sends.** Works for the plain ones and guesses at the rest; a wrong reference is worse than none.
- **Generating typed clients for the web app.** Possible once this exists. Not part of it. For `anahita-mobile` it is likely the first thing done with the reference.

## Phases and estimate

| Phase | Work | Time |
| --- | --- | --- |
| A | `anahita-libs/apidoc` and the generator; one small service through all of it (`hashtag-service`), with checks 1 and 2 | 5–6 h |
| B | `docs/api.md`; the shared shapes (`anahita-libs/requests`, `responses`) described once | 2–3 h |
| C | The other 19 services, in the order of the roadmap's milestones, maps to structs as each is done. Federation's services are described as they are built | 16–20 h |
| D | The page for people; links from each prose page to its routes | 2–3 h |
| E | Check 3, answers against the reference, in the handler tests there are | 3–4 h |
| | **Total** | **about 28–36 h, 7–9 sessions** |

A and B are worth having alone: they settle the form, and every service after is the same hour's work.

## Verification

- A route added to `routes.go` without a description fails `make test`, by name.
- A field renamed in a response struct changes `docs/api/` in the same commit, or fails.
- The combined file passes an OpenAPI 3.1 validator.
- A client made from the file by a generator signs in on a dev stack, reads a profile and posts a note.
- The page for people opens with the network off.

## Dependencies

- **None hard.** Better after item 27 (the clean schema) is decided, not necessarily done, since neither changes the wire.
- **Federation (item 8):** `activitypub-service` and `atproto-service` should be described as they are written, which needs phase A first.
- **[docs-for-agents.md](docs-for-agents.md)** reads these files for its route lookups.

## Documentation

- **New services page `api.md`**, and `docs/api/`.
- **services `getting-started.md`**: `make api_docs`, and what the three checks mean when they fail.
- **web `architecture.md`**, *Talking to the API*: a pointer to the reference, and that it describes the wire and not the camelCase the app sees.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-11 |
| Phases A and B | done 2026-10-15, before Pulse (row 38) at the user's word, so that Pulse's routes are described as they are written |
| Phase C | done 2026-10-15: all 19 services with routes are described, 344 routes in all; export-service's routes moved from its `main.go` to a `routes.go`, where the members-only guard reads them |
| Phases D and E | to do: the page for people, answers checked against the reference |

### What phases A and B did, and how it differs from the design above

- **`anahita-libs/apidoc` registers the route as well as describing it.** `doc.On(e)` gives a Router whose `GET`, `POST` and the rest register with echo and keep the description, so a path is written once (`hashtags.GET("/:alias", …).Summary(…).Viewer(…).Takes(…).Sends(…)`). Check 1 still compares echo's routes with the descriptions, which catches a route registered past it.
- **JSON, not YAML:** `docs/api/<service>.json` and `docs/api/anahita.json`. anahita-libs has no YAML library and `go mod tidy` is not run there; OpenAPI 3.1 reads JSON as well.
- **The checks are tests:** `cmd/api/apidoc_test.go` beside a service's routes (checks 1 and 2), and `TestTheCombinedReference` in anahita-libs for `anahita.json`. `make api_docs` runs them with `APIDOC_UPDATE=1`, which writes instead of comparing. A service is added to the target when it is described.
- **Field comments are read with `go/build` and `go/parser`** (no `go/packages`, which is not a dependency), skipping `_test.go`.
- **A list's answer is `responses.List[T]`**, replacing the anonymous `Response` structs handlers declared; the wire is the same. A generic or unnamed struct is written out where it is used; a named one of anahita-libs is a component by its name (`NodeItem`), one of a service by its module (`hashtag.HashtagNodeItem`).
- **One name for one place in a path:** echo allows `/hashtags/:alias` and `/hashtags/:id` side by side, OpenAPI does not. The first route's name is used, and the other's parameter says "Here, the `id`."
- **400, 401 and 403 are added** where a route of its kind can answer them, unless it lists its own.
- **`docs/api.md` corrects the plan on 404:** reading a post the viewer may not see answers `403`, and a removed one `410`; it is not always `404`. Refusals are listed per route.
- **The shared shapes are described once:** the fields of `anahita-libs/responses` used by the reference carry comments now (109 had none; what is left are self-evident address parts).
- **Checked with Redocly's linter:** no errors of structure. It flags `/hashtags/` for its trailing slash, which is the route's real address, and the 501-only route for having no 2xx or 4xx answer.
- **`PATCH /hashtags/:alias` answered 501 whoever asks.** Removed on 2026-10-15 at the user's word, with the unused `Add` beside it: nothing called it, and a hashtag is made from the words of posts, so changing one here would disagree with them. `NotBuilt` stays in apidoc for a route of the kind.

### Found while describing, and put right

Each was found by describing a service, and put right on 2026-10-15 at the user's word ("clearly inconsistencies in the code"), with the web app where it had to change:

- **Setting an avatar or a cover** answered with Go's field names (`Url`, `Size`, `Width`, `Height`); it answers with `url` and `size` as every other picture is sent (`responses.NewAvatar`). The web app reads it camel-cased, so it is unchanged.
- **A notification's own fields were camelCase** (`createdAt`, `updatedAt`, `isRead`); they are `created_at`, `updated_at` and `is_read`. The web app reads them camel-cased, so it is unchanged.
- **`DELETE /notifications/subscriptions/:target_id` answered `201 Created`**; it answers `200`.
- **Abuse cases and refused servers were paged by `offset`**; they take `start`, as every other list, and the web app sends it.
- **Liking, unliking or listing the likes of something that is not there answered 500**; it answers 404.
- **`PATCH /hashtags/:alias` answered 501** and is removed (above).
- **`composed`** was taken by every post and read by nothing; it is gone from the requests and the web app.
- **Approving a follow request answered `202 Accepted`**, though it is done before the answer; it answers `200`.
- **A group's or an event's "let people ask to follow" was never saved:** its request read `allowFollowRequest`, and the web app sends `allow_follow_request`, as a person's request reads. It reads `allow_follow_request` now.
- **Changing a profile's features took its id from the body**, though the path names it; the path is now the one source, and the web app no longer sends it twice.
- **Refusals said why under two names**, `message` in some services and `description` in others; actor-service says `description` now, as `responses.Refusal` does.
- **Handlers that answered 501 and were never routed** (a leader's `Add` and `Delete`, a hashtag's `Add`) are removed.
- **auth-service said why in four ways:** a bare JSON string (agreements, invitations), `message`, `description`, and `error_description` on routes that are not OAuth's. Each says `error` and `description` now, as `responses.Refusal` does; only `/oauth/token`, `/revoke`, `/introspect` and `/userinfo` keep RFC 6749's `error_description`. The web app read `description` on the OAuth-client and signing-key screens and found nothing, and read an invitation's refusal as a string; it reads `description` now, and says the server's words when somebody has too many invitations waiting.
- **A missing step-up was a bare 403** when changing a password, an email address or a username, and when turning two-factor authentication off; it says `step_up_required` there too, as everywhere else. The TOTP screen tells the two 403s apart by it.
- **The accounts list, the invitations and the signup requests were paged by `offset`**; they take `start`, and the web app sends it.
- **Signing keys were answered as stored**, in camelCase (`keyId`, `expiresAt`); they are `key_id`, `expires_at` and `created_at` now (`responses.OAuthSigningKeyItem`). The web app reads them camel-cased, so it is unchanged.
- **Taking an invitation back answered the stored row** (`startedAt`, `isUsed`, the recipient as `email`); it answers the invitation, as sending one does. An invitation's id was a string, every other id a number; it is a number now.
- **A wrong password answered `406`** when setting up two-factor authentication, and **a wrong passcode `401`** when getting recovery codes; they answer `401` and `403`, as `/reauth` and `/totp/verify` do. The QR code is `image/png`, not `image/x-png`. Setting up took a `token` it never read; it is gone.
- **The consent page's form answered JSON** to the browser when it could not go on; it shows the error page, as the other pages do.
- **The passcode step of signing in read the whole sign-in form**, password and all, for its two fields; it reads `OAuthLoginTotp`.
- **export-service's refusals were maps** and its list a map; they are `responses.Refusal`, `TooSoon` and `ExportList`, the wire unchanged.
- **A struct of a handler's own was the answer** in several services (an anonymous `Response`, or a map): replaced by `responses.List[T]`, `responses.Unpaged[T]` or a named type as each service was described, the wire unchanged.

