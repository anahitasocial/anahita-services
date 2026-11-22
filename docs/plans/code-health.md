# Code health (roadmap row 170)

A review of anahita-services, asked for by the user on 2026-11-22: what
needs refactoring, where the code repeats itself or does more work than it
needs to, and what is dead. Its findings, numbered as they were reported,
are the work below, in five phases. The user chose the order: the bugs,
then the quick wins for speed, then the rest as rows of their own.

How it was found: `deadcode` (from each service's `main`, and for
`anahita-libs` from every service at once) and `staticcheck` across every
module, then reading the code. Each finding here was checked in the code;
what is said of query plans (phase 4) is read from the indexes in
`migration-job/schema/init.sql`, not measured, and is measured before
anything changes.

## Phase 1: bugs

1. **Accepting an invitation signs in with a session nothing can revoke.**
   `auth-service/cmd/api/handlers/invite_accept.go` writes the IdP session
   itself and throws away its key, so the session is never indexed
   (`indexIdPSession`): a password reset or the account's deletion leaves
   it working until it expires. Sign-in (`oauth_login.go`,
   `writeViewerSession`) indexes it, marks the sign-in fresh and records
   the visit. Both now go through one helper, as the code's own TODO asks.
2. **The signup event goes nowhere.** `entity.auth-service.user.signup.v1`
   is published when somebody joins by invitation, and was by the old
   self-serve signup, and nothing listens for it. The unused publisher goes
   with phase 3; the invitation's is kept only if something uses it.
3. **An empty secure-cookie key is reported twice** by every service's
   configuration, as missing and as the wrong size. Once is enough.

## Phase 2: speed, the quick wins

4. **Every list query writes its whole SQL to the log**
   (`graph-grpc-service/data/repositories/helpers.go`, `FetchNodes`): on
   every feed, thread and search, a large write under the log's lock.
5. **The hashtag pattern is compiled for every post read**
   (`anahita-libs/data/entities/node_hashtag.go`), its owner, parent and
   author included; mentions read the site's address from the environment
   each time (`node_mention.go`). Both once, at start.
6. **Feed defaults are read from disk on every feed request**
   (`feed-service`, `feed_actor.go` and `feed_leaders.go`): once, at start.
7. **A post's page asks for the same relationships twice** (text-service,
   `medium.go`: `Read`, then `onAfterFetched`).
8. **like-service builds its gRPC clients twice** at start (`main.go`).
9. **A like makes eight to ten calls one after another**
   (`like-service`, `like.go`), reading its target again whole after the
   like for its counts.
10. **Quoted posts in a feed are read one at a time**
    (`anahita-libs/handlers/quotes.go`): one batch read of those the viewer
    may see.
11. **An event list reads each hosting group one at a time**
    (`actor-service`, `event.go`, `setEventHosts`): one batch read.
12. **Notifications past reading are deleted one row at a time**
    (`notification-service`, `keepReadable`); **mail templates are parsed
    for every email** (`mail-grpc-service`, `helpers.go`).

## Phase 3: dead code

13. auth-service's old OAuth path: authorization requests and CSRF tokens
    (`oauth_authorize.go`), the JWKS and key rotation that read keys from
    the database around KMS (`oauth_jwks.go`), the crypto helpers kept twice
    (`oauth_crypto.go`, `internal/signing/signer_keys.go`), the signing-key
    handler, the token blacklist (the last direct use of go-redis),
    `ValidateRefreshToken`, `Issuer`, the signup session writer.
14. like-service's four handler leftovers; about fifteen functions no
    service calls, in the services and in `anahita-libs`.
15. Seventeen commented-out access helpers in graph-grpc-service; twenty
    empty `config/helpers.go`; values never read and packages imported
    twice (cosmetic).

## Phase 4: speed, measured first

On a copy of banu.io's data, with `EXPLAIN`, before and after:

16. The Following feed: driven by whom the viewer follows, and no count on
    every page (one row more than the page instead, as custom feeds do).
