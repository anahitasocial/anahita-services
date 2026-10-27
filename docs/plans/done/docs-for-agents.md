# Documentation that people's agents can use, and an MCP server for it

*Roadmap item 39, planned 2026-10-11. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context

Most of the work of the last two weeks was done with an AI agent, and the next person to work on Anahita will very likely use one too. Two questions were asked on 2026-10-11:

1. Do the docs need a version written for agents?
2. Can an MCP server be set up so that others can learn what was done?

## Who this is for (user, 2026-10-11)

People with some technical skill who set up an Anahita site for their school, newsroom, neighbourhood, company, non-profit or niche community, to join the fediverse and move away from platforms such as Meta's and X. They need to know how to **set it up, keep it running, make it their own, and build services and clients of their own** against it. An `anahita-mobile` app will follow once the web app has settled, and what is given to others to build clients with is what it will be built with.

Most of them will never clone the repositories to change the code, and many will work with an agent.

**What that changes in this plan:**
- **The hosted server is part of this item, not a later extra.** These readers have no clone; a server that runs only from one is of no use to them. It is still built from a clone first, because that is how it is tested.
- **`AGENTS.md` is for the few who change the code; the many need a way in by what they are doing.** The docs index gains four doors, each a short page of links in order: *Setting up a site*, *Keeping it running* (upgrades, backups, the sweeper, moving storage), *Making it your own* (settings, recipes, themes, languages, legal texts), and *Building on it* (the API reference, OAuth clients, the events on the bus, the contract a replaceable service keeps, such as recommendations). No page is rewritten; pages that are missing are found by trying to fill a door.
- **Missing pages already known:** backing up and restoring; upgrading from one release to the next; writing a service of your own; writing a client. Each is a page to plan when its door is filled.

## The first question: no second version

**What there is works for an agent as it is.** The pages are plain Markdown, one subject each, with an index that lists every section, and they say where the code is. That is what an agent needs, and it is the same thing a person needs. A second set written "for machines" would be a second set to keep true, and the check of 2026-10-11 showed what happens to pages nobody is made to keep true.

**What is missing is not a rewrite. It is four smaller things:**

| Missing | Why it matters to an agent |
| --- | --- |
| **The working agreements are written nowhere in the repositories.** Migrations go in three places; tests never touch the dev database and how to give them one; `go mod tidy` is not run on `anahita-libs`; what a commit message looks like; a group has followers and never members; the docs are part of the work. They are in the memory of one person's agent on one machine | An agent starting cold breaks each of them once |
| **anahita-services has no entry file.** anahita-web has a `CLAUDE.md`, about MUI only | Nothing tells an agent to read `docs/README.md` first, or that the roadmap is in the other repository |
| **What the API takes and sends** ([api-reference.md](api-reference.md)) | Today an agent reads handlers to learn a field's name, and guesses |
| **Not every page says where its code is.** `feeds.md` and `recommendations.md` end with "Where the code is"; most do not | The first thing an agent does with a page is look for the files |

### Added 2026-10-14: plain language, and keeping the docs true

The user asked that the docs be "updated and descriptive for people to read and understand, otherwise they won't be useful", and asked whether plain language would make them worse for training AI; separate docs only if it had to, and hopefully not. It does not have to: plain language serves both, and a model reads clear, current text best, as a person does. So this item also takes on:

- **A plain-language pass over both repositories' docs.** Much of the prose is roundabout ("somebody", "left alone", "as it comes", "what is due to go"). Rewritten in short, direct sentences, with one word for one thing throughout: "profile", not "actor", in what people read; "by default", not "as it comes". The facts, file paths, settings, endpoints and tables stay; only the way they are said changes. *Decided against on 2026-10-17: see [Where it stands](#where-it-stands-and-what-is-next-2026-10-17).*
- **Keeping them true.** A sweep of both repositories on 2026-10-14 found about 57 statements the code no longer bore out (features "not built" that were, settings documented but never passed, services long merged) and one bug behind them. They were fixed; this item makes the sweep a habit: a short checklist in `AGENTS.md` (when a feature changes, grep the docs for it, and remove "not built yet" the day it is), and a check in `make test` that every link between pages and every heading anchor resolves.

## Decisions

- **No separate documentation for agents.** One set, for both, in plain language.
- **`AGENTS.md` at the root of each repository** holds the working agreements. It is the name most tools read; `CLAUDE.md` in each becomes one line that brings it in, so nothing is said twice.
- **The MCP server only reads.** Docs, plans and the API reference. It has no way to the database, to a running site or to anything that changes something.
- **It runs from a clone first, and is hosted as soon as the docs are public:** the people it is for have no clone.

### Settled with the user on 2026-10-17

- **Row 29 comes after row 27,** which changed the schema rules it writes
  down.
