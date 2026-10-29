# Anahita AI (item 95)

Asked for by the user on 2026-10-29, from a brief with seven candidate
features: a model that helps administrators, authors and readers, run as a
service of its own, with the model chosen by the installation. Planned, not
built. The first step is the brief's own: statistics for administrators,
and a box that answers questions about them in plain words.

## What there is

| What | Where it stands |
| --- | --- |
| AI of any kind | None in the services, the web app or anahita-mcp |
| Events between services | RabbitMQ, exchange `anahita_events` (`anahita-libs/event/`): posts added, edited and deleted, profiles, signups, follows. No event when a report is filed |
| Statistics for administrators | None by period. NodeInfo counts people (all, active in 30 and 180 days) and local posts (`auth-service`, `CountInstanceUsage`); `GET /reports/cases/summary` counts open cases |
| anahita-mcp | Public docs and the API reference only: no tokens, no API calls, and it promises to send nothing. Not where administrators' tools go ([Anahita MCP](../ai-agents.md)) |
| Alt text | Built (item 46): written by hand in the composer, kept in `photo_files.alt_text` |
| A post's language | Kept in `nodes.language`, chosen by the author (item 20) |
| Translation | Planned, not built: [translation.md](translation.md) (item 43), LibreTranslate and DeepL behind one interface |
| Search | Plain SQL (`graph-grpc-service/data/repositories/access_search.go`). MySQL 9.6 Community has a `VECTOR` type but not the functions to compare vectors (those are HeatWave's), so meaning-based search would need a store of its own |

## Principles

- **Off unless the installation turns it on.** Where the service is not
  running, the web app shows none of it, as with the recommendation and
  ActivityPub services.
- **The model is the installation's choice.** One interface, written to by
  each provider:
  - **Ollama**, a model run on the installation's own machines, which the
    documentation leads with, for the reason [translation.md](translation.md)
    leads with LibreTranslate;
  - **any OpenAI-compatible server** (vLLM, LM Studio and others);
  - **Anthropic**, for an installation that would rather pay for a larger
    model.
- **Nothing leaves the installation that its people did not agree to.**
  With a provider outside it, only what is open to everybody is sent, as
  for translation; the administrators' settings and the docs say in plain
  words which provider is in use. Counts sent for a question about
  statistics carry nobody's posts.
- **It suggests; people decide.** A draft is edited by its author before
  anything is saved; a flag goes to a human; nothing is removed, hidden or
  ranked by the model.
- **Nobody is tracked.** Statistics are counts for the whole site, never
  for one profile. Suggestions of whom to follow come only from interests a
  person has stated. No ranked default feed.
- **Nothing waits for the model.** The core services never call this one.
  It reads the event bus and the core's read APIs, keeps its own tables
  (`ai_` before their names, made by `migration-job`), and the web app calls
  it directly. When it is down or slow, only its buttons are missing.

## Phase 0 · The service

- `ai-service/`, laid out like the others: `cmd/api`, its own `go.mod`,
  `go.work`, `dockerfiles/ai.dockerfile`, `k8s/base/services/ai.yml` in
  `k8s/base/kustomization.yml`, Makefile targets, `docker-compose.yml`, and
  Ollama in compose for development.
- `ai-service/provider/`: the interface (`Complete`, with tools;
  `Describe`, for a picture; `Embed` later), the three providers, and their
  settings:

  | Setting | What it is |
  | --- | --- |
  | `AI_PROVIDER` | `ollama`, `openai` or `anthropic`; empty is off |
  | `AI_MODEL` | The model for text |
  | `AI_VISION_MODEL` | The model for pictures; empty is no alt-text drafts |
  | `AI_BASE_URL` | The provider's address, for Ollama and OpenAI-compatible servers |
  | `AI_API_KEY` | The provider's key, from a secret |

- Signing in as everywhere else (`anahita-libs/middleware/oauth.go`), the
  routes in the API reference through `apidoc`, and limits for each person
  with the shared limiter.
- `GET /ai/status`: which features are on, so the web app shows only those.
- Usage kept as counts: calls and tokens for each feature, each day.
  Questions and answers are not kept.

## Phase 1 · Statistics, and asking about them

- **Statistics, useful without a model:** `GET /admin/statistics` with
  `from`, `to` and `interval` (`day`, `week`, `month`), for administrators,
  in auth-service. New accounts, active accounts, local posts by kind,
  groups and events made, abuse cases opened and resolved, profiles and
  servers from elsewhere seen. One count query each, named constants laid
  out a clause a line.
- **A Statistics page** in Site settings: a period, a few numbers, a small
  chart for each.
- **Ask:** a box on that page. "How many new accounts this week?" goes to
  `POST /ai/admin/ask`; the model's only tools are the statistics, with the
  period as their argument. The answer is a sentence, the numbers it came
  from and the period taken, with a link to the chart. It reads only, sees
  no post or profile, and keeps nothing.

## Phase 2 · Alt-text drafts

- `POST /ai/photos/:id/describe`: a draft description from the vision
  model.
- **Suggest** beside the description field in the composer
  (`PhotoFilesEditor.jsx` in anahita-web): it fills the field, the author
  changes what they like, and nothing is saved until they post. A line
  beside it says the words are a model's draft.

## Phase 3 · Translation through the model

Once item 43 is built, the provider here becomes one more translation
service behind its interface. The original stays a tap away, as item 43
has it. Waits for item 43.

## Phase 4 · A thread in brief

- `GET /ai/threads/:id/summary`: a few lines, each linking to the reply it
  comes from.
- Kept in `ai_summaries`, and made again when the thread's
  `node.medium.*` events say it has changed.
- **Summarize** in the menu of a long thread only, past a number of
  replies the installation sets.

## Later, each to be planned on its own

| Feature | What it needs first |
| --- | --- |
| Search by meaning | Embeddings, a store to compare them in, the posts already here embedded, and what other servers allow |
| Groups and people to follow, from stated interests | A place for a person to state them; the recommendation service |
| Help with moderation | A `report.filed` event, flags with their reasons in the abuse-case queue, and examples to measure it against before it is trusted |
| Open science: citations from papers, answers from the community's posts | Search by meaning |

## How much

In sessions of about 4 hours, as the roadmap counts them, each with its
docs, its strings in every base language, tests, and a look in headless
Chrome.

| Phase | Sessions |
| --- | --- |
| 0 · The service and its providers | 2–3 |
| 1 · Statistics and the page | 2 |
| 1 · Ask | 2–3 |
| 2 · Alt-text drafts | 1–2 |
| 3 · Translation through the model, after item 43 (itself 3–4) | 1 |
| 4 · A thread in brief | 3–4 |
| **Phases 0 to 2** | **7–10** |

The features kept for later are larger: search by meaning about 8–12,
suggestions from interests 5–6, help with moderation 8–12 and the examples
to measure it, open science 12 or more.

## Open

- The service's name: `ai-service`, or a plain word for what it does.
- Whether Ask, which sends only counts, may use a provider outside the
  installation when the rest is kept to a model run in it.

## Checking it

- `go test ./...` in `ai-service` and auth-service, with a provider that
  answers what the test says.
- On the development stack with Ollama: ask how many accounts were made this
  week, and compare with a count in MySQL.
- Headless Chrome at 1280×900 and 390×844: the Statistics page, Ask, and
  Suggest in the composer.

## Documentation

| Page | What changes |
| --- | --- |
| `docs/ai.md` (new) | The service, its settings, the providers, what is sent where |
| [Anahita MCP](../ai-agents.md) | That it is not this service |
| [README](../README.md) | The new page and this plan |
| anahita-web `docs/README.md` | The Statistics page, Suggest, Summarize |
| The API reference | The new routes, through `apidoc` |

## Status

| Phase | State |
| --- | --- |
| 0 · The service | ⬜ |
| 1 · Statistics and Ask | ⬜ |
| 2 · Alt-text drafts | ⬜ |
| 3 · Translation through the model | ⬜ |
| 4 · A thread in brief | ⬜ |
