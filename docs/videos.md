# Videos

A video post is a post like any other: it has an audience, replies, likes,
a place in feeds. What is different is how it gets there. A video is too
big to be sent to the API, and nothing somebody uploads is ever shown as it
arrived, so a browser sends the file straight to storage, a worker makes
the files that are shown, and only then is there a post.

## Contents

- [The parts](#the-parts)
- [Sending a video](#sending-a-video)
- [What the worker does](#what-the-worker-does)
- [What a file has to be](#what-a-file-has-to-be)
- [The post](#the-post)
- [Playing it](#playing-it)
- [Sizes](#sizes)
- [GIFs and animated PNGs](#gifs-and-animated-pngs)
- [Downloading](#downloading)
- [Endpoints](#endpoints)
- [Settings](#settings)
- [Storage](#storage)
- [Running it, and not running it](#running-it-and-not-running-it)
- [What is not built](#what-is-not-built)

## The parts

| Part | What it does |
| --- | --- |
| `video-service` (port 8103, `/videos/`) | Begins and finishes uploads, says how each is getting on, and serves video posts: read, edit, delete, audience, replies, pin. Sends viewers to a video's files |
| `video-worker` | Takes each finished upload, checks it, makes its sizes and poster with ffmpeg, stores them, and makes the post. Answers no requests |
| `file-grpc-service` | Signs the addresses the browser sends parts to and the worker reads and writes at, and holds each to its place in storage |
| `video_uploads` | One row for each upload: the post as it will be, where the file is, how far it has got, and how many times it has been tried |

The two video containers are one module, `video-service`, with two
programs (`cmd/api`, `cmd/worker`) and two images. The worker's image has
ffmpeg in it and runs as a user of its own.

## Sending a video

1. `POST /videos/:owner_id/uploads` with what the post will say (`name`,
   `body`, `access`, `language`, `reply_access`, `quote_policy`) and the
   file's `size` and `content_type`. Everything about the post is settled
   now, by the rules for any post: whether this person may post on that
   profile, and to that audience. The answer has the upload's `id`, a
   `part_size` (8 MB) and a `part_count`.
2. `POST /videos/uploads/:id/parts` `{ "numbers": [1, 2, 3] }` answers with
   an address for each part. The browser `PUT`s each part to its address,
   in any order, again if one fails. Addresses are asked for a few at a
   time and are good for an hour.
3. `POST /videos/uploads/:id/complete` makes one file of the parts. **How
   much arrived is asked of storage, not of the browser**: a file past the
   limit is thrown away whatever size was given in step 1.
4. `GET /videos/uploads/:id` says how it is getting on: `uploading`,
   `processing`, `ready` with the post's `node_id`, or `failed` with why.

An upload can be given up at any point (`DELETE /videos/uploads/:id`). One
that is begun and never finished is thrown away after 24 hours.

There is no post until step 4 says `ready`. Nothing has to be hidden from
feeds in the meantime, and an upload that goes nowhere leaves nothing
behind.

## What the worker does

For each upload that is all there, one at a time:

1. Reads the file from storage, stopping at the largest a video may be.
2. Decides what it is; see [below](#what-a-file-has-to-be).
3. Makes each size, never larger than the video is: H.264 and AAC in an
   MP4 arranged so that it starts playing before all of it has arrived.
4. Makes the poster, a JPEG of a frame one second in.
5. Stores them under `assets/v1/video-service/videos/<profile id>/<upload id>/`.
6. Makes the post, as the person who uploaded it, and marks the upload
   `ready`.
7. Deletes the upload.

Several workers can run at once: each takes a different upload
(`SELECT … FOR UPDATE SKIP LOCKED`). A worker that dies leaves its upload
taken; another takes it once the first must be dead, which is after longer
than the work on one video is allowed to last. That allowance grows with
the longest video allowed and the sizes made.

| What went wrong | What happens |
| --- | --- |
| The file is not a video (`not_a_video`), or is too long (`too_long`), or too large (`too_large`), or is a GIF or PNG that does not move (`still`) | `failed`, for good, and the upload is deleted |
| Anything else: storage, the network, ffmpeg itself | Tried again, up to 3 times, then `failed` as `encoding` |
| The uploader gave it up meanwhile | What was made is deleted, and no post is made |

## What a file has to be

The file is a stranger's and nothing has looked at it.

- **Its first bytes say what it is**, and only these kinds are taken: MP4
  or QuickTime (an `ftyp` box), WebM or Matroska (EBML), and the two
  moving pictures, GIF (`GIF87a` or `GIF89a`) and animated PNG (a PNG that
  says how it moves, `acTL`, before its picture begins). This is read
  before ffmpeg is given the file. ffmpeg opens many kinds of file, and
  some of them are lists of other files to read and addresses to fetch.
- **ffmpeg is told which kind it is given** (`-f`), so it never guesses,
  and may open files and nothing else (`-protocol_whitelist file`).
- It must have a picture: sound alone, or a song with its cover, is not a
  video.
- It must be no longer than `VIDEO_MAX_SECONDS`, or, for a GIF or an
  animated PNG, `ANIMATION_MAX_SECONDS`.
- ffmpeg is run with nothing of the worker's environment but `PATH`.

**Nothing of the upload is kept.** Every file shown is encoded afresh with
no metadata copied over, so a title, a place or anything else written into
the upload stays in the upload, which is deleted.

What the browser says the file is (`content_type`) only turns away what
could never be a video. It decides nothing.

## The post

A video post is a node of type `node.medium.video-service.video.v1`. Its
files are described in its `meta` under `video`: the duration, each size
with its key in storage, and the poster. Nothing but the worker writes
that, and `anahita-libs/videos` is the one place that reads it.

Wherever a post is sent, a video post has:

```json
"video": {
  "duration": 61.5,
  "width": 1920,
  "height": 1080,
  "poster": "/videos/170301/files/poster.jpg",
  "files": [
    { "height": 1080, "width": 1920, "url": "/videos/170301/files/1080.mp4", "type": "video/mp4" },
    { "height": 720,  "width": 1280, "url": "/videos/170301/files/720.mp4",  "type": "video/mp4" },
    { "height": 360,  "width": 640,  "url": "/videos/170301/files/360.mp4",  "type": "video/mp4" }
  ]
}
```

It is in feeds, search and custom feeds (kind `video`) like any post, and
the web shell describes a public one at `/videos/<id>`.

Deleting the post deletes its files. So does purging the account that
posted it.

## Playing it

The addresses in `video` are paths under the API that stay the same. Each
is answered with a redirect to a signed address in storage, where the file
is read in ranges as a player asks.

**Who may see the post is asked every time** one of those paths is
followed. A file is never reached by somebody who may not see the post,
and an address does not go on working after the post is made private or
deleted. A player sends the viewer's session cookie with the request, as a
browser does for any picture or video on a page.

The signed address lasts 15 minutes. A player that asks for a new range
after that, on a video left paused, follows the path again.

## Sizes

`VIDEO_RENDITIONS` lists the sizes made, by height: `1080,720,360` unless
said. Up to `2160` is taken, which is 4K. A video is never made larger
than it is: one filmed at 720p has a 720p and a 360p file and no 1080p.
One smaller than every size listed is made once, at its own size.

## GIFs and animated PNGs

A GIF or an animated PNG is posted from the video composer, as Mastodon
does with a GIF: it becomes a short clip with no sound that plays on a
loop. It is a video post in every other way: who may see it, replies,
likes, downloading and deleting are the same.

- **It is held to less than a video:** `ANIMATION_MAX_BYTES` (25 MB) and
  `ANIMATION_MAX_SECONDS` (30), never more than a video's own limits.
  Beginning an upload checks the size it says, and the worker checks what
  the file really is and its real size again.
- **A picture that does not move is refused** with the reason `still`,
  which the web app turns into "post it as a photo". A GIF of one picture
  is one; so is a PNG with no `acTL`, which the worker reads from the
  file's chunks, since ffmpeg's reader of animated PNGs will not open it.
  A picture less than ten pixels high or wide is refused as not a video.
- **It is made once,** at its own size up to 720 high, in numbers that
  divide by two: H.264 with no sound, each picture shown for as long as
  it was (`-fps_mode passthrough`), and `+faststart`. A clip is a
  fraction of the GIF's size: a two-second test GIF of 100 KB came to
  16 KB.
- **What was see-through is laid on white,** since an MP4 and a JPEG have
  nothing see-through. That matters for stickers and not for the usual
  clip.
- **Its poster is its first picture,** on white.
- **The post says it loops:** `"loop": true` in `video`, in its `meta` and
  wherever it is sent. The web app plays it muted, on a loop, without
  controls, and still where the reader asked for less motion.

An **animated WebP is not taken**: ffmpeg 8.1, in the worker's image,
reads the file but not its pictures. A WebP is refused as not a video.

More and larger sizes cost in three places, and the settings go together:

| To allow | Also raise |
| --- | --- |
| 4K (`VIDEO_RENDITIONS=2160,1080,720,360`) | `VIDEO_MAX_BYTES`: ten minutes of 4K is several gigabytes. And give the worker processor: 4K takes about nine times as long to encode as 720p |
| Longer videos (`VIDEO_MAX_SECONDS`) | `VIDEO_MAX_BYTES`, and the worker's working space |

The player chooses between the sizes a video has. Nothing changes size
while it plays: that needs a different way of storing video (HLS), which
is not built.

## Downloading

As for a photo, and by the same rules: see
[Photo posts](photos.md#downloading).

- Whether a video may be downloaded is chosen when its upload begins
  (`allow_download`, or what the profile says when it is left out) and is
  carried to the post when the worker makes it. It is changed afterwards
  with `PATCH /videos/:id/download`.
- `GET /videos/:id/download` saves the largest size the video has, and
  `?size=720` another. What is downloaded is what the worker made: the
  upload itself was deleted.

## Endpoints

| Method and path | Who | What |
| --- | --- | --- |
| `POST /videos/:owner_id/uploads` | signed in, may post there | Begin an upload. With `parent_id`, a reply with a video or a GIF: `owner_id` is the uploader's own, no title is needed, and audience, reply rule and quoting are the thread's. The thread is asked before anything is sent, and the worker asks it again before making the reply; a thread closed meanwhile fails the upload as `closed` |
| `POST /videos/uploads/:id/parts` | the uploader | Addresses for some of its parts |
| `POST /videos/uploads/:id/complete` | the uploader | All sent: hand it to the worker |
| `GET /videos/uploads/:id` | the uploader | How it is getting on |
| `GET /videos/uploads` | signed in | What they have in hand: being sent, being made, or failed in the last day |
| `DELETE /videos/uploads/:id` | the uploader | Give it up |
| `GET /videos/:owner_id/` | anybody | A profile's videos |
| `GET /videos/:id` | whoever may see it | A video post |
| `GET /videos/:id/files/:name` | whoever may see it | Redirect to `1080.mp4`, `720.mp4`, `360.mp4` or `poster.jpg` |
| `PATCH /videos/:id`, `DELETE /videos/:id` | whoever may edit or delete it | Change its title and text; delete it and its files |
| `PATCH /videos/:id/access`, `/comment-status`, `/reply-access`, `/quote-policy`; `PUT`/`DELETE /videos/:id/pin` | as for any post | As for any post |
| `PATCH /videos/:id/download` | whoever may edit it | Whether it may be downloaded |
| `GET /videos/:id/download` | whoever may see it, where it is allowed | Redirect to the file, to be saved |

An upload belongs to whoever began it. Anybody else is told there is none.

Refusals when beginning an upload: `403 uploads_off`, `413 too_large`
(with `max_bytes`), `415 not_a_video`, `429 too_many`; for a reply, `403` when
the thread does not take the viewer's replies and `410` when what it answers
was removed.

## Settings

Read by `video-service`, `video-worker` and, for the first three,
`auth-service`, which tells the web app. Set them once, where every
service reads its settings: `.env` for docker-compose, `config.env` on
Kubernetes.

| Variable | Value |
| --- | --- |
| `VIDEO_UPLOADS` | `on` where people may post videos. Anything else, and unset, is off: no profile has the video feature (no composer, no Videos tab), and `video-service` refuses an upload. Videos already posted go on being shown in feeds and at their own addresses. Read by `graph-grpc-service` too, which is what leaves the feature off profiles |
| `VIDEO_MAX_BYTES` | The largest file taken. 500 MB (`524288000`) unless said |
| `VIDEO_MAX_SECONDS` | The longest video taken. `600`, ten minutes, unless said |
| `VIDEO_RENDITIONS` | The sizes made, by height. `1080,720,360` unless said; see [Sizes](#sizes) |
| `VIDEO_DAILY_UPLOADS` | How many videos one person may send in 24 hours. `20` unless said |
| `ANIMATION_MAX_BYTES` | The largest GIF or animated PNG taken. 25 MB (`26214400`) unless said, and never more than `VIDEO_MAX_BYTES` |
| `ANIMATION_MAX_SECONDS` | The longest one. `30` unless said, and never more than `VIDEO_MAX_SECONDS` |
| `VIDEO_WORK_DIR` | Worker only: where a video is kept while it is worked on. `/work` in the image. Needs room for the largest upload and everything made of it |

NodeInfo's `metadata` has `videoUploads`, `videoMaxBytes`,
`videoMaxSeconds`, `animationMaxBytes` and `animationMaxSeconds`.

## Storage

A browser sends the video to the store itself, from the web app's page,
so **the bucket has to allow that**: a CORS rule for `PUT` from the web
app's address. Without it every upload fails in the browser before
anything is sent. On AWS S3, under the bucket's Permissions:

```json
[
  {
    "AllowedOrigins": ["https://www.example.org"],
    "AllowedMethods": ["PUT"],
    "AllowedHeaders": ["*"],
    "MaxAgeSeconds": 3000
  }
]
```

Other stores have the same rule under their own name for it.

`file-grpc-service` holds each kind of large file to its place:

| Where | What | Who writes | Who reads |
| --- | --- | --- | --- |
| `assets/v1/video-service/uploads/<person>/<upload>/source` | What was uploaded, kept as `application/octet-stream` whatever it is | The browser, in parts | The worker. **Never a viewer**: no address is signed for one |
| `assets/v1/video-service/videos/<profile>/<upload>/` | Each size and the poster | The worker | Viewers, as `video/mp4` or `image/jpeg` by its name, whatever it was stored as |

A lifecycle rule on the bucket that aborts unfinished multipart uploads
after a few days is worth having: the worker aborts the ones it knows of,
and the rule catches any it could not.

## Running it, and not running it

`make video` builds and starts both containers. Set `VIDEO_UPLOADS=on`.

Both are optional. Without them there are no video posts, and with
`VIDEO_UPLOADS` unset the web app offers none. With `video-service`
running and no worker, uploads are taken and wait.

One worker makes one video at a time. Run more replicas to make more at
once; each needs processor and working space of its own.

## What is not built

- Telling the uploader, by a notification, that a video is ready or has
  failed. The composer asks.
- Changing size while playing (HLS), captions, and a choice of poster.
- Videos from other networks: a video posted here goes to other servers
  and Bluesky (see [Federation](federation.md)), but one posted there is not
  brought in.
- Animated WebP, until ffmpeg reads one; and keeping what is see-through in
  a GIF (WebM with transparency, or keeping the picture as it was).
