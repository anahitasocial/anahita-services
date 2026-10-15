# The Website actor: a website as something to follow

*Roadmap item 30, raised by the user on 2026-10-05. **Scope settled with the user on 2026-10-14: see [The scope](#the-scope-settled-2026-10-14).** The rest of the page is what was said before, kept for the reasons behind it.*

## The scope, settled 2026-10-14

**A Website is a verified page for anything that publishes a feed** on a domain its owner controls: its feed posts by itself, and its administrators post and reply as it, as on a business page. In the user's words: any website or app that can make an RSS or JSON feed, whether a content management system, a PHP site written by hand, or a static site such as Jekyll, should be able to join the fediverse through Anahita once it proves its domain. A WordPress or other site, an app's changelog, a podcast, a status page, a blog built by Jekyll or Hugo: nothing about the site's software is asked, only its feed and its domain. A static site proves its domain with one file committed beside its pages. A site that already speaks ActivityPub is followed as it is.

- **What it is.** A kind of profile (`node.actor.actor-service.website.v1`): name, avatar (the site's icon), cover, description, its address, followers. Like a public group in that it has administrators and followers, and like a business page in that only it posts: what its feed publishes (RSS, Atom and JSON Feed, read by `gofeed`), and what its administrators post as it. Followers read, like, share and reply; they do not post on it.
- **Who sets one up.** Only somebody who proves the domain is theirs: a DNS TXT record `anahita-site-verification=<token>`, or the token in `https://<domain>/.well-known/anahita-verification.txt`. Checked again daily; a Website whose proof is gone for a week pauses. On top of that, `WEBSITES_FROM`, as `GROUPS_FROM`: `administrators` unless an installation opens it to `registered`. One Website per feed address. Whoever proved it is its first administrator, and may add other members as administrators, as a group's are added. Ownership was chosen over letting anybody stand in for a site (as unofficial repeaters on Mastodon and Bluesky do): nobody can set up a site that is not theirs, and a feed on a domain one does not control (a YouTube channel's, a GitHub project's) is not taken.
- **Posts.** Each new item becomes a note owned by the Website: its title, a short excerpt and a link card to the original (the link-preview service), its categories as hashtags, its own date. Items are remembered by their ID or address and never posted twice. On setting up, what the feed holds is marked seen, not posted; an hourly cap holds back a burst and posts it a few at a time.
- **The page is the author (user, 2026-10-14).** Its administrators post notes, photos and videos from the Website's own page, through the same services and composer as any post, and such a post is the Website's: written by it, shown as its, sent out as its. In the threads under the Website's own posts, an administrator's reply is the Website's too, so the page answers its readers; anywhere else administrators reply as themselves. There is no "Post as" choice: posting as another actor was built in the PHP app and taken out again, and where one posts decides who the author is. Which administrator made a post is not kept: a post on a Website is reported like any other (abuse reports, item 4b), and the installation's moderators act on the Website, whose administrators are listed as a group's are.
- **Reading the feed.** By a `website-worker`, apart from the core services since it reads addresses on the internet, with the link-preview service's safe fetcher (no private addresses, limits of size and time). Asked every 15 to 60 minutes with `ETag` / `If-Modified-Since`, backing off from a site that fails. A feed naming a WebSub hub (`rel="hub"`) is subscribed to instead, and its posts arrive within seconds.
- **Between networks.** From the start, as an ActivityPub `Organization` actor served by `activitypub-service`, its posts public. First decided as a `Service`, which Mastodon marks as automated; since people post and reply as a Website too, it is a page run by people, and `Organization` says so. Not on Bluesky: a Website has no account there.
- **Replies.** Open, from here and from other servers. Its administrators answer as the Website, and hide a reply under its posts as the owner of a thread does now, which keeps it from everybody else here. Hiding it on other servers as well is item 44, for later.
- **A site that already speaks ActivityPub** (WordPress's or Drupal's plugin): adding it says so and offers to follow it directly, as it is already an account of the fediverse.
- **Handle.** Suggested, not yet confirmed by the user: an alias the owner picks, as a group has, defaulting to the domain's name (`@example@<installation>` for example.com), with the verified domain shown on the profile.
- **Not in it:** plugins or dashboards for content management systems, signing in to them with Anahita accounts, Bluesky, feeds on domains one does not control. Discussed on 2026-10-14, including a comparison with WordPress's ActivityPub plugin, and set aside as too large: a business plan more than a feature.
- **Estimate:** about 4–5 sessions, after replies of any kind (item 53), which gives replies their pictures and video first.

| Part | Time |
| --- | --- |
| Proving a domain: token, DNS and file checks, the daily check again, pausing | 2–3 h |
| The Website as a kind of profile in actor-service: setting up, administrators, `WEBSITES_FROM`, one per feed | 2–3 h |
| `website-worker`: finding the feed, reading it with the safe fetcher, polling with `ETag`, WebSub, items to posts with cards and hashtags, the first-fetch rule and hourly cap | 5–6 h |
| The page as author: posts from its page and replies under its posts made the Website's, who may | 2–3 h |
| ActivityPub: a Website as an `Organization` actor, its posts going out, replies to it coming in | 2–3 h |
| Web: setting one up (address, the proof, checking it), the Website's page, a Websites list | 3–4 h |
| Tests, docs | 2 h |
| **Total** | **about 18–24 h, 4–5 sessions** |

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
| Scope | settled 2026-10-14, posting as the Website added the same day (the handle style to confirm) |
| Design and work | not started |
