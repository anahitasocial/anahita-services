# Custom feeds

A custom feed is a feed somebody builds: where its posts come from, what they are about, and what to leave out. They name it, decide who else may see it, and keep it among the tabs of their home page. Posts in every feed are newest first.

Its routes are in the API reference: [feed-service](api/index.html#feed-service).

## Contents

- [The home page](#the-home-page)
- [A feed is a node](#a-feed-is-a-node)
- [The definition](#the-definition)
- [Limits](#limits)
- [Who sees a feed, and what they see in it](#who-sees-a-feed-and-what-they-see-in-it)
- [Reading a feed](#reading-a-feed)
- [Keeping feeds on the home page](#keeping-feeds-on-the-home-page)
- [In search](#in-search)
- [Discover](#discover)
- [Endpoints](#endpoints)
- [Where the code is](#where-the-code-is)
- [What is not built](#what-is-not-built)

## The home page

The first tab of somebody's home page is always the feed of the people, groups and events they follow, newest first (`GET /feeds/leaders/`). It is not a custom feed and cannot be removed or reordered. Custom feeds are the tabs after it.

Every feed orders its posts by date, newest first. (Search is the one place with another order, "Most relevant": see [Search](search.md).) A feed ordered by weights of its owner's choosing is planned as one more kind of custom feed, and would be named by whoever made it like any other. See the plan, `docs/plans/done/feed-generators.md`.

## A feed is a node

- Type `node.feed.feed-service.generator.v1`, owned by the person who made it (`owner_id`), with its name in `name`, what it is for in `body`, and its audience in `access`.
- The node's `meta` is JSON holding what it shows, the `definition`, and its `icon`: one of sixteen pictures, by name (`feeds.Icons`), with `feed` for one given none. Meta that is a definition alone, as the first feeds were kept, is read as that.
- Keeping a feed on the home page is an edge from the person to the feed, `edge.feed.pin.v1`, with its place among the tabs in the edge's `ordering`.
- There is no table of its own and no migration. Deleting a feed deletes its node and, with it, every edge that kept it on somebody's home page. Removing an account removes its feeds as it removes everything else it owns.

## The definition

```json
{
  "sources": {
    "following": true,
    "actors": [123, 456]
  },
  "filters": {
    "actor_types": ["group"],
    "media_types": ["note", "photo"],
    "hashtags_any": ["permaculture", "seeds"],
    "keywords_any": ["compost"],
    "locations_any": [42],
    "language": "en",
    "near": { "latitude": 49.283, "longitude": -123.121, "radius_km": 10 },
    "origin": "elsewhere"
  },
  "exclude": {
    "hashtags": ["ad"],
    "keywords": ["giveaway"],
    "actors": [789]
  },
  "include_replies": false,
  "order": "newest"
}
```

**Sources** are added together.

| Key | Posts from |
| --- | --- |
| `following` | Whoever the feed's owner follows: people, groups and events. The owner's, whoever is reading |
| `actors` | Profiles chosen by hand, of any kind |
| neither | The whole site, posts from the fediverse and Bluesky kept here included, which is allowed only with a hashtag, a keyword, a location or an area among the filters |

The web app offers this as one choice of three: **Everyone** (neither), **Profiles I follow** (`following`), or **Chosen profiles** (`actors`, with "And profiles I follow" for both). A new feed starts from the profiles its owner follows, so that it shows something; given a hashtag, a keyword, a location or an area before the owner chose, it switches to Everyone, since "a #dweb feed" almost always means everybody's.

**Filters** narrow the sources. A post must match every kind of filter given; within a list, matching one entry is enough.

| Key | Keeps |
| --- | --- |
| `actor_types` | Posts on these kinds of profile: `person`, `group`, `event`. `person` includes people on the fediverse and on Bluesky. Empty for any |
| `media_types` | These kinds of post: `note`, `article`, `topic`, `photo`, `video`. Empty for every kind |
| `hashtags_any` | Posts carrying any of these hashtags |
| `keywords_any` | Posts with any of these words in the title or the text, in any case |
| `locations_any` | Posts tagged with any of these locations, by id |
| `language` | Posts their author marked as written in this language |
| `near` | Posts within `radius_km` (1 to 500) of a point, `latitude` and `longitude`: tied to a location there, or carrying a point of their own there. Narrows as a location does. With `"live": true` the point is wherever whoever reads the feed is: none is kept, and each page is read with `near_latitude` and `near_longitude` (see below). Otherwise the point is kept with the feed, as the web app took it from the browser when it was made, to three decimals |
| `origin` | Where a post was written: `here`, on this installation, or `elsewhere`, on the fediverse or Bluesky. Left out for anywhere |

**Exclude** leaves posts out whatever else is true of them: by hashtag, by keyword, and by profile. An excluded profile's posts are left out wherever they were posted, and so is everything posted on it.

**`include_replies`** is off unless asked for: a reply is read in its thread. **`order`** is `newest`, the one value there is. The key is kept so that another order is a value added and not a change of shape.

A key the code does not know is refused, at any depth. A definition is tidied when it is read: lists lose their repeats and blanks, a hashtag loses its `#` and is lower-cased, and a keyword is trimmed, with each run of spaces inside it made one.

### A feed of what is near its reader

A feed whose area is live (`"near": {"live": true, "radius_km": 10}`) is
read around wherever its reader is, which they send with each page:
`GET /feeds/:id/items?near_latitude=49.283&near_longitude=-123.121`. The
point is rounded to three decimals, about a hundred metres, used for that
page, and kept nowhere; whoever reads a shared feed of this kind sees what is
near them. Read without a point it is answered `422` with
`location_needed`. A point saved with a live area is dropped. The web app
asks the browser for the point each time, a reading up to five minutes old
will do, and says why when the browser will not tell it.

## Limits

| What | Most |
| --- | --- |
| Profiles in `sources.actors`, and in `exclude.actors` | 50 each |
| Hashtags, in the filter and left out | 10 each |
| Keywords, in the filter and left out | 10 each, of 2 to 50 characters |
| Locations | 5 |
| An area's radius | 1 to 500 km |
| Feeds one person owns | 10 |
| Feeds on one person's home page | 10 |
| Posts in a page | 20 by default, 40 at most |

A definition that breaks a rule is answered `422` with the reason as a word: `too_broad`, `too_many_actors`, `too_many_hashtags`, `too_many_keywords`, `too_many_locations`, `keyword_length`, `invalid_hashtag`, `unknown_media_type`, `unknown_actor_type`, `invalid_actor`, `invalid_location`, `invalid_language`, `invalid_near`, `unknown_origin`, `unknown_order`, `unknown_field`, `malformed`.

## Who sees a feed, and what they see in it

These are two questions, and they have different answers.

**Who may see the feed itself** is its `access`: `myself` (the default), `followers` (people who follow its owner), `registered` or `public`. It is judged as a post's is, by its own access and its owner's, and a block either way hides it. A feed somebody may not see is answered as if it were not there. An installation that takes no new public content (`MIN_CONTENT_ACCESS`) takes no new public feed.

**What a reader sees in it** is what the definition selects, put through the same gate as every other feed, for the reader. Not for the feed's owner. So:

- A public feed built on a group only its followers can read shows that group's posts to its followers and to nobody else.
- "Whoever I follow" in a feed is whoever its owner follows, and a reader is shown, of those people's posts, the ones they could have seen on those people's profiles.
- A post by somebody who has blocked the reader, or whom the reader has blocked, is not in it.
- What is deleted, disabled or archived is not in it.

A feed never widens what anybody may see. This is the rule the tests in `feed_custom_integration_test.go` are there to keep.

Whoever may see a feed may read it, copy it and keep it on their home page. Only its owner, or a site administrator, may change or delete it.

## Reading a feed

`GET /feeds/<id>/items` returns a page: `data`, and `pagination` with `limit`, `has_more` and `next_cursor`. The next page is asked for with `?cursor=<next_cursor>`.

A page starts after the last post of the page before, not at a count from the top. The cursor is the date and id of the last post of the page before, and the next page is whatever is older. A post written while somebody is reading neither repeats a post nor pushes one out of sight. A feed has no total: counting it would be reading all of it.

A query that cannot finish in the time a query is given is answered `422` with `too_broad`.

## Keeping feeds on the home page

`PUT /feeds/<id>/pin` adds a feed after the ones already there, and `DELETE` takes it off. `PUT /feeds/pins` with `{ "ids": [...] }` replaces the whole list in that order, which is how the tabs are reordered. `GET /feeds/pinned` returns them in order.

A feed is put on its maker's home page when it is made, or copied, after the ones there. A person owns as many feeds as the home page holds, so there is room unless they keep other people's feeds there. With the home page full the feed is made all the same, and waits on the list of their feeds.

Keeping somebody else's feed on your home page is how a feed is followed. There is no separate following of feeds. A feed that is deleted, or that its owner stops sharing, drops out of the list.

Following a hashtag is the same thing: the web app makes a feed of that one hashtag, seen by its owner alone, and puts it on their home page.

## In search

Search finds shared feeds like anything else: `GET /search/` with `scope=all` has them among the profiles and posts that match, and `scope=feeds` has them alone. A feed matches by its name and by what it is for. One kept to its owner is never found, by its owner either. Each is gated for the viewer as a post is, by its own audience and its owner's.

A feed among search results is a node like the others, with a `feed` block: its `icon` and its `definition`.

## Discover

The web app's search finds shared feeds by their words. This endpoint lists them without any: it is the **Feeds** tab of the web app's Recommendations page.


`GET /feeds/discover` lists the feeds people have shared, newest first: public ones, ones for anyone signed in, and ones for the followers of somebody the viewer follows. Each is gated for the viewer by its own audience and its owner's, as a post is. A feed kept to its owner is in nobody's list, its owner's included; the viewer's own shared feeds are in theirs, as everybody else sees them.

The list is of feeds, not of posts: opening or adding one shows its reader only what they could see anyway.

## Endpoints

All on feed-service. Writing needs the `media:write` scope.

| Endpoint | Who | What |
| --- | --- | --- |
| `POST /feeds/` `{ name, body, access, icon, definition }` | signed in | Makes a feed, and puts it on the home page. `access` is `myself` unless given |
| `GET /feeds/mine` | signed in | The feeds the viewer made |
| `GET /feeds/pinned` | signed in | The feeds on the viewer's home page, in order |
| `GET /feeds/discover?q=&start=&limit=` | anybody | The feeds people have shared that the viewer may see, newest first. `q` is part of a name. Paged by `pagination.next_start` |
| `PUT /feeds/pins` `{ ids }` | signed in | Replaces the feeds on the home page |
| `POST /feeds/preview` `{ definition }` | signed in | The first page of a feed not saved yet, as the viewer's own |
| `GET /feeds/<id>` | whoever may see it | The feed, with `references`: the profiles and locations it names that the viewer may see |
| `PATCH /feeds/<id>` | its owner | Changes it |
| `DELETE /feeds/<id>` | its owner | Deletes it |
| `GET /feeds/<id>/items?cursor=&limit=` | whoever may see it | A page of its posts |
| `POST /feeds/<id>/copy` `{ name }` | whoever may see it | A feed of the viewer's own showing the same thing, seen by them alone. Called `name`, or "Copy of" the other's name |
| `PUT`, `DELETE /feeds/<id>/pin` | whoever may see it | Keeps it on the home page, or takes it off |

A feed is sent with its `icon`, `is_pinned` and `authorized` (`edit`, `delete`, `pin`, `copy`).

## Where the code is

| What | Where |
| --- | --- |
| The definition, its rules and limits, and the cursor | `anahita-libs/feeds` (`Parse`, `Check`). feed-service checks a definition with it when a feed is saved, and graph-grpc-service reads it again with the same code before building a query, so nothing reaches the database that was not read into its fields |
| The query | `graph-grpc-service/data/repositories/feed_custom.go`. Every value is a parameter. It ends with `SetFeedAccessQuery`, `SetLifecycleQuery` and `SetArchiveQuery`, for the reader |
| That a feed never widens what anybody may see | `graph-grpc-service/data/repositories/feed_custom_integration_test.go` |
| A feed's own visibility | `graph-grpc-service/data/repositories/nodes_visible.go` (`FilterVisible`) |
| The endpoints | `feed-service/cmd/api/handlers/custom_feed.go`. `custom_feed_test.go` runs them over a graph held in memory: who may see, change, copy and delete a feed, the limits, and the home page |

## What is not built

- **An order other than newest first.** See [The home page](#the-home-page).
- **Reposts in a custom feed.** A custom feed holds posts, and replies when asked for. The feed of people you follow holds reposts as it always did.
- **"Posts by the followers of a group"** as a source, which the plan had as `members_of`. A feed of the groups somebody follows is `following` with `actor_types: ["group"]`.
- **"With images"** as a filter. `media_types: ["photo"]` is the photo posts.
- **A cache of a public feed's first page.** Nothing is slow enough to need one yet.
- **Feeds made for a new account.** The feed of people you follow is the first tab, and the rest are for each person to make.
- **Emptying a feed's description.** A description can be changed and cannot be set back to nothing: the graph's node edit leaves an empty text as it was.
- **Feeds across the fediverse.** A custom feed is a feature of the installation and is not sent to other servers. Posts from people followed on other servers appear in one like any others, and a feed can name such people among its profiles.
