# Websites: a site's feed as something to follow

*Roadmap item 30, raised by the user on 2026-10-05 as "the Website actor". Narrowed to the feed alone by the user on 2026-10-15, and named Pulse then; **named Website again on 2026-10-16, for trademarks: see [The name](#the-name-settled-2026-10-16).** The sections after the design are what was said before, kept for the reasons behind it.*

## The scope, settled 2026-10-15

<a id="the-name-settled-2026-10-16"></a>

**A website is the feed of a website, verified, as something to follow.** Any website or app that publishes an RSS, Atom or JSON feed on a domain its owner controls can have one: a WordPress or other site, a PHP site written by hand, a static site made by Jekyll or Hugo, an app's changelog, a podcast, a status page. Nothing about the site's software is asked, only its feed and its domain. Its posts reach people here and across the fediverse, and send readers to the site: the site stays where it is written. A site that already speaks ActivityPub is followed as it is.

- **The name, settled 2026-10-16.** *Website*, and *websites*: "Connect your website", "Follow our website on the fediverse". It was *Pulse* from 2026-10-15, chosen for the steady beat of a site's posts and the project's words of life (Anahita is the goddess of love, fertility and wisdom, and its releases are called *birth* and *embryo*). The user gave it up on 2026-10-16 for trademarks: several businesses already trade as Pulse (pulse-syndicate.com, pulseservices.ca, mediapulse.ca), and an operator who runs Anahita for a business would show the name to its members. A plain word for the very thing it names can be trademarked by nobody, which a coined or themed one can. *Website* was the first name, set aside because "create a website" read as if Anahita made one; connecting a website, never creating one, answers that, and the profile is shown with its verified domain ("example.com ✓"). Weighed and refused on 2026-10-16: Pulls (it sounds like Pulse, is GitHub's word for pull requests, and is a sweater in French), Syndicate (as a noun it reads as a cartel), Spring and Wellspring (themed, so no safer than Pulse), Source, Site feed. *Syndication* stays the word for the mechanism, in code and technical writing: the service that finds feeds, proves domains and reads feeds is **syndication-service**, with **syndication-worker** (the user, 2026-10-16); the website itself, as a profile, is actor-service's.
- **Only the feed posts (user, 2026-10-15).** No composer: nobody posts notes, photos, videos or articles on a website, its administrators included. Each new item of the feed becomes a post owned by the website. Everybody else interacts with those posts as with any: replies, likes, reposts, quotes. The site's own software is where it writes.
- **Its administrators reply as the website, under its own posts (user, 2026-10-15).** In the threads under a website's posts, an administrator's reply is the website's: its author is the website (`created_by` is the website's id), and it is shown and sent out as the website's, so the site answers its readers. Anywhere else administrators reply as themselves; there is no "reply as" choice. Which administrator wrote each such reply is kept, shown to nobody, for the installation's moderators when it is reported (the user, the same day).
- **What it is.** A kind of profile (`node.actor.actor-service.website.v1`): name, avatar (the site's icon to begin with), cover, description, the site's address and its verified domain, followers. Administrators as a group has, who look after it; followers, who read.
- **Its administrators** set it up and keep it right (name, picture, the feed's address), and look after the threads under its posts as any post's owner does: they hide replies, and block somebody from the website as a group's administrators do (item 55). Under the website's own posts they reply as the website (above); elsewhere, as themselves.
- **Who sets one up.** Only somebody who proves the domain is theirs (below). On top of that, `WEBSITES_FROM`, as `GROUPS_FROM`: `administrators` unless an installation opens it to `registered`. One website per feed address. Whoever proved the domain is its first administrator, and adds others as a group's are added. Offered as **"Connect your website"**, never "create": the first step asks for the site's address. A feed on a domain one does not control (a YouTube channel's, a GitHub project's) is not taken.
- **Proving the domain (user, 2026-10-15).** Built once, so that people and groups use the same proof later (verified domains, item 26): one table `verified_domains` keyed by profile and one checker, a DNS TXT record at `_anahita.<domain>` or the token at `https://<domain>/.well-known/anahita-verification.txt` (one committed file for a static site), checked again daily; a website whose proof is gone for a week pauses. This replaces the earlier TXT record `anahita-site-verification=<token>` at the domain itself.
- **Posts.** Each new item becomes a note of the website, its title in bold as a note has none (the user, 2026-10-16, after it was an article for a day: an item is a short post pointing elsewhere, Mastodon shows a `Note` with its card where it shows an `Article` as a title and a link, and articles stay writing done here). A website's own notes are never edited or narrowed, its administrators' included: they follow the site. Private feeds, behind a login or with a secret in the address, are refused: its title, a short excerpt and a link card to the original (the link-preview service), its categories as hashtags, its own date. Items are remembered by their ID or address and never posted twice. On setting up, its ten newest items are posted and the rest marked seen, not posted (the user, 2026-10-16: a website of a site that posts seldom opened empty); an hourly cap holds back a burst and posts it a few at a time. An item whose title or excerpt changes while it is still in the feed changes its post; one that leaves the feed leaves its post as it is, since feeds seldom say that something was deleted (user, 2026-10-15).
- **Reading the feed.** By a `syndication-worker`, apart from the core services since it reads addresses on the internet, with the link-preview service's safe fetcher (no private addresses, limits of size and time). Asked every 15 to 60 minutes with `ETag` / `If-Modified-Since`, backing off from a site that fails. A feed naming a WebSub hub (`rel="hub"`) is subscribed to instead, and its posts arrive within seconds.
- **Between networks.** An ActivityPub actor served by `activitypub-service`: people on Mastodon and the rest of the fediverse find and follow a website as any account here (the user, 2026-10-15), and its posts and its replies reach them; replies from there come back to its threads. An `Organization`, not a `Service`: Mastodon marks a `Service` as automated, and its administrators write as it. Its posts public. **On Bluesky through its own account (the user, 2026-10-16, reversing "not on Bluesky"):** one of its administrators connects the site's own Bluesky account, its domain as its handle if they like (`@rmdstudio.com`), and its notes are posted there too; so a website is followed on both. Its handle here keeps no dot, since here a dot names somebody on Bluesky.
- **Replies.** Open, from here and from other servers.
- **"Follow our website on the Fediverse" (user, 2026-10-15).** A few lines of HTML in the website's settings that the site can paste in, leading to its website here.
- **Handle (user, 2026-10-15).** An alias the owner picks, as a group has, suggested from the domain (`@example` for example.com), with the verified domain shown beside it.
- **Not in it:** posting on a website (replying as it under its own posts is in it), plugins or dashboards for content management systems, signing in to them with Anahita accounts, an account on Bluesky made for it by this installation, feeds on domains one does not control.
- **Estimate:** about 4 sessions.

| Part | Time |
| --- | --- |
| Proving a domain, built to be shared: token, DNS and file checks, the daily check again, pausing | 2–3 h |
| The website as a kind of profile in actor-service: connecting one, administrators, `WEBSITES_FROM`, one per feed | 2–3 h |
| `syndication-worker`: finding the feed, reading it with the safe fetcher, polling with `ETag`, WebSub, items to posts with cards and hashtags, the first-fetch rule and hourly cap | 5–6 h |
| Replies as the website under its own posts, with who wrote each kept for moderators | 2 h |
| ActivityPub: a website as an `Organization` actor, followed from the fediverse, its posts and replies going out, replies to it coming in | 2–3 h |
| Web: connecting a website (address, the proof, checking it), the website's page, a websites list, the follow snippet | 4–5 h |
| Tests, docs | 2 h |
| **Total** | **about 18–23 h, 4–5 sessions** |

## The design, 2026-10-16

Settled with the user on 2026-10-16: a feed's item becomes a **note** (an article for a day first; see Posts above); the work that fetches the internet (finding a site's feed, checking the domain proof, reading feeds) lives in a **syndication-service** of its own with a **syndication-worker**, as video-service has, and nothing in actor-service fetches anything; a website's notes go out as anyone's do.

**Who does what**

| Piece | Service | Does |
| --- | --- | --- |
| Connecting a website | syndication-service (API) | finds the feed, gives the token, checks the proof, makes the website |
| Reading feeds, checking domains again | syndication-worker | polls each feed, posts new items as notes, checks every proof daily, pauses a website whose proof is gone for a week |
| The website as a profile | actor-service | read, edit, browse, administrators, delete, as for a group |
| Its posts and the replies under them | text-service | replies by its administrators are the website's |
| On the fediverse | activitypub-service | an `Organization`, always on, its notes as anyone's |

**Tables** (owned by syndication-service)

| Table | Holds |
| --- | --- |
| `verified_domains` | a domain somebody is proving or has proved: the person proving it, the profile it is for (empty until the website is made), the token, when it was proved, last checked, failing since. Kept apart from websites so that people and groups use the same proof later (row 63) |
| `websites` | one row per website, keyed by its node: the feed's address (unique), the site's address, its domain, its state (`active`, `paused`), `ETag` and `Last-Modified`, the WebSub hub, when it is next read, failures in a row, the last error, how many it posted this hour |
| `website_items` | every item seen, by its id or address (hashed), with the note it became and a hash of its title and excerpt, so nothing is posted twice and a changed item changes its post |

**Connecting a website**, three calls, signed in, for somebody `WEBSITES_FROM` allows (`administrators` by default):

1. `POST /websites/_connect` with the site's address: the feed is found (the address itself, or the page's `<link rel="alternate">` for RSS, Atom or JSON Feed), and the answer is the domain, the feed, its title, and the token with both places to put it: a TXT record at `_anahita.<domain>`, or the file `https://<domain>/.well-known/anahita-verification.txt`.
2. `POST /websites/_connect/:id/check`: either place is looked at; the answer says which was found, or why neither was.
3. `POST /websites/_connect/:id` with a name, a handle (suggested from the domain) and a description: the website is made, with whoever proved the domain as its first administrator and first follower, and what the feed holds now is marked seen, not posted.

A feed already connected is refused (`409`), as is a handle that is taken.

**Its posts.** Each new item becomes a note of the website: the item's title in bold, then a short excerpt (300 characters, cut at a word), its categories as hashtags, and its address on a line of its own, which gives the link card; the item's own date, if it is not in the future. Made by the worker as the website's first administrator, as the video-worker makes a post as its uploader. At most `WEBSITE_HOURLY_CAP` (10) an hour; the rest wait for the next hour.

**Replies as the website.** In a thread whose root is the website's, a reply by one of its administrators is the website's (owner and author the website); the administrator is kept in the reply's meta as `written_by`, never sent, read only by moderators handling a report. **Built for replies in words (2026-10-16); to settle with the user:** a reply with pictures, a GIF or a video by an administrator under a website's post is still the administrator's own, since its files are uploaded for them, and making them the website's reaches into photo-service and video-service.

**On the fediverse.** WebFinger finds a website by its handle, its actor is an `Organization`, it needs no opt-in, its notes go out as anyone's, and replies to them come back into its threads, as they do for a person's notes.

**Parts, one commit each**

| # | Part | Repo |
| --- | --- | --- |
| 1 | Foundations: the website and article types in anahita-libs; a post made by a service may keep its own date | services |
| 2 | Proving a domain: `anahita-libs/domains` (DNS TXT, the well-known file, by `safehttp`), `verified_domains` | services |
| 3 | syndication-service: connecting a website, `websites`, `WEBSITES_FROM`, compose, nginx, k8s | services |
| 4 | actor-service and the rest of the stack know a website: routes, features without composers, every list of kinds | services |
| 5 | syndication-worker: reading feeds, items to articles, the cap, edits, the daily proof, pausing | services |
| 6 | Replies as the website, `written_by` for moderators | services |
| 7 | ActivityPub: `Organization`, always on, `Article` out, replies in | services |
| 8 | WebSub: a hub named by the feed is subscribed to | services |
| 9 | Web: Connect your website, a website's page and settings, websites in the left menu, the follow snippet | web |
| 10 | Docs: websites.md, federation, configuration, permissions, architecture | services + web |

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

- **A kind of actor,** served by actor-service ([done/actor-service.md](actor-service.md)): a name, an avatar, followers, and the address of the website it stands for. Type `node.actor.actor-service.website.v1`. On ActivityPub, a `Service` actor, which Mastodon labels as automated.
- **A fetcher, as a worker of its own.** It asks for arbitrary addresses on the internet, which is the risk the link-preview plan keeps apart from the core services, and it runs on a schedule and not per request. It reads each website's feed, skips what it has seen, and posts what is new as posts owned by that Website, each linking to the original.

## Three ways a website can take part

1. **It already speaks ActivityPub.** WordPress and Drupal have plugins for it. Once this installation federates, such a site is followed directly and needs nothing from us.
2. **It offers only a feed (RSS or Atom).** A Website actor here follows the feed and posts an excerpt and a link.
3. **It runs a plugin of ours.** What a feed cannot do: prove who owns the site, tell us the moment something is published instead of being asked, and carry replies back to the site.

## To be decided before any design

- **Who may make one.** Anybody, administrators only, or only somebody who proves they control the site. Standing in for a site you do not own is republishing somebody else's work. Proof of control is already planned for people and groups in [verified-domains.md](../verified-domains.md), and would be the same proof.
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
| Goal and name | recorded 2026-10-05; renamed website 2026-10-15 |
| Scope | settled 2026-10-14; narrowed to the feed alone 2026-10-15; its open points settled the same day |
| Design | 2026-10-16: a note per item (an article for a day first), syndication-service and syndication-worker, a website's own notes never edited, private feeds refused |
| Parts 1–10 (as each was first built: articles, and the Pulse's name, were changed after) | done 2026-10-16: foundations, proving a domain, syndication-service, the stack knowing a website, syndication-worker, replies as the website, on the fediverse, WebSub, the web app, docs ([Websites](../../websites.md)) |
| Bluesky | 2026-10-17: a website's own Bluesky account, connected by one of its administrators under Settings › Fediverse; its notes posted there as it |
| Tried | 2026-10-17: rmdstudio.com proved and connected, its ten newest items posted as notes, its Bluesky account (@rmdstudio.com) connected |
| Done | 2026-10-17 |
| Left for later | a follow from a real Mastodon server, not yet tried; replies with pictures, a GIF or a video as the website, which are still their writer's ([Websites](../../websites.md#what-is-not-built)) |
