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

## Decisions

- **No separate documentation for agents.** One set, for both.
- **`AGENTS.md` at the root of each repository** holds the working agreements. It is the name most tools read; `CLAUDE.md` in each becomes one line that brings it in, so nothing is said twice.
- **The MCP server only reads.** Docs, plans and the API reference. It has no way to the database, to a running site or to anything that changes something.
- **It runs from a clone first, and is hosted as soon as the docs are public:** the people it is for have no clone.

## Design

### 1. `AGENTS.md` in both repositories

Short, and only what cannot be found by reading the code:

- **Where to begin:** `docs/README.md`; the roadmap in anahita-web; a plan's "What was built" and "Different from the plan" for why something is as it is.
- **How to build and test:** `make up_build`, `make <service>`, `make test`; the throwaway MySQL and the `*_TEST_DSN` variables (a pointer to `getting-started.md`, not a copy).
- **Rules that have each been broken once:**
  - a schema change is a goose migration and the same change in `schema/init.sql` and `legacy/upgrade.sql`, then `check_sql_order.py`;
  - tests are never run against the dev database;
  - a single node read from the graph is not access-checked: use `handlers.MaySee`;
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
| `list_recipes`, `read_recipe(name)` | The kinds of site there is a starting configuration for, and what one decides and why ([recipes.md](recipes.md)) |

- **Search is by words, in the program.** No embeddings and no outside service: it works offline and sends nothing anywhere.
- **From a clone:** `.mcp.json` in each repository starts it over stdio (`go run ./tools/docs-mcp`), given where the other repository is. Somebody working on the web app can then ask about the services without knowing their layout, which is the case the two repositories meet daily.
- **Hosted, later:** the same program over HTTP with the docs built into it, at an address beside the public docs, once anahita-services is public. Read by anybody, like the docs.

### 4. `llms.txt`, when the docs are on the web

A list of the pages with a line on each, made from `docs/README.md` by a script and checked by a test. It is for an agent that meets the docs as a website. In a clone the index does the same job, so it waits for the docs to be hosted.

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
| **Total** | **about 14–19 h, 4–5 sessions**, without the missing pages themselves |

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
- **The recipe tools** need [recipes.md](recipes.md).

## Documentation

- **services and web `getting-started.md`**: the MCP server and how an agent is pointed at it.
- **services `README.md`**, and **web `README.md`**: a line on `AGENTS.md`.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-11 |
| Implementation | not started; place in the order not decided |
