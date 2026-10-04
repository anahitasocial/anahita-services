# auth-service — security hardening

Written 2026-09-08, for a future anahita session. **No code in this repository
has been changed.**

> **Roadmap, 2026-09-28.** Findings 1, 2, 4 and 5 are roadmap item **0b**. Authenticating and
> encrypting gRPC calls is item **0c**, done after item 9, which sets up cert-manager.
> On 2026-10-03 the shared-secret interceptor was moved from 0b into 0c: with no live site yet,
> an interim secret wasn't worth a second configuration step, and mutual TLS covers both. Finding 3 (login throttle) is part of item 6
> ([login-code.md](done/login-code.md)). Finding 5 must land before item 21
> ([data-export.md](data-export.md)) relies on step-up.

A security review of a sibling service produced seventeen findings. That
service and this one share ancestry, so each finding was re-checked against
this codebase rather than assumed. Some things are already right here. The rest
are below.

Verification level is stated per item. Where a claim was not checked, it says so
rather than implying it was.

---

## What anahita already gets right — do not regress these

Recording them because a future "bring the two codebases into line" pass could
otherwise undo them here, in the belief it was fixing something.

- **`authLogHandler.Delete` checks row ownership.**
  `cmd/api/handlers/auth_log.go:348` compares `authLog.CreatedBy != user.ID`
  after the read, with a comment explaining exactly why. The sibling did not,
  and it was the most serious finding of the review: the permission is
  evaluated against the user named in the request while the row acted on comes
  from a different parameter, so passing your own uid plus any auth log id
  force-logged-out someone else's session. Keep this.
- **`authLogs.Trim`'s subquery is scoped.** `data/repositories/auth_logs.go:187`
  repeats `WHERE created_by = ?` inside the `NOT IN`. The sibling's did not,
  so it retained the newest N rows across the entire table and deleted
  essentially every row belonging to the acting person — which also silently
  broke "sign out everywhere", since that flow finds refresh tokens by iterating
  auth-log rows.
- **Per-scope rate limits.** `cmd/api/handlers/rate_limit.go:79` gives every
  scope its own `max` and `window` rather than one global pair. That is the
  better design; it is the one to carry outward, not away from here.

---

## 1. `getIPLocation` — rewrite

`cmd/api/handlers/helpers.go:35`. Verified by reading.

The private-address short-circuit here is the good idea and should stay: without
it, every auth log on a developer's machine reads "an unknown city, unknown
country". But four things around it need fixing:

- **`strings.HasPrefix(ip, "172.")` is too broad.** It claims all of
  172.0.0.0/8 when only 172.16.0.0/12 is private, so a genuine public address
  such as `172.32.5.9` is labelled local and never looked up. Use `net.IP`'s own
  predicates: `IsLoopback() || IsPrivate() || IsLinkLocalUnicast() ||
  IsUnspecified()` covers 10/8, 172.16/12, 192.168/16 and IPv6 unique-local in
  one call. Add carrier-grade NAT (100.64.0.0/10) by hand — mobile networks put
  real users behind it and it is equally unplaceable.
- **Nil dereference on a network error.** Line 63 logs the error and falls
  through into `defer resp.Body.Close()` on a nil response. Any blip reaching
  ipapi.co panics. Return instead.
- **Unchecked type assertions.** `json_map["city"].(string)` (line 82) and
  `json_map["reason"].(string)` (line 77) panic on a JSON null, which ipapi.co
  returns for `city` on plenty of ordinary addresses — datacentre ranges
  especially. Decode into a typed struct so an absent field is empty.
- **No timeout.** `http.Client{}` has none. **Check whether this call is
  synchronous on the token-issuance path here** — in the sibling it was,
  which made an unresponsive provider able to hold `POST /oauth/token` open
  indefinitely, and that was arguably worse than the panic. It is called from
  `cmd/api/handlers/auth_log.go:297`; I did not trace what reaches that.

Also worth deciding rather than inheriting: a malformed address should probably
**not** be reported as local. It means something upstream is wrong — most likely
a forged `X-Forwarded-For` — and claiming it is a LAN hides that.

Make the base URL and the client package-level `var`s while rewriting it. That
is what lets the whole function be covered by tests rather than only its
short-circuit.

## 2. `getClientIP` trusts the first `X-Forwarded-For` entry

`cmd/api/handlers/helpers.go:20`. Verified by reading.

```go
ip := c.Request().Header.Get("X-Forwarded-For")
if ip != "" {
    return strings.TrimSpace(strings.Split(ip, ",")[0])
}
```

The leftmost entry is the one the *client* supplied. Each proxy appends, so the
trustworthy value is at the right-hand end, counted in from however many hops
you control.

