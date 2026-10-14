# Anahita Recipes: a starting configuration for each kind of site

*Roadmap item 40, planned 2026-10-11 at the user's asking. **Moved out of M10 on 2026-10-13, to be done for the public release:** recipes are for when the code is made public, to show people how to set Anahita up for their kind of organisation, and that is not yet. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context

Somebody setting up Anahita for a school and somebody setting it up for a newsroom want different sites, and today both are handed the same `config.env` with some sixty settings and one page that explains each. The settings that matter most are a handful, and they have to agree with one another: a school that closes reading to members but leaves registration open has not done what it meant to.

A **recipe** is those choices made together, for one kind of site, with the reasons written beside them: a newsroom, a city council, a public library, a school, a university, a research group, a neighbourhood, a family or a group of friends.

**What exists:**
- `k8s/base` is every service; `k8s/overlays/{generic,aws,local}` say **where** a site runs (which cluster, which storage, which hostnames); `k8s/components/{mysql,redis,rabbitmq}` are parts an overlay may add.
- What kind of site it is, is decided by settings already there: `REGISTRATION_MODE`, `INVITES_FROM`, `SITE_READ_ACCESS`, `MIN_CONTENT_ACCESS`, `VIDEO_UPLOADS` and its limits, the export's waiting time, and by **which optional services are run at all**: recommendations, link previews, video, the data export, the web shell and, when they are built, the two federation services.
- The defaults for what people may do on a profile are files (`graph-grpc-defaults/actor_features/*.json`), as are the reasons for a report and the kinds of post in a feed.

## Decisions

- **A recipe says what kind of site; an overlay says where it runs.** The two are chosen separately and any recipe goes with any overlay.
- **A recipe is a Kustomize component,** `k8s/recipes/<name>/`, added to an overlay in one line. Nothing new to install or learn.
- **A recipe is a start, not a lock.** The overlay's own `config.env` is read after it, so whatever the operator sets wins.
- **A recipe invents no setting.** It only chooses among the ones there are. Where a kind of site needs something no setting gives, that is written down as a gap, and becomes a feature to plan or not, on its own.
- **How big a site is, is a third choice,** apart from what kind: `k8s/sizes/{small,medium,large}` set how many copies of each service run and what each may use. A recipe names the size it expects; the operator may take another.

## Design

### What a recipe is made of

```
k8s/recipes/school/
  kustomization.yml      kind: Component
  config.env             the settings it chooses, each with a line saying why
  README.md              who it is for, what it decides, what to change first
  patches/               services it leaves out, if any
  defaults/              its own actor_features or report reasons, if any
```

And in an overlay:

```yaml
components:
  - ../../components/mysql
  - ../../recipes/school
  - ../../sizes/small
```

### The first recipes

Only settings that exist are named. "Federation" means the services of item 8, and is in a recipe from the day they exist.

