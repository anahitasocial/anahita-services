# Keeping what comes from other networks within bounds (item 47)

Raised by the user on 2026-10-13: "We may want to clip the imported content
from the fediverse and limit them to the latest 100 from each outside
profile. It should be a number set by the sys admin", and then: "for a
small budget operation they could easily get overwhelmed". Planned, not
built. Planned again the same day, after looking at what other projects
do: by age more than by number, and pictures and videos before words.

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

## What other projects do

- **Mastodon** treats the files and the words apart. Its copy of a picture
  or a video from another server is deleted after a number of days the
  administrator sets; the post stays, with where the file came from, and
  the file is fetched again when somebody asks for it. A second setting
  deletes old posts from other servers outright, which cannot be undone
  and which its own documentation is careful about.
- **GoToSocial** does the first of those, and has it on as it comes.
- **Pleroma and Akkoma** have an age for posts from elsewhere and a
  command an administrator runs to prune them.
- **Bluesky** does not have the problem: a personal server holds only its
  own people's data.

The shape of each is certain; the numbers each ships with were not checked
and are not quoted.

**What is taken from them:** Mastodon's first setting, on by default as
GoToSocial has it. It removes what costs and keeps what does not, and it
loses nothing that is still there to be had.

## The files: pictures and videos

A file is nearly all of what a post from elsewhere costs to keep. A
picture is kept at five sizes; a video is larger than any picture.

1. **Removed after an age, and copied again when asked for.** A picture of
   a post from elsewhere is deleted from storage once it is older than
   `REMOTE_MEDIA_DAYS`. The post stays, with its words, what each picture
   showed, and where each came from. When somebody opens the post, the
   picture is copied here again, as it was the first time: checked,
   written out at this installation's sizes, and never shown from where it
   came from.
2. **When it cannot be had again,** because its own server has deleted it
   or is gone, the post says so in the picture's place, with what the
   picture showed if its author said.
3. **This needs where each file came from to be kept,** which it is not
   today: one more column on `photo_files`, set when the file is first
   copied. For a file from Bluesky it is what the address is made from,
   the account and the file's name there.
4. **Avatars and covers** of people elsewhere are small, and one of each:
   kept while the person is kept.
5. **The picture of a link's card** is the link-preview service's, which
   keeps and forgets its own.

**Videos.** No video from elsewhere is kept today: a post with one is kept
for its words. When they are brought in, they come under this from the
start, and more strictly, since one video can weigh what a thousand
pictures do:

- **not copied when the post arrives,** only its poster picture;
- **copied when somebody first presses play,** through the video worker,
  as an upload is;
- **kept for a shorter age of its own** (`REMOTE_VIDEO_DAYS`), and copied
  again if asked for after that;
- **with a limit on length and weight** above which the post leads to
  where the video is, and it is not copied at all.

A video is never played straight from another server: that tells the
other server who watched.

## The words

Kept much longer than the files, since they cost almost nothing, and
clipped only as a second layer:

| Setting | Says | Default suggested |
| --- | --- | --- |
| `REMOTE_MEDIA_DAYS` | How long a picture from elsewhere is kept before it is removed, to be copied again when asked for. `0` for no limit | 30 |
| `REMOTE_VIDEO_DAYS` | The same for a video, once videos from elsewhere are kept | 7 |
| `REMOTE_POSTS_DAYS` | Nothing from elsewhere older than this is kept at all. `0` for no limit | 0: off |
| `REMOTE_POSTS_KEPT` | The most posts kept from one outside profile, newest first. `0` for no limit | 0: off |
| `REMOTE_UNFOLLOWED_DAYS` | How long the posts of a profile nobody here follows any more are kept | 30 |
| `REMOTE_BACKFILL` | How many of somebody's earlier posts are read when they are first followed | 20, as now |

The first is on as it comes, because it loses nothing. The two that
remove posts are off as it comes, because they cannot be undone: an
operator who needs them turns them on knowing that.

## What is never removed

Whatever the numbers say:

- a post somebody here liked, reposted, quoted, saved or answered, and its
  files;
- a post in a thread somebody here wrote in;
- a post an open abuse report is about;
- **anything written here.** Only what has a `uri`, and so came from
  elsewhere, is ever looked at.

## What is written here is not clipped

Asked by the user: only what comes from elsewhere, or what is written
here too? Only what comes from elsewhere. That is a copy, and the thing
itself is still on its own server. What somebody wrote here is theirs, is
the only one there is, and is not an operator's to delete by a setting: it
would undo what the site says of itself, that what you post stays yours.
None of the projects above do it.

What some of them have is each person's own choice to have their old posts
deleted after a while, with what is pinned or much liked left out. It is a
feature of its own and is not part of this.

## Where it runs

In the sweeper that already runs each hour (`graph-grpc-service/sweeper`),
under its lock, a few hundred at a time and oldest first, so that it never
holds up the site and a long backlog is worked off over several runs.

Copying a file again is asked of the service the post came through
(`activitypub-service`, `atproto-service`), which already knows how.

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

- Whether an administrator sees, in the admin area, how much of the
  storage is other networks' and how much is the site's own. It is what
  an operator would want to know before choosing a number.
- Whether a picture that was removed is copied again when the post comes
  into view in a feed, or only when the post is opened. Only when opened
  is assumed: a feed scrolled back a long way would otherwise copy
  everything again.
- Whether a person may ask for more to be kept of a profile they care
  about. Not assumed.

## Order of building

1. Where each file came from is kept; pictures are removed after their age
   and copied again when asked for. This is most of the saving, and can be
   on by default.
2. The posts of profiles nobody follows.
3. The two settings that remove posts, off as they come.
4. Videos, when videos from elsewhere are brought in at all.

## Size

About three sessions for the first three; the fourth belongs with
bringing videos in.

## Depends on

Rows 25 and 26 (what there is to limit). The migration baseline (row 27)
comes after this, not before: it is last in M10, so that it takes in the
migrations of everything before it.

## What was built

### Step 1, pictures: 2026-10-13

Built as planned, with one thing simpler than planned: **where each file
came from is not kept.** Copying a post's pictures again is reading the
post again from where it came from, which both services already knew how
to do, and which is right when a picture's address has changed. So there
is no migration.

- `anahita-libs/federation/media.go`: which posts are due
  (`OldPicturePosts`), removing (`Keeper.RemovePictures`), copying again
  (`Keeper.RestorePictures`), and what the post says of it in its meta
  (`media_removed`).
- Each service sweeps what came through it, every hour, and answers
  `POST /…/posts/:id/pictures`.
- `REMOTE_MEDIA_DAYS`, thirty unless said, `0` for never.
- The API sends `media_removed` with a post; the web app draws a
  placeholder with what the pictures showed and a button, and asks at
  once on the post's own page (`components/RemovedPictures.jsx`).

"Left alone" is by the post's `updated_at`, which is when it arrived, was
changed, or had its pictures copied again; not by when it was written,
which for a post read on a first follow can be years ago.

Tried on the dev database by making one post from Bluesky look forty days
untouched: its three pictures went from storage within the sweep, and the
post said so.

**Still to build:** steps 2 to 4.

