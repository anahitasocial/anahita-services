# Anahita as an AT Protocol server (item 51)

*Recorded on 2026-10-14 at the user's asking. **To be decided, in the
future**: not scheduled, and not to be built until the user takes it up
again. See the web app's roadmap for where it sits.*

## The question

Asked by the user on 2026-10-14: how much work would it be for Anahita to be
an atproto server, the way it is an ActivityPub one, and can it be both?

## Where things stand

- **On the fediverse Anahita is a server.** Anybody who turns federation on
  is `@name@your-site.example`, with a signing key Anahita keeps; other
  servers deliver to its inbox and it delivers to theirs.
- **On Bluesky Anahita is an app.** A person connects the Bluesky account
  they already have (a handle and an app password), and Anahita writes to
  their own data server (PDS) as them and listens for what comes back. It
  hosts nobody's repository and gives nobody a Bluesky identity. This was
  the user's choice when the Bluesky work was built, over running a PDS as
  first planned (`federation.md`). Somebody with no Bluesky account is not
  on Bluesky at all.

## What it would be

Being both is possible, and Anahita's layout helps: each network already
has a service of its own. A person would have two identities,
`@ana@your-site.example` on the fediverse and `ana.your-site.example` on
Bluesky.

Anahita would not write its own atproto server. It would run Bluesky's own
PDS beside the services, and `atproto-service` would look after it:

1. **The PDS** at `pds.your-site.example`, with its files in the
   installation's S3-compatible storage and not on disk (the PDS can do
   this, and the rule against local file storage holds).
2. **Handles under the site's domain:** `name.your-site.example`, proven by
   `/.well-known/atproto-did` on each subdomain, with wildcard DNS and a
   wildcard certificate.
3. **Accounts made on request:** turning Bluesky on creates the person's
   account on the site's PDS, with a DID, its credentials kept encrypted as
   connected accounts' are now. Somebody who already has a Bluesky account
   may still connect it instead.
4. **Writing and reading: mostly built.** Posts, pictures, replies, likes,
   reposts and follows are written to a PDS already, and what comes back is
   listened for; hosted accounts would be written to the site's own.
5. **Joining the network:** Bluesky's relay asked to crawl the site's PDS.
6. **Leaving with one's identity.** By default the PDS, which is to say the
   operator, holds each person's identity keys. A person should be able to
   add a recovery key of their own and move their account, identity and
   all, to another server: owning it as an app built on atproto lets them
   own it.

## Size

About **5–7 sessions**, less than first planned because the writing and the
listening exist.

| Part | Sessions |
| --- | --- |
| The PDS in compose and Kubernetes, files in S3, TLS, wildcard handles, its email | 1½–2 |
| Accounts on request, hosted or connected, deleting and purging, moving away with one's identity | 1½–2 |
| The existing writer and listener for hosted accounts, the relay's crawl | 1 |
| Trying it on the real network through a public address, and documentation | 1 |

## Before it is taken up

- **Whether Bluesky lets it grow.** As understood on 2026-10-14, Bluesky's
  relay limits how many accounts a new independent PDS may host until it is
  trusted. Worth building when Bluesky opens the network further to servers
  like this one; to be checked again then, from Bluesky's own documentation.
- **Perhaps a repository of its own,** an optional part an operator adds,
  rather than inside anahita-services: the PDS is a separate program
  (Node.js, with its own database) with its own upgrades and backups, and
  most installations, small ones above all, would leave it off.
- **Testing needs a public address kept for good.** The relay must reach the
  PDS, and identities made on Bluesky's directory (PLC) are as good as
  permanent, so a hostname used for trying it must never be reused.
- **Public only.** atproto repositories are public: a members-only site
  leaves it off, as it does ActivityPub, and only public posts go to it.

## Depends on

Row 26 (AT Protocol, done) and row 35 (Bluesky leftovers, done). Nothing in
M10.
