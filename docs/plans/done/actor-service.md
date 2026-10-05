# One actor service: people and groups merged

*Roadmap item 29, raised by the user on 2026-10-05 and done the same day. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md).*

## Context

People and groups were served by `person-service` and `group-service`. The user asked whether they should be one actor service that could also serve events and, later, a profile that stands for a website.

Measured before deciding:

- 12 of the person service's 13 routes had a route of the same shape in the group service.
- Each service was about 2,000 lines, with matching files for access, features, purge and step-up, and identical start-up and configuration apart from one setting.
- What differed: groups add administrators, creation and restore (6 routes); people are read by username.
- The duplication had already cost a bug: in item 5 the group purge lacked an error answer the person purge had.

The events plan had already put events inside group-service as a second kind. One service for every kind makes that the rule.

## Decisions

- **Merge into `actor-service`.** Decided by the user.
- **Addresses do not change.** `/people/` and `/groups/` stay; the gateway sends both to the one service. The web app is untouched.
- **Type names do not change.** `node.actor.person-service.person.v1` and `node.actor.group-service.group.v1` are in every node and edge and in where avatars and covers are stored. They are names now, not addresses. Renaming them would be a large migration for nothing.
- **New kinds are named for the service they are born in:** `node.actor.actor-service.event.v1`.
- **Signing in and accounts stay in auth-service.** The actor service owns profiles, not credentials.

## What was built (2026-10-05)

- **`actor-service`**, a module named `actor`, made by moving person-service and bringing the group handlers into it. `person-service` and `group-service` are gone.
- **Written once now:** the handler environment, the helpers (viewer, presigning, the refusals), step-up, permissions, the gRPC clients, configuration, start-up, the Dockerfile, the compose entry, the Kubernetes manifest, the Makefile target.
- **Still apart, on purpose:** `person*.go` and `group*.go`. Measured with the names evened out, the two main handlers differ in about 600 of 750 lines: a person is a different entity type with different rules. Folding them into one generic handler would have been a rewrite of working, differently behaving code, not a merge. The purge and feature handlers are close (29 and 33 differing lines) and are the ones to fold when a third kind arrives.
- **No database address.** group-service opened a connection it never used, and both required a `DSN`. The merged service reads and writes through graph-grpc only, so `DSN` is gone from its configuration, its compose entry and its manifest.
- **Deployment:** one compose service on port 8082 (8083 is free), one gateway upstream for both paths, `k8s/base/services/actor.yml`, `make actor` in place of `make person` and `make group`.
- **Dependencies were not upgraded.** The module file is the two old ones combined.

**Checked.** Every module in the workspace vets. The tests of actor-service, anahita-libs and auth-service pass, including the guard test that reads the two access handlers by path. On the dev stack, through the gateway: a list of people, a person by username, a list of groups and a group by id answer 200; an edit, a creation and a list of administrators without a token answer 401.

**Not checked.** Anything that needs a signed-in session: editing a profile, creating or deleting a group, purging, administrators. Nothing in a browser. The Kubernetes manifests were edited and not applied.

**To do when deploying.** The images `anahita/person-service` and `anahita/group-service` are no longer built or referenced; `anahita/actor-service` has to be built and pushed, and the two old Deployments and Services deleted from a cluster that has them (removing them from the manifests does not remove them from the cluster).

## Status

| Item | State |
| --- | --- |
| Plan and work | done 2026-10-05, on branch `actor-service` |
