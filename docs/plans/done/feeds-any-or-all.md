# Custom feeds: posts that match any of these, or all (roadmap row 149)

## Context

The user, 2026-11-15: "I should be able to say any posts with these keywords
or any posts from these profiles or these hashtags." Today a custom feed's
sources (profiles I follow, chosen profiles) add up, but every kind of filter
narrows them (docs/feeds.md, "The definition"): chosen profiles AND a hashtag
AND a keyword AND a location. "Posts from Naomi, or anything tagged #ceres"
cannot be made. Settled with the user the same day:

1. **One choice for the whole feed**, not AND/OR on each filter: that would
   be logic with hidden brackets, and feeds that show nothing.
2. **New feeds start at Any.** Existing feeds keep All, so none changes.

## The shape

The definition gains one key, `match`: `"any"` or `"all"`.

| Group | Fields | With `any` | With `all` (today) |
| --- | --- | --- | --- |
| What a post matches | `sources.following`, `sources.actors`, `hashtags_any`, `keywords_any`, `locations_any`, `near` | A post matching one is in | Sources add up; each filter narrows them |
| What it must be | `media_types`, `actor_types`, `language`, `origin`, `include_replies` | Always narrow | Always narrow |
| Exclude | `exclude.*` | Always out | Always out |

1. A definition without `match` is read as `all`: every feed saved before
   this keeps its meaning, and an old client changes nothing.
2. The web app's form sends `any` for a new feed; the choice is shown only
   when two or more matching fields are filled, as "Show posts that match
   **Any of these** / **All of these**" (radio buttons, a `RadioGroup`
   labelled by its line, for row 148's accessibility).
3. The whole-site rule stays: with no source, a feed needs a hashtag, a
   keyword, a location or an area (`Check` in anahita-libs/feeds).
4. With `any` and `following`: the people one follows plus everything with
   the hashtag, from anywhere, which is the case the user described.

## Changes

1. **anahita-libs/feeds/definition.go**: `Match string json:"match"` on
   `Definition`; `Parse` tidies an empty value to `all`; `Check` refuses
   another value (`unknown_match`, beside `unknown_order`). Tests in its
   `_test.go` for both and for an old definition.
2. **graph-grpc-service/data/repositories/feed_custom.go**: today
   `feedCustomSources` adds one `sq.Or` of sources and `feedCustomFilters`
   adds a `Where` per filter. With `any`, the source conditions,
   `feedCustomTaggedWith`, `feedCustomPlacedAt`, `feedCustomMentions` and
   the `near` condition go into one `sq.Or`; the narrowing filters and
   `feedCustomExclusions` stay as they are; `SetFeedAccessQuery` and the
   rest still end it, so a feed never widens what a reader may see.
3. **feed_custom_integration_test.go**: `any` finds a post from a chosen
   profile without the hashtag, and one with the hashtag from somebody
   else; `all` keeps today's results; access tests rerun with `any`.
4. **anahita-web**: `src/containers/feeds/Form.jsx` (the choice, default
   `any` for Add, the saved value for Edit), `src/utils/customFeeds.js`
   and `DetailsDialog.jsx` where a feed is described in words ("… or …"
   for any), and the five language bases.
5. **Docs**: anahita-services `docs/feeds.md` (the definition, its example,
   the two groups as a table, the API reference generated from it);
   anahita-web `docs/architecture.md` where the feed form is described;
   both changelogs.

## Phases

| Phase | What | Status |
| --- | --- | --- |
| 1 | `match` in the definition, the query, and their tests | ✅ 2026-11-16: `all` stored as nothing, so a service not yet upgraded reads a feed of all |
| 2 | The choice in the web app's form, and a feed described in words | ✅ 2026-11-16 |
| 3 | The docs of both repositories; tried at both sizes; changelogs | ✅ 2026-11-16 |

## Verification

1. `go test` in anahita-libs/feeds and graph-grpc-service; the feed
   integration tests on a throwaway MySQL (`ACCESS_TEST_DSN`), removed after.
2. On dev: a feed of one chosen profile and #tag with Any shows both kinds
   of post; switched to All, only posts with both; an old feed unchanged.
3. Headless Chrome at 1280×900 and 390×844 (touch): the choice appears with
   the second matching field and nothing jumps; `yarn test` for the strings.
4. Delete the test feeds afterwards.
