# Why Anahita

Anahita is a social software platform organizations use to foster community
engagement in settings such as schools, newsrooms, libraries, university
research groups, or non-governmental organizations.

Anahita instances communicate with each other and the global network of
social networks, blogs, and online forums that utilize either ActivityPub or
the AT Protocol. This enables seamless interaction while keeping your
original data and files on your server, governed by your privacy and
moderation policies. Key concepts in Anahita's design philosophy include:

## Contents

- [The principles](#the-principles)
- [Two networks, not a third](#two-networks-not-a-third)
- [Everything is a node](#everything-is-a-node)
- [Further reading](#further-reading)

## The principles

Five choices, each a rule for whoever changes the code.

1. **Federation first, and only with consent.** Anahita speaks ActivityPub
   and the AT Protocol natively, so that a community on it is not a walled
   garden: its people can follow and be followed by people on other servers
   and on Bluesky. Each profile chooses whether to federate. Sharing is
   consented to, never assumed. Nothing is built that does not federate
   sensibly. See [Federation](federation.md).
2. **The feed is chronological.** The home page is what the people,
   groups and events somebody follows posted, newest first. There is no
   ranked feed chosen for them. A ranked feed is one a person makes, and is
   always called a custom feed. See [Custom feeds](feeds.md).
3. **Nobody is tracked.** There are no profile analytics, no share counts,
   no lists of who viewed what. A like is counted because somebody chose to
   like; a view is not, because nobody chose to be seen viewing. Who may
   see what is each person's choice: see [Privacy](privacy.md).
4. **Old and useful counts as much as new.** A community builds knowledge
   as well as news: a school's notes, a library's guides, a research group's
   findings stay useful for years. Nothing is pushed down only for being
   old, and how fast interest fades is the operator's choice. See
   [How things are found](how-things-are-found.md).
5. **The breadth is the product.** Groups, events, articles, topics,
   photos, videos and custom feeds, in one place, for the organisations that
   need more than a stream of short posts.

Each has a cost. A chronological feed asks people to choose whom to follow
with care. Without tracking, there is less to tell an operator how the site
is used. Without ranking by engagement, a lively post does not reach
strangers on its own. These are costs Anahita chooses to pay.

## Two networks, not a third

There are two open ways for social software to talk to each other now:
ActivityPub, which Mastodon and most of the fediverse speak, and the AT
Protocol, which Bluesky speaks. They are built on different ideas.

- **ActivityPub** is servers sending each other messages. A person's
  identity is on the server they joined (`@ana@example.org`), and that
  server holds their keys and speaks for them.
- **The AT Protocol** gives a person an identity of their own, a DID, and a
  repository of what they posted that any app can read and that can move
  between hosts. Apps read the whole network through relays.

Anahita takes part in both rather than inventing a third. On the fediverse
it is a server; on Bluesky it is an app, with the account a person already
has. [A server on one network, an app on the other](federation.md#a-server-on-one-network-an-app-on-the-other)
says what follows from that.

## Everything is a node

Inside, Anahita is one idea: everything is a node, and every relationship
between two things is an edge. A person, a group, a post, a hashtag, a
place and a feed are nodes; following, liking, tagging, citing and saving
are edges. A post from another server is a node too, so feeds, follows and
blocks need nothing new for it.

That one shape is what lets a group's events, a person's photos and a
hashtag's posts be read, permitted, searched and suggested in the same way.
See [The graph](the-graph.md).

## Further reading

The protocols Anahita speaks, for those who want to know them from the
source. Each is described for Anahita in [Federation](federation.md#further-reading).

| | Read |
| --- | --- |
| ActivityPub | [The W3C recommendation](https://www.w3.org/TR/activitypub/), and [activitypub.rocks](https://activitypub.rocks/) |
| The AT Protocol | [atproto.com's docs](https://atproto.com/docs), beginning with [the overview](https://atproto.com/guides/overview) |
| The AT Protocol's design | [Bluesky and the AT Protocol: Usable Decentralized Social Media](https://arxiv.org/abs/2402.03239), by the people who designed it |
