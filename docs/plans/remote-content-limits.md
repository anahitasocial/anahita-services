# Keeping what comes from other networks within bounds (item 47)

Raised by the user on 2026-10-13: "We may want to clip the imported content
from the fediverse and limit them to the latest 100 from each outside
profile. It should be a number set by the sys admin", and then: "for a
small budget operation they could easily get overwhelmed". Planned, not
built.

## Why

Nothing that arrives from Mastodon or Bluesky is ever removed. Every post
of every account followed from here is kept, with its pictures copied into
the installation's storage at every size a photo is kept at, for good. A
site of a few people who each follow a few hundred busy accounts stores
far more of other people's posts than of its own, and pays for it. An
installation is not meant to be an archive of other networks.

The aim is that a small installation is safe **without its operator doing
anything**: the limits are on from the first day, with numbers a household
could afford, and a larger site raises them.

## What limits there are

Each is a setting of the installation's, the same for both networks.

| Setting | Says | Default suggested |
| --- | --- | --- |
| `REMOTE_POSTS_KEPT` | The most posts kept from one outside profile, newest first. `0` for no limit | 100 |
| `REMOTE_POSTS_KEPT_DAYS` | Nothing from elsewhere older than this is kept. `0` for no limit | 365 |
| `REMOTE_UNFOLLOWED_DAYS` | How long the posts of a profile nobody here follows any more are kept | 30 |
| `REMOTE_BACKFILL` | How many of somebody's earlier posts are read when they are first followed | 20, as now |
| `REMOTE_PICTURES` | Which sizes of a picture from elsewhere are stored: `all`, or `small`, which leaves out the largest and shows it from the size below | `all` |

## What is never removed

Whatever the numbers say:

- a post somebody here liked, reposted, quoted, saved or answered;
- a post in a thread somebody here wrote in;
- a post an open abuse report is about;
- anything written here. Only what has a `uri`, and so came from
  elsewhere, is ever looked at.

Otherwise a like or a reply here would point at nothing.

## What goes with a post that is removed

Its pictures in storage, at every size; the picture of its link's card,
when no other post has the same card; its hashtag edges; and news of it in
anybody's notifications.

## Where it runs

In the sweeper that already runs each hour (`graph-grpc-service/sweeper`),
under its lock, a few hundred at a time and oldest first, so that it never
holds up the site and a long backlog is worked off over several runs.

The profile of somebody elsewhere says, where the list ends, that earlier
posts are on their own server, with the way there.

## Profiles nobody follows

When the last person here stops following an outside profile, its posts
are removed after `REMOTE_UNFOLLOWED_DAYS`. The profile itself is kept
while anything here still refers to it (a like, a reply, a block), and is
removed with its pictures when nothing does.

## For the recipes (item 40)

Each recipe sets these for its kind of site: a household low, a newsroom
high. They belong with the sizes in `k8s/`.

## To decide when it is built

- Whether the limits are on by default (assumed above) or off until an
  operator sets them.
- Whether an administrator sees, in the admin area, how much of the
  storage is other networks' and how much is the site's own. It is what
  an operator would want to know before choosing a number.
- Whether a person may ask for more to be kept of a profile they care
  about. Not assumed.

## Size

About two sessions: the sweep and what it must not touch, with tests
against a real database; removing the stored files; the settings and
their documentation; the words on a profile.

## Depends on

Rows 25 and 26 (what there is to limit). The migration baseline (row 27)
comes after this, not before: it is last in M10, so that it takes in the
migrations of everything before it.
