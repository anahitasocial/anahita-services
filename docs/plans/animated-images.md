# Animated images: GIF, APNG and animated WebP, through the video service

*Roadmap item 41, planned 2026-10-12 at the user's asking. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context

An animated GIF cannot be posted. The upload code names no such type, so one is refused; an animated PNG (APNG) passes for an ordinary PNG and would, it is expected, come out of the photo service as its first frame, still. Neither has been tried.

People post short loops constantly, and both networks Anahita is to federate with carry them as video:

- **Mastodon** takes an uploaded GIF and makes a silent, looping MP4 of it with ffmpeg. It is sent to other servers as a video attachment and played muted, on a loop, without controls.
- **Bluesky** offers GIFs from Tenor, played from Tenor's servers and not kept by Bluesky, and has video uploads.

(Both from memory on 2026-10-12, to be checked when this is built.)

## Decisions

- **In `video-service`, not in `photo-service` and not a service of its own** (user, 2026-10-12). The worker has ffmpeg already; an animation as MP4 is a fraction of its size as a GIF; and it is what Mastodon sends and expects.
- **Two kinds are taken:** GIF and APNG, one case: an image file with more than one frame. **Animated WebP was dropped on 2026-10-14:** ffmpeg 8.1, in the worker's image, reads the file but finds no picture in it. It is refused as not a video until ffmpeg reads one.
- **A still picture is not taken here.** A PNG or WebP with one frame sent to the video service is refused with a reason the web app turns into "post it as a photo". The photo service is where a still goes.
- **No GIFs from Tenor or Giphy.** Every reader's browser would call a third party. A pasted link to one shows as an ordinary link card.

## Design

**What a post records.** `meta.video` gains `"loop": true` (`anahita-libs/videos`). It is a video post in every other way: its type, who may see it, replies, likes, downloads and purging are unchanged.

**Upload.** The same flow as a video: parts sent straight to storage, then the worker.
- `UploadTypes` gains `image/gif`, `image/png` and `image/webp`.
- A limit of its own, far below a video's: `ANIMATION_MAX_BYTES` (25 MB) and `ANIMATION_MAX_SECONDS` (30). A two-hundred-megabyte GIF is a mistake or an attack.

**Worker.**
- **What the file really is, is read from its first bytes,** as for video (`encode.Container`), and never taken from the name or the type the browser gave.
- **ffprobe says how many frames.** One frame is refused as a still. More frames than the limits allow is refused.
- **One encode,** at the animation's own size (even numbers, no more than 720 high): H.264, no sound, every frame's own timing kept, `+faststart`. And a poster from the first frame.
- **Transparency is lost,** since plain MP4 has none: the animation is laid on white, or on the colour the file names as its background. That matters for stickers and not for the usual clip. Said in the docs; a format that keeps it (WebM with alpha, or keeping the APNG) is left for when somebody needs it.

**Player (web).** A looping post plays muted, on a loop, inline, without the quality cog or the scrubber, and stops when it scrolls out of sight. It honours "reduce motion": then it waits to be pressed. A small "GIF" mark sits in its corner.

**Composer (web).** An animation is posted from the **video composer**, which takes a GIF, an animated PNG or an animated WebP besides videos, and says so in its list of what it takes (user, 2026-10-14). The photo composer is left as it is: it takes JPEG and PNG pictures.

**Where video is off** (`VIDEO_UPLOADS` not `on`): there is no video composer, so animations are not taken, and the API refuses them. Nothing is kept as a still photo of its first frame instead (user, 2026-10-14): a moving picture silently made still is worse than a clear no.

**Federation.** To ActivityPub as a video attachment, marked as Mastodon marks a converted GIF, so it loops there too; to Bluesky as a video. Arriving ones are for phase 3, with the rest of media.

## Estimate

| Part | Time |
| --- | --- |
| Types, limits, `loop` in the post's meta, refusing a still | 1–2 h |
| Worker: frame count, the one encode, poster, tests in the worker's image with real GIF, APNG and WebP files, and hostile ones | 3–4 h |
| Web: composer routing with the animated-PNG check, the looping player, reduce motion | 3–4 h |
| Docs (`videos.md`, `photos.md`, `configuration.md`, web `architecture.md`) | 1 h |
| **Total** | **about 8–11 h, 2–3 sessions** |

## Verification

- A GIF, an APNG and an animated WebP each become a looping post; a still PNG sent the same way is refused and posts as a photo.
- A 10 MB GIF becomes an MP4 a fraction of that size, with the same timing.
- A file named `.gif` that is a zip, an HTML page or a script is refused by its bytes.
- With "reduce motion" on, nothing moves until pressed.
- With video off, the composer does not offer it and the API refuses it.

## Dependencies

- **Hard:** item 16, the video service (done).
- **Soft:** item 8 phase 3, for federating it.

## Documentation

- **services `videos.md`**: animations, their limits, what is lost (transparency, sound there never was).
- **services `photos.md`**: an animated image is not a photo, and where it goes.
- **services `configuration.md`**: `ANIMATION_MAX_BYTES`, `ANIMATION_MAX_SECONDS`.
- **web `architecture.md`**: the composer's routing and the looping player.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-12 |
| Implementation | services done 2026-10-14 (the worker, limits, `loop`, NodeInfo; tested with real files in the worker's image); web in progress |
