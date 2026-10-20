# Search

Search finds people, groups, events, posts and shared feeds by what they are
called or what they say. This page is about the order results come in, and
how an operator tunes it.

Its route is in the API reference: [search-service](api/index.html#search-service).

## Contents

- [Two orders](#two-orders)
- [Most relevant](#most-relevant)
- [Old and useful counts as much as new](#old-and-useful-counts-as-much-as-new)
- [Tuning it](#tuning-it)
- [Each person's choices](#each-persons-choices)
- [What is found at all](#what-is-found-at-all)
- [Where the code is](#where-the-code-is)
- [What is not built](#what-is-not-built)

## Two orders

The search page offers two: **Most relevant**, the default, and **Most
recent**, newest first. Most recent is only that. The rest of this page is
about Most relevant, which also orders a hashtag's and a location's posts
([hashtags.md](hashtags.md#a-hashtags-and-a-locations-posts)), without the
words.

## Most relevant

Results are ordered by three things, one after the other. The second only
decides between results the first puts level, and the third between those
the second puts level.

1. **Where the words are.**

   | Where | Points by default |
   | --- | --- |
   | The handle exactly (`rastin.example`, `ana`), with or without the `@` typed before it | 100 |
   | The name exactly | 90 |
   | The name begins with them | 70 |
   | A word in the name begins with them ("Jericho **Bea**ch") | 60 |
   | Somewhere inside the name, or inside the handle (`decaf`**`coffee`**`club`) | 50 |
   | A hashtag on the post, or, on a photo or a video, the name or the handle of somebody in it, their tag approved | 40 |
   | A whole word in the body | 20 |
   | Only inside a word in the body ("**coffee**house") | 10 |

   The first that holds is the one counted. Only the order of the points
   matters: a profile called *Coffee* comes before a post that mentions
   coffee however much the post is liked.

2. **Its standing:** how useful people have found it, and whether the
   searcher follows it.

   ```
   standing = scale × ln(1 + like × likes + save × saves
                           + reply × (replies, reposts and quotes)
                           + follow × followers
                           + cite × citers)
            + following, when the searcher follows it or the profile it is on
   ```

   The logarithm keeps one post liked a thousand times from burying
   everything: it comes first, not a hundred times first. Each like, save,
   reply or follow counts by when it happened (below).

   **Citers** are the people who show it within an article or a topic of
   their own, each once however many of their posts do, and never its own
   author; a profile's are those of its posts. Showing a profile is naming
   it, a mention, and not a citation: `nodes.citation_count`, kept in step by the
   graph (`repositories/citations.go`). Only a published post anybody may
   read counts as citing. A citation counts as it stands, not halved with
   time: citing is deliberate and lasting, and useful knowledge stays
   useful. `cite` starts at 3, above a like (1) and a save (2), since citing
   takes effort; it is a starting value, to be tuned once there are
   citations to look at.

3. **Its id,** the newest first, so that results level on everything keep
   their places from one page to the next.

With a distance chosen, the nearest come first among results level on the
first two.

## Old and useful counts as much as new

Anahita is for building knowledge as well as for news: a school's notes, a
library's guides, a research group's findings stay useful for years. So
nothing is pushed down for being old. What may fade is **interest**: each
like, save, reply or follow is counted by when it happened, halved every
`half_life_days`.

- A guide written years ago that people saved this month stands as high as
  a post from this week that was saved as often.
- A post nobody has touched in a long while weighs only its old interest.
- `half_life_days: 0` lets nothing fade: interest counts the same however
  old.

By default it is 365 days. A newsroom may want a month; a library, a
school or a research group a year or more, or 0.

## Tuning it

Every number above is the operator's, in
`graph-grpc-defaults/search_ranking.json`:

```json
{
  "match": {
    "handle": 100, "name": 90, "name_starts": 70, "word_starts": 60,
    "name_contains": 50, "hashtag": 40, "body_word": 20, "body_contains": 10
  },
  "following": 15,
  "useful": { "scale": 5, "like": 1, "save": 2, "reply": 2, "follow": 1, "cite": 3 },
  "half_life_days": 365
}
```

- graph-grpc-service reads it from `SEARCH_RANKING_PATH`, which is
  `/app/defaults/search_ranking.json` in the image. docker-compose mounts
  the working tree's copy there. On Kubernetes, mount a ConfigMap over that
  path to change it.
- **It is read again when it changes,** without a restart.
- **What it leaves out keeps its default,** so a file with only
  `{"half_life_days": 30}` changes that and nothing else.
- **A file that cannot be read, or has a number below 0 or above 1000, is
  not taken:** the default weights are used, and the service says so in
  its log.

## Each person's choices

Two, off as they come, sent with a search as `followed_first=true` and
`recent_first=true` and only with "Most relevant":

- **Profiles I follow first:** what the searcher follows, or what
  is on a profile they follow, before everything else, however it matches.
  Nothing changes for a visitor, who follows nobody.
- **Newer results first:** among results that match alike, the newest first, and
  only then by standing. The match still comes first: a profile called
  *Coffee* is still above a post that mentions coffee.

The web app keeps them in the searcher's browser, per person.

### Their own weights

Each person who is signed in can also change how much each sign counts for
them, as they can for their suggestions (the "Adjust" chip by "Most
relevant"). There are six: likes, saves, replies, followers, citations,
and who or what they follow (the people, groups and events, and what is
posted on them). Each is set to a step that multiplies the operator's
weight:

| Step | Multiplies by |
|---|---|
| Off | 0 |
| Less | 0.5 |
| As is | 1 |
| More | 1.5 |
| Much more | 2 |

So with the default file, a citation set to Much more counts 6 for that
person, and a like set to Off counts nothing. The match always comes
first; the steps only change the order among results that match alike.

- `GET /search/preferences` answers with the person's steps, which may be
  set (`adjustable`) and to what (`options`). A name not in `steps` is As
  is.
- `PUT /search/preferences` with `{"steps": {"cite": 2, "like": 0}}`
  replaces them all. A name that may not be set, or a value that is not a
  step, is refused with 422 `invalid_weight`, and nothing changes.
- They are kept on the server, in `search_preferences`, so they follow the
  person to any device. Only steps other than As is are stored, and the
  row is removed when the profile is purged.
- The names and steps are `constants.SearchAdjustable` and
  `constants.SearchSteps` in anahita-libs. The graph applies them with
  `ranking.Weights.Adjusted`.

## What is found at all

The order does not change what is found. A result has the words in its
name, its handle (matched without a typed `@`), or its body, and is one the
searcher may see; see [Permissions](permissions.md).

**What may be searched for** (`search_query`, `anahita-libs/validators`):
letters, marks and numbers in any language ("Montréal", "Zürich", "東京"),
spaces, and what people type in a name, a handle or an address: `-` `_` `.`
`,` `+` `#` `@` `"` `'` `’` `:` `/` `?` `&` `=` `~` `!` `(` `)`. Up to 100
characters. It refused everything but unaccented Latin letters until
2026-10-19. A search among names (`name_search_query`, the lists of a
profile's followers) takes the same letters with `-` `_` `.` `+` `"` `'` `’`.
None of it is written into SQL: the words are bound parameters, `LIKE`'s
wildcards are taken out, and a regular expression made of them is quoted.

## Where the code is

- `graph-grpc-service/data/repositories/search.go`: what is found
  (`wordsMatch`), the two scores (`relevance`), and the order, ranking the
  page's ids before reading their rows (`browseByRelevance`).
- `graph-grpc-service/ranking`: the weights, the file, reading it again, and
  each person's steps (`Adjusted`).
- `graph-grpc-service/data/repositories/search_preferences.go` and
  `search-service/cmd/api/handlers/preferences.go`: each person's steps,
  kept and given.
- `graph-grpc-service/data/repositories/search_ranking_integration_test.go`:
  the order checked against MySQL, with searches as people make them.

## What is not built

- **An index of words.** Search matches with `LIKE '%…%'`, which reads every
  row and knows nothing of words or languages. MySQL's `FULLTEXT` index with
  the `ngram` parser was tried and put off: it missed posts that `LIKE`
  finds (two-letter stopwords, text indexed before the index existed,
  Japanese). Search is fast enough without it at today's sizes; a search
  engine beside the database is the likelier answer when it is not.
- **Ranking by the citers' own standing,** as Google ranks the web, where
  a citation from what is itself much cited counts for more. Citations
  count today each as one (above); weighing them by who cites is for when
  there are enough to tune it against. See the web app's
  `docs/plans/done/search-improvements.md`.
