# Link previews in notes

*Roadmap item 10 (part A), planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
1. **Link previews:** when someone pastes a link into a note, show a preview card (title, description, image, site) in the composer and on the published note.
2. **Open Graph:** when an Anahita URL is shared on Facebook, LinkedIn, Mastodon, Bluesky, Slack, Discord, WhatsApp and so on, it should show the right title, description and image for *that* page.

**Why the past attempt didn't work:**
- Per-page tags are set in the browser with `react-helmet-async` (`anahita-web/src/components/HeaderMeta.jsx`).
- Crawlers don't run JavaScript. They only see `public/index.html`, which carries **site-wide** tags (`%REACT_APP_NAME%`, `ogimage.jpg`). So every share looks identical.
- `HeaderMeta` also emits `og:site` where the tag should be `og:url`.
- Legacy PHP rendered OG on the server (`libraries/default/base/view/og.php`), which is why it worked there.

**Nothing exists** for link previews in either repo.

## Part A — Link previews in notes (new `link-preview-service`, Go)
**Why a separate service:** it fetches **arbitrary URLs from the internet**, the classic SSRF risk. It stays isolated and swappable, like the federation services.

1. **Fetch safely:**
   - http/https only, ports 80/443 only;
   - resolve DNS and **refuse private, loopback, link-local and metadata IPs** (re-checked on every redirect, and pinned to the resolved IP to prevent DNS rebinding);
   - at most 3 redirects;
   - a 5 s timeout;
   - read at most 1 MB and only `text/html`;
   - a clear User-Agent (`AnahitaBot/1.0 (+https://DOMAIN)`).
2. **Parse, in this order:** Open Graph, then `twitter:*` tags, then oEmbed discovery (for rich providers), then `<title>` / `meta description` / favicon. Use `golang.org/x/net/html`.
3. **Preview image:**
   - Download it with the same fetcher and **re-encode it as a thumbnail into our own storage** through file-grpc (item 9's rule: images are always re-encoded).
   - Viewers' browsers then never contact the third-party site, the thumbnail can't break later, and nothing active can slip through.
4. **Cache table `link_previews`:**
   - `url_hash` (PK), `url`, `final_url`;
   - `title`, `description`, `site_name`, `image_key`;
   - `status` (`ok` | `failed` | `blocked`), `fetched_at`.
   - Keep entries for 7 days; failures are kept for 1 hour.
5. **API:** `GET /link-previews?url=…` (logged-in users only, rate-limited per person with the shared limiter from item 4b). Returns the preview or 204.
6. **On the note:**
   - `POST` / `PATCH /notes` accept a `link_preview` `{ url }`.
   - text-service calls the service and stores a **snapshot** in the note's `meta` (title, description, image_key, url, site). The card stays as it was when posted, as on Mastodon and Bluesky.
   - Only the first link, and notes only (your scope).
   - Removing the card (✕) stores nothing.
7. **Web:**
   - The note composer (`src/containers/media/Composer/Forms/Note.jsx`) finds the first URL on paste, and after a 500 ms pause while typing.
   - It fetches the preview and shows a `LinkPreviewCard` with a remove ✕.
   - The published note renders the card from `meta` (in `NodeBody` / `MediumNode`).
   - URLs that `components/Player.jsx` already embeds (YouTube, Vimeo, …) keep the player instead of a card.
8. **Federation tie-in (item 8):**
   - On Bluesky, the snapshot maps straight onto `app.bsky.embed.external` (uri, title, description, thumb blob).
   - Mastodon builds its own previews by fetching the link, which is where part B matters.
9. **Moderation:** the domain blocklist from item 8 also blocks previews (`status=blocked`), and item 4b reports can target the note as usual.

Part B (Open Graph through the web shell), the combined estimate, verification, decisions and dependencies are in anahita-web's `docs/plans/open-graph.md`.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `architecture.md`**: link-preview-service, its SSRF protections, and the preview snapshot on notes.
- **services `configuration.md`**: preview cache TTLs and rate limit.
- The web-shell documentation is listed in anahita-web's `docs/plans/open-graph.md`.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Implementation | not started |