17. A profile's Liked and Tagged tabs, and their counts.
18. Search: a full-text index, and no separate count. Left for row 141
    (scaling, its phase 5).
19. Trending: cached for a minute. Left out: 32 to 82 ms.
20. Federation fan-out: moved to phase 5.
21. The duplicate indexes on `nodes` and `edges`, dropped by a migration.

## Phase 5: repeated code

The federation fan-out, moved here from phase 4 at the user's word, since
the code it touches is reworked here:

20. Federation fan-out: built once, one insert, the sender read once;
    tagged and mentioned people in one batch; the inbox's link card after
    the answer, as its pictures are. Measured on a copy of banu.io first.

Then the repeated code, a few at a time, those whose copies already
disagree first:

22. The post-to-gRPC converter, kept twice and drifted (one never sends the
    UUID, the other whether the viewer is blocked).
23. Where a kind of post lives and whether it federates, written in four
    places that disagree.
24. The settings handlers of text, photo and video posts (about 1,000
    lines); configuration (about 2,000) and start-up (about 700) shared from
    `anahita-libs`, with a graceful shutdown; one helper for reading and
    checking a request; the access-context builder; hashtag and location
    services' shared handlers; event names parsed once; the reconnecting
    publisher's copies; pagination in one shape with its cap; the smaller
    helpers kept two or three times.
25. Splitting what mixes several jobs: `getRelations`, atproto's `out.go`,
    feed-service's `custom_feed.go`; `anahita-libs/utils` named for what it
    is, images.

## Status

1. Planned 2026-11-22.
2. **Phases 1 and 2 done 2026-11-22.** The invitation's session is written by
   `writeViewerSession` (`auth-service`, `invite_accept.go`), tested end to end
   (`invite_accept_session_test.go`); the signup event and both publishers of
   it removed; the cookie key reported once in every service. Quick wins 4 to
   12 as planned; for 4, the 89 lines logging each gRPC request whole went
   with the SQL. Quoted posts are read with `MediaGRPC.BatchRead`, which reads
   as `Read` does; 7 has no test of its own, text-service having no harness
   for `Read`.
3. **Phase 3 done 2026-11-22.** What `deadcode` and `staticcheck` found, in
   every service and `anahita-libs`, removed or moved into the tests that use
   it, and nothing is left but the `policy` setters other packages' tests
   call. With it went go-redis (auth-service) and go-sqlmock
   (`anahita-libs`). Removing `SetPolymorphicAccessQuery`, a gate nothing
   used, found a bug: the gate a hashtag's or a location's page does use
   judged a reply by its own audience, so no reply ever showed there; it now
   judges it by its thread, as the removed one did, and the integration
   test written for that rule tests the gate in use. A test of `CardLink`
   moved to `anahita-libs` with the function, and one of how a place is
   read now reads it through `ReadPlaces`.
4. **Phase 4 done 2026-11-22**, on a copy of banu.io's data (28,536 nodes,
   10,769 edges), timed from a Mac, the median of five:

   | Query | Before | After |
   | --- | --- | --- |
   | Following, its count | 463 ms | not run |
   | Following, a page | 20–24 ms | 15–22 ms |
   | A profile's tabs, page and count | 14–46 ms | unchanged |
   | Search "coffee", its count | 282–307 ms | open |
   | Search "coffee", a page | 175–311 ms | open |
   | Trending hashtags, a day to a month | 35–115 ms | 32–82 ms |

   16: the Following feed is not counted; feed-service asks for one post
   more than the page and answers `has_more`, and the gRPC `Count` it called
   is removed. 17 needs nothing: a profile's tabs are quick. 21: the seven
   twin indexes dropped (`20261122000025`); nodes' and edges' indexes went
   from 61 to 54 MB and nothing read slower. At the user's word: search
   (18) waits for row 141, which plans it, with these figures; trending (19)
   is left out; the fan-out (20) moves to phase 5. The copy of banu.io's
   data was deleted once measured.
