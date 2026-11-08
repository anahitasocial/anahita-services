# Testing and scaling Anahita (roadmap row 141)

Asked for by the user on 2026-11-07, after asking what Mastodon's and
Pixelfed's monoliths do at scale that Anahita does not: "write a plan for
testing and scaling up anahita". Planned the same day. Not started, and
**kept for later on 2026-11-08, at the user's word:** Anahita as it is can
take tens of thousands, and no site is likely to pass several thousand
people for a while; it is last in the roadmap's order.

## Contents

1. [Why](#why)
2. [The target](#the-target)
3. [What there is now](#what-there-is-now)
4. [What Mastodon and Pixelfed do](#what-mastodon-and-pixelfed-do)
5. [Phase 1: seeing inside](#phase-1-seeing-inside)
6. [Phase 2: the load test](#phase-2-the-load-test)
7. [Phase 3: more than one copy of every service](#phase-3-more-than-one-copy-of-every-service)
8. [Phase 4: a writer and readers](#phase-4-a-writer-and-readers)
9. [Phase 5: search on an index](#phase-5-search-on-an-index)
10. [Phase 6: only if the test calls for it](#phase-6-only-if-the-test-calls-for-it)
11. [Phase 7: what was learned](#phase-7-what-was-learned)
12. [Out of scope](#out-of-scope)
13. [Documentation](#documentation)

## Why

Anahita is built of services so that each can be given what it needs: video
more workers, search more memory, without the rest. Whether one site can
serve a hundred thousand people has never been measured. Today's ceiling is
estimated, not known, at several thousand to twenty thousand people active
in a month. This plan measures first, then changes what the measurement
shows, in the order it shows it, and writes down what it learned for any
system built the same way: Go services, MySQL, RabbitMQ, Redis, Kubernetes.

## The target

Set by the user on 2026-11-03: **100,000 active people on one site, with
room toward a few hundred thousand.** Millions on one site is not a goal: the
fediverse grows by many sites, not one large one.

An active person, for the test, does in an hour of use what the model in
[Phase 2](#phase-2-the-load-test) says, and is online for a share of the day;
how many are online at the busiest hour is what the test is sized by.

## What there is now

Checked against the code on 2026-11-07.

1. **The event bus allows one copy of a service.** Each service's queue is
   named by the server, not durable and exclusive
   (`anahita-libs/event/event.go`): a second copy would receive every event
   again, and an event on its way is lost when a service restarts. Eleven
   services listen to it.
2. **Every deployment is one copy** (`replicas: 1` throughout `k8s/base`).
3. **The home feed is worked out when it is read,** joining who somebody
   follows with what they posted.
4. **Search reads every row:** `LIKE '%…%'` on names, handles and the text
   of posts (`graph-grpc-service/data/repositories/search.go`).
5. **One MySQL, one address.** Each service has one `DSN`, and its own pool
   (`anahita-libs/data/db.go`, `MAX_DB_CONNECTIONS`), so the connections to
   MySQL grow with every copy of every service.
6. **Redis holds sessions** and a few lists; posts and profiles are not
   cached.
7. **The notification bell asks** every so often; nothing is pushed.
8. **Nothing is measured:** no metrics from the services, no traces.
9. **Already as it should be:** what is sent to other servers waits in a
   table that any number of workers take from (`FOR UPDATE SKIP LOCKED`,
   `anahita-libs/federation/queue.go`); copies from other networks are
   pruned by age; each service can be given its own copies and resources.

## What Mastodon and Pixelfed do

Both are monoliths, and neither scales by being one. They scale by:

1. **Many copies of one process,** any of which answers any request.
2. **Durable job queues on Redis** (Sidekiq, Laravel Horizon), one queue for
   each kind of work, each with its own workers; a job survives a restart
   and is tried again.
3. **Home timelines written ahead:** a new post's ID is put into a Redis list
   for each follower on the site, and reading the home feed reads the list.
4. **Caches** of posts, profiles and counts, so that most reads never reach
   the database.
5. **Read replicas** of the database, and a pooler in front of it.
6. **A search engine** (Elasticsearch or OpenSearch for Mastodon).
7. **A streaming server** for what changes while a page is open, and a CDN
   for media.

## Phase 1: seeing inside

Nothing can be tuned that cannot be seen.

1. **Metrics from every service:** Prometheus's format at `/metrics` on a
   port of its own, not through the gateway: requests by route, how long
   they take, errors; database pool in use and waiting; events received
   and how long they take; the delivery queue's depth.
2. **From what they stand on:** MySQL's slow query log and Performance
   Schema, RabbitMQ's queue depths, Redis's memory and hits, each pod's
   processor and memory.
3. **Grafana** to read them, as a component an overlay may add
   (`k8s/components/monitoring`), so that an operator has it too.
4. About two sessions.

## Phase 2: the load test

1. **A population made for it:** a generator writes a site of 100,000
   people (and a second of 300,000) into a copy of the schema: follows as
   real networks have them, most following a few hundred and a few
   followed by tens of thousands; posts, replies, likes and photos over two
   years; groups and events; people elsewhere followed. Made from the
   Star Trek names the rest of the tests use, with numbers after them.
2. **What people do, as k6 scripts** (Grafana k6, open source): opening the
   home feed and scrolling, opening a post and its replies, a profile, the
   notifications and the bell's count, searching, posting a note and a
   photo, liking, following. Each person's hour of use is a mix of these,
   read many times for each write.
3. **Other servers, too:** a script plays a busy fediverse, sending the
   inbox posts, likes and follows at a rate that grows, and Bluesky's
   firehose replayed faster than it came.
4. **In steps:** 1,000, 10,000, 50,000, 100,000 and 300,000 people in an
   hour, each step held until it is steady, to find **where it breaks
   first**: the first route past its limit (95% of answers within 300 ms for
   a read, a second for a write), or errors past 0.1%.
5. **Where:** a cluster made for the test and taken down after, in
   ca-central-1: EKS with Aurora MySQL, ElastiCache and Amazon MQ, sized as a
   large site would be, so that what is learned applies to the managed path;
   and the same population on one k3s server, banu.io's size, to know what
   one server holds. The cost of each run is written down.
6. **Run again after each phase below,** so that each change is shown to
   have helped.
7. About three sessions, and the runs.

## Phase 3: more than one copy of every service

1. **Durable queues with competing consumers:** each service's queue named
   for the service and durable, so that its copies share the work; each
   event handled once, and safely handled twice (idempotent), since a
   message can come again; a dead-letter queue for what fails three times;
   the publisher's confirms on.
2. **Copies by load:** a HorizontalPodAutoscaler for each service the test
   shows needs it, and a PodDisruptionBudget so that an upgrade never takes
   every copy down at once.
3. **Connections counted:** each service's pool sized so that all of them,
   at their most copies, stay under what MySQL allows.
4. About two sessions.

## Phase 4: a writer and readers

Amazon RDS for MySQL, and Aurora, give one address that writes and reads,
and addresses that only read, Aurora's one reader address spreading reads
over all its replicas (the user, 2026-11-07). A site on one server has the
same with a MySQL replica and ProxySQL.

1. **Two addresses:** `DSN` writes, `DSN_READ` reads, and a service without
   `DSN_READ` uses `DSN` for both, so that nothing changes for a small site.
   Two pools in `anahita-libs/data`.
2. **Reads go to a replica, except** what must see a write just made: the
   answer to the write itself, and what the same person reads in the next
   few seconds, which a replica may not have yet. A request that wrote reads
   from the writer to its end.
3. **On RDS:** RDS Proxy, or Aurora's own endpoints, in front of both, so
   that copies of services coming and going do not each open new
   connections.
4. About two sessions.

## Phase 5: search on an index

1. **MySQL's own full-text index** on names, handles and post text, with
   the ngram parser for languages written without spaces; the same ranking
   on what it finds.
2. **Behind an interface,** so that a large site may use Meilisearch or
   OpenSearch, kept up to date from the event bus, without the rest
   knowing.
3. About one session for the first, two more for the second if the test
   asks for it.

## Phase 6: only if the test calls for it

Each is planned on its own when the test shows it is the next limit.

1. **Home feeds written ahead,** as Mastodon's: a new post's ID put into a
   Redis list for each follower on the site, the latest few hundred kept;
   the busiest profiles' posts added when the feed is read, not written into
   a hundred thousand lists; the feed worked out as now for somebody whose
   list has gone, as after a long absence. About three to four sessions.
2. **A cache of finished posts and profiles** in Redis, emptied by the event
   that changes them.
3. **The bell told, not asking:** server-sent events from a small service,
   and push for the mobile app (row 138), in place of asking every so often.
4. **A CDN** in front of photos and videos.

## Phase 7: what was learned

1. **`docs/scaling.md`:** what each size of site needs, measured, not
   guessed: how many copies of what, how large a database, which phases it
   needs, and what it costs a month, on one server and on EKS. The recipes'
   sizes (`k8s/sizes/{small,medium,large}`, row 30) are made from it.
2. **Written for any system built this way,** not only Anahita: what broke
   first, what fixed it, and the numbers, so that the findings can be used
   where the same architecture serves many people.
3. **The load test kept,** to be run before each Birth release.

## Out of scope

1. Millions of people on one site.
2. More than one region, and a site that survives a region going down.
3. Changing database: MySQL stays.

## Documentation

1. `docs/scaling.md`, new: the findings.
2. `docs/deploying.md`: the sizes, and `DSN_READ`.
3. `docs/configuration.md`: `DSN_READ` and every new setting.
4. `docs/architecture.md`: the event bus's queues, the metrics.
5. `CHANGELOG.md`, for each phase.
