# OAuth scopes and clients

[Documentation](README.md) › OAuth scopes and clients

How scopes are defined and enforced, and how first-party clients are seeded.

## Contents

- [Scopes](#scopes)
- [Seeding clients](#seeding-clients)

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
`comments:write`, `likes:write`, `socialgraph:read`, `socialgraph:write`,
`notifications:read`, `notifications:write`, `taxonomy:write`,
`account:read`, `account:write`, `oauth:read`, `oauth:write`.

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

This cascades to that client's refresh tokens and authorizations, logging
every one of its users out. On a live site, `PATCH /clients/:id` instead.

Scopes are validated against the catalog both at boot (a typo fails
startup) and at the `/clients` BREAD boundary (a bad scope returns 400
naming it).
