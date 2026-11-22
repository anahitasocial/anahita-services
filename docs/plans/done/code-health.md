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

Where each item stands: ✅ done, ⬜ still to do, ➖ left out, ➡️ moved to
another row. **Done 2026-11-22.** What each one came to is in the notes after the table.

| Item | What | Where it stands |
| --- | --- | --- |
| 1 | An invitation's session that nothing could revoke | ✅ 2026-11-22 |
| 2 | The signup event nothing heard | ✅ 2026-11-22, removed |
| 3 | An empty cookie key reported twice | ✅ 2026-11-22 |
| 4 | Every list's SQL written to the log | ✅ 2026-11-22 |
| 5 | The hashtag pattern compiled for every post | ✅ 2026-11-22 |
| 6 | Feed defaults read from disk on every request | ✅ 2026-11-22 |
| 7 | A post's relationships asked for twice | ✅ 2026-11-22 |
| 8 | like-service's gRPC clients built twice | ✅ 2026-11-22 |
| 9 | A like's eight to ten calls | ✅ 2026-11-22 |
| 10 | Quoted posts read one at a time | ✅ 2026-11-22 |
| 11 | An event list's hosts read one at a time | ✅ 2026-11-22 |
| 12 | Notifications deleted one at a time; mail templates parsed each time | ✅ 2026-11-22 |
| 13 | auth-service's old OAuth path | ✅ 2026-11-22, removed |
| 14 | Handler leftovers and functions nothing calls | ✅ 2026-11-22, removed |
| 15 | Commented-out access helpers, empty files, cosmetic | ✅ 2026-11-22, removed |
| 16 | The Following feed counted on every page | ✅ 2026-11-22, `has_more` (v1.7.0) |
| 17 | A profile's Liked and Tagged tabs | ✅ 2026-11-22, measured quick: nothing to do |
| 18 | Search on a full-text index | ➡️ row 141, scaling |
| 19 | Trending cached | ➖ 32 to 82 ms, not worth a cache |
| 20 | Federation fan-out | ✅ 2026-11-22 |
| 21 | Twin indexes on `nodes` and `edges` | ✅ 2026-11-22, migration `20261122000025` |
| 22 | The post-to-gRPC converter kept twice | ✅ 2026-11-22 |
| 23 | Where a kind of post lives, written in six places | ✅ 2026-11-22, `posts/kinds.go` |
| 24a | Settings handlers of text, photo and video posts | ✅ 2026-11-22, `handlers.PostSettings` |
| 24b | Configuration in every service | ✅ 2026-11-22, `settings.Common` |
| 24c | Start-up, with a graceful stop | ✅ 2026-11-22, `anahita-libs/server` |
| 24d | One helper for reading and checking a request | ✅ 2026-11-22, `handlers.Bound` |
| 24e | The access-context builder | ✅ 2026-11-22, `handlers.AccessContext` |
| 24f | hashtag-service's and location-service's shared handlers | ✅ 2026-11-22, `handlers.TaggedWith` |
| 24g | Event names parsed once | ✅ 2026-11-22, `EventPayload.Action` |
| 24h | The reconnecting publisher's copies | ✅ 2026-11-22, `event.NewLazyPublisher` |
| 24i | Pagination in one shape, with its cap | ✅ 2026-11-22, the cap: `requests.PageSize`; the shapes stay |
| 24j | The smaller helpers kept two or three times | ✅ 2026-11-22, the two worth one copy |
| 25 | Splitting what mixes several jobs | ✅ 2026-11-22 |

### Notes

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
5. **Item 20, the federation fan-out, done 2026-11-22.** A post is
   marshalled once and queued with `Queue.EnqueueMany`, five hundred rows a
   statement (on a throwaway MySQL: 100 servers 258 to 29 ms, 1,000 2.6 s
   to 92 ms, 5,000 14 s to 0.57 s); the sender's UUID is kept a minute
   (`senderUUID`); tagged people are read with `BatchRead`, and the names
   a post mentions with `BatchReadByAliases`, a read for each kind of
   profile, the single read kept for a name not read ahead; the inbox's link
   card is read after the answer (`addCard`), four at a time, and not put
   on a post changed since. Measured on a test database rather than a copy
   of banu.io, whose servers' followers are few. Each has a test.
6. **Item 22, the post-to-gRPC converter, done 2026-11-22.** One converter,
   `services.NodeToGrpc` in `anahita-libs`, which graph-grpc-service now
   uses too; the two had drifted, the clients' never sending the UUID or
   when an actor is to be deleted, the graph's never whether the viewer has
   blocked them. Nothing read the missing fields across, so nothing people
   see changes. `TestEveryFieldCrossesBothWays` fills every field of the
   proto and fails, by name, on one that does not cross both ways, apart
   from three left out on purpose: when a person last visited, an actor's
   features (set by graph-grpc-service on what it sends) and whether the
   viewer quoted a post (set by nothing). With it, a profile or cover
   photo's name with no extension is no file rather than a crash.
7. **Item 23, the kinds of post, done 2026-11-22.** One table,
   `anahita-libs/posts/kinds.go`: each kind's type, word, page and whether
   it goes out. It was written in six places: the quote's link, Bluesky's
   card and the sweep (atproto-service), the fediverse's pages and sweep
   (activitypub-service), embeds, web-shell and custom feeds. Bluesky's had
   no topics, which never go there; the fediverse's set a post's page by
   kind only once its file or pictures were read, else `/notes/`; the
   embeds had no audio, so an embedded audio post was not cited. Each now reads the table, and
   `TestEveryKindOfPostHasOnePageAndOneAnswer` guards it.
