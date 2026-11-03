# People

A person is somebody with an account: a profile at their username, made when
they sign up. What every profile has, a person has too
([Profiles](profiles.md)). This page is what only a person has, and where
each is described.

## Contents

1. [Their account](#their-account)
2. [What only a person has](#what-only-a-person-has)
3. [The API](#the-api)
4. [Where the code is](#where-the-code-is)

## Their account

A person's profile is made by signing up, in auth-service, never by
actor-service: [Registration](registration.md). They sign in with a
password, a passkey or a code by email ([Signing in](signing-in.md)), and
look after their own profile, with no admins. A person who deletes their
account has thirty days to come back by signing in
([Account lifecycle](account-lifecycle.md#deleting-and-the-30-days)).

## What only a person has

| What | Said in |
| --- | --- |
| An account type: registered, administrator or super administrator | [Permissions](permissions.md#account-types) |
| Pronouns, and the language they usually post in | [Registration](registration.md#onboarding), [The language of a post](post-language.md) |
| Being tagged in photos and videos, approved by them first unless they say otherwise | [Permissions](permissions.md#people-in-pictures-and-videos) |
| Who may mention them | [Permissions](permissions.md#mentions) |
| Who may quote their posts | [Permissions](permissions.md#quotes) |
| Whether their reposts are among their posts | [Permissions](permissions.md#reposts) |
| Being on the fediverse and Bluesky, which each person turns on for themselves | [Federation](federation.md#who-is-on-the-fediverse) |
| Their notifications, saved posts, custom feeds and suggestions | [Notifications](notifications.md), [Custom feeds](feeds.md), [Recommendations](recommendations.md) |
| Downloading everything of theirs | [Download your data](data-export.md) |

## The API

Under `/people/`: listing people and reading one (`GET /people/`,
`GET /people/:username`), changing their profile, their quote policy and
whether reposts are among their posts, tags in photos and videos
(`/people/:id/tags/:post_id`, `GET /people/tagged-in/:post_id`), removing
one as spam, and purging many at once. What every profile has is there too,
at `/people/:id/…` ([Profiles](profiles.md#the-api)). Each is sent as
`actor.PersonNode`. Their account type is sent only to themselves and to the
site's administrators.

## Where the code is

| What | Where |
| --- | --- |
| The routes | `actor-service/cmd/api/routes.go`, under `people` |
| The handlers | `actor-service/cmd/api/handlers/person*.go` |
| What a person is sent as | `actor-service/responses/person.go` |
| Signing up and in | `auth-service` ([Registration](registration.md), [Signing in](signing-in.md)) |
