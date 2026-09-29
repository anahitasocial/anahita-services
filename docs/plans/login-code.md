# Sign in with an emailed code, and codes instead of links

*Roadmap item 6 and 6b, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Phase 1 — sign in with a code (roadmap item 6)

### Context
- **What we want:** a passwordless sign-in option. The person enters their email or username and gets a 6-digit code by email, which they type in the same browser. It's a code, not a link, so the sign-in completes in the browser where it started, and so the same design carries over to SMS later.
- **Scope:** this item builds the shared code machinery and sign-in with a code. Signup confirmation, password reset, email change and step-up move to codes in item 6b, which builds on this one.
- **Reference:** `protect-services/auth-service` (commit `e7d8186`, `cmd/api/handlers/otp.go`, `oauth_login_otp.go`).

### What exists
**In Anahita, part of the scaffolding is already committed but unused** (noted in `docs/plans/actor-profile-management.md:650-653`):
- `auth-service/constants/token.go:21-49` (`TokenTypeLoginOTP`, `OTPCodeLength=6`, `TokenExpiryTimeOTP=10 min`, `OTPMaxAttempts=5`);
- `auth-service/data/entities/token.go:30-42,68-117` (`Code`, `CodeAttempts`, `GenerateCode` using `crypto/rand`, constant-time `IsCodeValid`, `HasAttemptsLeft`);
- tests in `token_test.go:200-280`.

**Missing:** the DB columns, the repository wiring, the handlers, the templates and the rate-limit scopes.

**The login screens are Go templates in auth-service** (`auth-service/web/templates/`, `handlers/oauth_login.go`). The web app only redirects to them (`anahita-web/src/containers/auth/index.jsx`), so **there's no anahita-web work** in this item.

### Port from protect (with its hard-won rules)
- **Three steps, each POST answered with a redirect** (identifier, then method, then verify), with `pendingOTPState` kept in the server-side session.
- **The code is looked up by token id, never by code.** The token id lives only in the session, because a `WHERE code = ?` lookup would match *anybody's* live code.
- **No revealing whether an account exists:**
  - `Sent` is its own field and is **always** written, whether or not a code was sent.
  - An unknown address gets the same page and spends the same five-attempt budget from a session counter.
  - Delivery runs on a detached context after the response, so response time doesn't reveal whether the account exists.
- **Attempts:** a wrong code spends an attempt **before** responding. Malformed input (not 6 digits) spends nothing.
- **The code replaces the password, not the second factor.** If the person has TOTP, hand over to the existing `/oauth/login/totp` step (`pendingTOTPState`).
- **Account state is re-checked at verify time, not only at send.**
- **Session writes:** Edit first, then Add.
- **Email copy:** "expires in 10 minutes, only works in this browser, we will never ask you for it". The action link goes to the login page, never to anything carrying the code.
- **Channel abstraction:** the method step and a `loginCodeChannel` type with only `email` for now, where an unknown value folds into email. SMS later adds `SmsSenderInterface` (log sender and SNS sender) and a branch, with no change to the flow.

### Fix rather than port (protect's weaknesses)
1. **Store an HMAC of the code, not the code.** The key comes from a server secret (new env var `OTP_HMAC_KEY`, falling back to a key derived from an existing secret). Plain SHA-256 of 6 digits can be brute-forced offline.
2. **Atomic attempt counter:** `UPDATE tokens SET code_attempts = code_attempts + 1 WHERE id = ? AND code_attempts < 5`, then check rows affected. Protect's read-modify-write loses updates when guesses arrive at the same moment.
3. **Atomic consume:** `UPDATE … SET status = used WHERE id = ? AND status = unused`, requiring exactly one affected row.
4. **Resend retires earlier codes.** Call the existing `InvalidateOutstanding(TokenTypeLoginOTP, email)` before issuing a new one.
5. **Rate limits use Anahita's per-scope limiter** (`auth-service/cmd/api/handlers/rate_limit.go`), not protect's single global budget. New scopes: `login_code_email` (5 per hour) and `login_code_ip` (20 per hour). Charge them before the account lookup.

### Implementation (auth-service + migration-job)
1. **Migration** (goose with the guarded-procedure pattern, as in `migrations/20260903000001_tokens_new_email.sql`; mirrored in `schema/init.sql` and `legacy/upgrade.sql`):
   - `tokens.code_hash varchar(64) NULL`
   - `tokens.code_attempts int NOT NULL DEFAULT 0`
   - `tokens.channel varchar(16) NULL`, recording `email` now and `sms` later, for auditing
   - `tokens.type` is a varchar, so the new `loginotp` type needs no schema change.
2. **Entity and repository:**
   - `GenerateCode` returns the plain code for sending and sets `CodeHash`. `IsCodeValid` compares HMACs in constant time.
   - Add the columns to `Add`, the select list (`tokens_helper.go:11-27`, with COALESCE) and the scan.
   - New methods `IncrementCodeAttempts` and `ConsumeCode` (both atomic), and a type-checked `ReadByID`.
   - Update `fakes_test.go`.
