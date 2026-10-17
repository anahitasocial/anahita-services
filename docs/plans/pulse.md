# Pulse: a website's feed as something to follow

*Roadmap item 30, raised by the user on 2026-10-05 as "the Website actor". **Renamed Pulse and narrowed to the feed alone by the user on 2026-10-15: see [The scope](#the-scope-settled-2026-10-15).** What follows it is what was said before, kept for the reasons behind it: where it says Website, it means a Pulse.*

## The scope, settled 2026-10-15

**A Pulse is the feed of a website, verified, as something to follow.** Any website or app that publishes an RSS, Atom or JSON feed on a domain its owner controls can have one: a WordPress or other site, a PHP site written by hand, a static site made by Jekyll or Hugo, an app's changelog, a podcast, a status page. Nothing about the site's software is asked, only its feed and its domain. Its posts reach people here and across the fediverse, and send readers to the site: the site stays where it is written. A site that already speaks ActivityPub is followed as it is.

- **The name (user, 2026-10-15).** *Pulse*: the steady beat of a site's posts. It is what a site would say of it: "Follow our pulse on the Fediverse". *Website* was the name before; it was given up because "create a website" reads as if Anahita made one, and because a Pulse is not the site but its feed. The verified domain is shown on the profile, which is what says what is behind it ("example.com ✓"). Kept as *Pulse* in every language. Weighed and kept the same day: a pulse is checked more than followed, and in British English *pulses* are also lentils and beans; the user chose Pulse over Website and Channel all the same. It belongs with the words of life of the project: Anahita is the goddess of love, fertility and wisdom, and its releases are called *birth* and *embryo*; and it is a Pink Floyd album, which the user likes. Considered on the way: Channel, Publisher, Feed (taken by the home feed and custom feeds), Relay, Repeater, Syndicate, Outpost.
- **Only the feed posts (user, 2026-10-15).** No composer: nobody posts notes, photos, videos or articles on a Pulse, its administrators included. Each new item of the feed becomes a post owned by the Pulse. Everybody else interacts with those posts as with any: replies, likes, reposts, quotes. The site's own software is where it writes.
- **Its administrators reply as the Pulse, under its own posts (user, 2026-10-15).** In the threads under a Pulse's posts, an administrator's reply is the Pulse's: its author is the Pulse (`created_by` is the Pulse's id), and it is shown and sent out as the Pulse's, so the site answers its readers. Anywhere else administrators reply as themselves; there is no "reply as" choice. Which administrator wrote each such reply is kept, shown to nobody, for the installation's moderators when it is reported (the user, the same day).
- **What it is.** A kind of profile (`node.actor.actor-service.pulse.v1`): name, avatar (the site's icon to begin with), cover, description, the site's address and its verified domain, followers. Administrators as a group has, who look after it; followers, who read.
- **Its administrators** set it up and keep it right (name, picture, the feed's address), and look after the threads under its posts as any post's owner does: they hide replies, and block somebody from the Pulse as a group's administrators do (item 55). Under the Pulse's own posts they reply as the Pulse (above); elsewhere, as themselves.
- **Who sets one up.** Only somebody who proves the domain is theirs (below). On top of that, `PULSES_FROM`, as `GROUPS_FROM`: `administrators` unless an installation opens it to `registered`. One Pulse per feed address. Whoever proved the domain is its first administrator, and adds others as a group's are added. Offered as **"Connect your website"**, never "create": the first step asks for the site's address. A feed on a domain one does not control (a YouTube channel's, a GitHub project's) is not taken.
- **Proving the domain (user, 2026-10-15).** Built once, so that people and groups use the same proof later (verified domains, item 26): one table `verified_domains` keyed by profile and one checker, a DNS TXT record at `_anahita.<domain>` or the token at `https://<domain>/.well-known/anahita-verification.txt` (one committed file for a static site), checked again daily; a Pulse whose proof is gone for a week pauses. This replaces the earlier TXT record `anahita-site-verification=<token>` at the domain itself.
- **Posts.** Each new item becomes an article owned by the Pulse (the user, 2026-10-16: since row 74 a note has no title): its title, a short excerpt and a link card to the original (the link-preview service), its categories as hashtags, its own date. Items are remembered by their ID or address and never posted twice. On setting up, what the feed holds is marked seen, not posted; an hourly cap holds back a burst and posts it a few at a time. An item whose title or excerpt changes while it is still in the feed changes its post; one that leaves the feed leaves its post as it is, since feeds seldom say that something was deleted (user, 2026-10-15).
- **Reading the feed.** By a `pulse-worker`, apart from the core services since it reads addresses on the internet, with the link-preview service's safe fetcher (no private addresses, limits of size and time). Asked every 15 to 60 minutes with `ETag` / `If-Modified-Since`, backing off from a site that fails. A feed naming a WebSub hub (`rel="hub"`) is subscribed to instead, and its posts arrive within seconds.
- **Between networks.** An ActivityPub actor served by `activitypub-service`: people on Mastodon and the rest of the fediverse find and follow a Pulse as any account here (the user, 2026-10-15), and its posts and its replies reach them; replies from there come back to its threads. An `Organization`, not a `Service`: Mastodon marks a `Service` as automated, and its administrators write as it. Its posts public. Not on Bluesky.
- **Replies.** Open, from here and from other servers.
- **"Follow our pulse on the Fediverse" (user, 2026-10-15).** A few lines of HTML in the Pulse's settings that the site can paste in, leading to its Pulse here.
- **Handle (user, 2026-10-15).** An alias the owner picks, as a group has, suggested from the domain (`@example` for example.com), with the verified domain shown beside it.
- **Not in it:** posting on a Pulse (replying as it under its own posts is in it), plugins or dashboards for content management systems, signing in to them with Anahita accounts, Bluesky, feeds on domains one does not control.
- **Estimate:** about 4 sessions.

| Part | Time |
| --- | --- |
| Proving a domain, built to be shared: token, DNS and file checks, the daily check again, pausing | 2–3 h |
| The Pulse as a kind of profile in actor-service: connecting one, administrators, `PULSES_FROM`, one per feed | 2–3 h |
| `pulse-worker`: finding the feed, reading it with the safe fetcher, polling with `ETag`, WebSub, items to posts with cards and hashtags, the first-fetch rule and hourly cap | 5–6 h |
| Replies as the Pulse under its own posts, with who wrote each kept for moderators | 2 h |
| ActivityPub: a Pulse as an `Organization` actor, followed from the fediverse, its posts and replies going out, replies to it coming in | 2–3 h |
| Web: connecting a website (address, the proof, checking it), the Pulse's page, a Pulses list, the follow snippet | 4–5 h |
| Tests, docs | 2 h |
| **Total** | **about 18–23 h, 4–5 sessions** |

## The design, 2026-10-16

Settled with the user on 2026-10-16: a feed's item becomes an **article** (a note has no title since row 74); the work that fetches the internet (finding a site's feed, checking the domain proof, reading feeds) lives in a **pulse-service** of its own with a **pulse-worker**, as video-service has, and nothing in actor-service fetches anything; the ActivityPub `Article` is built for a Pulse's posts only, and row 84 reuses it for everybody's.

**Who does what**

| Piece | Service | Does |
| --- | --- | --- |
| Connecting a website | pulse-service (API) | finds the feed, gives the token, checks the proof, makes the Pulse |
| Reading feeds, checking domains again | pulse-worker | polls each feed, posts new items as articles, checks every proof daily, pauses a Pulse whose proof is gone for a week |
| The Pulse as a profile | actor-service | read, edit, browse, administrators, delete, as for a group |
| Its posts and the replies under them | text-service | replies by its administrators are the Pulse's |
| On the fediverse | activitypub-service | an `Organization`, always on, its posts as `Article`s |

**Tables** (owned by pulse-service)

| Table | Holds |
| --- | --- |
| `verified_domains` | a domain somebody is proving or has proved: the person proving it, the profile it is for (empty until the Pulse is made), the token, when it was proved, last checked, failing since. Kept apart from Pulses so that people and groups use the same proof later (row 63) |
| `pulses` | one row per Pulse, keyed by its node: the feed's address (unique), the site's address, its domain, its state (`active`, `paused`), `ETag` and `Last-Modified`, the WebSub hub, when it is next read, failures in a row, the last error, how many it posted this hour |
| `pulse_items` | every item seen, by its id or address (hashed), with the article it became and a hash of its title and excerpt, so nothing is posted twice and a changed item changes its post |

**Connecting a website**, three calls, signed in, for somebody `PULSES_FROM` allows (`administrators` by default):

1. `POST /pulses/_connect` with the site's address: the feed is found (the address itself, or the page's `<link rel="alternate">` for RSS, Atom or JSON Feed), and the answer is the domain, the feed, its title, and the token with both places to put it: a TXT record at `_anahita.<domain>`, or the file `https://<domain>/.well-known/anahita-verification.txt`.
2. `POST /pulses/_connect/:id/check`: either place is looked at; the answer says which was found, or why neither was.
3. `POST /pulses/_connect/:id` with a name, a handle (suggested from the domain) and a description: the Pulse is made, with whoever proved the domain as its first administrator and first follower, and what the feed holds now is marked seen, not posted.

