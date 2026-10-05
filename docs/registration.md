# Registration and agreements

[Documentation](README.md) › Registration and agreements

How people join an installation, how invitations and the approval queue work,
how the Terms of Service and Privacy Policy are accepted, and how a new member
is walked through filling in their profile.

## Contents

- [Where it happens](#where-it-happens)
- [Registration modes](#registration-modes)
- [What signup asks for](#what-signup-asks-for)
- [Email verification](#email-verification)
- [Invitations](#invitations)
- [The approval queue](#the-approval-queue)
- [Protection against bots](#protection-against-bots)
- [Terms of Service and Privacy Policy](#terms-of-service-and-privacy-policy)
- [Onboarding](#onboarding)
- [NodeInfo](#nodeinfo)
- [The first super administrator](#the-first-super-administrator)

## Where it happens

Signing in, signing up, accepting an invitation and confirming an email address
are all server-rendered pages on auth-service. The web app only links to them.
Passwords never pass through the web app.

| Page | Path |
| --- | --- |
| Sign up | `/signup` |
| Confirm an email address, with the emailed code | `/signup/confirm` |
| Ask for a new confirmation code | `/signup/confirm/resend` |
| Accept an invitation | `/invites/accept` |

## Registration modes

`REGISTRATION_MODE` decides how open the installation is.

| Mode | Self-serve signup | Invitations |
| --- | --- | --- |
| `open` | Anybody, once their email is confirmed | Work |
| `approval` | Anybody may apply; an administrator decides | Work |
| `invite` | None | Work |
| `closed` | None | **Do not work**, including ones already sent |

**Unset means `closed`.** An installation nobody has configured should not be
open to the internet.

`invite` is the normal state for a private community. `closed` is a kill
switch — for an abuse incident or an archived community — which is why it also
suspends invitations already sent.

## What signup asks for

| | `open` | `invite` | `approval` |
| --- | --- | --- | --- |
| Username | required | required | required |
| Email | required | from the invitation | required |
| Password | required | required | required |
| Display name | — | optional | required |
| Bio | — | — | required |
| Website | — | — | required |

- Passwords are at least 15 characters.
- Usernames are 3–32 letters, numbers, `_` or `-`, and share one namespace with
  every other actor, groups included.
- In `approval` mode the extra fields are the request: an administrator
  cannot decide on an email address alone.
- A display name is optional everywhere. An empty one displays as the username.
- Avatar, bio and pronouns for everyone else are for onboarding, after signing
  in.

## Email verification

A self-serve account cannot sign in until its email address is confirmed.
Signing up leads straight to a page waiting for a 6-digit code, which is
emailed to the address. The code is valid for 15 minutes and allows five
tries. A new one can be sent from that page, or later from
`/signup/confirm/resend`, at most three times an hour for one address.

It is a code and not a link so that it can be read on any device and typed
where the signup started, and because a link is confirmed by any mail scanner
that follows it. Signing in with an emailed code
([Signing in](signing-in.md#signing-in-with-a-code)) confirms the address too.

A request to join, in approval mode, is confirmed the same way, and only then
goes to the administrators.

Accepting an invitation confirms the address, because the invitation was
delivered to it.

## Invitations

| Variable | Effect |
| --- | --- |
| `INVITES_FROM` | `registered`, `administrators` (default) or `super-administrators` — the minimum role that may send invitations |
| `INVITE_PENDING_MAX` | Outstanding invitations one person may have at once. Default `5` |
| `INVITE_PENDING_MAX_ADMIN` | The same for administrators. Default `100` |

- The cap counts **outstanding** invitations only. A slot frees when one is
  accepted, expires or is revoked.
- Anybody may revoke an invitation they sent. Browsing all invitations is for
  administrators.
- Rate limits: 30 invitations an hour per sender, and 3 a day to any one
  address.

## The approval queue

In `approval` mode somebody signing up confirms their email first, and only
then joins the queue. No account exists until the request is approved.

Rows live in `signup_requests`. The name is deliberate: a follow request is a
request to follow, a signup request is a request to sign up. "Application" is
avoided because everywhere else in this product it means an OAuth client.

| Endpoint | Does |
| --- | --- |
| `GET /signup-requests` | Lists pending requests, oldest first |
| `POST /signup-requests/:id/approve` | Creates the account |
| `POST /signup-requests/:id/reject` | Rejects the request |

All three are for administrators. Approval checks again that the username is
still free, since somebody else may have taken it while the request waited.

When a request joins the queue, every administrator is emailed a link to it:
`/admin/signup-requests` in the web app, a tab of its administration area.

`approve` and `reject` are named in the path rather than being a bare `POST`
and `DELETE` the way follow requests are answered. Ignoring a follow request is
a silent dismissal; rejecting a signup request records a decision note, stamps
the deciding administrator and sends mail.

**Invitations and the queue.** An invitation from an administrator is already a
decision, so it skips the queue. An invitation from someone who is not one
(`INVITES_FROM=registered`) is a recommendation, so in `approval` mode it joins
the queue like any other request.

The administration screen for the queue is planned as part of the settings work
in the web app.

## Protection against bots

There is no CAPTCHA. The signup form uses:

- **A honeypot field**, hidden from people. A submission that fills it in is
  shown a success page and nothing is created.
- **A minimum fill time.** A form submitted less than 2 seconds after it was
  rendered is refused.
- **A CSRF token** bound to the session, used once per submission.
- **Rate limits** per source: 3 accounts an hour from one IP address, 10 an
  hour from one network (`/24` for IPv4, `/64` for IPv6), and 3 confirmation
  emails an hour to one address.

## Terms of Service and Privacy Policy

Agreements are not collected at signup. They are checked **every time somebody
uses the web app**, so a revised document asks everyone again, new members and
existing ones alike.

- **The text** lives in the web app, in `src/statics/legal/`.
- **The versions** live beside it, in `src/statics/legal/index.js`. Raising a
  version is how a revision is published.
- When either version is ahead of what a signed-in person accepted, the web app
  sends them to `/agreements` before anything else. The legal pages and
  support stay reachable.
- Accepting calls `PATCH /agreements/tos` or `PATCH /agreements/privacy` for the
  signed-in person. auth-service records the version together with the time,
  IP address, user agent and operating system, which it takes from the request.
- `TOS_URL` and `PRIVACY_URL` tell auth-service where the documents can be read,
  for links on its own pages. They default to `/legal/tos` and `/legal/privacy`
  on the web app.

This is enforced by the web app's interface, not the API.

## Onboarding

Signup asks only for a username, an email address and a password. After the
agreements, the web app walks a signed-in person with an incomplete profile
through adding an avatar, a display name, a bio and pronouns. Existing members
with an empty profile get the same walk-through on their next sign-in.

- **Whether they have been through it** is `auth_accounts.onboarded_at`. It is
  set when the person reaches the dashboard through the flow, whether they
  completed the steps or skipped them, and only once: a profile that later
  becomes incomplete does not send them back.
- **Whether the profile is incomplete** is read from `/oauth/userinfo`, under
  the `profile` scope: `avatar_urls` is absent when there is no avatar, and
  `has_bio` is false when the bio is empty or only whitespace.
- **`onboarded_at`** is also on userinfo — the timestamp, or `null`.
- Finishing calls `PATCH /onboarding` for the signed-in person, with no body.
  It answers `200` with `{ "onboarded_at": "<timestamp>" }`, and a second call
  keeps the first date.

**Who invited them** is `auth_accounts.invited_by`. It is written when an
invitation is accepted, from the invitation, and when a signup request a member
vouched for is approved. A self-serve signup has none. Onboarding shows the
inviter first among the accounts it offers to follow, and never follows anybody
on the person's behalf.

`GET /onboarding` returns it for the signed-in person, by alias:
`{ "invited_by": "<alias>" }`. It is `null` when nobody invited them, and when
the inviter's account is gone, disabled or being deleted.

**Featured accounts** are the ones onboarding offers to follow. A super
administrator features a person or a group from its profile:

| Endpoint | Does |
| --- | --- |
| `PATCH /people/:id/featured` | `{ "featured": true }` or `false` |
| `PATCH /groups/:id/featured` | The same, for a group |

Super administrators only; anybody else gets `403`. The answer is `200` when
something changed and `204` when it was already so. A disabled, archived or
deleted account cannot be featured (`409`). Featured actors carry
`featured_at` in their JSON.

`GET /people/?featured=1` and `GET /groups/?featured=1` list them, most
recently featured first unless `ordering` is given, through the ordinary browse:
the viewer's relationship to each is filled in and blocked accounts are left
out. A featured account that is later disabled, archived or deleted drops out
of the list without being unfeatured.

Like the agreements, this is enforced by the web app's interface, not the API.

## NodeInfo

auth-service publishes [NodeInfo 2.1](https://nodeinfo.diaspora.software/) at
`/.well-known/nodeinfo` and `/nodeinfo/2.1`. It is the standard other fediverse
servers read, and the web app reads it too.

- `openRegistrations` — true in `open` and `approval`. The web app shows its
  "Create account" button from this.
- `metadata` — `nodeName`, `registrationMode`, `approvalRequired`,
  `invitationsAccepted`, `termsOfServiceUrl`, `privacyPolicyUrl`,
  `supportEmail`, `supportPhone` and `supportWebsite`. Also `invitesFrom` and
  `groupsFrom`, the least role that may invite or create a group, and
  `readAccess`, `public`, `preview` or `registered`: what somebody who is not
  signed in may read, and `minContentAccess`, the widest audience anything new may be
  given (see
  [Privacy](privacy.md#what-the-installation-allows)).

## The first super administrator

Nothing in signup can create a super administrator. On a fresh database, set
`SUPER_ADMIN_EMAIL` (and optionally `SUPER_ADMIN_USERNAME` and
`SUPER_ADMIN_NAME`) and start auth-service:

1. If no active super administrator exists, it creates the account with an
   unusable password and a confirmed email address.
2. It prints a set-password link, valid for 48 hours, to its log, and also
   emails it. The log works before SMTP is configured:

   ~~~sh
   docker compose logs auth-service | grep "super admin seed"
   ~~~

3. On an installation that already has a super administrator it does nothing,
   so the variable can stay set.
