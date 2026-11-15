# Sign-in emails and refresh rotation

Written 2026-11-10; cleaned up 2026-11-15. **Done 2026-11-15,** each fix
with its tests, and tried on the dev stack: a sign-in with a password
emailed once, a silent re-authorization from another browser not at all.

Three faults in auth-service, found on another deployment of the same code,
where a person was sent six "New sign-in" emails in an afternoon. Anahita's
auth-service has all three in some form. Fix each in a commit of its own,
each with a test that fails on the code before it.

Status: ✅ done · 🔄 in flight · ⬜ open · ⏸️ blocked · ❓ decision

---

## 1. ✅ A cancelled refresh burns the refresh token

The most important of the three, because nothing else in Anahita masks it.

`auth-service/data/services/oauth_token.go`, `RefreshTokenPair`: the old
token is claimed with `RevokeIfActive` (around L249), then the successor is
persisted (L259) and the access token signed with `MintAccessToken(ctx, …)`
(L268) on the **request's** context. The KMS signer passes that context to
`kms.Sign` (`internal/signing/signer_kms.go` L133). A request cancelled
between claim and signing leaves the old token revoked and no successor. The
client retries: `ErrRefreshTokenRaced` for 60 s, then `ErrRefreshTokenReuse`,
which revokes every token the person holds for that client. At the gateway,
validate answers with no `X-OAuth-Token` (the services answer 401) until the
reuse cascade deletes the session.

Even with the local signer, any error after the claim burns the token the
same way.

**Fix:** once claimed, finish the rotation on
`context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)`
(`rotationContext`), with a test that a cancelled parent does not cancel it.

## 2. ✅ "New sign-in" email on silent re-authorization (mostly masked)

`auth-service/cmd/api/handlers/oauth_token.go`: `recordAuthLogAfterIssuance`
emails after every authorization-code exchange unless
`hasRecentLoginForClient` finds an auth log for the person and client under
30 minutes old (L38, L249, L340). A second suppression masks most of it:
`AddOrReuseSystem` (`handlers/auth_log.go` L167) reuses the auth log row for
the same person, client, device, OS, browser **and IP** while its refresh
token is live, and a reused row sends no email.

Silent re-authorizations still email when:
- the IP or user agent changed (mobile networks, IPv6 privacy addresses, a
  browser update);
- the previous session's refresh token is already revoked or expired, e.g.
  straight after a fault-1 cascade;
- the 30-minute window is measured from the reused row's original
  `created_at`, so it rarely applies after the first half hour.

anahita-web does not re-authorize on its own (`login()` in `api/session.js`
runs only on clicks), so today this needs a click on Login with a live
sign-in session; still, the email announces the wrong event.

**Fix:** completing a sign-in (`writeViewerSession` in
`handlers/oauth_login.go`, which both ways of signing in pass through) sets `auth:fresh_signin:<person>` in Redis for
10 minutes; the code exchange emails only if the mark is there, and removes
it (`markFreshSignIn` / `consumeFreshSignIn`, failing open). Remove the
30-minute window. Keep `AddOrReuseSystem`; it is about auth-log rows, not
only the email. Update the comments in `oauth_token.go` L225–233 and
`auth_log.go` L150–156 that describe a 15-minute silent re-authorization
cycle the web app no longer drives.

## 3. ✅ Validate compares expiry in whole seconds (latent)

`auth-service/data/services/jwt.go` L235–240: `now := time.Now().Unix();
if now > int64(exp)`, so a token counts as valid through its whole last
second, and `ReadValidate` (`handlers/oauth_session.go` L148–186) has no
refresh-ahead margin. Where the services refuse once `now >= exp`, this
starts an email loop. Anahita's services allow 30 s of leeway
(`anahita-libs/middleware/oauth.go` L184, `jwt.WithLeeway(30*time.Second)`),
so it does not bite here, but it is the same mismatch with less margin than
it looks.

**Fix:** compare at full precision (`!now.Before(time.Unix(exp, 0))`), and
have validate refresh a session token with under a minute left
(`validateRefreshAhead`, `refreshDue`), forwarding the current token if that
early refresh fails.

## Checks after the fixes

- `go test ./...` in auth-service.
- On the dev stack: sign in, leave the app open past two token lifetimes
  (30 min); one "New sign-in" email, and no `token is expired` 401s in the
  services' logs.
