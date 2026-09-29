# Download your data (export-service)

*Roadmap item 21, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
- **What we want:** people can export everything they created and download it as an archive. It's a right to data portability (GDPR Art. 20, and Canada's PIPEDA principles) and a fediverse norm: Mastodon offers an archive download.
- **The reference implementation,** `protect-services/dxp-service`, exports a whole **organization** (super admins only):
  - EventBridge Scheduler (a 24 h delay) puts a message on **SQS**, a worker builds a zip in memory through file-grpc and uploads it to S3, and a cleanup job deletes old packages.
  - **The flow and its hard-won rules carry over. The content, the permission model and the AWS pieces don't.**
- **Decided 2026-09-28: no scheduler and no delay.** A request starts building right away, and the person is told as soon as the archive is ready.
- **Anahita has no export code yet.**

## No SQS or EventBridge: open-source options
- **Queue: a MySQL job table plus a worker** that claims jobs with `SELECT … FOR UPDATE SKIP LOCKED`.
  - It needs **no new infrastructure** and is the same pattern as item 8's delivery queue and item 16's video jobs, so build it once in `anahita-libs/jobs` and reuse it.
  - It's durable (a row survives restarts) and retries are explicit.
  - The job row doubles as the status record the UI reads.
- **Alternatives, not recommended here:**
  - A RabbitMQ durable queue with manual ack and a dead-letter exchange. RabbitMQ is already deployed, but `anahita-libs/event/consumer.go` is pub/sub (random, non-durable queues, auto-ack), so it would need a new work-queue consumer. And the job still needs a status row, so it's two systems instead of one.
  - Redis Streams.
- **Cleanup:** a k8s **CronJob**, copying `k8s/03-sweeper-cronjob.yml` (`concurrencyPolicy: Forbid`, advisory lock). With no delay, there's no scheduler anywhere.
- **Storage:** item 9's S3-compatible storage (MinIO, Garage, OVH, Scaleway…), not AWS-specific.

## Design
**Service:** a new `export-service` (API) plus an `export-worker` (a separate Deployment, as in dxp, so a long build never touches the API, and API replicas never add consumers).

**Table `data_exports`:**
- `id` (UUID, public), `person_id`, `status`, `package_key`, `size_bytes`;
- `created_at`, `started_at`, `ready_at`, `expires_at`, `error`;
- `ip`, `user_agent` (the audit record; IP taken from the rightmost trusted `X-Forwarded-For` entry, per dxp).
- **States:** `pending → building → ready → expired`, plus `failed` and `cancelled`. Every transition is a compare-and-set: `UPDATE … WHERE id=? AND status=?`, then check rows affected.
- **One active export per person:** a generated column `active_person = IF(status IN ('pending','building'), person_id, NULL)` with a UNIQUE index. MySQL has no partial indexes.

**API (export-service)**

| Endpoint | Rules |
| --- | --- |
| `POST /exports` | **Step-up required** (password, passkey, or the item 6b code). Returns 409 if an export is already active, and **429 if the last export was under 7 days ago** (configurable). |
| `GET /exports`, `GET /exports/:id` | Your own exports only; anyone else's gives 404. Adds a presigned `download_url` (15 min, re-issued on each read) only when `ready` and the viewer is the owner. |
| `DELETE /exports/:id` | Cancel (`pending` or `building`) or delete (`ready`) |