Consequences: the IP-keyed throttle at `password.go:84`
(`rateLimitScopePasswordIP`) is bypassed by rotating a header, which makes
`/password/forgot` an unmetered outbound-mail amplifier; and the IP recorded in
the sign-in audit log is forgeable, which is a bad property for an audit log.

Fix is `echo.ExtractIPFromXFFHeader(...)` with the trusted ranges set from the
real ingress topology, rather than hand-parsing. Unfixed for the reason it is
easy to get wrong: it needs someone to confirm the deployed proxy chain first,
and guessing at that is worse than leaving it.

## 3. `POST /oauth/login` has no throttle

Verified: `grep rateLimitExceeded cmd/api/handlers/oauth_login.go` returns
nothing, while `password.go` uses the limiter in four places.

Every other credential-checking endpoint here is
throttled; the front door is not. That is unlimited password guessing against a
known address, plus a cheap CPU-exhaustion vector, since each attempt costs one
bcrypt comparison.

Two counters, identifier and IP, placed **before**
the user lookup and before `CheckPassword` so a throttled attempt costs a Redis
`INCR` rather than a round trip and a key derivation. Clear them once the
password *and* the enabled checks pass — not straight after the password check,
since a correct password for a disabled account is not a successful sign-in.

This also closes a compounding gap: `pendingTOTPState.Attempts` caps passcode
guesses, but restarting step 1 mints a fresh state with `Attempts: 0`, so
unlimited password submissions mean unlimited batches of guesses.

Give it its own entry in the `rateLimits` map — that per-scope design is already
here and is the better of the two designs.

## 4. The refresh grant never re-checks `user.Enabled`

Verified: `grep -n "Enabled" data/services/oauth_token.go` returns nothing, while
`oauth_login.go:150` and `:229` both check it.

A refresh token lives for its full lifetime and each rotation stamps a fresh
expiry, so anything the refresh path declines to re-check is frozen at sign-in
time. Disabling an account therefore does not stop it minting access tokens until
someone explicitly revokes the refresh token.

The fix is a `refreshAdmitsAccount` free function, called after the
user lookup and before the client-admission gate, so it is unit-testable without
faking the whole grant. Note it deliberately does **not** revoke the token on
refusal — a disabled-then-re-enabled account keeps working instead of forcing
everyone to sign in again.

Anahita has no organization concept, so this is the person check only.

## 5. `PATCH /totp` stamps freshness with no passcode — latent here

`cmd/api/handlers/totp.go:92` sets `TotpLastVerifiedAt = time.Now()` while
verifying no passcode.

**But it is currently harmless here, and that is the interesting part.** Grepping
the whole repository, `TotpLastVerifiedAt` is written in three places and
persisted in two, and **nothing reads it** — no `IsMfaExpired` equivalent, no
gate anywhere. So today it is a field with no consumer.

That makes this a trap rather than a bug. The moment anyone adds a freshness gate
reading it — a data export, a destructive admin action, a step-up requirement —
the field is already untrustworthy, and the gate will look correct while proving
nothing. In the sibling exactly that had happened: it gated data export and
passkey revocation.

Cheapest fix: move the stamp to where a passcode is
actually verified. `totp.go:63` already does verify one; the enrolment endpoint
at `:92` should stamp nothing.

While there: check whether enabling TOTP requires a secret to exist. In
the sibling it did not, so a direct call could set `TotpEnabled` on an
account with nothing able to generate passcodes — a self-inflicted lockout.

---

## Not checked

Everything else in that review. Specifically **not** looked at
here: JWT claim validation (`iss`/`aud`/`exp` handling in `ValidateAccessToken`
and the middleware), whether reset tokens are stored hashed, the consent CSRF
comparison, session encryption mode, PKCE `plain`, public-client authentication
on introspect/revoke, and the signing-key verifier's treatment of deactivated
keys.

Several of those were real in the sibling and the two codebases share
ancestry, so they are worth a pass rather than an assumption. The full write-up
lives with that codebase, not here.

---

# gRPC TLS and authentication — do this here first

Added 2026-09-09. **This is where the work starts**, deliberately: anahita is
the smaller system, so it is the better place to learn the moving parts before
the same change lands in the sibling. Once it works here and the operational
shape is understood, port it.

Nothing below is implemented.

## The current state, verified

Three gRPC servers, all bare:

- `file-grpc-service/cmd/server/main.go:48`
- `graph-grpc-service/cmd/server/main.go:88`
- `mail-grpc-service/cmd/server/main.go:61`

```go
s := grpc.NewServer()
```

Seventeen client dial sites, all in `anahita-libs/data/services/*_grpc.go`, all:

```go
opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
```

No TLS, no interceptor, no per-call identity. Every method is callable by
anything that can reach the port. At roughly a
third the size — three certificates instead of nine, which is the main reason to
start here.

## Two separable pieces — do them in this order

