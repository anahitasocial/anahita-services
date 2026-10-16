# Pulse: a website's feed as something to follow

*Roadmap item 30, raised by the user on 2026-10-05 as "the Website actor". **Renamed Pulse and narrowed to the feed alone by the user on 2026-10-16: see [The scope](#the-scope-settled-2026-10-16).** What follows it is what was said before, kept for the reasons behind it: where it says Website, it means a Pulse.*

## The scope, settled 2026-10-16

**A Pulse is the feed of a website, verified, as something to follow.** Any website or app that publishes an RSS, Atom or JSON feed on a domain its owner controls can have one: a WordPress or other site, a PHP site written by hand, a static site made by Jekyll or Hugo, an app's changelog, a podcast, a status page. Nothing about the site's software is asked, only its feed and its domain. Its posts reach people here and across the fediverse, and send readers to the site: the site stays where it is written. A site that already speaks ActivityPub is followed as it is.

- **The name (user, 2026-10-16).** *Pulse*: the steady beat of a site's posts. It is what a site would say of it: "Follow our pulse on the Fediverse". *Website* was the name before; it was given up because "create a website" reads as if Anahita made one, and because a Pulse is not the site but its feed. The verified domain is shown on the profile, which is what says what is behind it ("example.com ✓"). Kept as *Pulse* in every language. Weighed and kept the same day: a pulse is checked more than followed, and in British English *pulses* are also lentils and beans; the user chose Pulse over Website and Channel all the same. It belongs with the words of life of the project: Anahita is the goddess of love, fertility and wisdom, and its releases are called *birth* and *embryo*; and it is a Pink Floyd album, which the user likes. Considered on the way: Channel, Publisher, Feed (taken by the home feed and custom feeds), Relay, Repeater, Syndicate, Outpost.
- **Only the feed posts (user, 2026-10-16).** No composer: nobody posts notes, photos, videos or articles on a Pulse, its administrators included. Each new item of the feed becomes a post owned by the Pulse. Everybody else interacts with those posts as with any: replies, likes, reposts, quotes. This replaces "the page is the author" of 2026-10-14: the site's own software is where it writes.
- **What it is.** A kind of profile (`node.actor.actor-service.pulse.v1`): name, avatar (the site's icon), cover, description, the site's address and its verified domain, followers. Administrators as a group has, who look after it; followers, who read.
- **Its administrators** set it up and keep it right (name, picture, the feed's address), and look after the threads under its posts as any post's owner does: they hide replies, and block somebody from the Pulse as a group's administrators do (item 55). They reply as themselves, from their own profiles, which says that a person of the site is answering.
- **Who sets one up.** Only somebody who proves the domain is theirs (below). On top of that, `PULSES_FROM`, as `GROUPS_FROM`: `administrators` unless an installation opens it to `registered`. One Pulse per feed address. Whoever proved the domain is its first administrator, and adds others as a group's are added. Offered as **"Connect your website"**, never "create": the first step asks for the site's address. A feed on a domain one does not control (a YouTube channel's, a GitHub project's) is not taken.
- **Proving the domain.** *Proposed 2026-10-16, to confirm with the user:* built once, so that people and groups use the same proof later (verified domains, item 26): one table `verified_domains` keyed by profile and one checker, a DNS TXT record at `_anahita.<domain>` or the token at `https://<domain>/.well-known/anahita-verification.txt` (one committed file for a static site), checked again daily; a Pulse whose proof is gone for a week pauses. Earlier wording was a TXT record `anahita-site-verification=<token>` at the domain itself.
- **Posts.** Each new item becomes a note owned by the Pulse: its title, a short excerpt and a link card to the original (the link-preview service), its categories as hashtags, its own date. Items are remembered by their ID or address and never posted twice. On setting up, what the feed holds is marked seen, not posted; an hourly cap holds back a burst and posts it a few at a time. *To confirm:* an item whose title or excerpt changes while it is still in the feed changes its post; one that leaves the feed leaves its post as it is.
- **Reading the feed.** By a `pulse-worker`, apart from the core services since it reads addresses on the internet, with the link-preview service's safe fetcher (no private addresses, limits of size and time). Asked every 15 to 60 minutes with `ETag` / `If-Modified-Since`, backing off from a site that fails. A feed naming a WebSub hub (`rel="hub"`) is subscribed to instead, and its posts arrive within seconds.
- **Between networks.** As an ActivityPub `Service` actor served by `activitypub-service`, which Mastodon marks as automated, as it is: nobody writes as it. Its posts public. Not on Bluesky.
- **Replies.** Open, from here and from other servers.
- **"Follow our pulse on the Fediverse."** *Suggested 2026-10-16, to confirm:* a button or a few lines of HTML the site can paste in, leading to its Pulse here.
- **Handle.** Suggested, not yet confirmed by the user: an alias the owner picks, as a group has, defaulting to the domain's name (`@example@<installation>` for example.com).
- **Not in it:** posting on a Pulse, plugins or dashboards for content management systems, signing in to them with Anahita accounts, Bluesky, feeds on domains one does not control.
- **Estimate:** about 4 sessions.

| Part | Time |
| --- | --- |
| Proving a domain, built to be shared: token, DNS and file checks, the daily check again, pausing | 2–3 h |
| The Pulse as a kind of profile in actor-service: connecting one, administrators, `PULSES_FROM`, one per feed | 2–3 h |
| `pulse-worker`: finding the feed, reading it with the safe fetcher, polling with `ETag`, WebSub, items to posts with cards and hashtags, the first-fetch rule and hourly cap | 5–6 h |
| ActivityPub: a Pulse as a `Service` actor, its posts going out, replies to it coming in | 2–3 h |
| Web: connecting a website (address, the proof, checking it), the Pulse's page, a Pulses list | 3–4 h |
| Tests, docs | 2 h |
| **Total** | **about 16–21 h, 4 sessions** |

## Before 2026-10-16: the Website actor

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
| Goal and name | recorded 2026-10-05; renamed Pulse 2026-10-16 |
| Scope | settled 2026-10-14; narrowed to the feed alone 2026-10-16 (the domain proof, edited items, the follow button and the handle style to confirm) |
| Design and work | not started |
