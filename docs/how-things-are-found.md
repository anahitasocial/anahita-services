# How things are found

There are six ways somebody comes across a post or a profile on Anahita:
their home page, a custom feed, search, suggestions of whom to follow, and
what is trending. This page is how they fit together, and what each one
orders by. The pages it links to have the numbers.

## Contents

- [Six ways in](#six-ways-in)
- [What they share](#what-they-share)
- [What is never used](#what-is-never-used)
- [Interest fades; usefulness does not](#interest-fades-usefulness-does-not)
- [Each person's own weights](#each-persons-own-weights)
- [The operator's numbers](#the-operators-numbers)

## Six ways in

| Way | What it shows | In what order | Page |
| --- | --- | --- | --- |
| The home page | What the people, groups and events somebody follows posted | Newest first, always | [Custom feeds](feeds.md#the-home-page) |
| A custom feed | What a person chose: sources, hashtags, words, locations, languages, a distance | Newest first | [Custom feeds](feeds.md) |
| Search | People, groups, events, posts and shared feeds that match the words | Most relevant, or most recent | [Search](search.md) |
| A hashtag's or a location's posts | The posts that carry it | Most relevant, without the words | [Hashtags and locations](hashtags.md#a-hashtags-and-a-locations-posts) |
| Suggestions | People, groups and events to follow | By the reasons for each, scored | [Recommendations](recommendations.md) |
| Trending | The hashtags and locations the most profiles used lately | By how many profiles, each once | [Trending](hashtags.md#trending) |

The home page and custom feeds are by date and nothing else. Only search,
a tag's posts and suggestions rank, and every one of them says why.

## What they share

Every way in walks the same [graph](the-graph.md), over the same edges,
and reads only what the viewer may see.

| Sign | The edges and columns | Used by |
| --- | --- | --- |
| Who somebody follows | `edge.actor.follow.v1` | Every one: the home page is made of it; search puts what you follow higher; suggestions look at whom your follows follow |
| What people found useful | Likes (`edge.base.voteup.v1`), saves (`edge.base.bookmark.v1`, private: never shown to anybody), replies, reposts and quotes | Search's standing; `you_engaged` in suggestions |
| What people cite | `edge.medium.citation.v1`, counted once for each person who cites | Search's standing; `cited` and `cited_by_leaders` in suggestions |
| Who is in a photo or a video together | `edge.tag.person.v1`, approved tags only | `pictured_with_followed` in search; `pictured_with_you` and `pictured_with_leaders` in suggestions |
| What a post is about | `edge.tag.hashtag.v1`, `edge.tag.location.v1` | Custom feeds' filters; search; `shared_interest` in suggestions; trending |
| How lately a profile was active | When it last posted, or was visited | Suggestions put the quiet last, and leave out those quiet longer than the reader chose |

A sign is never more than one of several. A logarithm keeps one post liked
a thousand times from burying everything else in search, and every reason
for a suggestion has a most it can add.

## What is never used

What somebody looked at, for how long, what they scrolled past, what they
clicked on, where they were when they did: none of it is recorded, so none
of it can rank anything. Each sign above is something a person chose to do
and could see themselves doing: follow, like, save, reply, cite, tag. There
are no hidden signals, and no profile of interests is built from behaviour
to rank by ([Why Anahita](why-anahita.md)).

## Interest fades; usefulness does not

A guide a library wrote years ago is as useful as one from this week. So
nothing is pushed down for being old. What may fade is **interest**: each
like, save, reply or follow is counted by when it happened, halved every
`half_life_days`, a year by default. A guide that people saved this month
stands as high as a new post saved as often.

A citation does not fade. Citing takes effort and is lasting, and what is
cited stays useful. See
[Old and useful counts as much as new](search.md#old-and-useful-counts-as-much-as-new).

## Each person's own weights

Each person who is signed in can change how much each sign counts, for them
alone, and nobody else's results change:

- **In search,** six weights: likes, saves, replies, followers, citations,
  and what they follow ([Their own weights](search.md#their-own-weights)),
  and two choices, *Profiles I follow first* and *Newer results first*.
- **In suggestions,** a weight for each reason, from 0, not used, to 10;
  whether what they liked and wrote may be used at all (`use_activity`);
  and how lately somebody must have been active to be suggested
  ([Each person's own weights](recommendations.md#each-persons-own-weights)).
- **Custom feeds** are the rest: a person who wants a different feed makes
  one, and calls it what they like.

## The operator's numbers

Every default is the installation's to change:

| What | Where |
| --- | --- |
| Search: the points for a match, the weights of each sign, `half_life_days` | `graph-grpc-defaults/search_ranking.json`, read again when it changes ([Tuning it](search.md#tuning-it)) |
| Suggestions: the score of each reason and its most | `recommendation-service/constants`, in the code: a change is a new build |
| Trending: the window | `TRENDING_WINDOW` ([Configuration](configuration.md#trending)) |

A newsroom may want interest to fade in a month; a library, a school or a
research group in a year or more, or never.
