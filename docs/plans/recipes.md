# Anahita Recipes: a site ready to run, for each purpose

*Roadmap row 30 (item 40). Planned 2026-10-11 by kind of organisation;
**planned again 2026-11-07 by purpose, at the user's word,** after banu.io had
run on one low-cost server for a fortnight. See the
[roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md)
for order.*

## Contents

1. [Why](#why)
2. [Decisions](#decisions)
3. [The recipes](#the-recipes)
4. [What a site is made of](#what-a-site-is-made-of)
5. [What recipes will show is missing](#what-recipes-will-show-is-missing)
6. [Kept true](#kept-true)
7. [Phases](#phases)
8. [Documentation](#documentation)
9. [Status](#status)

## Why

The hardest part of running Anahita is not Anahita: it is Kubernetes and
the knowledge of running servers around it. Somebody who wants a site for
their newsroom or their street should not have to learn either. banu.io
shows what it takes when that knowledge is written down once: one low-cost
server with k3s, an overlay of a dozen files, and a Makefile in which every
task is one word (`make apply`, `make web`, `make status`, `make logs`).

A recipe is that, ready to copy, for one purpose: the configuration chosen
and explained, the Makefile, and the steps from an empty server to a site
people use. The microservices are then an advantage, not a burden: what a
site does not need is not run, and what it needs more of is given more.

(The user, 2026-11-07: "The biggest challenge for setting up Anahita is the
knowledge of devops and k8s. If we make it really simple for them with
prewritten config and make files, then they can really benefit from the
microservices architecture.")

## Decisions

1. **Recipes by purpose,** not by kind of organisation (the user,
   2026-11-07). Whether a site is open or for its members only, as a school or
   a company needs, is one choice inside any recipe.
2. **Their own repository, `anahita-recipes`,** on anahitasocial, public
   (settled 2026-10-26). **Each recipe a directory that stands alone**
   (the user, 2026-11-08): copied out on its own, it is a whole site, with
   its README, Makefile, `k8s/`, `deploy/` and `web/`, laid out the same in
   every one so that a reader knows where to look. What the recipes share is
   copied into each, and a check says when the copies differ. This replaced
   a shared `site/` with each recipe a Kustomize component added to it.
   **Named for a place or a practice** (the user, 2026-11-08): `commons`,
   `coffeehouse`, `journal`, `theatre`, `gathering`, and `php-migration`
   for the move from Anahita PHP. Plain words that collide with nothing in
   the code; `stream`, `channel` and `bridge` were left for that reason.
3. **One server first.** The path a recipe documents is banu.io's: one
   server, k3s, its own MySQL, Redis and RabbitMQ, a bucket for files. A
   larger site's path (EKS, managed databases) is in anahita-services'
   `deploying.md`, as now.
4. **A recipe is a start, not a lock.** Once copied, its `config.env` is
   the site's own; whatever the operator sets wins.
5. **A recipe invents no setting.** It chooses among the ones there are, and
   leaves out the services it does not need, with the line to put one back
   in its README (the user, 2026-10-26). What a purpose needs that no setting
   gives is written down as a gap, to plan on its own.
6. **Not all at once.** The first three first; the others when the web app
   can show a site's purpose (row 139) and their gaps are weighed.
7. **Built on what is there** (found 2026-11-07, before starting):
   anahita-services' `k8s/overlays/k3s` (everything on one server, Garage
   among it, with commented `config.env` and `secrets.env.example`) and
   banu.io's Makefile, web build and web-shell patch. `docs/deploying-k3s.md`,
   which the k3s overlay names, was never written (row 93's manual): the
   README of each recipe is that path, and the services' docs point to it.
8. **Reaching the server: `make tunnel` over SSH** (the user, 2026-11-07),
   `ssh -L 16443:127.0.0.1:6443`, so that Kubernetes' port is never open to
   the internet, at any provider. banu.io's tunnel, through AWS's SSM, is
   AWS's alone.
9. **Files in Garage on the same server** (the user, 2026-11-07): no cloud
   account needed; the README says how to use a provider's S3 store instead.
10. **The web app built by the operator for now** (the user, 2026-11-07):
    `make web`, with Node and yarn, as for banu.io. The first follow-up:
    anahita-web reads its server's address when it starts, not when it is
    built, so that one ready-made build each release serves every site and a
    recipe needs no Node (about a session, in anahita-web).
11. **Three providers written first** (the user, 2026-11-08): AWS in
    `ca-central-1`, which banu.io has proven; OVHcloud in Beauharnois,
    Quebec, which Gander Social runs on; and Hetzner in Europe. OVHcloud and
    Hetzner are widely used by Mastodon servers but untried with Anahita,
    and the docs say so and ask for reports. Other providers in Canada and
    Europe that should work are listed as untried candidates, ThinkOn among
    them for an organisation that needs a Canadian company
    ([providers.md](https://github.com/anahitasocial/anahita-recipes/blob/main/docs/providers.md)).

## The recipes

| Recipe | Like | What it runs and puts first | Gaps it shows | When |
| --- | --- | --- | --- | --- |
| **php-migration** | | Moving a site from Anahita PHP: row 92, made simple from what was done for banu.io. Archive the old site and prove the archive; copy the files to a new bucket; `upgrade.sql`; check; the old site retired. One `make` target a step, each one that changes anything shown first with nothing changed (`tidy`, then `tidy-apply`, as banu.io does). Examples of thinning a site, from banu.io's `migration/` (the user, 2026-11-08): inactive accounts purged but a list kept; old posts by kind and date; posts whose author is gone; photos moved to another profile with their files; groups purged; then the file sweep. Each counts first and keeps a copy of what it removes | Each step tried on a copy | First |
| **commons** | Mastodon and Pixelfed, but more | Everything: notes, photos, video, articles, groups, events, federation on both networks. What `generic` is now | | First |
| **coffeehouse** | Mastodon, Bluesky | Notes and photos; articles, topics, events and websites off; short limits on video | Needs row 139's switches | First, once 139 is built |
| **journal** | WordPress, Medium | Articles first: a home of article cards with their covers, notes beside them | RSS feeds; newsletters by email | Later |
| **theatre** | YouTube, PeerTube | Video first: a grid of thumbnails, larger uploads, the video worker given more | Adaptive streaming (HLS); captions; playlists | Later |
| **gathering** | Meetup, Luma | What is on, first; groups as the organisers | A calendar to subscribe to (iCal). No tickets: nothing is sold | Later |

## What a site is made of

```
commons/                each recipe a directory; copy one, and it is a site
  README.md             who it is for, what it decides, the three things to
                        decide before the first person joins, and the steps
                        from an empty server to the first person signed in
  Makefile              every task one word; `make` lists them
  k8s/
    kustomization.yml   anahita-services' manifests at a version, and the
                        site's names
    config.env          the settings it chooses, each with a line saying why
    secrets.env.example
    patches/            the services it leaves out, and how to put one back
  deploy/               the server and its DNS: aws/, ovhcloud/, hetzner/
  web/                  the theme, build.sh and publish.sh, as banu.io's
coffeehouse/            laid out the same
php-migration/          the same README and Makefile; migration/ in place
                        of k8s/
docs/providers.md       where a recipe runs, tried and untried
```

1. **`make init`** asks for the few things only the operator knows: the
   domain, the email the site sends from, where files are kept.
   It writes `config.env` and `secrets.env`, with keys it makes itself.
2. **The Makefile** is banu.io's, without what was banu.io's alone (the
   PHP site's clean-up, the archive): `tunnel`, `status`, `apply`, `web`,
   `restart`, `logs`, `tidy`, `mail-off`, `backup`, `upgrade VERSION=`.
3. **Upgrading** is changing one version and `make apply`; the README says
   what each release needs, from the services' `CHANGELOG.md`.

## What recipes will show is missing

1. **Turning kinds of post and of profile off,** and choosing how lists are
   shown: row 139, which `coffeehouse` and the later three need.
2. **Hashtags and places to begin with** (the user, 2026-10-12 and
   2026-10-14): a new site has none, so signing up's Interests step and the
   place picker offer nothing. A seed file each recipe carries, featured
   hashtags filling the Interests list, and a tool that writes the places
   for the operator to read before loading. A row of its own when a recipe
   needs it.
3. **A site-wide default for a new profile's privacy,** and for a reader's
   content warnings (row 122), for a site with minors.
4. **RSS feeds** (`journal`), **HLS and captions** (`theatre`), **iCal**
   (`gathering`): each a feature to plan when its recipe is reached.
5. **Terms and a privacy policy** to begin from, marked as a start and not
   legal advice.

## Kept true

1. **Every recipe builds:** a check runs `kubectl kustomize` on each
   recipe's `k8s/` with the example secrets, which needs no cluster.
2. **The copies agree:** the same check says when what the recipes share
   (the Makefile's common tasks, `deploy/`, `web/`) differs between them.
3. **Every setting a recipe names exists,** checked against anahita-services'
   `docs/configuration.md` at the version the recipe names.
4. **A recipe is tried for real** before it is published: a server made
   from it, the first person signed in, a post made, and the server taken
   down after.

## Phases

1. **`commons`:** banu.io's overlay and Makefile made general,
   `make init`, the README from an empty server; tried on a new server:
   about two sessions.
2. **`php-migration`:** row 92's steps as `make` targets, and the examples
   of thinning a site made general from banu.io's scripts, tried on a copy
   of a PHP site's data: about two sessions.
3. **`coffeehouse`,** after row 139: half a session.
4. **A Recipes page on anahita.io:** half a session.
5. **`journal`, `theatre`, `gathering`:** each with its gaps weighed first.

## Documentation

1. anahita-recipes: its README, and each recipe's.
2. anahita-services: `deploying.md` begins with "choose a recipe";
   `configuration.md` says which recipe chooses what.
3. anahita-web: `docs/customising.md`, what a recipe suggests for the web
   app.
4. anahita.io: a Recipes page.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-11 by kind of organisation; written again 2026-11-07 by purpose; each recipe a directory that stands alone, named, and three providers chosen, 2026-11-08 |
| Implementation | not started |
