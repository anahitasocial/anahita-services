# Documentation

Documentation for anahita-services, the Go microservices behind Anahita.

## Contents

1. [Getting started](getting-started.md) — running the stack locally
   - [Requirements](getting-started.md#requirements)
   - [Starting](getting-started.md#starting)
   - [First run](getting-started.md#first-run)
   - [Developing](getting-started.md#developing)
   - [Tests](getting-started.md#tests)
   - [Other targets](getting-started.md#other-targets)
2. [Configuration](configuration.md) — every environment variable
   - [Shared](configuration.md#shared)
   - [Client addresses](configuration.md#client-addresses)
   - [Internal calls](configuration.md#internal-calls)
   - [Site identity](configuration.md#site-identity)
   - [Registration](configuration.md#registration)
   - [First super administrator](configuration.md#first-super-administrator)
   - [OAuth signing and encryption](configuration.md#oauth-signing-and-encryption)
   - [Storage](configuration.md#storage)
   - [Places](configuration.md#places)
   - [Mail](configuration.md#mail)
   - [Abuse reports](configuration.md#abuse-reports)
   - [Account cleanup](configuration.md#account-cleanup)
3. [Architecture](architecture.md) — services, routing and the shared library
   - [Components](architecture.md#components)
   - [Services](architecture.md#services)
   - [Routing](architecture.md#routing)
   - [anahita-libs](architecture.md#anahita-libs)
4. [OAuth scopes and clients](oauth.md)
   - [Scopes](oauth.md#scopes)
   - [Seeding clients](oauth.md#seeding-clients)
5. [Registration and agreements](registration.md) — how people join
   - [Registration modes](registration.md#registration-modes)
   - [Email verification](registration.md#email-verification)
   - [Invitations](registration.md#invitations)
   - [The approval queue](registration.md#the-approval-queue)
   - [Protection against bots](registration.md#protection-against-bots)
   - [Terms of Service and Privacy Policy](registration.md#terms-of-service-and-privacy-policy)
   - [NodeInfo](registration.md#nodeinfo)
   - [The first super administrator](registration.md#the-first-super-administrator)
6. [Permissions](permissions.md) — who can post, comment, like and add followers
   - [Access levels](permissions.md#access-levels)
   - [Seeing posts](permissions.md#seeing-posts)
   - [Comments](permissions.md#comments)
   - [Reporting](permissions.md#reporting)
   - [Removing accounts](permissions.md#removing-accounts)
   - [The defaults](permissions.md#the-defaults)
   - [What profile owners can change](permissions.md#what-profile-owners-can-change)
   - [Accepted values](permissions.md#accepted-values)
7. [Signing in](signing-in.md) — passwords, passkeys, emailed codes and their limits
   - [The ways in](signing-in.md#the-ways-in)
   - [Signing in with a code](signing-in.md#signing-in-with-a-code)
   - [What a code can and cannot do](signing-in.md#what-a-code-can-and-cannot-do)
   - [Limits](signing-in.md#limits)
   - [How codes are kept](signing-in.md#how-codes-are-kept)
   - [Resetting a forgotten password](signing-in.md#resetting-a-forgotten-password)
   - [Changing an email address](signing-in.md#changing-an-email-address)
   - [Adding text messages later](signing-in.md#adding-text-messages-later)
8. [Abuse reports](abuse-reports.md) — reporting, the reasons list, and what administrators do
   - [Reports and cases](abuse-reports.md#reports-and-cases)
   - [Reporting something](abuse-reports.md#reporting-something)
   - [The reasons list](abuse-reports.md#the-reasons-list)
   - [Using your own reasons](abuse-reports.md#using-your-own-reasons)
   - [What administrators do](abuse-reports.md#what-administrators-do)
   - [When the reported thing is deleted](abuse-reports.md#when-the-reported-thing-is-deleted)
   - [Who sees what](abuse-reports.md#who-sees-what)
9. [Account lifecycle](account-lifecycle.md) — disabling, archiving, deleting, purging, and clearing out unused accounts
   - [The four states](account-lifecycle.md#the-four-states)
   - [Deleting and the 30 days](account-lifecycle.md#deleting-and-the-30-days)
   - [What a purge removes](account-lifecycle.md#what-a-purge-removes)
   - [What a purge leaves](account-lifecycle.md#what-a-purge-leaves)
   - [Purging now](account-lifecycle.md#purging-now)
   - [The accounts list](account-lifecycle.md#the-accounts-list)
   - [The sweeper](account-lifecycle.md#the-sweeper)
   - [Groups left without an administrator](account-lifecycle.md#groups-left-without-an-administrator)
10. [Deploying](deploying.md) — a live site on Kubernetes, on any provider
   - [What you need](deploying.md#what-you-need)
   - [How the manifests are laid out](deploying.md#how-the-manifests-are-laid-out)
   - [Hostnames](deploying.md#hostnames)
   - [First deployment](deploying.md#first-deployment)
   - [Certificates between the services](deploying.md#certificates-between-the-services)
   - [Object storage](deploying.md#object-storage)
   - [Database, Redis and RabbitMQ](deploying.md#database-redis-and-rabbitmq)
   - [On AWS](deploying.md#on-aws)
   - [Upgrading](deploying.md#upgrading)
   - [Running more than one copy of a service](deploying.md#running-more-than-one-copy-of-a-service)
   - [Trying it on your own machine](deploying.md#trying-it-on-your-own-machine)

## Plans

Design documents for work in progress and work already done, in
[`plans/`](plans/). They record why things were built the way they were, and
are not kept up to date once the work lands — the pages above are.

The order they are meant to be done in, with estimates and dependencies, is
the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md)
in anahita-web. Finished plans move to [`plans/done/`](plans/done/).

| Plan | Roadmap item | Status |
| --- | --- | --- |
| [Recommendation service](plans/recommendation-service.md) | 3 | not started |
| [Complete hard delete and bulk cleanup](plans/actor-hard-delete.md) | 5 | services side (parts A, B and C) done 2026-10-04; the web part, D, not started |
| [Sign in with a code; codes instead of links](plans/done/login-code.md) | 6, 6b | done 2026-10-03 (step-up by code left out by decision) |
| [Notes as replies](plans/notes-as-replies.md) | 7 | not started |
| [Federation: ActivityPub and AT Protocol](plans/federation.md) | 8 | not started |
| [Link previews](plans/link-previews.md) | 10 | not started |
| [Invitations](plans/actor-invitations.md) | 11 | not started |
| [Events](plans/events.md) | 12 | not started |
| [Instance privacy and the composer audience picker](plans/instance-privacy.md) | 13 | not started |
| [Feed generators (custom feeds)](plans/feed-generators.md) | 14 | not started |
| [Video service](plans/video-service.md) | 16 | not started |
| [Several images per photo post; photo sets removed](plans/photo-albums.md) | 17, 18 | 18 done 2026-10-03; 17 not started |
| [Creative Commons licences](plans/content-licences.md) | 19 | not started |
| [Language on posts](plans/content-language.md) | 20 | not started |
| [Download your data](plans/data-export.md) | 21 | not started |
| [Post insights](plans/post-insights.md) | 22 | not started |
| [Profile tabs for replies and reposts](plans/profile-tabs-and-reposts.md) | 24 | not started |
| [Quote posts](plans/quote-posts.md) | 25 | not started |
| [Verified domains](plans/verified-domains.md) | 26 | not started |
| [One clean schema](plans/migration-baseline.md) | 27 | not started |
| [One name for timestamps: `_at`](plans/timestamp-names.md) | 28 | not started |
| [auth-service hardening](plans/auth-service-hardening.md) | 0b, 0c | 0b and 0c done 2026-10-03; the plan's "Not checked" list is still open |
| [Actor profile management](plans/actor-profile-management.md) | — | partly done; the rest is in items 5 and 8 |
| [Signup, registration modes, and agreements](plans/done/signup-and-onboarding.md) | — | done |
| [Onboarding](plans/done/onboarding.md) | — | done (phase 3 moved to the recommendation service) |
| [Dissolve `people_people`](plans/done/dissolve-people-people.md) | — | done |
| [Actor lifecycle states](plans/done/actor-lifecycle-states.md) | — | done (archiving media deferred) |
| [Feed access leak](plans/done/feed-access-leak.md) | 0 | done 2026-10-03 |
| [Remove unused legacy node columns](plans/done/remove-unused-node-columns.md) | 0d | done 2026-10-02 |
| [Remove private notes](plans/done/remove-private-notes.md) | 1 | done 2026-10-03 |
| [Abuse reports](plans/done/abuse-report-service.md) | 4b | done 2026-10-03 |
| [Cloud-neutral storage and deployment](plans/done/cloud-neutral.md) | 9 | done 2026-10-03 (non-AWS providers other than Garage untested) |
