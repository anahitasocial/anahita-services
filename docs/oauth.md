# OAuth scopes and clients

[Documentation](README.md) › OAuth scopes and clients

How scopes are defined and enforced, and how first-party clients are seeded.

The OAuth routes are in the API reference: [auth-service](api/index.html#auth-service), from [`/oauth/authorize`](api/index.html#getOauthAuthorize) and [`/oauth/token`](api/index.html#postOauthToken) on.

## Contents

- [Scopes](#scopes)
- [Seeding clients](#seeding-clients)
- [Where the code is](#where-the-code-is)

## Scopes

Scope identifiers are defined **once**, in Go, at
`anahita-libs/constants/oauth_scopes.go`. Every permission check must
reference an `OAuthScope*` constant — never a string literal. `make test`
runs a guard that walks the whole workspace and fails on any
`HasScope("...")` / `RequireScope("...")` with a literal argument.

That guard exists because the fleet once carried three generations of
scope names simultaneously. Since `HasScope` is a plain slice lookup, a
literal matching no constant fails closed and silently — the gate returns
"insufficient scope" forever, for everyone, with nothing in the logs
naming the cause. The compiler catches a mistyped constant; only the
guard catches a reintroduced literal.

Scopes describe **API surface**. Authority (admin / super-admin) is a
separate role check inside each handler's permission method, layered on
top of the write scope. There is deliberately no `admin:*` scope tier.

A scope family covers its sub-resources rather than giving each one its
own scope: `actors:*` covers avatars, covers, access, features and group
admins; `media:*` covers photos, articles, notes, topics and their
hashtag/location tags. Reads of public content need no scope at all — a
read scope gates the authenticated-visibility uplift, not the public
surface — which is why `taxonomy` has no read half.

A service with no family of its own borrows the one its actions belong to.
Filing an abuse report needs `account:write`: it is something a person does
on their own account's behalf. Reading and deciding reported cases needs
`actors:read` and `actors:write`, as the signup queue does.

The current catalog: `openid`, `profile`, `email`, `offline_access`,
`actors:read`, `actors:write`, `media:read`, `media:write`,
`likes:write`, `socialgraph:read`, `socialgraph:write`,
`notifications:read`, `notifications:write`, `taxonomy:write`,
`account:read`, `account:write`, `oauth:read`, `oauth:write`.

There is no scope for replies. A reply is a post, written under `media:write`.
`comments:write` was removed with comments: a client registered with it has to
be registered again without it, and the seeded clients are in
`auth-defaults/oauth_seed_clients.json`.

`oauth:*` is the sole super-admin-only family. A token carrying
`oauth:write` is equivalent to owning the identity provider — its holder
can add a redirect URI to any client and harvest authorization codes for
every person on the site.

## Seeding clients

`auth-defaults/oauth_seed_clients.json` bootstraps the first-party
clients (`anahita-web`, `anahita-mobile`) on first boot.

**The seed file is bootstrap-only, not a source of truth.** A client_id
that already exists is left untouched, so editing `scopes` in this file
and restarting does *nothing* — the only signal is a single
`already exists, skipping` log line. That is the right default: it
guarantees a redeploy never silently reverts an edit made through the
`/clients` admin endpoint.

To apply a changed shipped scope list:

~~~sh
docker compose exec mysql mysql -uroot -proot anahita \
  -e "DELETE FROM oauth_clients WHERE client_id = 'anahita-web';"
docker compose restart auth-service
~~~

This removes the client's row and nothing else: the schema has no foreign
keys, so its refresh tokens and authorizations stay. They name the client by
its `client_id`, so when the restart seeds the client again, they work with it
as before, and nobody is signed out. On a live site, `PATCH /clients/:id`
instead.

`DELETE /clients/:id` is different: it revokes the client's refresh tokens and
deletes its authorization codes, then the client. Everybody signed in with it
is signed out within 15 minutes, when their access token expires, and a client
made later under the same `client_id` starts with no tokens.

Scopes are validated against the catalog both at boot (a typo fails
startup) and at the `/clients` BREAD boundary (a bad scope returns 400
naming it).

## Where the code is

| What | Where |
| --- | --- |
| The scope catalog | `anahita-libs/constants/oauth_scopes.go` (`OAuthScopeDefinitions`, `OAuthScopeExists`) |
| The guard against scope literals | `anahita-libs/constants/oauth_scopes_guard_test.go` (`TestNoScopeStringLiterals`, `TestOnlyOAuthScopesAreSuperAdminOnly`) |
| Checking requested scopes | `auth-service/data/services/oauth_scopes.go` (`ValidateScopes`, `ValidateScopesForClient`) |
| Seeding clients | `auth-service/data/services/oauth_seed.go` (`SeedDefaultClients`), `auth-defaults/oauth_seed_clients.json`; setting `OAUTH_SEED_CLIENT_PATH` |
| The `/clients` endpoints | `auth-service/cmd/api/handlers/oauth_clients.go` (`DeleteClient`, tested in `oauth_clients_test.go`), `auth-service/cmd/api/permissions/oauth.go` |
| Clients and what hangs off them | tables `oauth_clients`, `oauth_authorizations`, `oauth_refresh_tokens` |
