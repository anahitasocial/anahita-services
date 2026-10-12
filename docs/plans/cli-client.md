# A command-line client for Anahita

*Roadmap item 42, recorded 2026-10-12 for later. The user has long wanted one. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context

Some people cannot use the web app, or a mobile app when there is one: a very slow connection, a terminal over SSH, a screen reader that does better with text, or simply a liking for the command line. A client that asks only for text gives them the network in a few kilobytes.

**The clients that exist for Mastodon and Bluesky will not do.** Federation is how servers speak to each other; a client speaks to its own server by that server's API. Mastodon's command-line clients (`toot`, `tut`) speak Mastodon's client API, which Anahita does not offer. A Bluesky client pointed at the account Anahita keeps for somebody on its PDS would show them Bluesky, not Anahita, and what they posted there would pass Anahita by.

**What exists (checked 2026-10-12):**
- The whole API a client needs: feeds, posts, replies, likes, follows, notifications, search.
- OAuth 2.0 with PKCE, and clients that hold no secret (`confidential: false`), each with the addresses it may be sent back to (`docs/oauth.md`, `oauth_seed_clients.json`).
- **No way to sign in without a browser on the same machine.** auth-service has no device flow. Signing in is its own pages, in a browser.

## Decisions

- **Our own client, against Anahita's API.** Not a Mastodon-compatible API on the server: see *Considered* below.
- **After the API reference** ([api-reference.md](api-reference.md)). The client's calls and types are generated from it, which is most of what makes this small, and it is the first real test of that reference before `anahita-mobile` is built on it.
- **One program, in Go,** with nothing to install beside it, for Linux, macOS and Windows.
- **Text only unless asked.** Nothing is fetched that was not asked for: no pictures, no avatars, no previews.

## Design

### What it does, first

| Command | Does |
| --- | --- |
| `anahita login <site>` / `logout` / `whoami` | Signs in to an installation, and out |
| `anahita feed [--custom <name>] [-n 20]` | Home, or one of the person's custom feeds, newest first |
| `anahita read <post>` | A post and its replies, as a thread |
| `anahita post [--to followers] [--lang fr]` | A note, from an argument, a file or standard input |
| `anahita reply <post>` | A reply |
| `anahita like` / `unlike` / `repost` / `save <post>` | The one thing each says |
| `anahita follow` / `unfollow <handle>` | People, groups and events |
| `anahita notifications` | What is new, and marks it read when asked |
| `anahita profile [<handle>]` | A profile, or the person's own |
| `anahita search <words>` | People and posts |
| `anahita open <post>` | Prints its web address, or opens it where there is a browser |

- **Everything is also plain data:** `--json` on any command, one object a line, so it can be piped and scripted.
- **Posts are numbered in what is shown,** so `anahita reply 3` answers the third one listed; the numbers are remembered until the next list.
- **A picture in a post is a line saying there is one,** with its description and its address. `--images` fetches thumbnails for terminals that can draw them; left out by default.
- **Long text goes through the pager;** Markdown is shown as plain text with its links listed underneath.

### Signing in

Two ways, because there are two situations:

1. **A browser on the same machine.** The usual OAuth with PKCE: the client opens the installation's sign-in page and listens on `127.0.0.1` for the answer. Needs a seeded public client whose redirect address is the loopback one, on any port (RFC 8252).
2. **No browser there** (SSH, a console). **The device flow** (RFC 8628), which auth-service does not have and would gain: the client shows a short code and an address; the person opens that address on any device, signs in as they always do and types the code; the client, which has been asking meanwhile, is given its tokens. This is the only server work in the plan. It reuses the code machinery of item 6 (hashed, few attempts, short life), and is of use to a television or console app as well.

Tokens are kept in the system's keychain where there is one, and otherwise in a file only the person can read. The client asks for the scopes it uses and no more.

### For a slow connection

- Small pages by default (`-n 20`), and the next page only when asked.
- Compression asked for on every request.
- What was last fetched is kept and shown at once, with a mark saying how old it is, while the new is fetched; `--offline` shows only what is kept.
- A post written while offline waits in an outbox and is sent by `anahita send`.

## Considered and not chosen

- **Offering Mastodon's client API on the server,** as GoToSocial and Pixelfed do, so that every existing Mastodon client works: command-line ones, and phone and desktop apps too. It would give Anahita apps before `anahita-mobile` exists. It is also much the larger piece of work (ten to fifteen sessions for a useful part, held loosely), and those clients would show only what Mastodon has a place for: no groups, no events, no articles, none of the audiences beyond Mastodon's four. **Not ruled out; a decision of its own,** to be taken when the mobile app is planned, since the two answer the same question.
- **A full-screen terminal app** (as `tut` is for Mastodon). A later face on the same client code, if wanted. Plain commands come first: they work over the worst connections and with screen readers.

## Estimate

| Part | Time |
| --- | --- |
| The device flow in auth-service: two endpoints, the page where the code is typed, limits, tests, `docs/oauth.md` | 4–6 h |
| A seeded public client for the loopback sign-in | 1 h |
| The client: generated calls, sign-in both ways, keeping tokens | 3–4 h |
| The commands above, `--json`, the pager, numbering | 6–8 h |
| Kept pages, `--offline`, the outbox | 2–3 h |
| Builds for the three systems, a page of docs | 2 h |
| **Total** | **about 18–24 h, 5–6 sessions**, after the API reference |

## Verification

- On a machine with no browser, `anahita login` over SSH signs in with a code typed on a phone.
- With the connection held to a few kilobytes a second, `anahita feed` shows what it has kept at once and the new within seconds; nothing but JSON is fetched.
- `anahita feed --json | jq` works; `echo "hello" | anahita post` posts.
- A post made followers-only from the command line is followers-only in the web app.
- A screen reader reads a thread in order, with nothing drawn that it cannot say.

## Open questions

1. **Where it lives:** in anahita-services, or a repository of its own (`anahita-cli`), as the web app has. A repository of its own is the likelier, for its own releases.
2. **Whether the device flow should be built sooner,** on its own, since the mobile app may want it too.

## Dependencies

- **Hard:** [api-reference.md](api-reference.md), item 38, phases A to C.
- **Soft:** [docs-for-agents.md](docs-for-agents.md): the client is one of the "clients of their own" its *Building on it* door points to, and a worked example for whoever writes another.

## Documentation

- **services `oauth.md`**: the device flow, and the loopback client.
- **A page for the client itself,** where it lives: installing, signing in, each command.

## Status

| Item | State |
| --- | --- |
| Idea recorded | 2026-10-12 |
| Implementation | kept for later; waits for the API reference |