A feed already connected is refused (`409`), as is a handle that is taken.

**Its posts.** Each new item becomes an article owned by the Pulse: the item's title; as its content, a short excerpt (300 characters, cut at a word), its categories as hashtags, and its address on a line of its own, which gives the link card; the item's own date, if it is not in the future. Made by the worker as the Pulse's first administrator, as the video-worker makes a post as its uploader. At most `PULSE_HOURLY_CAP` (10) an hour; the rest wait for the next hour.

**Replies as the Pulse.** In a thread whose root is the Pulse's, a reply by one of its administrators is the Pulse's (owner and author the Pulse); the administrator is kept in the reply's meta as `written_by`, never sent, read only by moderators handling a report.

**On the fediverse.** WebFinger finds a Pulse by its handle, its actor is an `Organization`, it needs no opt-in, its articles go out as `Article` (`name`, `summary` the excerpt, `content`, `url` the original), and replies to them come back into its threads, as they do for a person's notes.

**Parts, one commit each**

| # | Part | Repo |
| --- | --- | --- |
| 1 | Foundations: the Pulse and article types in anahita-libs; a post made by a service may keep its own date | services |
| 2 | Proving a domain: `anahita-libs/domains` (DNS TXT, the well-known file, by `safehttp`), `verified_domains` | services |
| 3 | pulse-service: connecting a website, `pulses`, `PULSES_FROM`, compose, nginx, k8s | services |
| 4 | actor-service and the rest of the stack know a Pulse: routes, features without composers, every list of kinds | services |
| 5 | pulse-worker: reading feeds, items to articles, the cap, edits, the daily proof, pausing | services |
| 6 | Replies as the Pulse, `written_by` for moderators | services |
| 7 | ActivityPub: `Organization`, always on, `Article` out, replies in | services |
| 8 | WebSub: a hub named by the feed is subscribed to | services |
| 9 | Web: Connect your website, a Pulse's page and settings, Pulses in the left menu, the follow snippet | web |
| 10 | Docs: pulses.md, federation, configuration, permissions, architecture | services + web |

