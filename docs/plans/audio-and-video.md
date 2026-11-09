# Audio and video from everywhere (roadmap rows 120 and 137)

Row 120, asked for by the user on 2026-11-01 after a Bluesky post came with
its words only: videos from other networks played here. Row 137, from
FediDB's comparison with Mastodon, Pixelfed and Loops (2026-11-07): audio
posts, and the sounds that arrive from Mastodon played here. Built
together: both are a file from elsewhere copied on demand, and both use
video-service's worker and ffmpeg. Decided with the user on 2026-11-08:
copied on first play, one size kept, audio a kind of post of its own, up
to an hour. Phases 1 and 2 built 2026-11-08; phase 3 not.

## Contents

1. [Why](#why)
2. [What there is now](#what-there-is-now)
3. [Part 1: videos and sounds from elsewhere](#part-1-videos-and-sounds-from-elsewhere)
4. [Part 2: audio posts](#part-2-audio-posts)
5. [Settings](#settings)
6. [Out of scope](#out-of-scope)
7. [Phases](#phases)

## Why

A post from Bluesky or Mastodon with a video shows its poster and leads to
the video on its own server (2026-11-08), and a sound only leads there.
People follow people for what they post, and a video they cannot watch
here is half a post. Sound is the one kind of post Anahita has none of, and
the one schools, libraries and newsrooms make most after writing: lectures,
readings, interviews, podcasts.

## What there is now

1. **Videos posted here** are uploaded in parts straight to storage, made by
   video-service's worker into MP4s at 1080, 720 and 360 with a poster, and
   played by the browser's own player ([Videos](../videos.md)).
2. **A video from Bluesky** keeps its poster as the post's picture; the web
   app draws it with a play button that opens the video on Bluesky
   (`components/VideoElsewhere.jsx`). Nothing reads the video itself.
3. **A video or a sound from the fediverse** ends its post with a line
   leading to its page; only an attachment's type, address and name are
   read.
4. **No audio anywhere:** no kind of post, no upload type, no player; a
   file that is only sound is refused as `not_a_video`.

## Part 1: videos and sounds from elsewhere

A file from elsewhere is **never played from its own server,** which would
tell that server who watched. It is copied here, once somebody wants it.

1. **When the post arrives,** only what it needs to be shown: its poster,
   kept as its picture (Bluesky's thumbnail; a fediverse attachment's
   preview, `icon`, when it names one, and otherwise none), and where the
   file is, in the post's meta (`media_elsewhere`: video or audio, its
   address, its page, its size and length when said). For Bluesky the
   address is the author's own server (`com.atproto.sync.getBlob` on the
   PDS their DID names), which gives the original file.
2. **On first play,** the web app asks for it
   (`POST /activitypub/posts/:id/media`, `/atproto/posts/:id/media`), shows
   "Getting the video…", and asks again until it is ready. The service puts
   a job on video-service's queue, the table its uploads already use.
3. **The worker** fetches the file, at most `REMOTE_MEDIA_MAX_BYTES`, looks
   at it with ffprobe, and keeps **one file**: a video up to 720, or its own
   size when smaller; a sound as AAC. An MP4 that already plays (H.264 and
   AAC, 720 or less) is kept as it is, made to start at once
   (`+faststart`), not encoded again, so most take seconds. Longer than
   `REMOTE_MEDIA_MAX_SECONDS`, or not a video or a sound, and the post goes
   on leading to its page.
4. **The post plays it** as a video or an audio post here does, from this
   site's storage.
5. **Kept for `REMOTE_VIDEO_DAYS`,** then removed, as pictures are after
   `REMOTE_MEDIA_DAYS`; the next play copies it again. One whose server no
   longer has it says so, with the link.
6. **Under the same rules as pictures:** not from a server or an account
   refused here; a post marked sensitive keeps none of it until content
   warnings (row 122) can put it behind one.

## Part 2: audio posts

1. **A kind of its own,** `node.medium.video-service.audio.v1`, made and
   kept by video-service, which has the worker and ffmpeg. An audio post
   has a title, words and a cover picture as a video post has, and an
   Audio tab on a profile that has any.
2. **Uploaded** as a video is, in parts straight to storage: MP3, M4A, AAC,
   Ogg and Opus, WAV, FLAC, WebM sound. In the composer, an Audio button
   beside Photo and Video.
3. **The worker** keeps one file, AAC at 128 kbit/s in an M4A, at most
   `AUDIO_MAX_SECONDS`, an hour unless the operator says otherwise; the
   picture a file carries (an album cover) becomes its cover, and the
   author may choose another.
4. **Played** by the browser's own player, `<audio>`, under its cover, in
   feeds, on profiles and on its own page; downloadable as a video is.
5. **To the fediverse** as Mastodon's `Audio` attachment (`audio/mp4`,
   its cover as `icon`), its title first, as a video's is. **To Bluesky,**
   which has no sound, as its title and opening with a card leading to it
   here, its cover the card's picture, as an article goes.
6. **From the fediverse** a sound comes in by Part 1, and plays here as an
   audio post does.

## Settings

| Setting | Default | Means |
| --- | --- | --- |
| `AUDIO_MAX_SECONDS` | `3600` | The longest audio post |
| `AUDIO_MAX_BYTES` | `500` MB | The largest file taken to make one |
| `REMOTE_MEDIA_MAX_BYTES` | `100` MB | The largest file from elsewhere copied here |
| `REMOTE_MEDIA_MAX_SECONDS` | `3600` | The longest video or sound from elsewhere copied here |
| `REMOTE_VIDEO_DAYS` | `7` | How long a copy from elsewhere is kept after it was last played; `0` for never removed |

## Out of scope

1. Streaming (HLS) and captions, for videos here or from elsewhere: as in
   [Videos](../videos.md#what-is-not-built).
2. Podcast feeds (RSS with enclosures) of a profile's audio posts: a row of
   its own if asked for.
3. Waveforms, chapters and playlists.
4. Copying a file from elsewhere when the post arrives: decided against, as
   most are never played.

## Phases

1. **Videos from elsewhere:** the meta, the queue, the worker's copy, the
   two routes, the web app's play and wait, removal after
   `REMOTE_VIDEO_DAYS`; Bluesky and the fediverse: about two sessions.
   **Done 2026-11-08** ([Videos from other networks](../videos.md#videos-from-other-networks)),
   tried on dev with a Bluesky video. A sound is recorded as one already, and
   waits for phase 3.
2. **Audio posts:** the kind, the upload, the worker, the player, the
   composer, the profile tab, both networks out: about two sessions.
   **Done 2026-11-08** ([Audio posts](../videos.md#audio-posts)). Posted
   where videos may be (`VIDEO_UPLOADS`): no `AUDIO_UPLOADS` of its own, the
   same two containers making both.
3. **Sounds from the fediverse,** by phase 1's way, played by phase 2's
   player: half a session.