**Worker**
1. Claims a `pending` job, sets `building`, **re-reads the row between steps**, and stops if it has been cancelled.
2. **Collects the data through graph-grpc** (never direct SQL on other services' tables), paged:
   - **Profile:** name, alias, bio, pronouns, website, avatar, cover, settings (posting language, default licence, interests, recommendation preferences, notification preferences).
   - **Account:** email, username, created, onboarded and verified timestamps, sign-ins from the last 90 days (`auth_logs`).
   - **Everything they wrote,** anywhere (`created_by`, the same set item 5's purge walks): notes, articles, topics, photos, videos, replies or comments, and reposts, each with its licence, language and access.
   - **Relationships:** following, followers, blocks, groups and events they belong to or administer, sent and received invitations (item 11), and their RSVPs (item 12).
   - **Activity:** their likes, their bookmarks or pins, their custom feed definitions (item 14), and reports they filed (item 4b; not reports *about* them, which stay with admins).
   - **Files:** the largest stored size of each of their images, and video renditions. Originals don't exist after re-encoding (items 9, 16, 17).
3. **Formats:**
   - **`index.html`:** a readable offline browser of the archive (profile, posts with images, relationships), a static page with no scripts from outside.
   - **`data/*.json`:** everything above as JSON (NDJSON for large lists).
   - **ActivityPub-compatible files** (`actor.json`, `outbox.json`, `likes.json`, `media_attachments/`), in the same layout as a Mastodon archive, so fediverse tools and importers can read it.
   - **If the person opted into Bluesky (item 8):** `bluesky/repo.car`, their AT repository from the PDS (`com.atproto.sync.getRepo`).
4. **Streams the zip, never in memory** (dxp built it in a buffer and is capped at about 1 GB):
   - `archive/zip` writes into an `io.Pipe`, which feeds a **multipart upload**.
   - New streaming RPCs on file-grpc: `ReadStream` (server-streaming) for large files, and `WriteStream` (client-streaming into a multipart upload). Today's file-grpc has only whole-message calls with the 4 MB gRPC default. item 16's worker needs the same RPCs, so build them once.
   - Add an abort-incomplete-multipart rule on the bucket.
5. Uploads to `exports/<person_uuid>/<export_uuid>.zip`, then sets `ready` with `expires_at = ready_at + 7 days`.
   - If that last update is refused (cancelled meanwhile), it **deletes the package immediately**, as dxp does.
   - Any error sets `failed` with the reason, so the person can request again.
6. **Tells the person when it's ready:**
   - an in-app notification (`data_export_ready`, notification-service);
   - an email through mail-grpc (`message_action`, linking to **the app's export page, not the file**).
   - A failure also gets an in-app notification.

**Cleanup CronJob (hourly)**
- Expires `ready` exports past `expires_at` (delete the object, set `expired`).
- Deletes orphaned objects under `exports/`, but only when the row is **definitely not found**. Any other error leaves the file.
- Resets or fails jobs stuck in `building` for more than 2 h (dxp lacks this reaper).
- Exits non-zero when anything failed.

**Interactions**
- **Purge (item 5):** deletes the person's exports and their packages.
- **Members-only sites (item 13):** exporting still works; it's the person's own data.
- **Rate limits:** the shared limiter from item 4b, plus the 7-day rule.

**Security notes from dxp to carry over:**
- The step-up gate must be real: auth-service must stop stamping `TotpLastVerifiedAt` without a passcode (`auth-service/cmd/api/handlers/totp.go:93`). That fix is roadmap item 0b ([auth-service-hardening.md](auth-service-hardening.md)).
- `ENV` must have no silent default.
- An empty key lookup is refused.
- Nothing from the archive is ever logged.

## Web (anahita-web)
- **Settings → Account → "Download your data":**
  - a Request button (opens the step-up dialog first);
  - the current export's status (Building… / Ready, with a Download button and the expiry date / Failed with a Retry button);
  - a short history;
  - a note on what's included.
  - Downloading uses `download_url` from a fresh `GET`.
- The `data_export_ready` notification links to this page.
- i18n in en-GB and fr-FR.

## Estimate

| Part | Time |
| --- | --- |
| `anahita-libs/jobs` (SKIP LOCKED worker, retries); skip if item 8 or item 16 already built it | 2–3 h |
| export-service API, table, compare-and-set states, one-active rule, 7-day rule, step-up, tests | 4–5 h |
| export-worker: data collection over graph-grpc for every content type, JSON / HTML / ActivityPub formats, Bluesky CAR | 8–10 h |
| file-grpc `ReadStream` / `WriteStream` (multipart), streaming zip | 3–4 h |
| Cleanup CronJob, notifications + email, purge hook, deploy wiring | 2–3 h |
| Web: "Download your data" page, notification, i18n | 3–4 h |
| **Total** | **about 22–29 h, 6–7 sessions** |

## Verification
- **Flow:**
  - Request, then status goes `building`, then `ready` within minutes for a typical account. The notification and email arrive, and the download works.
  - A second request gives 409 while one is building, and 429 within 7 days.
  - Cancelling during a build stops it, and no package remains.
- **Content:** the archive contains every post the person wrote, including ones in groups and on other people's profiles, their images and videos, and their follows. It contains nothing written by others except what's needed for context. `outbox.json` validates as ActivityStreams, and `index.html` opens offline.
- **Scale:** a person with 2 GB of photos exports with the worker's memory staying flat (streaming).
- **Security:**
  - another person's export id gives 404;
  - requesting without step-up gives 403;
  - the email contains no file link;
  - after 7 days the object is gone and the row is `expired`.
- **Resilience:** killing the worker mid-build means the reaper fails the job and the person can retry, and an orphaned object is removed by cleanup.

## Decisions (item 21)
- No scheduler or delay: build immediately and notify when ready.
- A MySQL job table plus worker instead of SQS; cleanup is a k8s CronJob. No AWS services.
- Formats: HTML plus JSON plus an ActivityPub-compatible archive (and the Bluesky CAR for opted-in people).
- One export per 7 days, archives kept 7 days, step-up required.

## Dependencies
- **Hard:** item 9 A (S3-compatible storage).
- **Shared:** `anahita-libs/jobs` (with items 8 and 16), file-grpc streaming RPCs (with item 16), the rate limiter (item 4b).
- **Better after:** item 5 (the same authored-content walk), item 6b (step-up by code) and the `TotpLastVerifiedAt` fix. Covers more data after items 3, 11, 12, 14 and 8.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **New services page `data-export.md`** (operator view): what an archive contains, its formats, retention, the 7-day rule, and how to size the worker.
- **services `configuration.md`**: export retention, cooldown, and worker settings.
- **services `architecture.md`**: export-service, export-worker, `anahita-libs/jobs`, and the file-grpc streaming RPCs.
- **web `architecture.md`**: the "Download your data" settings page.
- **web `customising.md`**, *Terms of Service and Privacy Policy*: a note that operators should mention data export in their privacy policy.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
