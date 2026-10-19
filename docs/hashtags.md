# Hashtags and locations

How a post gets its hashtags and its locations, how many it can have, and the
hashtags offered to somebody while they write.

## Contents

- [How a post gets its hashtags](#how-a-post-gets-its-hashtags)
- [At most ten hashtags](#at-most-ten-hashtags)
- [At most five locations](#at-most-five-locations)
- [Trending](#trending)
- [Suggestions while writing](#suggestions-while-writing)
- [A hashtag's and a location's posts](#a-hashtags-and-a-locations-posts)
- [What the web app does](#what-the-web-app-does)
- [Where the code is](#where-the-code-is)

## How a post gets its hashtags

A hashtag is a `#` and the letters after it, at the start of the text or
after a space: letters of any language, digits, `_` and common emoji
(`entities.HashtagRegexPattern`). `a#b` is not one.

hashtag-service reads the text of a post each time it is added or changed,
and files the post under each hashtag in it. A hashtag that does not exist
yet is made then. Capitals don't matter: `#Anahita` and `#anahita` are one
hashtag, found at `/hashtags/anahita`, and shown as the first post to use it
wrote it.

A post from another server gets its hashtags the same way, from its text; see
[Federation](federation.md).

## At most ten hashtags

A post is filed under **its first ten different hashtags**, counted in the
order they are written, whatever their capitals. The rest stay in the text
as plain words: not links, not filed, not sent as tags.

| Where | What happens |
| --- | --- |
| hashtag-service | Files the post under the first ten |
| The web app | Links the first ten in the text, and says so while writing past them |
| ActivityPub | Sends the first ten as links and as the note's tags |
| Bluesky | Marks the first ten as tags in the post's facets |

Nothing is refused. A post with more is saved whole, so an old post with
fifteen hashtags can still be edited. A post from another server with more
arrives whole too, and is found under its first ten only.

The number is `constants.MaxHashtagsOfAPost` in anahita-libs, and
`Medium.MAX_HASHTAGS` in the web app. Ten is also how many hashtags a custom
feed can filter on (`feeds.MaxHashtags`). For comparison: Mastodon, Pixelfed
and the other fediverse servers set no limit in code; Bluesky allows eight
extra tags outside a post's text, and no limit in it.

## At most five locations

A post can be tied to **at most five locations**. Locations are added one at a
time (`POST /locations/:id/inbounds/`), so a sixth is refused with
`422` and `{"error": "too_many_locations"}`. Tying a post to a location it
already has is not a sixth.

The number is `constants.MaxLocationsOfAPost` in anahita-libs, and
`Medium.MAX_LOCATIONS` in the web app. Five is also how many locations a custom
feed can filter on (`feeds.MaxLocations`).

## Trending

`GET /hashtags/_browse/trending` and `GET /locations/_browse/trending` list
what the most people posted with lately.

- **Each person once.** One account posting a hashtag twenty times counts
  as one, so nobody can make something trend alone. Ties go to the one on
  more posts.
- **By when a post was written,** not when it arrived. A profile's older
  posts fetched when it is followed don't trend that day.
- **Public posts only,** on profiles anybody may see, so what trends says
  nothing about what is kept from view.
- **Over a window** the installation chooses (`TRENDING_WINDOW`, see
  [Configuration](configuration.md#trending)): daily where it federates,
  monthly where only its own members post. A window with fewer than ten is
  widened, to a week, a month, then a year, and the answer's `window` says
  which it is of. The web app shows it under the tabs.

## Suggestions while writing

hashtag-service offers hashtags already in use to somebody signed in who is
writing. There are two ways to ask:

| Request | For | Offers |
| --- | --- | --- |
| `GET /hashtags/_suggest?q=ana` | Typing a hashtag | Those that begin with `q` (what follows the `#`) |
| `POST /hashtags/_suggest` with `{"title": "…", "content": "…"}` | Long content | Those that are words of the title and content |

Both take `limit`, 1 to 20, and offer 8 when it is not given. Both answer
with the best first:

```json
{
  "data": [
    { "id": 138587, "alias": "anahita", "name": "anahita", "uses": 1031, "followed": false }
  ]
}
```

- `alias` is the hashtag in lower case; `name` is how it is shown.
- `uses` is how many posts it is on that anybody may see, plus the person's
  own.
- `followed` is true when it is in one of the person's custom feeds, which
  is what following a hashtag is.

**The order.** Hashtags the person follows come first. For a typed start,
the hashtag that is exactly what was typed comes next, then the most used.
For a text, the words the text uses most come next, then the most used.

**What is counted.** Only posts anybody may see (public, published, switched
on, on a profile anybody may see) and the person's own. So a hashtag used
only where the person can't look is never offered to them, and what is
offered says nothing about what is kept from view.

**Words of a text.** The title and body are split into words of three
letters or more, and each two words next to each other are also tried run
together, so "open science" finds `#OpenScience`. Addresses, code,
mentions and hashtags already written are not words. A word is offered only
when it is a hashtag on more than one post, and hashtags the text already
has are left out. The first 100,000 characters are read.

Matching ignores capitals and accents: `cafe` finds `#café`. `%` and `_`
are matched as themselves.

## A hashtag's and a location's posts

`GET /hashtags/:id/inbounds` and `GET /locations/:id/inbounds` list what is
tied to a hashtag or a location, in one of two orders, by `sort`:

| `sort` | Order |
| --- | --- |
| `recent` | Newest first. The web app's first tab, and its default |
| `relevant` | What people have found useful lately, as search's [Most relevant](search.md#most-relevant) ranks it, without the words: likes, saves, replies and quotes, each fading by the operator's half-life, citations, and a bonus for what the viewer follows, with the viewer's own leanings. Newest first among equals |

`popular`, an all-time sum of a post's counts, is still taken, and the web
app no longer asks for it: an old post that did well stayed on top for good.

## What the web app does

- **While typing a hashtag**, in the short box, a reply and long content,
  hashtags in use that begin with what was typed are offered under the line
  being written, or above it when there is no room (on a phone, the
  keyboard takes the bottom of the screen). They float over the page, so
  nothing under them moves.
  - On a phone: a row of chips; one tap puts one in.
  - On a desktop: a short list with how many posts each is on; the arrow
    keys move through it, Enter or Tab puts one in, Escape puts it away.
- **Under the long content editor**, a row of suggested hashtags made from
  the title and content, asked for a moment after writing stops once there
  are 80 characters or more. One tap adds a hashtag at the end, on the line
  of hashtags the text ends with, or on a line of its own.
- **At ten hashtags**, both say "Only the first 10 hashtags count" instead
  of offering more.
- **At five locations**, a post's Locations card says a post can have up to 5
  locations in place of its add button.

## Where the code is

- `anahita-libs/constants/tags.go`: the two caps.
- `hashtag-service/cmd/api/handlers/hashtag_events.go`: filing a post under
  its hashtags.
- `hashtag-service/cmd/api/handlers/suggest.go`: the two suggestion
  requests, and the words of a text (`wordsOf`).
- `graph-grpc-service/data/repositories/hashtag_suggestions.go`: which
  hashtags are offered, and in what order (`HashtagService.Suggest` in
  `proto/graphpb/hashtag.proto`).
- `location-service/cmd/api/handlers/inbound.go`: the cap on locations.
- `anahita-libs/federation/content.go` and `atproto-service/bsky/outgoing.go`:
  the cap in what is sent to other servers.