| Recipe | Who reads | Who joins | Optional services | Why |
| --- | --- | --- | --- | --- |
| **newsroom** | Anybody (`SITE_READ_ACCESS=public`) | Open, or by approval | Web shell, link previews, video, federation. Recommendations on | The point is to be read and passed on: a shared link has to show its own headline and picture, and posts should reach readers on Mastodon and Bluesky |
| **council** (a city, a public body) | Anybody | By approval | Web shell, link previews, federation, the data export. Events to the fore | Public by duty. Residents should be able to take what they wrote away with them, and meetings are events |
| **library** (a public library) | Anybody | By approval, or open | Web shell, link previews, federation, the data export. Events to the fore. Recommendations off | Public by duty, like a council, with programmes, readings and clubs as events and groups. A library does not say what its readers read: nothing here records who looked at what, and nothing is suggested from what somebody did unless the library turns that on |
| **school** | Members only (`registered`), and nothing new may be public (`MIN_CONTENT_ACCESS=registered`) | By invitation, from administrators only | Link previews (pages are fetched by the server, so no outside site sees a pupil's browser). No federation: a members-only site does not federate. Downloads off by default. Recommendations off | Minors. Nothing is seen from outside, nobody arrives uninvited, and nothing leaves |
| **university** | A preview to visitors (`preview`), the rest to members | By approval, or by invitation from any member | Video, events, the data export, recommendations. Federation is the operator's choice | Groups for courses and societies, public enough to be found, people who come and go each year |
| **research** (a laboratory, a research or scientific group, a learned society) | Anybody, or a preview to visitors | By invitation from any member, or by approval | Federation, link previews, video (talks), events (seminars, meetings), the data export. Small or medium | The work is meant to be read and passed on, and the people it is for are already on Mastodon and Bluesky. Groups for projects, which may be closed while the site is open. People move between institutions and should take what they wrote with them |
| **neighbourhood** | A preview to visitors | By approval; members may invite | Events, link previews. Small | Neighbours should find it and see that it is alive, and say who they are before reading what the street says |
| **household** (family, friends, one person) | Members only | By invitation from any member | As little as will run: no recommendations, video on with small limits, the data export. The smallest size | A few people who know each other. The cost of running it is what matters |
| **community** (the default; what `generic` is now) | Anybody | Closed until the operator chooses | Everything optional left to the operator | For whoever fits none of the others |

Each recipe's `README.md` ends with **the three things to decide before the first person joins**: usually the registration mode, the terms and privacy policy, and whether to federate, since the last cannot be cleanly undone.

### What recipes will show is missing

Writing them will turn up settings that do not exist. Expected, to be confirmed while writing and listed in the plan as they are found, not built here:

- ~~who may create a group or an event at all~~: there is one, `GROUPS_FROM` and `EVENTS_FROM`, found while writing this. A school's recipe sets both to administrators;
- a site-wide default for a new profile's own privacy;
- turning the report reasons to a kind of site (a school's list is not a newsroom's): possible today by replacing the file, which a recipe can do;
- **hashtags to begin with (user, 2026-10-12).** A recipe could carry the hashtags a site of its kind is likely to use (a school's subjects and years, a newsroom's beats, a library's subjects and programmes, a research group's fields and methods, a neighbourhood's streets and notices), put in when the site is first set up. A new site has none, so the "Interests" step planned for signup ([item 45](https://github.com/purplerat/anahita-web/blob/main/docs/plans/onboarding-recommendations.md)) would have nothing to offer until people had written. Needs a way to put hashtags in that is not somebody writing a post, which there is not today: a hashtag is made when a post names it, and belongs to whoever wrote that post;
- **a licence on what is posted (user's list, 2026-10-13: research groups).** A research group, and a library, would want to say that what is posted may be reused, and how. Creative Commons licences are planned and kept for later (item 19); the `research` recipe is the reason to look at it again;
- terms and a privacy policy fitted to each kind. A recipe can carry a **starting text marked as such**; it is not legal advice and must say so.

### The web app

A recipe is mostly the server's. For the web app it may suggest a name and description and which legal texts to begin from (`docs/customising.md`). Themes are not part of a recipe.

### Kept true

- **Every recipe builds.** `make test` runs `kubectl kustomize` on each recipe with the generic overlay and its example secrets, which needs no cluster.
- **Every setting a recipe names exists.** A check reads each recipe's `config.env` and fails on a name that `docs/configuration.md` does not have, and on a value that setting does not take.
- **The table in the docs is made from the recipes,** not written beside them.

## Estimate

| Part | Time |
| --- | --- |
| The form: components, sizes, how an overlay takes one; `community` and `school` as the first two, since they are furthest apart | 4–5 h |
| The other seven, each with its README | 6–7 h |
| The checks, and the table made from the files | 2 h |
| `docs/recipes.md`; `deploying.md` begins with "choose a recipe" | 2 h |
| **Total** | **about 14–16 h, about 4 sessions** |

## Verification

- `kubectl kustomize` of `generic` with `recipes/school` gives a ConfigMap with reading closed to members, invitations from administrators, and no recommendation service among the Deployments.
- The same overlay with `SITE_READ_ACCESS=preview` in its own `config.env` gives `preview`: the operator's word is last.
- A recipe naming a setting that does not exist fails `make test`.
- On a local cluster, `school` and `newsroom` each come up, and a visitor who is not signed in sees nothing on the one and the front page on the other.

## Open questions

1. **Are these the right ones,** and the right names? The user's own list (2026-10-11) also has **a company, a non-profit and a niche community**, which are to be added: a company's own staff is close to `school` without the care for minors (members only, by invitation, no federation unless chosen); a non-profit is close to `council` (public, with supporters joining by approval); a niche community is `community` with federation on, since reaching the fediverse is why such a site is made. A conference was considered and left for the asking.
2. **Should a recipe leave services out, or only turn them off?** Leaving them out costs less to run; turning them off is one setting to change later. The plan assumes out, with the line to put one back in the README.
3. **Where do recipes live? Not decided; still being thought over (user, 2026-10-11).** Two places were raised:
   - **A repository of its own, `anahita-devops`,** for recipes and whatever else is about running a site and not about the code. For it: people who run a site and never build it clone only that; others can add a recipe without touching the services; it has its own pace of release. Against it: `k8s/base` is in anahita-services, so a recipe would point at a release of another repository, and a setting renamed there breaks a recipe here with nothing to say so until somebody builds it.
   - **In anahita-services, beside `k8s/`,** as this plan is written. For it: the checks above run with every change to a setting. Against it: the services repository grows a part its developers seldom touch.
   - Whichever is chosen, the form of a recipe is the same, so the choice can wait until the first two exist.

## Dependencies

- **None hard:** every setting the first recipes use exists (items 9 and 13).
- **Item 8 (federation):** each recipe gains its line on federating when the services exist.
- **[docs-for-agents.md](docs-for-agents.md):** its MCP server lists and reads recipes, so that somebody can ask an agent "which recipe for a school of 300, and what do I change first?"

## Documentation

- **New services page `recipes.md`**: what a recipe is, the table, how to take one and how to write one.
- **services `deploying.md`**: the first step becomes choosing a recipe and a size.
- **services `configuration.md`**: each setting a recipe chooses says which recipes choose what.
- **web `customising.md`**: what a recipe suggests for the web app.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-11; `library` and `research` added 2026-10-13 |
| Implementation | not started, and not before the public release (2026-10-13). A sketch while the idea is talked through: where recipes live, and which ones, are open |
