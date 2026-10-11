# Custom feeds

A custom feed is a feed somebody builds: where its posts come from, what they are about, and what to leave out. They name it, decide who else may see it, and keep it among the tabs of their home page. Posts in every feed are newest first.

## Contents

- [The home page](#the-home-page)
- [A feed is a node](#a-feed-is-a-node)
- [The definition](#the-definition)
- [Limits](#limits)
- [Who sees a feed, and what they see in it](#who-sees-a-feed-and-what-they-see-in-it)
- [Reading a feed](#reading-a-feed)
- [Keeping feeds on the home page](#keeping-feeds-on-the-home-page)
- [Endpoints](#endpoints)
- [Where the code is](#where-the-code-is)
- [What is not built](#what-is-not-built)

## The home page

The first tab of somebody's home page is always the feed of the people, groups and events they follow, newest first (`GET /feeds/leaders/`). It is not a custom feed and cannot be removed or reordered. Custom feeds are the tabs after it.

Nothing on the site orders posts by anything but their date. A feed ordered by weights of its owner's choosing is planned as one more kind of custom feed, and would be named by whoever made it like any other. See the plan, `docs/plans/feed-generators.md`.

## A feed is a node

- Type `node.feed.feed-service.generator.v1`, owned by the person who made it (`owner_id`), with its name in `name`, what it is for in `body`, and its audience in `access`.
- What it shows, the definition, is JSON in the node's `meta`.
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
    "language": "en"
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
| neither | The whole site, which is allowed only with a hashtag, a keyword or a place among the filters |

**Filters** narrow that. Each kind given has to hold; inside a list, any one entry will do.

| Key | Keeps |
| --- | --- |
| `actor_types` | Posts on these kinds of profile: `person`, `group`, `event`. Empty for any |
| `media_types` | These kinds of post: `note`, `article`, `topic`, `photo`. Empty for every kind |
| `hashtags_any` | Posts carrying any of these hashtags |
| `keywords_any` | Posts with any of these words in the title or the text, in any case |
| `locations_any` | Posts tagged with any of these places, by id |
| `language` | Posts their author marked as written in this language |

**Exclude** leaves posts out whatever else is true of them: by hashtag, by keyword, and by profile. An excluded profile's posts are left out wherever they were posted, and so is everything posted on it.

**`include_replies`** is off unless asked for: a reply is read in its thread. **`order`** is `newest`, the one value there is. The key is kept so that another order is a value added and not a change of shape.

A key the code does not know is refused, at any depth. A definition is tidied when it is read: lists lose their repeats and blanks, a hashtag loses its `#` and is lower-cased, and a keyword's spaces are closed up.

## Limits

| What | Most |
| --- | --- |
| Profiles in `sources.actors`, and in `exclude.actors` | 50 each |
| Hashtags, in the filter and left out | 10 each |
| Keywords, in the filter and left out | 10 each, of 2 to 50 characters |
| Places | 5 |
| Feeds one person owns | 20 |
| Feeds on one person's home page | 10 |
| Posts in a page | 20 by default, 40 at most |

A definition that breaks a rule is answered `422` with the reason as a word: `too_broad`, `too_many_actors`, `too_many_hashtags`, `too_many_keywords`, `too_many_locations`, `keyword_length`, `invalid_hashtag`, `unknown_media_type`, `unknown_actor_type`, `invalid_actor`, `invalid_location`, `invalid_language`, `unknown_order`, `unknown_field`, `malformed`.

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

A feed is paged by what came before, not by a count from the top. The cursor is the date and id of the last post of the page before, and the next page is whatever is older. A post written while somebody is reading neither repeats a post nor pushes one out of sight. A feed has no total: counting it would be reading all of it.

A query that cannot finish in the time a query is given is answered `422` with `too_broad`.

## Keeping feeds on the home page

`PUT /feeds/<id>/pin` adds a feed after the ones already there, and `DELETE` takes it off. `PUT /feeds/pins` with `{ "ids": [...] }` replaces the whole list in that order, which is how the tabs are reordered. `GET /feeds/pinned` returns them in order.

Keeping somebody else's feed on your home page is how a feed is followed. There is no separate following of feeds. A feed that is deleted, or that its owner stops sharing, drops out of the list.

Following a hashtag is the same thing: the web app makes a feed of that one hashtag, seen by its owner alone, and puts it on their home page.

## Endpoints

All on feed-service. Writing needs the `media:write` scope.

| Endpoint | Who | What |
| --- | --- | --- |
| `POST /feeds/` `{ name, body, access, definition }` | signed in | Makes a feed. `access` is `myself` unless given |
| `GET /feeds/mine` | signed in | The feeds the viewer made |
| `GET /feeds/pinned` | signed in | The feeds on the viewer's home page, in order |
| `PUT /feeds/pins` `{ ids }` | signed in | Replaces the feeds on the home page |
| `POST /feeds/preview` `{ definition }` | signed in | The first page of a feed not saved yet, as the viewer's own |
| `GET /feeds/<id>` | whoever may see it | The feed, with `references`: the profiles and places it names that the viewer may see |
| `PATCH /feeds/<id>` | its owner | Changes it |
| `DELETE /feeds/<id>` | its owner | Deletes it |
| `GET /feeds/<id>/items?cursor=&limit=` | whoever may see it | A page of its posts |
| `POST /feeds/<id>/copy` | whoever may see it | A feed of the viewer's own showing the same thing, seen by them alone |
| `PUT`, `DELETE /feeds/<id>/pin` | whoever may see it | Keeps it on the home page, or takes it off |

A feed is sent with `is_pinned` and `authorized` (`edit`, `delete`, `pin`, `copy`).

## Where the code is

- **`anahita-libs/feeds`:** the definition, its rules and limits (`Parse`, `Check`), and the cursor. feed-service checks a definition with it when a feed is saved, and graph-grpc-service reads it again with the same code before building a query, so nothing reaches the database that was not read into its fields.
- **`graph-grpc-service/data/repositories/feed_custom.go`:** the query. Every value is a parameter. It ends with `SetFeedAccessQuery`, `SetLifecycleQuery` and `SetArchiveQuery`, for the reader.
- **`graph-grpc-service/data/repositories/nodes_visible.go`:** a feed's own visibility, in `FilterVisible`.
- **`feed-service/cmd/api/handlers/custom_feed.go`:** the endpoints.

## What is not built

- **An order other than newest first.** See [The home page](#the-home-page).
- **Reposts in a custom feed.** A custom feed holds posts, and replies when asked for. The feed of people you follow holds reposts as it always did.
- **"Posts by the followers of a group"** as a source, which the plan had as `members_of`. A feed of the groups somebody follows is `following` with `actor_types: ["group"]`.
- **"With images"** as a filter. `media_types: ["photo"]` is the photo posts.
- **A cache of a public feed's first page.** Nothing is slow enough to need one yet.
- **Feeds made for a new account.** The feed of people you follow is the first tab, and the rest are for each person to make.
- **Emptying a feed's description.** A description can be changed and cannot be set back to nothing: the graph's node edit leaves an empty text as it was.
- **Feeds across the fediverse.** A custom feed is a feature of the installation. Posts from followed remote accounts will appear in one as they will in any feed.
