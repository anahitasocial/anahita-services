# Registration and agreements

[Documentation](README.md) › Registration and agreements

How people join an installation, how invitations and the approval queue work,
and how the Terms of Service and Privacy Policy are accepted.

## Contents

- [Where it happens](#where-it-happens)
- [Registration modes](#registration-modes)
- [What signup asks for](#what-signup-asks-for)
- [Email verification](#email-verification)
- [Invitations](#invitations)
- [The approval queue](#the-approval-queue)
- [Protection against bots](#protection-against-bots)
- [Terms of Service and Privacy Policy](#terms-of-service-and-privacy-policy)
- [NodeInfo](#nodeinfo)
- [The first super administrator](#the-first-super-administrator)

## Where it happens

Signing in, signing up, accepting an invitation and confirming an email address
are all server-rendered pages on auth-service. The web app only links to them.
Passwords never pass through the web app.

| Page | Path |
| --- | --- |
| Sign up | `/signup` |
| Confirm an email address | `/signup/confirm` |
| Resend the confirmation | `/signup/confirm/resend` |
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
- In `approval` mode the extra fields are the application: an administrator
  cannot decide on an email address alone.
- A display name is optional everywhere. An empty one displays as the username.
- Avatar, bio and pronouns for everyone else are for onboarding, after signing
  in.

## Email verification

A self-serve account cannot sign in until its email address is confirmed. The
confirmation link is valid for 24 hours and can be resent from
`/signup/confirm/resend`.

Accepting an invitation confirms the address, because the invitation was
delivered to it.

## Invitations

| Variable | Effect |
| --- | --- |
| `INVITES_FROM` | `admins` (default) or `members` — who may send invitations |
| `INVITE_PENDING_MAX` | Outstanding invitations a member may have at once. Default `5` |
| `INVITE_PENDING_MAX_ADMIN` | The same for administrators. Default `100` |

- The cap counts **outstanding** invitations only. A slot frees when one is
  accepted, expires or is revoked.
- Anybody may revoke an invitation they sent. Browsing all invitations is for
  administrators.
- Rate limits: 30 invitations an hour per sender, and 3 a day to any one
  address.

## The approval queue

In `approval` mode a self-serve applicant confirms their email first, and only
then joins the queue. No account exists until the application is approved.

| Endpoint | Does |
| --- | --- |
| `GET /applications` | Lists pending applications, oldest first |
| `POST /applications/:id/approve` | Creates the account |
| `POST /applications/:id/reject` | Rejects the application |

All three are for administrators. Approval checks again that the username is
still free, since somebody else may have taken it while the application waited.

**Invitations and the queue.** An invitation from an administrator is already a
decision, so it skips the queue. An invitation from a member
(`INVITES_FROM=members`) is a recommendation, so in `approval` mode it joins the
queue like any other application.

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

## NodeInfo

auth-service publishes [NodeInfo 2.1](https://nodeinfo.diaspora.software/) at
`/.well-known/nodeinfo` and `/nodeinfo/2.1`. It is the standard other fediverse
servers read, and the web app reads it too.

- `openRegistrations` — true in `open` and `approval`. The web app shows its
  "Create account" button from this.
- `metadata` — `nodeName`, `registrationMode`, `approvalRequired`,
  `invitationsAccepted`, `termsOfServiceUrl`, `privacyPolicyUrl`,
  `supportEmail`, `supportPhone` and `supportWebsite`.

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