**Authentication first, encryption second.** They solve different problems and
the first is much cheaper. In a private cluster, "anything can *call* these
methods" is the real exposure; "anything can *read* the traffic" needs a
foothold that already implies worse access.

### 1. A server interceptor with a shared secret

Roughly 50 lines in `anahita-libs`, plus one line per server and one per client.

- A `UnaryServerInterceptor` reading a token from call metadata, compared with
  `subtle.ConstantTimeCompare`, rejecting with `codes.Unauthenticated`.
- A matching `UnaryClientInterceptor` attaching it.
- The secret from the existing k8s Secret; a fixed dev value in
  docker-compose so local work is unaffected.
- Add a **streaming** interceptor too, or streaming RPCs bypass the check
  entirely — an easy and total gap.

Worth doing even if TLS never follows: it converts "anything in the namespace"
into "anything holding the secret."

### 2. mTLS

This is the part to spend the learning time on, because the certificates are the
work and the Go code is not.

- **Issuance.** cert-manager with a self-signed root: a `selfSigned` Issuer mints
  a CA, a second `ca` Issuer signs per-service `Certificate` resources.
  No public CA — nothing outside the cluster ever validates these.
- **SANs must match the dialled name exactly.** Both the short service name and
  the fully-qualified `<svc>.<ns>.svc.cluster.local`. Go verifies the name that
  was dialled, not the one that was meant, and a mismatch reads like a config
  error rather than a certificate one.
- **Rotation is the part that bites.** cert-manager renews at two-thirds of
  lifetime and rewrites the Secret; the kubelet updates the mounted files within
  about a minute. But grpc-go has already read the certificate into memory and
  holds it for the process lifetime. Either implement `GetCertificate` /
  `GetClientCertificate` callbacks that re-read from disk (with an mtime check
  and a cache — not a disk read per handshake), or restart pods on Secret change.
  Get this wrong and it fails weeks later, at renewal, across everything at once.
- **Local docker-compose needs a plan.** All three gRPC services run locally.
  Either generate a dev CA and mount it, or gate TLS on an env var — and if the
  latter, make the production path fail closed rather than silently downgrading.
  The sibling had exactly that bug in its session backend, where a missing
  `REDIS_ADDR` quietly selected a weaker implementation.

**Also evaluate Linkerd before hand-rolling.** It provides mTLS with zero Go
changes, rotates certificates itself on a 24-hour cycle, derives identity from
the Kubernetes ServiceAccount, and does not touch local development. The trade is
"own a certificate pipeline" versus "run one more platform component". Worth
trying both here, where the cost of being wrong is low — that comparison is
most of what makes this a useful learning exercise.

## What to carry outward

Record, as you go:

- Which issuance path you chose and why.
- What rotation actually did at the first renewal — that is the claim hardest to
  verify from documentation.
- How much the local development story cost.
- Whether the interceptor alone would have been enough.

The sibling has nine gRPC servers and twelve client sites, and its
`docs/plans/network-policy-grpc.md` holds the parked plan for the network-level
control there. Both belong in the same conversation once this is understood.

---

## Estimate (added 2026-09-28)

| Part | Roadmap item | Time |
| --- | --- | --- |
| Finding 1, `getIPLocation` rewrite | 0b | 2 h |
| Finding 2, `X-Forwarded-For` walked right to left with trusted proxies | 0b | 1 h |
| Finding 4, the refresh grant re-checks `Enabled` | 0b | 1 h |
| Finding 5, `PATCH /totp` stops stamping freshness without a passcode | 0b | 1 h |
| gRPC shared-secret interceptors (unary and streaming, server and client), only if still wanted beside mTLS | 0c | 2–3 h |
| gRPC mTLS: cert-manager issuers and certificates, reloading certificates, dev CA; or Linkerd, compared | 0c | 6–10 h |
| **Total** | | **about 13–18 h, 3–5 sessions** |

## Documentation

A phase is finished only when these pages match the code. When a page is added, add it to
`docs/README.md` too.

- **0b:** services `configuration.md` (how client addresses are resolved, and the location lookup).
- **0c:** services `architecture.md` (internal gRPC calls are authenticated and encrypted); services `configuration.md`; services `deploying.md`
  (cert-manager issuers and rotation, or Linkerd).

## What was built for 0b (2026-10-03)

