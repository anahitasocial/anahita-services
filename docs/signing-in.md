# Signing in

[Documentation](README.md) › Signing in

The ways a person proves who they are, and the limits around them. Everything
here is in auth-service, whose pages the web app sends people to.

## Contents

- [The ways in](#the-ways-in)
- [Signing in with a code](#signing-in-with-a-code)
- [What a code can and cannot do](#what-a-code-can-and-cannot-do)
- [Limits](#limits)
- [How codes are kept](#how-codes-are-kept)
- [Resetting a forgotten password](#resetting-a-forgotten-password)
- [Changing an email address](#changing-an-email-address)
- [Adding text messages later](#adding-text-messages-later)

## The ways in

| Way | Page | Then |
| --- | --- | --- |
| Password | `/oauth/login` | Two-factor, if the account has it |
| Passkey | `/oauth/login`, the passkey button | Nothing more: a passkey is both factors |
| Emailed code | `/oauth/login/code` | Two-factor, if the account has it |

All three end the same way, and all three apply the same rules to the account
once it has been proved: a deleted account is offered its restore, an archived
or disabled one is refused.

## Signing in with a code

1. The person follows **Sign in with a code instead** and enters their email
   address or username.
2. If the account exists, it is emailed a 6-digit code. The code is in the
   subject as well, so it can be read from a notification.
3. They type the code on the page that is waiting for it.

The email carries no link. The page that wants the code is already open, and
a link is what mail scanners follow.

The pages never say whether an account exists. An address with no account
gets the same pages and wording, and the same five tries, as a real one. It
is just never sent anything. A username is never answered with its address.

A code sent to an account whose address was never confirmed confirms it: the
code arriving and coming back is what the confirmation link proves.

## What a code can and cannot do

- It replaces the **password**, not the second factor. An account with
  two-factor turned on is still asked for its authenticator code.
- It works **once**, for **10 minutes**, and only in the browser that asked
  for it: the browser's session holds which code it is waiting for.
- A **new code retires the old one.**
- It is checked against the account **as it is when the code is typed**, not
  when it was sent.
- A code is issued for one purpose and refused for any other.

## Limits

| What | Limit | Then |
| --- | --- | --- |
| Tries at one code | 5 | The code is dead; ask for another |
| Codes requested for one address or username | 5 an hour | Nothing more is sent. The pages look the same |
| Codes requested from one network address | 20 an hour | The same |
| Password tries for one address or username | 10 in 15 minutes | Refused until the time has passed |
| Password tries from one network address | 50 in 15 minutes | The same |

Something that is not six digits is not counted as a try. Every limit is
counted before the account is looked up, so reaching one says nothing about
whether the account exists. A successful sign-in clears that address's count.

Somebody guessing at an account's password can use up its ten tries and lock
password sign-in for a quarter of an hour. A code or a passkey still works.

The counts are kept in Redis. If Redis cannot be reached they are not
enforced, and the log says so.

The sign-in forms carry a token tied to the browser's session, so another
site cannot submit them on somebody's behalf.

## How codes are kept

The six digits are never stored. The `tokens` row holds an HMAC of them,
keyed with a value derived from `SECURE_COOKIE_KEY` and salted with the
token's own random identifier. Reading the database does not reveal a code,
and trying all million possibilities against it is not possible without the
key. Changing `SECURE_COOKIE_KEY` invalidates the codes outstanding at that
moment.

Counting a try and using a code up are each a single statement in the
database, so guesses arriving at the same moment cannot share a try, and only
one of two correct submissions signs in.

## Resetting a forgotten password

1. At `/password/forgot` the person enters their email address.
2. If it has an account, it is emailed a 6-digit code, valid for 15 minutes
   and five tries.
3. They type the code on the page that is waiting for it, and only then are
   shown the form for a new password.

The code is used up when the password is set, so a mistake on the password
form can be corrected without a new code. Asking for a new code retires the
earlier one, including in a browser that had already entered it.

Setting the password this way removes every passkey and signs out every
session, and the account is told by email. It does not sign the person in.

At most three codes an hour are sent to one address, and the pages do not say
whether the address has an account.

One link remains: the set-password link for the
[first super administrator](registration.md#the-first-super-administrator),
which is read from a log with no page open to type a code into.

## Changing an email address

Done from the app's settings, signed in. The address is what password reset
delivers to, so moving it takes two proofs:

1. **It is you.** `PATCH /email/change` with the new address needs a recent
   step-up (a passkey, or the password and two-factor). One step-up buys one
   request.
2. **You hold the current inbox.** A 6-digit code is mailed to the *current*
   address, naming the new one. The person types it into the same settings
   screen, which calls `POST /email/change/confirm` with `{ "code": "…" }`.

| Answer | Meaning |
| --- | --- |
| 204 | Changed. Every passkey is removed and every session is signed out, this one included. Both addresses are told |
| 400 | Not six digits. Nothing was counted |
| 409 | The new address was taken in the meantime |
| 410 | Nothing is pending, or the code expired (15 minutes), was replaced by a newer request, or ran out of its five tries |
| 422 | Wrong code. The body is `{ "remaining": n }` |

The code is found by whose it is, never by its digits, so one person cannot
use or wear out another's.

## Adding text messages later

The flow has a step for choosing how the code is sent, which is skipped while
email is the only way. Text messages need a verified phone number on the
account, a sender, and `sms` added to the list of channels in
`auth-service/cmd/api/handlers/oauth_login_code.go`. The pages, the limits and
the storage do not change.