8. **Item 24, first part, the settings handlers, done 2026-11-22.** Who may
   see a post, its replies' switch and who may reply, who may quote it, its
   content warning and its pin: `PostSettings` in
   `anahita-libs/handlers/post_settings.go`, with the access-context
   builder (`AccessContext`). text-service, photo-service and video-service
   each keep a file of one-line methods that give it their clients
   (`post_settings.go`); eighteen files of 1,559 lines went, for 479. Where
   the copies differed, text-service's was kept, the most complete: a
   channel's item refused alike, and a reply's warning judged by its thread
   with its status. Tested once, for every kind
   (`TestAPostsSettingsAreChangedByWhoeverMayEditIt`).
9. **Item 24, second part, configuration, done 2026-11-22.** What every
   service reads alike, `settings.Common` in `anahita-libs/settings`, with
   its getters and `Missing`, which checks the common settings a service
   needs, said as before. All 24 services' `Config` embed it and keep
   their own settings and checks; 2,337 lines of code went, for 340.
   file-grpc-service's `AmqpAddr`, read from nowhere, went with them. Six
   services check in a form of their own and keep it (activitypub, atproto,
   import-export, mail, file-grpc, web-shell).
10. **Item 25 done 2026-11-22.** `getRelations` (graph-grpc-service,
    `socialgraph_rel.go`, 419 lines to 276): a list of the relations, each
    a query, read by `idsOf`, which closes its rows before the next; the
    nine used to stay open until the end, a connection each. Mutes, which
    say more, in `readMutes`. `TestEveryRelationIsReadWhereItHolds` sets
    every relation at once, and passed before and after. atproto-service's
    `out.go` and feed-service's `custom_feed.go` split by what each part
    does, moved as they were. `anahita-libs/utils` is `anahita-libs/images`,
    as its package was called already.
11. **Item 24, third part, start-up, done 2026-11-22.** `anahita-libs/server`:
    `New`, `SignIn`, `Run` and `ServeGRPC`, used by every API service and
    the three gRPC services; web-shell, which serves the web app's pages
    with Go's own server, stays as it is. A graceful stop, which no service
    had: told to stop, a service finishes what is in flight, for up to 20
    seconds (`TestAServiceFinishesWhatItIsAnsweringBeforeItStops`); tried on
    the dev stack with hashtag-service, auth-service and graph-grpc-service.
12. **Item 24, fourth part, reading a request, done 2026-11-22.**
    `handlers.Bound` in `anahita-libs`: a request read and checked, logging
    why when it is not, in 129 places where the same ten lines were
    written out, in twelve services (876 lines went, for 169). The other
    108 that read a request answer or log in a way of their own, and keep
    it.
13. **Item 24, fifth part, hashtag and location services, done
    2026-11-22.** The two had grown apart since the review, each helper of
    its own kind; what they still shared was the list of the posts a tag is
    on, now `handlers.TaggedWith` in `anahita-libs`, with a test of its own
    (`TestThePostsATagIsOnAreListedAlike`). About 110 lines went, not the
    250 first thought.
14. **Item 24, sixth part, event names, done 2026-11-22.** `Action()` on
    `EventPayload`, with a constant for each action, reads what an event
    says was done from its name once; the 50 or so tests of the name by
    `strings.Contains`, `HasSuffix` and by splitting it in eight services
    read it instead. actor-service's person events took the fifth part of
    a split name without asking whether there was one, and would have
    stopped on a shorter name.
15. **Item 24, seventh part, the publisher's copies, done 2026-11-22.**
    activitypub-service and atproto-service each kept an `announcer`, a
    publisher that connects when it first has something to say and again
    after the bus has gone. The shared publisher reconnects already; it
    gains `NewLazyPublisher` for the first part, and both copies went. The
    copies' first connection waited up to 30 seconds for the bus; the
    shared one tries once, and the next event tries again, as it does
    after a restart.
16. **Item 24, eighth part, pagination, done 2026-11-22.** The cap was the
    part that mattered: nine of graph-grpc-service's list queries read as
    many rows as asked, search among them, open to visitors. They now read
    a page and one more at most (`pageLimit`, 101), the one more for
    whoever asks whether there is another page; and 21 list handlers in
    eleven services take the page to 100 at most, 20 when none is asked
    (`requests.PageSize`), and say so. On the dev stack a search asked for
    100,000 sent 100 of 1,918. The answers' shapes stay as they are: a
    custom feed's cursor and the Feeds page's `next_start` are what the web
    app reads, and one shape would change the API for nothing people see.
    Edges, read by the services alone, keep their 0 for all.
17. **Item 24, last part, the smaller helpers, done 2026-11-22.** Of the
    functions kept with the same body in two services or more (26, 462
    lines), two were worth one copy: the check every sort column passes
    before it goes into a query (`data.SafeOrderColumn`, auth-service and
    graph-grpc-service, each keeping its tests of its own lists), and
    cutting text by characters (`federation.CutRunes`, both federation
    services). The rest are a service's own: constructors, `MustValidate`
    around its own checks, methods of its own handler, or alike by chance;
    one copy of those would take more lines to connect than it saves.
18. **Row 170 done 2026-11-22**, the plan moved to `docs/plans/done/`.

