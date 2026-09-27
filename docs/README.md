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
   - [Site identity](configuration.md#site-identity)
   - [Registration](configuration.md#registration)
   - [First super administrator](configuration.md#first-super-administrator)
   - [OAuth signing and encryption](configuration.md#oauth-signing-and-encryption)
   - [Storage](configuration.md#storage)
   - [Mail](configuration.md#mail)
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
   - [Comments](permissions.md#comments)
   - [The defaults](permissions.md#the-defaults)
   - [What profile owners can change](permissions.md#what-profile-owners-can-change)
   - [Accepted values](permissions.md#accepted-values)

## Plans

Design documents for work in progress and work already done, in
[`plans/`](plans/). They record why things were built the way they were, and
are not kept up to date once the work lands — the pages above are.

| Plan | Status |
| --- | --- |
| [Signup, registration modes, and agreements](plans/signup-and-onboarding.md) | done |
| [Onboarding](plans/onboarding.md) | not started |
| [Dissolve `people_people`](plans/dissolve-people-people.md) | done |
| [Actor lifecycle states](plans/actor-lifecycle-states.md) | — |
| [Actor profile management](plans/actor-profile-management.md) | — |
| [auth-service hardening](plans/auth-service-hardening.md) | — |
| [Feed generators](plans/feed-generators.md) | — |