## Before 2026-10-15: the Website actor

The sections below are the plan as it stood, with the scope settled on 2026-10-14 (a page its administrators also posted as, an `Organization` on ActivityPub), which the section above replaces.

## The goal, in the user's words

To help every existing website become a node in the fediverse: to network the traditional websites of the internet. Not to replace them or take their content. To send them readers and make them part of the wider community.

The first sites in mind are the ones run on the popular open-source PHP systems: WordPress, Drupal and Joomla. Plugins for those, to connect a site to an Anahita installation, are in scope if they are needed.

## The name

**Website.** Chosen by the user on 2026-10-05. People, Groups, Events, Websites: "follow this website" needs no explaining.

**Syndication** is the word for the mechanism, how a website's posts arrive, in code and in technical writing. It is not the name of the thing.

Considered and not chosen:

| Name | Why not |
| --- | --- |
| Relay | Already means something in the fediverse: a server that passes public posts between instances |
| Repeater | Right for the mechanism, but hides that there is a publisher behind it |
| News | Too narrow. A shop, a portfolio or a manual is not news |
| Syndicate | As a noun it reads as a cartel |
| Site | Our own documentation and settings use "site" for the Anahita installation |

## What it would be

- **A kind of actor,** served by actor-service ([done/actor-service.md](done/actor-service.md)): a name, an avatar, followers, and the address of the website it stands for. Type `node.actor.actor-service.website.v1`. On ActivityPub, a `Service` actor, which Mastodon labels as automated.
- **A fetcher, as a worker of its own.** It asks for arbitrary addresses on the internet, which is the risk the link-preview plan keeps apart from the core services, and it runs on a schedule and not per request. It reads each website's feed, skips what it has seen, and posts what is new as posts owned by that Website, each linking to the original.

## Three ways a website can take part

1. **It already speaks ActivityPub.** WordPress and Drupal have plugins for it. Once this installation federates, such a site is followed directly and needs nothing from us.
2. **It offers only a feed (RSS or Atom).** A Website actor here follows the feed and posts an excerpt and a link.
3. **It runs a plugin of ours.** What a feed cannot do: prove who owns the site, tell us the moment something is published instead of being asked, and carry replies back to the site.

## To be decided before any design

- **Who may make one.** Anybody, administrators only, or only somebody who proves they control the site. Standing in for a site you do not own is republishing somebody else's work. Proof of control is already planned for people and groups in [verified-domains.md](verified-domains.md), and would be the same proof.
- **An excerpt or the whole text.** An excerpt with a link is safer and sends readers to the site, which is the goal. The whole text reads better here.
- **Replies.** The author is not here to answer. Either replies are off on a Website's posts, or somebody who has proved they own the site can claim the Website and look after it, as a group's administrators do. With a plugin, replies could go back to the site.
- **How far the plugins go,** and which system first.
- **Whether a Website on this installation is followable from the fediverse** before the site itself is, which makes this installation the site's presence there. That is useful, and it is also speaking for the site.

## Depends on

- actor-service (done).
- A durable job queue for the fetcher (planned with federation and the video service).
- Federation (item 8), for a Website to be followed from elsewhere.
- Verified domains (item 26), for proof of control.
- Stable addresses for media, if images are copied.

## Status

| Item | State |
| --- | --- |
| Goal and name | recorded 2026-10-05; renamed Pulse 2026-10-15 |
| Scope | settled 2026-10-14; narrowed to the feed alone 2026-10-15; its open points settled the same day |
| Design | 2026-10-16: an article per item, pulse-service and pulse-worker, Article out for Pulses only |
| Parts 1–10 | not started |
