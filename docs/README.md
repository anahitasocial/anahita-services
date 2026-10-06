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
   - [Privacy of the installation](configuration.md#privacy-of-the-installation)
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
   - [Naming dates and times](architecture.md#naming-dates-and-times)
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
6. [Permissions](permissions.md) — who can post, reply, like and add followers
   - [Access levels](permissions.md#access-levels)
   - [Seeing posts](permissions.md#seeing-posts)
   - [Choosing who can see a post](permissions.md#choosing-who-can-see-a-post)
   - [Replies](permissions.md#replies)
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
9. [Privacy](privacy.md) — who can see a post, a profile and the site, and the settings an installation makes
   - [The audiences](privacy.md#the-audiences)
   - [Who can see a post](privacy.md#who-can-see-a-post)
   - [Who can see a profile or a group](privacy.md#who-can-see-a-profile-or-a-group)
   - [What the installation allows](privacy.md#what-the-installation-allows)
   - [Setting them](privacy.md#setting-them)
   - [Checking them](privacy.md#checking-them)
   - [What the web app does](privacy.md#what-the-web-app-does)
   - [Where it is enforced, and how it is tested](privacy.md#where-it-is-enforced-and-how-it-is-tested)
   - [Federation](privacy.md#federation)
10. [The language of a post](post-language.md) — what is stored, how it is chosen, and what uses it
   - [What it is for](post-language.md#what-it-is-for)
   - [The tags](post-language.md#the-tags)
   - [On a post](post-language.md#on-a-post)
   - [On a person](post-language.md#on-a-person)
   - [What the web app does](post-language.md#what-the-web-app-does)
   - [What is not built](post-language.md#what-is-not-built)
11. [Photo posts](photos.md) — several images in one post: uploading, storing, changing and removing them
   - [One post, several images](photos.md#one-post-several-images)
   - [Posting](photos.md#posting)
   - [What is stored](photos.md#what-is-stored)
   - [Reading](photos.md#reading)
   - [Changing a post's images](photos.md#changing-a-posts-images)
   - [Removal](photos.md#removal)
   - [Limits](photos.md#limits)
   - [What is not built](photos.md#what-is-not-built)
12. [Account lifecycle](account-lifecycle.md) — disabling, archiving, deleting, purging, and clearing out unused accounts
   - [The four states](account-lifecycle.md#the-four-states)
   - [Deleting and the 30 days](account-lifecycle.md#deleting-and-the-30-days)
   - [What a purge removes](account-lifecycle.md#what-a-purge-removes)
   - [What a purge leaves](account-lifecycle.md#what-a-purge-leaves)
   - [Purging now](account-lifecycle.md#purging-now)
   - [The accounts list](account-lifecycle.md#the-accounts-list)
   - [The sweeper](account-lifecycle.md#the-sweeper)
   - [Groups left without an administrator](account-lifecycle.md#groups-left-without-an-administrator)
13. [Deploying](deploying.md) — a live site on Kubernetes, on any provider
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
| [Sign in with a code; codes instead of links](plans/done/login-code.md) | 6, 6b | done 2026-10-03 (step-up by code left out by decision) |
| [Notes as replies](plans/done/notes-as-replies.md) | 7 | done 2026-10-05: every comment is a reply, a post's author chooses who can reply, and comment-service is removed |
| [Federation: ActivityPub and AT Protocol](plans/federation.md) | 8 | not started |
| [Link previews](plans/link-previews.md) | 10 | not started |
| [Invitations](plans/actor-invitations.md) | 11 | not started |
| [Events](plans/events.md) | 12 | not started |
| [Feed generators (custom feeds)](plans/feed-generators.md) | 14 | not started |
| [Video service](plans/video-service.md) | 16 | not started |
| [Creative Commons licences](plans/content-licences.md) | 19 | kept for later (2026-10-04); look again after items 16 and 17 |
| [Download your data](plans/data-export.md) | 21 | not started |
| [Post insights](plans/post-insights.md) | 22 | not started |
| [Profile tabs for replies and reposts](plans/done/profile-tabs-and-reposts.md) | 24 | done 2026-10-05 |
| [Quote posts](plans/done/quote-posts.md) | 25 | done 2026-10-05 |
| [Verified domains](plans/verified-domains.md) | 26 | not started |
| [The Website actor](plans/website-actor.md) | 30 | goal and name recorded 2026-10-05; scope open, not to be built yet |
| [One clean schema](plans/migration-baseline.md) | 27 | not started |
| [auth-service hardening](plans/auth-service-hardening.md) | 0b, 0c | 0b and 0c done 2026-10-03; the plan's "Not checked" list is still open |
| [Actor profile management](plans/actor-profile-management.md) | — | partly done; the rest is in item 8 |
| [Signup, registration modes, and agreements](plans/done/signup-and-onboarding.md) | — | done |
| [Onboarding](plans/done/onboarding.md) | — | done (phase 3 moved to the recommendation service) |
| [Dissolve `people_people`](plans/done/dissolve-people-people.md) | — | done |
| [Actor lifecycle states](plans/done/actor-lifecycle-states.md) | — | done (archiving media deferred) |
| [Feed access leak](plans/done/feed-access-leak.md) | 0 | done 2026-10-03 |
| [Remove unused legacy node columns](plans/done/remove-unused-node-columns.md) | 0d | done 2026-10-02 |
| [Remove private notes](plans/done/remove-private-notes.md) | 1 | done 2026-10-03 |
| [Abuse reports](plans/done/abuse-report-service.md) | 4b | done 2026-10-03 |
| [Complete hard delete and bulk cleanup](plans/done/actor-hard-delete.md) | 5 | done 2026-10-04 (the web part not yet tried in a browser) |
| [One name for timestamps: `_at`](plans/done/timestamp-names.md) | 28 | done 2026-10-04 |
| [One identity image service: avatars and covers merged](plans/done/identity-image-service.md) | 31 | done 2026-10-05 (nothing needing a signed-in session was tried) |
| [One actor service: people and groups merged](plans/done/actor-service.md) | 29 | done 2026-10-05 (nothing needing a signed-in session was tried) |
| [Several images per photo post; photo sets removed](plans/done/photo-albums.md) | 17, 18 | 18 done 2026-10-03; 17 done 2026-10-05 (posting and the web parts not yet tried in a browser) |
| [Language on posts](plans/done/content-language.md) | 20 | done 2026-10-05 in a smaller form: the author chooses, nothing is detected |
| [Instance privacy and the composer audience picker](plans/done/instance-privacy.md) | 13 | done 2026-10-05 (the web parts not yet tried in a browser) |
| [Cloud-neutral storage and deployment](plans/done/cloud-neutral.md) | 9 | done 2026-10-03 (non-AWS providers other than Garage untested) |
