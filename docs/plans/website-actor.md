# The Website actor: a website as something to follow

*Roadmap item 30, raised by the user on 2026-10-05. **Scope settled with the user on 2026-10-14: see [The scope](#the-scope-settled-2026-10-14).** The rest of the page is what was said before, kept for the reasons behind it.*

## The scope, settled 2026-10-14

**A Website is a verified bot account for anything that publishes a feed** on a domain its owner controls: a WordPress or other site, an app's changelog, a podcast, a status page.

- **What it is.** A kind of profile (`node.actor.actor-service.website.v1`): name, avatar (the site's icon), description, its address, followers. An automated account that posts what the feed publishes: RSS, Atom and JSON Feed, read by `gofeed`.
- **Who sets one up.** Only somebody who proves the domain is theirs: a DNS TXT record `anahita-site-verification=<token>`, or the token in `https://<domain>/.well-known/anahita-verification.txt`. Checked again daily; a Website whose proof is gone for a week pauses. On top of that, `WEBSITES_FROM`, as `GROUPS_FROM`: `administrators` unless an installation opens it to `registered`. One Website per feed address. Whoever proved it administers it, as a group's administrators do, and may add others. Ownership was chosen over letting anybody stand in for a site (as unofficial repeaters on Mastodon and Bluesky do): nobody can set up a site that is not theirs, and a feed on a domain one does not control (a YouTube channel's, a GitHub project's) is not taken.
- **Posts.** Each new item becomes a note owned by the Website: its title, a short excerpt and a link card to the original (the link-preview service), its categories as hashtags, its own date. Items are remembered by their ID or address and never posted twice. On setting up, what the feed holds is marked seen, not posted; an hourly cap holds back a burst and posts it a few at a time.
- **Reading the feed.** By a `website-worker`, apart from the core services since it reads addresses on the internet, with the link-preview service's safe fetcher (no private addresses, limits of size and time). Asked every 15 to 60 minutes with `ETag` / `If-Modified-Since`, backing off from a site that fails. A feed naming a WebSub hub (`rel="hub"`) is subscribed to instead, and its posts arrive within seconds.
- **Between networks.** From the start, as an ActivityPub `Service` actor (Mastodon marks it as automated) served by `activitypub-service`, its posts public. Not on Bluesky: a Website has no account there.
- **Replies.** Open, from here and from other servers. The profile says they are a conversation among readers, not with the site.
- **A site that already speaks ActivityPub** (WordPress's or Drupal's plugin): adding it says so and offers to follow it directly, as it is already an account of the fediverse.
- **Handle.** Suggested, not yet confirmed by the user: an alias the owner picks, as a group has, defaulting to the domain's name (`@example@<installation>` for example.com), with the verified domain shown on the profile.
- **Not in it:** plugins or dashboards for content management systems, signing in to them with Anahita accounts, Bluesky, feeds on domains one does not control. Discussed on 2026-10-14, including a comparison with WordPress's ActivityPub plugin, and set aside as too large: a business plan more than a feature.
- **Estimate:** about 3 sessions, after replies of any kind (item 53).

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
| Goal and name | recorded 2026-10-05 |
| Scope | settled 2026-10-14 (the handle style to confirm) |
| Design and work | not started |
