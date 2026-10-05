# The Website actor: a website as something to follow

*Roadmap item 30, raised by the user on 2026-10-05. **The scope is not settled and nothing is to be built until it is.** This page records the goal and what has been said so far.*

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
| Scope | open |
| Design and work | not started |