- **Finding 1, `getIPLocation`:** rewritten after the sibling's fix (`auth-service/cmd/api/handlers/helpers.go`). `net.IP` predicates instead of string prefixes, carrier-grade NAT added, a 3 second timeout, a typed decode, and a return on every failure. A malformed address is no longer called local. The call is synchronous on token issuance (`buildAuthLog`), so the timeout matters.
- **Finding 2, client address:** `main.go` installs `echo.ExtractIPFromXFFHeader()` as the `IPExtractor`, and `getClientIP` is now `c.RealIP()`. The sign-in log (`auth_log.go`) already used `c.RealIP()`, which was forgeable for the same reason until the extractor was set.
- **Finding 4, refresh grant:** `refreshAdmitsAccount` in `data/services/oauth_token.go` refuses a disabled, deleted or archived account before the token is claimed, without revoking it. One error for every reason, so the response doesn't describe the account's state.
- **Finding 5, TOTP freshness, plus two faults found beside it:**
  - `PATCH /totp` no longer stamps `TotpLastVerifiedAt`; `POST /totp/recovery-codes` does, because that is where a passcode is checked.
  - `PATCH /totp` now answers 409 unless a secret and recovery codes exist. Before, a direct call enabled 2FA on an account with nothing able to produce a passcode.
  - `POST /totp` now answers 409 while 2FA is on. It replaces the secret and clears `TotpEnabled`, so a password alone switched a working second factor off. It also clears recovery codes left by an abandoned enrolment.
- **Tests:** the extractor through both proxy chains, the lookup's failure modes against a local server, the refresh check, and the TOTP enrolment rules through the handlers.

### Still open
- **Finding 3** (login throttle) was done with roadmap item 6 on 2026-10-03: 10 tries per identifier and 50 per address in 15 minutes, counted before the account is looked up. See `docs/signing-in.md`.
- **0c** is done: see below.
- **The "Not checked" list above** (JWT claim validation, PKCE `plain`, and the rest) is still not checked.

## What was built for 0c (2026-10-03)

**The choice: certificates loaded by the services, issued by cert-manager.** Linkerd was the alternative. It needs no Go code and suits a site with services in many languages, but it is one more system for every site to run, and it exists only on Kubernetes, so docker-compose would have stayed plain text and the secured path would never have run during development. `GRPC_TLS=off` leaves the door open for a site that prefers a mesh.

- **`anahita-libs/grpctls`:** `ServerOption()` and `DialOption()`, used by the three gRPC servers and all 17 clients. TLS 1.3, both sides present a certificate, and the client checks the server's certificate names the host it dialled.
- **Settings:** `GRPC_TLS` (`on`/`off`; unset means on, except with `ENV=dev`) and `GRPC_TLS_DIR` (default `/run/secrets/grpc`, holding `tls.crt`, `tls.key`, `ca.crt`, the names cert-manager writes).
- **Fails closed:** with TLS on and no certificates a server refuses to start and a client refuses to call. An unknown `GRPC_TLS` value is an error.
- **Renewal without a restart:** the files are looked at once a minute at most. A replaced certificate, and a replaced authority, are used from then on. A renewal caught half-written keeps the working certificate.
- **docker-compose:** `scripts/grpc-certs-init.sh` makes a development authority and one certificate in `./grpc-certs` (ignored by git), run by `make up` and `make up_build`. The authority's key is deleted after signing. Every service has `GRPC_TLS=on` and the mount.
- **Kubernetes:** `k8s/base/grpc-tls.yml` has a self-signed issuer, the authority, a certificate for each gRPC service and one shared by the callers. 90 days, renewed 30 days early. `make k8s-start` installs cert-manager.
- **No shared-secret interceptor.** The certificate does that job.

**Answers to "what to carry outward":**

- **Issuance path:** cert-manager with its own authority, as above.
- **What rotation did:** on a minikube cluster both the graph-grpc certificate and the callers' certificate were renewed by hand. cert-manager wrote the new Secrets within seconds. person-service logged the change after about one minute and graph-grpc-service after about two (the kubelet's file sync plus the one-minute check). Neither restarted, and 40 requests made through the gateway during the change all succeeded.
- **Local development cost:** one script, one mount per service, and starting with `make` instead of `docker-compose up`.
- **Would the interceptor alone have been enough?** For "anything in the namespace can call", yes. It would not have encrypted the calls, and the secret would have been one value shared by everything and never rotated.

**Tests:** `anahita-libs/grpctls` covers both sides connecting, a plain-text caller, a caller with no certificate, a caller and a server from another authority, a server under another name, renewal, a replaced authority, a broken renewal, and the settings. By hand: the dev stack serves requests with TLS on, and a plain-text call to graph-grpc's published port is refused.

**Not done:**

- **Which service may call which.** Any certificate from the authority is accepted everywhere. Per-service certificates and an allow-list would be the next step.
- **A real 60-day renewal** has not been waited for; the forced one stands in for it.
- **Load balancing across graph-grpc replicas** still depends on the cluster's Service, which pins a long-lived gRPC connection to one pod. It matters only once graph-grpc runs several replicas.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-08, scheduled 2026-09-28 |
| 0b | done 2026-10-03, on branch `auth-hardening` |
| 0c | done 2026-10-03, on branch `m2-foundations` |
| The "Not checked" list | open |