3. **`handlers/otp.go`:** port `issueOTP`, `verifyOTP` and `consumeOTP` with their error sentinels. Adapt `TokenID uint64` to `ID int64` and drop the organization parts.
4. **`handlers/oauth_login_code.go`:** port the three-step state machine.
   - Routes `GET/POST /oauth/login/code`, `/oauth/login/code/method` and `/oauth/login/code/verify`, next to the existing login routes (`routes.go:144-166`).
   - The identifier can be an email **or a username**, via the existing `Users.ReadByEmailOrUsername`. The resolved email is kept only on the server.
   - **Verify** follows Anahita's existing gate order from `AddCredentials` (`oauth_login.go:75-191`):
     - soft-deleted → the existing restore form;
     - archived → the archived message;
     - disabled → the generic message;
     - otherwise TOTP if enrolled, otherwise `finalizeLogin`.
   - **Unverified email:** a correct code proves the person controls the inbox, so stamp `email_verified_at` (`Users.MarkEmailVerified`) and continue. People in approval mode don't have an account until they're approved, so it doesn't apply to them.
   - On success, reset the `login_code_email` counter.
5. **Templates:**
   - Port `login_code.html`, `login_code_method.html` (email only for now; the method step is skipped automatically while there's one channel, but the template stays for SMS) and `login_code_verify.html`, restyled on Anahita's `base.html`/`logo.html`.
   - Add a "Sign in with a code instead" link on `login.html`.
   - Error codes: `?error=identifier_required|session_expired|code_expired`, with a neutral hint for everything else.
6. **Email:** `message_action` through `MailGRPC.SendFromTemplate` with no `actionURL` (the template already makes it optional), sent directly, not through notification-service. Add the copy to `auth-service/messages/`.
7. **Nearby fixes worth doing in the same pass** (small, and they touch the same code):
   - Add the missing `/oauth/login` password throttle, with scopes `login_identifier` and `login_ip` (`docs/plans/auth-service-hardening.md` §3).
   - Set `CSRFToken` in `loginContext` (`oauth_login.go:424-442`) and check it on the login and login-code POSTs. The template renders it today, but it's never set or checked.
8. **Docs:** a "Sign in with a code" section in `docs/registration.md` (or a new `docs/authentication.md`), covering the flow, limits, TTL and how SMS will plug in.

### Estimate

| Part | Time |
| --- | --- |
| Migration, entity (HMAC), repository atomic ops, fakes | 2–3 h |
| `otp.go` port + rate-limit scopes | 2 h |
| Three-step handlers + gate order + TOTP hand-off + email verification stamp | 3–4 h |
| Templates, copy, login-page link | 2 h |
| Password-login throttle + CSRF on login forms | 1–2 h |
| Handler tests + docs | 2–3 h |
| **Total** | **about 12–16 h, 3–4 sessions** |

### Verification
- **Unit tests:** code generation and HMAC comparison, atomic increment when guesses arrive at the same moment (5 parallel wrong guesses leave exactly 5 attempts spent, and a 6th is refused), atomic consume (the second consume fails), and resend invalidating the previous code.
- **Handler tests, ported from protect's `oauth_login_ratelimit_test.go` and extended:**
  - An unknown address gets the same page and wording, and is locked out after the same 5 attempts.
  - An expired code gives `code_expired`.
  - A disabled account is refused at verify even though the code was sent.
  - A TOTP account continues to `/oauth/login/totp`.
  - An unverified account is verified after a successful code.
  - The 6th send in an hour is rate-limited.
- **Manual:** from the SPA, click Sign in, then "Sign in with a code". Mailpit (or the SMTP catcher) shows the email, the code works once in the same browser, and the SPA session is established.

### Later (SMS, a separate item when wanted)
- Phone storage on `auth_accounts` (E.164, unique, NULL allowed) and the phone set/verify endpoints from `protect-services/auth-service/cmd/api/handlers/phone.go`, including the check that the phone number hasn't changed between send and confirm.
- The `sms` branch in the method step, with fallback to email when the number isn't verified.
- `SMS_SENDER=log|sns` config.

### Decisions (item 6)
- A successful code marks the email as verified.
- Because of the upcoming anahita-mobile app, codes replace links in the other flows as well (item 6b below). Invites stay as links.

### Documentation (phase 1)

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `registration.md`**: a *Signing in with a code* section (flow, TTL, attempts, rate limits, how SMS will plug in).
- **services `configuration.md`**: `OTP_HMAC_KEY` and the new rate-limit scopes, including the password-login throttle.

## Phase 2 — codes for signup, password reset, email change and step-up (roadmap item 6b)

### Context
The mobile app will run auth-service's pages inside an in-app login window (`ASWebAuthenticationSession` / Custom Tabs). An emailed link opens in the default browser instead, which doesn't share that window's cookies, and the person may be reading the email on a different device. So a half-finished signup or reset is lost. A code works across devices, apps and browsers.

Invites stay as links: the recipient has no account yet, and later app links can open the app if it's installed. Everything here reuses item 6's `otp.go`, the HMAC storage and the atomic attempts and consume. Each flow gets its **own token type**, so a code issued for one purpose can never be used for another.

### Flows
1. **Signup confirmation** (`signup.go`: `sendConfirmation` :397-427, `GET /signup/confirm` :546-635, the approval-mode path :824-878, resend :652-685)
   - New token type `signupconfirmotp`, valid **15 minutes** (not the link's 24 hours), 5 attempts.
   - After `POST /signup`, redirect to `/signup/confirm`, which becomes a code form. The token id is pinned in the pending session. Resend reuses `POST /signup/confirm/resend` and retires the previous code. The existing `signup_confirm_email` limit still applies.
   - This also fixes today's state-changing `GET /signup/confirm?token=`.
   - **Transition:** keep the old `?token=` handler for 24 hours after deploy, so links already sent still work, then remove it.
   - When someone tries to sign in with an unverified email, the message offers "send me a code" instead of "resend the link".
2. **Password reset** (`password.go`: forgot :56-148, reset :169-430)
   - New token type `passwordresetotp`, valid 15 minutes.
   - The steps: `POST /password/forgot` shows the same page whether or not the account exists, then `/password/reset/verify` (the code), then `/password/reset` (the new-password form, allowed only with a verified token id in the session).
   - Keep today's side effects: revoke passkeys, refresh sessions and IdP sessions, and send the "password changed" email.
   - `pw_forgot_email` and `pw_forgot_ip` stay. `pw_reset_token` is replaced by the per-token attempt budget.
   - Same 24-hour transition for links already sent.
3. **Email change** (`email_change.go`: `PATCH /email/change` :66-157, confirm :162-339)
   - New token type `emailchangeotp`, valid 15 minutes. Keep today's rule: step-up first, and the code goes to the **current** inbox. The new address still gets its notice.
   - The pending token id is stored server-side under `emailchange:<personID>`.
   - `POST /email/change/confirm` becomes a JSON call with `{ code }`, used by the SPA and the mobile app.
   - **Web:** `anahita-web/src/containers/auth/Email/index.js:103-122` and `EmailEdit.jsx` get a code step ("Enter the 6-digit code we sent to your current address") with resend. Update the copy in `languages/*/email.js:5-6`, which says "confirmation link… 15 minutes".
4. **Step-up by code, restricted**
   - `POST /reauth/code/send` and `POST /reauth/code/verify` behind `authorize`, token type `reauthotp`. On success, call the existing `markStepUp` (`reauth_helper.go:69-81`), with the step-up marked as coming from a code.
   - **Only offered to accounts with neither a password nor a passkey.** `requireStepUp` refuses a code-based step-up for email change.
   - Note: signup requires a password today, so every existing account probably has one and this path stays dormant until passwordless accounts exist. Check this during implementation. If it would be dormant, ask whether to switch to the variant where anyone can use a code but it isn't accepted for credential changes (email, password, TOTP or passkey removal).
   - **Web:** `anahita-web/src/containers/auth/StepUp/index.js` and `StepUpDialog.jsx` get an "Email me a code" option when the capabilities say it's allowed. Add `api/reauth.js#codeSend` and `#codeVerify`.
5. **Cleanup:** delete the dead legacy link screens in the web app, `containers/auth/PasswordReset.jsx` (route `/passwordreset`) and `containers/auth/Token.jsx` (routes `/token/:token...`), which call APIs from the old PHP app.

### Estimate

| Part | Time |
| --- | --- |
| Signup confirmation by code + transition | 3–4 h |
| Password reset by code + transition | 3–4 h |
| Email change by code (service + web) | 2–3 h |
| Step-up by code (service + StepUp dialog) | 3–4 h |
| Legacy screen cleanup, docs (`docs/registration.md`) | 1 h |
| **Total** | **about 12–16 h, 3–4 sessions** |

**Items 6 + 6b together: about 24–32 h, 6–8 sessions.**

### Verification
- **Each flow:** the code works once. A code issued for one purpose is rejected by the others' verify step (for example, a signup code can't reset a password). The 6th wrong attempt is refused, and an expired code shows the expired message.
- **Transition:** an old signup or reset link sent before deploy still works during the 24-hour window, and gives 404 after the old handler is removed.
- **Email change in the SPA:** the code arrives at the old address, the change is applied, and both addresses get notices.
- **Mobile readiness:** run signup and password reset in a private window. Read the code from Mailpit on another device and type it in; both complete without any link.

### Decisions (item 6b)
- Signup confirmation, password reset, email change and step-up (restricted) all move to codes. Invites stay as links.

### Documentation (phase 2)

- **services `registration.md`**, *Email verification* and *Invitations*: signup confirmation and password reset use codes; invitations still use links.
- **web `architecture.md`**, *Signing in*: email change by code, and step-up by code where offered.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
