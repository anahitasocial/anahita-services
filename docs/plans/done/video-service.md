# Video: video-service and a transcoding worker

*Roadmap item 16, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Decision
**A new `video-service`** (upload, metadata, reads) **plus a separate `video-worker`** (ffmpeg transcoding). It isn't merged into photo-service.

## Why separate
- **Different resources.** Transcoding is long and CPU-heavy (minutes per video), while photo resizing is instant and runs inside the request (`anahita-libs/utils/images.go`). Merging them would make photo uploads compete with transcodes, and photo-service would have to scale for the worst case.
- **Different upload path.** Videos can't go through the API:
  - nginx caps request bodies at **10 MB** (`nginx/nginx.conf:8`);
  - the whole file is passed through file-grpc as one gRPC message, which is limited to **4 MB by default** (no `MaxRecvMsgSize` is set anywhere).
  
  Videos must upload **straight to storage** with presigned multipart uploads, a flow photos don't need.
- **Optional per instance.** Video is expensive to store and serve. An instance switches it off by not deploying the two containers (`VIDEO_UPLOADS=off` hides the UI), the same swappable pattern as the other optional services.
- **Worker isolation.** ffmpeg parses untrusted files. Running it in its own container, with no database write access beyond its job rows, limits the damage from a malicious file.

## Design
**Type and data**
- `node.medium.video-service.video.v1`: title, description, owner, access and comments, like other media.
- A side table `videos(node_id PK, status, duration_s, width, height, source_key, rendition_keys JSON, poster_key, error, created_at)`, with `status` = `uploading` | `processing` | `ready` | `failed`.

**Upload flow**
1. `POST /videos/uploads` `{ filename, size, content_type }`:
   - checks the limits: default **500 MB**, **10 minutes**, types `video/mp4`, `video/quicktime`, `video/webm`;
   - creates the node with `status=uploading`, hidden from every listing until it's `ready`;
   - starts an **S3 multipart upload** and returns presigned part URLs for the storage public endpoint (item 9, S3-compatible only).
