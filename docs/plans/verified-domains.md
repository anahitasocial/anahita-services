# Verified domains (roadmap row 63)

Planned on 2026-10-02 as item 26, taken out of M9 on 2026-10-13, and
planned again on 2026-11-23, at the user's word: "I prefer the domain
verification approach, it's the most solid for profile verification." A
profile proves it holds a domain by a DNS record, and shows it with a mark:
a school's, a newsroom's or a person's own site, said by the one place
nobody else can write.

## Status

| # | What | Status |
| --- | --- | --- |
| 1 | `verified_domains`, the DNS check, its routes; `verified` set by the check alone | ⬜ |
| 2 | Checked again every day, as a channel's is; lapsed after three failures, its owner told | ⬜ |
| 3 | Settings: the record to add, Check now, its state; the badge beside the name, the domain in its tooltip and the About card | ⬜ |
| 4 | The fediverse: the domain sent as a profile link Mastodon can tick | ⬜ |
| 5 | The docs; tried with a real domain's record; changelogs | ⬜ |

## Contents

1. [The proof](#the-proof)
2. [What a verified domain does](#what-a-verified-domain-does)
3. [What exists](#what-exists)
4. [Out of scope](#out-of-scope)
5. [Settled](#settled)
6. [Open](#open)
7. [Documentation](#documentation)

## The proof

1. **One domain for each profile**, a person's, a group's or a channel's,
   by whoever may edit it. `example.org`, or a name under it,
   `news.example.org`. Stored in punycode; shown in Unicode only when each
   of its labels is in one script, the usual defence against look-alikes.
2. **A TXT record, either of two:**
   1. `_anahita.<domain>`, holding the token, `anahita-` and 32 hexadecimal
      characters, made for that profile and that domain: the record a
      channel proves its website's domain with already
      ([Channels](../channels.md#proving-a-domain)), through the same
      `anahita-libs/domains`, built to be shared. A channel may also prove it
      by a file on its site; a profile by the record alone, the most solid
      (the user, 2026-11-23);
   2. `_atproto.<domain>`, holding `did=<their DID>`, for a person whose
      connected Bluesky account already uses the domain as its handle: the
      record Bluesky asked of them proves it here too, with nothing to add.
3. **Looked up as a channel's is** (`domains.Checker`), and rate-limited,
   as lookups are.
4. **One profile for each domain:** a domain verified by one is refused to
   another (`domain_taken`).
5. **Checked again every day**, as a channel's domain is. Three failures in
   a row lapse it: the mark goes, its owner is told by a notification, and
   it can be checked again at any time.
6. **`nodes.verified`**, kept since 2026-10-02 for this, is set by the
   check alone; an edit of the profile no longer sets it.

**Routes** (actor-service, the same for each kind of profile, as
`graph-access` is):

| Route | What |
| --- | --- |
| `GET /:kind/:id/domain` | The domain, its state and the records that prove it |
| `PUT /:kind/:id/domain` `{ "domain": "example.org" }` | Starts, or changes, the domain: a new token; after step-up |
| `POST /:kind/:id/domain/check` | Checks now |
| `DELETE /:kind/:id/domain` | Takes it off; after step-up |

## What a verified domain does

1. **Here: a badge beside the name** wherever the name is shown, the
   profile's header, cards and posts, saying the profile is verified; the
   domain in its tooltip and in the About card (the user, 2026-11-23). The
   handle stays as it is: a domain in its place would read as a Bluesky
   handle, which a server of the fediverse cannot look up, and would hide
   the handle people elsewhere follow by. The profile's answer carries
   `verified_domain`.
2. **On the fediverse:** the profile is sent with the domain as a profile
   link (a `PropertyValue`), as Mastodon shows profile links. Mastodon ticks
   it itself when the site links back to the profile with `rel="me"`; the
   settings say so in a line, with the link to paste.
3. **On Bluesky:** nothing is changed from here. Settings say how to use
   the domain as a Bluesky handle, in Bluesky; done, the `_atproto` record
   proves it here as well.

## What exists

1. `nodes.verified`, returned on profiles, set by nobody yet.
2. `website_url`, one link a profile gives: unchanged, and not proof.
3. `anahita-libs/domains`: the token, the record's name, the check, as a
   channel's website uses them, and its daily check again.
4. The notifications, step-up and the shared rate limiter.

## Out of scope

1. Proving a domain by a link on the site (`rel="me"`): Mastodon does that
   itself, and DNS is what the user chose.
2. Setting a Bluesky handle from here: whether an app password may is not
   settled, and Bluesky's own app does it.
3. Marks for accounts from elsewhere, and a WebFinger file for one's own
   domain.

## Settled

1. **A badge beside the name, everywhere the name is** (the user,
   2026-11-23), not the domain as a handle.
2. **People, groups and channels** (the user, 2026-11-23), by whoever may
   edit the profile.
3. **A channel's feed** (the user, 2026-11-23, "a valid Atom or RSS feed is
   also validation"): open, below.

## Open

1. **A feed as a channel's proof.** A feed is public: anybody can connect
   one, so a valid feed on a domain proves the feed is there, not that the
   channel's admins hold the site. Proposed: the channel's token inside its
   feed (in its description, say) proves it, as only whoever writes the
   feed can put it there; for a blog, a setting away.

## Documentation

1. anahita-services: `docs/profiles.md` (the domain and its routes),
   `docs/federation.md` (the profile link), `docs/channels.md` (the proof,
   shared), `docs/features.md` ("Verified profile links": yes), the API
   reference, `CHANGELOG.md`.
2. anahita-web: `docs/architecture.md` (Settings, the mark), `CHANGELOG.md`.