- **The rules are split:** the project's in `AGENTS.md`, in each repository;
  how the user and their agent work together stays in `CLAUDE.md` (the web
  app's at the root, the services' in `.claude/`), which loads `AGENTS.md`
  with `@AGENTS.md` and adds only that.
- **The plain-language pass begins with one page,** whose style the user
  approves before the rest are done, one commit a page.
- **The MCP server comes last** in this row: it serves the docs the rest
  improves.

## Design

### 1. `AGENTS.md` in both repositories

Short, and only what cannot be found by reading the code:

- **Where to begin:** `docs/README.md`; the roadmap in anahita-web; a plan's "What was built" and "Different from the plan" for why something is as it is.
- **How to build and test:** `make up_build`, `make <service>`, `make test`; the throwaway MySQL and the `*_TEST_DSN` variables (a pointer to `getting-started.md`, not a copy).
- **Rules that have each been broken once:**
  - a schema change is a goose migration and the same change in `schema/init.sql` and `legacy/upgrade.sql`, then `check_sql_order.py`;
  - tests are never run against the dev database;
  - a single node read from the graph is not access-checked: use `permissions.CanReadByID`;
  - `go mod tidy` is not run on `anahita-libs`; add a library with `go get` at a version that moves nothing else;
  - after a change to `anahita-libs` or a `.proto`, everything is rebuilt.
- **Names:** dates end in `_at`; a group has followers; "custom feeds"; "profile", not "account"; SQL as named constants, one clause a line.
- **What is not to be built yet,** with the reason and the plan that says so (the Website actor, stories, collections).
- **A phase is finished when its docs match the code,** and finished plans move to `docs/plans/done/`.
- **Commit messages:** `area - added|changed|removed|fixed: …`.

For anahita-web, the same shape: lint only what changed, the MUI docs rule that `CLAUDE.md` has now, the buttons and spacing conventions.

**What stays out:** anything about one person's machine, accounts or cloud. Those are theirs.

### 2. Every page says where its code is

A "Where the code is" section at the end of each of the nineteen services pages and the five web pages: the packages, the tables, the settings. A table, a few lines. Two pages have it now.

### 3. The MCP server: `tools/docs-mcp`

A small program in anahita-services, in Go like the rest, on the official SDK (`github.com/modelcontextprotocol/go-sdk`; its state to be checked when this is built).

| Tool | Answers |
| --- | --- |
| `list_docs` | The pages and their sections, from the index |
| `read_doc(page, section?)` | A page, or one section of it |
| `search_docs(query)` | The sections that match, best first, from both repositories' docs and plans |
| `roadmap` | The rows and their states |
| `read_plan(item or name)` | A plan, with what was built |
| `list_endpoints(service?)` | Routes and a line on each, from the API reference |
| `read_endpoint(method, path)` | What one route takes, sends and refuses |
| `list_recipes`, `read_recipe(name)` | The kinds of site there is a starting configuration for, and what one decides and why ([recipes.md](../recipes.md)) |

- **Search is by words, in the program.** No embeddings and no outside service: it works offline and sends nothing anywhere.
- **From a clone:** `.mcp.json` in each repository starts it over stdio (`go run ./tools/docs-mcp`), given where the other repository is. Somebody working on the web app can then ask about the services without knowing their layout, which is the case the two repositories meet daily.
- **Hosted, later:** the same program over HTTP with the docs built into it, at an address beside the public docs, once anahita-services is public. Read by anybody, like the docs.

### 4. `llms.txt`, when the docs are on the web

A list of the pages with a line on each, made from `docs/README.md` by a script and checked by a test. It is for an agent that meets the docs as a website. In a clone the index does the same job, so it waits for the docs to be hosted.

### The model: what Material UI does (user, 2026-10-11)

The idea came from Material UI, which gives its users an MCP server and recipes, and whose server anahita-web already uses (`.mcp.json`, `@mui/mcp`). How theirs works is worth copying where it fits:

- **It is a small program people run themselves** (`npx @mui/mcp`), which reads the documentation from the project's own website. Nothing is installed on a server for each user, and it is always the current docs.
- **It has two tools:** one that gives the index of a package's docs, and one that fetches a page by an address the first one returned. The agent is told to go by those addresses and not by memory.
- **The index it reads is an `llms.txt`** on the docs site. So for readers without a clone, `llms.txt` and the docs being on the web are not an afterthought, as part 4 above has them: they are what the server stands on. When this is built, part 4 moves ahead of hosting anything.

What ours adds to that shape is the API reference's route lookups and the recipes. Whether ours is a program people run that reads the public docs, as theirs is, or a server we host, is decided when the docs have an address; the first is less to run and is the likelier.

## How much the MCP server is worth, said plainly

