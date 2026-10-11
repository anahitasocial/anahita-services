# Recommendations

recommendation-service suggests people, groups and events to follow. It uses no model: each suggestion is worked out from the graph as it stands, by rules written down here, and is sent with the reasons for it.

It is optional. An installation that does not want suggestions leaves the service out, and the web app shows none. An installation that wants them chosen another way runs another service that answers the same paths.

## Contents

- [The contract](#the-contract)
- [Why somebody is suggested](#why-somebody-is-suggested)
- [Who is never suggested](#who-is-never-suggested)
- [What each person chooses](#what-each-person-chooses)
- [Each person's own weights](#each-persons-own-weights)
- [Not interested](#not-interested)
- [Running it, and not running it](#running-it-and-not-running-it)
- [Where the code is](#where-the-code-is)
- [What is not built](#what-is-not-built)

## The contract

Every path needs somebody signed in: a suggestion is made to a person, about that person.

| Endpoint | What |
| --- | --- |
| `GET /recommendations/actors?kind=&context=&limit=` | Who the viewer might follow, best first |
| `GET /recommendations/preferences` | What the viewer has chosen about suggestions |
| `PUT /recommendations/preferences` | Changes any of those choices. One not sent is left as it was |
| `GET /recommendations/dismissals` | The profiles the viewer said they were not interested in |
| `POST /recommendations/dismissals` `{ actor_id }` | Not interested in this one |
| `DELETE /recommendations/dismissals/<actor_id>` | Takes that back |

`GET /recommendations/actors` takes:

| Parameter | Values |
| --- | --- |
| `kind` | `person` (the default), `group` or `event` |
| `context` | `home` (the default), or `onboarding` for somebody who has just joined and follows nobody: nothing worked out from who they follow is used |
| `limit` | Up to 50. 10 when left out |

and answers:

```json
{
  "items": [
    {
      "actor_id": 4,
      "score": 9,
      "reasons": [
        { "type": "followed_by_leaders", "count": 3, "ids": [2, 3, 9] }
      ],
      "actor": { "id": 4, "type": "node.actor.person-service.person.v1", "name": "…" }
    }
  ],
  "related": [
    { "id": 2, "type": "node.actor.person-service.person.v1", "name": "…" }
  ]
}
```

- `actor` is the profile as the viewer is sent it anywhere else.
- `reasons` is why, the one that counted most first. `count` is how strongly, and `ids` up to three of what it was found through.
- `related` is what the reasons name, to say them in words: the people who follow somebody suggested, the hashtags shared. Only the ones the viewer may see.

## Why somebody is suggested

Each reason adds to a score, up to a most. The score is then kept in full for a profile that was visited or posted on in the last 90 days, halved for one quiet for up to a year, and cut to a tenth for one quiet longer, so that the dormant come last and are still there when nobody else is.

| Reason | Found when | Each | Most |
| --- | --- | --- | --- |
| `follows_you` | They follow the viewer, who does not follow them. People only | 4 | 4 |
| `followed_by_leaders` | People the viewer follows, follow them. `ids` are those people | 3 | 15 |
| `inviter` | They invited the viewer, or the viewer invited them. People only | 3 | 3 |
| `hosted_by_leader` | A group the viewer follows is hosting it. Events only. `ids` are those groups | 3 | 3 |
| `you_engaged` | The viewer liked what is posted on their profile, in the last 90 days | 2 | 8 |
| `shared_interest` | Their public posts of the last year carry hashtags the viewer has used or liked. `ids` are those hashtags | 2 | 8 |
| `featured` | The installation's administrators featured them | 0.5 | 0.5 |
| `popular` | Many people followed them in the last 30 days | 0.5 | 0.5 |

Two profiles that score the same are in the order of their ids, so an answer is the same each time it is asked for.

`shared_interest` looks only at public posts. The hashtags come back as the reason, and a reason must not say what is in a post the viewer could not read.

The numbers are constants in `recommendation-service/constants`. They are the service's own judgement of what makes a good suggestion, and each person can set their own in its place: see [Each person's own weights](#each-persons-own-weights).

### Events

An event is suggested while it is still to come or on, and has not been called off. For an event, `followed_by_leaders` is people the viewer follows who have answered it, and following already is having answered it. An event is not asked to have a picture or a description, and is not marked down for when it was last visited: one that is still to come is as alive as anything is. It is sent with its `event` block, as it is in search results.

## Who is never suggested

Whatever the reason, a profile is left out when it is:

- the viewer, or somebody they follow already, or have asked to follow;
- on either side of a block with the viewer;
- one the viewer said they were not interested in;
- somebody who asked not to be suggested;
- disabled, archived or deleted;
- an event that is over or was called off;
- a person who has not confirmed their email address;
- a person or group without a picture and with nothing said about itself;
- one the viewer may not see.

All but the last are one clause every signal's query ends with (`eligibleCandidate`). The last is the graph's rule and is asked of the graph: the best hundred of what was found are put to `FilterVisible`, and only what it returns is sent. So is everything in `related`.

## What each person chooses

`GET` and `PUT /recommendations/preferences`:

| Choice | Means |
| --- | --- |
| `suggest_people` | To be suggested people |
| `suggest_groups` | To be suggested groups |
| `suggest_events` | To be suggested events |
| `use_activity` | That what they liked and wrote may be used to choose for them. Off, `you_engaged` and `shared_interest` are not looked for. Who they follow is still used |
| `discoverable` | That they may be suggested to other people |

Every choice is yes until its owner says otherwise. They are theirs to read and change and nobody else's.

### Each person's own weights

Somebody can set how much each reason counts for when choosing for them. It changes the order they are suggested things in, and nothing about what anybody else is suggested.

- `GET /recommendations/preferences` sends `weights` (what each reason counts for, for them), `default_weights` (the service's own), `max_weight` (10) and `reasons` (the order to list them in).
- `PUT` takes `weights`, by the reason's name. The ones sent are set and the others left. A weight is from 0 to 10; a reason there is not, or a number out of range, is answered `422` with `invalid_weight` and nothing is changed.
- A reason set to 0 is not used for them at all.
- What a reason may count for at most keeps its proportion: `followed_by_leaders` counts for up to five times what it counts for once, whatever that is set to.
- `reset_weights: true` goes back to the service's own.

Kept as JSON in `recommendation_preferences.weights`, empty until set (migration `20261021000001_recommendation_weights`).

## Not interested

A profile somebody turns down is not suggested to them again. It is theirs alone: whoever was turned down is not told, and nobody else can read the list. It can be taken back from the list.

## Running it, and not running it

- It reads the installation's database for its signals and writes two tables of its own, `recommendation_preferences` and `recommendation_dismissals` (migration `20261020000001_recommendations`).
- It takes the settings every service does (`DSN`, `GRAPH_SERVICE_ADDR`, `FILE_SERVICE_ADDR`, `AMQP_ADDR`, the OAuth settings) and has none of its own.
- nginx sends `/recommendations/` to it. `make recommendation` builds and starts it on the development stack.
- When an actor is removed for good, what it chose and what was chosen about it are removed with it (`node.actor.*.*.purged.v1`).
- **Without it:** the gateway answers `/recommendations/` with an error, which the web app takes as there being no suggestions and shows nothing of them. On Kubernetes, leave `services/recommendation.yml` out of the kustomization.
- **In its place:** another service answering the paths above. It should ask the graph which profiles the viewer may see, as this one does.

## Where the code is

- `recommendation-service/data/repositories/signals.go`: the queries, one a reason, and the clause that keeps each to profiles that may be suggested. Tested against a real database in `signals_integration_test.go`.
- `recommendation-service/scoring`: the ranking. It reads nothing, so the same finds give the same answer.
- `recommendation-service/cmd/api/handlers`: the endpoints.

## What is not built

- **Interests somebody picks.** What they are interested in is taken from the hashtags on what they wrote and liked. The plan had interests chosen by hand, in onboarding and in settings, as edges of their own.
- **Suggestions in onboarding.** `context=onboarding` is answered; the web app's onboarding does not ask yet.
- **Nearby.** Nothing uses where people are.
- **A cache.** Each answer is worked out when it is asked for, which on the development data takes under a tenth of a second.
- **Suggested posts.** The same contract would take a second path for them.