2. The browser uploads the parts directly, with resume and retry, then calls `POST /videos/uploads/:id/complete`.
3. **Complete** finishes the multipart upload, sets `status=processing`, and adds a job row to `video_jobs` (the durable queue pattern from item 8's phase 0.7, not the lossy event bus).
4. Abandoned uploads (more than 24 h in `uploading`) are aborted and deleted by the sweeper.

**Worker (`video-worker`, Go + ffmpeg in its container)**
- Claims jobs with `SELECT … FOR UPDATE SKIP LOCKED`, running one or two at a time per replica.
- Downloads the source and **probes it with ffprobe**, refusing anything that isn't really a video, over the duration limit, or with no video stream.
- **Always re-encodes** (item 9's rule: nothing uploaded is served as-is): H.264 + AAC MP4 at **720p** (plus **360p** for slow connections), faststart, metadata stripped.
- Creates a **poster image** at about 1 s (re-encoded JPEG).
- Uploads the renditions and poster, **deletes the original**, sets `status=ready`, and publishes `node.medium.video-service.video.added.v1`. The feed, hashtags and notifications then treat it like any other medium.
- On failure: `status=failed` with the error, the uploader is notified, and there are up to 3 retries.

**Serving**
- Plain progressive **MP4 with HTTP range requests**, straight from storage through the stable media URLs (item 9 A / item 8 phase 0.4), from the media domain.
- No HLS or DASH in v1: at these sizes, MP4 with two renditions is simpler and works everywhere. HLS can come later if long videos matter.

**Reads and permissions**
- Copy photo-service: browse by owner, read, edit, delete (including the storage objects), access, comment status.
- Access, likes, replies (item 7), reports (item 4b) and pins (item 15) all work, because it's a medium.
- The purge (item 5) collects the video's objects like any other files.

**Limits and config:** `VIDEO_MAX_BYTES`, `VIDEO_MAX_SECONDS`, `VIDEO_RENDITIONS=720,360`, `VIDEO_UPLOADS=on|off`, and a per-person daily upload quota (the shared limiter).

**Federation (item 8, later phases)**
- ActivityPub: a `Note` with a `Document` attachment (`mediaType: video/mp4`, the poster as `icon`), which Mastodon shows inline. Remote servers have their own size limits (Mastodon's is around 100 MB), so send the 360p or 720p rendition.
- AT Protocol: `app.bsky.embed.video` accepts up to 3 minutes and 100 MB. Longer videos get an external-link card to Anahita instead.

**Embeds:** add **PeerTube** to `anahita-web/src/components/Player.jsx` alongside YouTube and Vimeo. It's useful whether or not uploads are on, and it's the sovereign option for instances that don't host video.

## Web (anahita-web)
- **Composer:** a video tab (or photo/video in one picker) with the file, title and description, plus the item 13 audience picker.
  - Chunked upload with progress, pause and resume, using `fetch` to the presigned part URLs.
  - Then a "Processing…" card that polls status or updates on refresh.
- **Player:** a native `<video>` with the poster, both renditions as `<source>`s, and captions later.
- `videos` namespace in `src/api/index.js`, a profile Videos tab (`TAB_COMPONENTS`), and a video card in feeds and search.
- Everything is hidden when NodeInfo says video uploads are off.

## Estimate

| Part | Time |
| --- | --- |
| video-service: type, side table, presigned multipart upload flow, reads/edit/delete/access, limits, abandoned-upload cleanup | 7–9 h |
| video-worker: job queue, ffprobe checks, ffmpeg renditions + poster, retries, container image | 6–8 h |
| Deploy wiring (compose, nginx, k8s, Makefile), config, docs | 2–3 h |
| Web: chunked uploader with resume, processing state, player, Videos tab, PeerTube embed | 6–8 h |
| **Total** | **about 21–28 h, 5–7 sessions** |

## Verification
- **Upload:** a 300 MB MP4 uploads directly to MinIO/Garage in parts; killing the network halfway and resuming completes it. The original is gone after processing, and the 720p and 360p renditions plus the poster exist.
- **Safety:**
  - a `.mp4` that is really an HTML file or a zip is refused by ffprobe (`failed`);
  - an 11-minute video is refused;
  - a polyglot file is re-encoded;
  - only `video/mp4` renditions are ever served, from the media domain.
- **Playback:** seeking works (range requests) in Safari, Firefox and Chrome, and on mobile.
- **Visibility:** a followers-only video is invisible to strangers, and the video appears in feeds only once `ready`.
- **Switch:** with `VIDEO_UPLOADS=off` and the containers stopped, the UI has no video option and PeerTube embeds still play.

## Dependencies
- **Hard:** item 9 A (S3-compatible storage with a public endpoint; also needed for presigned multipart uploads), and stable media URLs (item 9 A / item 8 phase 0.4).
- **Reuses:** item 8's durable queue pattern (or builds it first), and the shared rate limiter.
- **Soft:** item 13 (audience picker in the composer), item 7 (replies), item 4b (reports), item 5 (purge collects video objects).

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `configuration.md`**: `VIDEO_UPLOADS`, `VIDEO_MAX_BYTES`, `VIDEO_MAX_SECONDS`, `VIDEO_RENDITIONS`.
- **services `architecture.md`**: video-service, video-worker, the upload flow, the jobs queue and the file-grpc streaming RPCs.
- **services `getting-started.md`**: running the worker locally (ffmpeg in its image).
- **services `deploying.md`**: sizing and turning video off.
- **web `architecture.md`**: the chunked uploader and the player; PeerTube among the supported embeds.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | built 2026-10-11, services and web app |

## What was built

The services, 2026-10-11. The page that describes them is [Videos](../../videos.md).

- `file-grpc-service`: large files sent to the store in parts, and addresses for a worker to read and write at, each held to its place (`uploads/`, `videos/`).
- `video-service` and `video-worker`, one module with two programs. Migration `20261022000001_video_uploads`.
- `video` on a post wherever it is sent; the kind `video` in custom feeds; `/videos/<id>` in the web shell; a video's files removed with the post and by an account purge.
- `VIDEO_UPLOADS`, `VIDEO_MAX_BYTES`, `VIDEO_MAX_SECONDS` published in NodeInfo.

**Different from the plan:**

- **There is no post until the video is ready.** The plan made the node when the upload began and hid it until it was `ready`. Instead the upload row carries what the post will say, and the worker makes the post last. Nothing in the graph has to know about a post that is not finished, and an abandoned upload leaves no node behind.
- **One table, `video_uploads`**, is the upload, the job and its record. No separate `videos` and `video_jobs`: what a video post has is in its node's `meta`.
- **Sizes are `1080,720,360`** by default, and up to 4K by setting, at the user's request (2026-10-11). The plan had 720 and 360.
- **Files are reached through `/videos/:id/files/:name`**, which asks who may see the post and redirects to a signed address. The plan assumed stable public media addresses, which do not exist.
- **How much was uploaded is asked of the store** when an upload is finished, and the worker checks again as it reads.
- **The worker tidies up abandoned uploads itself**; the account sweeper is not involved.
- **Not built:** a notification when a video is ready or has failed, and PeerTube embeds, which are the web app's.

The web app's part was built the same day: a composer of its own that sends the file in parts and follows it until it is posted, and a player with a choice of size. It is described in anahita-web's `docs/architecture.md`, under Videos. Profiles have the video feature only where `VIDEO_UPLOADS=on`, which the graph service decides, so the web app hides nothing itself.