- **Somebody with both repositories cloned** gains little for prose: their agent reads the files. The gain is the route lookups, and one way into both repositories.
- **Somebody without a clone** (writing an app against an installation, or running one and asking an agent how) gains everything, and that is who a hosted one is for.
- So the order is: `AGENTS.md` first, which is an afternoon and helps the very next session; the server after the API reference has its first services, because the routes are what it adds.

## Estimate

| Part | Time |
| --- | --- |
| 1. `AGENTS.md` in both repositories; `CLAUDE.md` brought down to one line | 2 h |
| 2. "Where the code is" on every page | 2–3 h |
| 3. The server, from a clone: docs, plans and the roadmap | 4–5 h |
| 3. Its route tools, once `docs/api/` exists | 1–2 h |
| 3. Hosted | 2–3 h, when the docs are public |
| 4. `llms.txt` and its check | 1 h, then |
| The four doors in the index, and the list of pages found missing | 2–3 h |
| The plain-language pass over both repositories' docs | 6–8 h; decided against |
| The link and anchor check in `make test`, and the checklist | 1 h |
| **Total** | **about 21–28 h, 6–7 sessions**, without the missing pages themselves |

## Verification

- An agent started in a fresh clone with no memory is asked to add a column. It writes the migration in three places and runs the order check, having been told by `AGENTS.md` alone.
- From anahita-web, `search_docs("who can download a video")` returns the section of the services' `permissions.md`.
- `read_endpoint("PATCH", "/videos/:id/download")` returns the same shapes as `docs/api/`.
- The server refuses a path outside the docs (`read_doc("../.env")`).
- With the network off, everything above still answers.

## Open questions

1. **Are the docs to be public before anahita-services is?** The web app's README says the services come later in 2026. The hosted server waits for whichever comes first.
2. **How much of the working agreements belongs in the repository?** Some are how the project is built; a few are one person's way of working with an agent. A first draft will mark the ones in doubt.

## Dependencies

- **Part 1: none.** It can be done now.
- **The route tools** need [api-reference.md](api-reference.md), phase A at least.
- **Hosting** needs the docs to be public.
- **The recipe tools** need [recipes.md](../recipes.md).

## Documentation

- **services and web `getting-started.md`**: the MCP server and how an agent is pointed at it.
- **services `README.md`**, and **web `README.md`**: a line on `AGENTS.md`.

## Where it stands, and what is next (2026-10-17)

Written at the end of the session that began the row, for the next one to
start from. Both repositories are on a branch `docs-for-agents`, not yet
pushed.

**Done:**

1. Part 1: `AGENTS.md` in both repositories, with `CLAUDE.md` (the web
   app's at the root, the services' in `.claude/`) loading it and keeping
   only how the user and their agent work together. Services commit
   `4a333b2`, web `4a0fda17`.
2. On the web branch, roadmap rows 85 (regional English and French), 86
   (German), 87 (Spanish), 88 (Brazilian Portuguese) and 89 (a test that
   every language has every string), all after this row. Japanese was
   decided against: nobody to read it.
3. Part 2, "Where the code is", as a table (What | Where) on every page
   about one subject: thirteen services pages and the web app's
   Configuration and Customising, each linked from its index. Privacy's
   "Where it is enforced, and how it is tested" was already its code map and
   gained three rows. The pages about the whole (architecture, getting
   started, deploying, configuration in the services, the API) have none:
   the whole page is about where things are. Every path and name in them
   was checked against the code.
4. Found while doing part 2 and fixed: `oauth.md` said deleting a client
   cascades to its tokens and signs its users out. The schema has no
   foreign keys; the tokens stay and work again with a client seeded under
   the same `client_id`. And `DELETE /clients/:id` left them too, though
   its route said "Its tokens stop working": it now revokes them first
   (user, 2026-10-17).
5. The link and anchor check: `make docs_check` here
   (`scripts/check_docs_links.py`), part of `make test`, and
   `src/__tests__/docsLinks.test.js` in the web app, part of `yarn test`.
   Every relative link in a Markdown file git keeps must lead to a file
   there is, and a `#heading` to a heading of it, made as GitHub makes them.
   Links to the web are not followed. The web app's Markdown in `src/` is
   left out: its links are the app's addresses. Both found nothing broken.
   The checklist is in both `AGENTS.md`, under Docs.

6. The plain-language pass, decided against (user, 2026-10-17). A sample
   rewrite of `feeds.md` changed words for words of the same kind, which
   nobody would notice. What stays of it is four sentences that were hard to
   follow, and the page's code map as a table. From now on a sentence is
   rewritten only when it is unclear or wrong.
7. What part 2 found on the pages and the code did not bear out, fixed:
   `abuse-reports.md` (a `403`, and `409 already_open`), `web-shell.md`
   (videos, websites and `/tags/<tag>`), `permissions.md` (the five default
   files, and two sections missing from its Contents), the pages that left
   videos and replies out of `access` and `language`, the API reference's
   `POST /invites` (`INVITES_FROM`, not `REGISTRATION_MODE`), and in the web
   app, `src/containers/StaticPage.jsx`, which nothing used, removed.

8. The four doors: [Setting up a site](../../setting-up.md),
   [Customising](../../customising.md),
   [Operating](../../operating.md) and
   [Developing](../../developing.md), each a numbered list of links in
   the order to read them, under "Where to begin" in the index. The web
   app's index points to them. Each door ends with the pages found missing
   in filling it: backing up and restoring, and upgrading step by step
   (Keeping it running); the events on the bus, writing a service, and
   writing a client (Building on it); starting configurations for kinds of
   site, which is row 30 (Making it your own). Each is a page to plan.

**Settled with the user** (2026-10-17): the four lines of the services' old
`.claude/CLAUDE.md` left out of part 1 come back, in both repositories. Asking
before every commit, showing its message, and branching under `issues` on
`main` go in each `CLAUDE.md`; Star Trek names in tests and "Founder & Core
Architect: Rastin Mehr" go in each `AGENTS.md`. The two lines written new
in `AGENTS.md`, the sovereignty line and "every string is in
`src/languages/en-GB` and `fr-FR`", stay as written.

**Waiting for anahita-services to be public** under
`github.com/anahitasocial` (user, 2026-10-17): the MCP server
(`tools/docs-mcp`, Go, read only, offline, one server for both repositories'
docs, started from each `.mcp.json`), as Design part 3 describes, and
`llms.txt`. The people it is for have no clone, and cannot reach it before
then. Publishing also changes the web app's 55 links to
`github.com/purplerat/anahita-services`, which until then lead nowhere for
anybody else; they are left as they are and changed together. The recipe
tools wait for row 30 as well.

## anahita-mcp, a server of our own (user, 2026-10-27)

GitMCP (gitmcp.io) served the docs for a morning, and was dropped the same
day: its search was rough (a search for the half-life answered with the
README), its page broke in places we could not mend, and it is a service in
the United States, as Context7 and DeepWiki are; no European or Canadian
one was found. What was built instead is Design part 3 in the shape of
Material UI's server: a program people run themselves, reading the public
docs.

| Item | What it is |
| --- | --- |
| Repository | [anahitasocial/anahita-mcp](https://github.com/anahitasocial/anahita-mcp), MIT, TypeScript on the official MCP SDK, not `tools/docs-mcp` in Go: MCP clients start servers with `npx`, and a Go binary downloaded on macOS is blocked until it is signed and notarised |
| Run with | `npx -y anahita-mcp` (on [npm](https://www.npmjs.com/package/anahita-mcp) from 2026-10-27, published by rmdstudio), over standard input and output |
| Reads | `www.anahita.io/docs/index.json`, every page of both repositories' docs cut into sections, written by the website's build (`src/app/docs/index.json/route.js` in anahita.io), and `www.anahita.io/api/anahita.json` |
| Keeps | a copy of each in the system's cache folder, asked for again after an hour only if it changed (ETag), and used offline |
| Tools | `search_docs`, `list_docs`, `read_doc`, `list_endpoints`, `read_endpoint`, `read_shape`; all read-only |
| Search | by words (BM25), in the program; nothing sent anywhere |
| Hosted | also at `https://mcp.anahita.io/mcp` (Streamable HTTP), from 2026-10-27: Lambda behind an API Gateway HTTP API, all in ca-central-1, in the banu.io account; logs Lambda's own lines for a day and nothing of the requests (anahita-mcp `docs/hosting.md`) |

- **Left out of Design part 3:** `roadmap` and `read_plan`. The plans are
  work notes, read on GitHub, and are not on the website the server reads.
  The recipe tools go with [recipes.md](../recipes.md), row 30.
- **`llms.txt`** is on the website (`www.anahita.io/llms.txt`), written by
  hand, not made from `docs/README.md` by a script as part 4 has it.
- **Pointed to** from the "MCP server" card on `www.anahita.io/docs/`, from
  `llms.txt`, and from both repositories' `docs/README.md`.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-11; the plain-language pass and keeping the docs true added 2026-10-14 |
| Implementation | begun 2026-10-17. Parts 1 (`AGENTS.md`) and 2 (where the code is), and the link check, done; the plain-language pass decided against; the four doors done, 2026-10-17. `llms.txt` on the website, and the MCP server, [anahita-mcp](https://github.com/anahitasocial/anahita-mcp), 2026-10-27 ([anahita-mcp](#anahita-mcp-a-server-of-our-own-user-2026-10-27)). **Done** |
