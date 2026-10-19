# Rules as checklists: the site, groups and events

*Roadmap row 102, planned 2026-10-19. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** planned, not started. After rows 100 and 101.

## Context

The user, 2026-10-19, asked for:

1. **Group rules for joining:** an administrator sets rules that people agree
   to before they follow, shown on the group's profile at all times.
2. **The same for events,** as one feature, not two.
3. **Site rules,** shown to the first person who signs up (the first super
   administrator) and to everybody who signs up after, by any way in:
   an invitation, sign-up with approval, or open sign-up. They remind
   people not to use Anahita for harm: hate groups, material that hurts
   children and women, far-right content, anti-science and anti-vaccine
   misinformation. A super administrator can keep them or switch them off in
   the configuration; whoever installs Anahita is reminded of them once
   regardless.

"An instance is one big fancy group after all" (the user): one checklist
feature for all three.

## Decisions

Settled with the user on 2026-10-19:

1. **A checklist, not a box of text.** Each rule is a short line with its own
   tick; Continue (or + Follow) is enabled only when all are ticked. Terms
   of Service stay as they are, for the law (`agreements`, auth-service).
2. **One feature:** rules belong to a profile (a group, an event, or the
   site), ordered, each a title and an optional line of explanation.
3. **Shown at all times** on a group's and an event's profile, in their
   About card; the site's on a Rules page linked from Legal.
4. **Agreeing is recorded:** who, which profile, which version, when; as
   `agreements` records the Terms of Service.
5. **Site rules on by default,** with the starting list above; the
   configuration can turn them off. The first super administrator sees them
   whatever the setting.

**Open, to settle before building:**

1. A remote follower from the fediverse cannot tick anything: accept them as
   now, or send them to approval when a group has rules? Proposed: a group
   with rules takes remote follows as requests.
2. When rules change: do followers agree again before their next post?
   Proposed: yes for the site and for groups; shown, not asked, for events.
3. The site's starting list, in words for all five bases.

## Design

1. **Data** (migration after `20261122000001`): `rules` (profile, position,
   title, body, version) and `rule_agreements` (person, profile, version,
   created_at).
2. **Services:** BREAD for a profile's rules by its administrators (actor
   service; the site's by super administrators); `agree` records the
   version. Following a group or answering an event with rules needs the
   current version agreed (graph-grpc-service, at follow and at
   `answer_request`). Sign-up and the first super administrator need the
   site's (auth-service, beside `agreements`); `SITE_RULES` in the
   configuration (default on).
3. **Web:** a Rules editor in a group's and an event's settings; the
   checklist dialog before + Follow and before an event's answer; the
   checklist step in sign-up (every way in) and at the first sign-in of the
   first super administrator; the rules in the About card.

## Checking

Sign up by each way in; follow a group with rules and one without; change a
rule and follow again; a remote follow on the dev PDS and fediverse.

## Docs to change

`docs/configuration.md` (`SITE_RULES`), `docs/api.md` and the reference,
`docs/permissions.md`, `docs/deploying.md` (the first super administrator),
anahita-web `docs/` for sign-up, groups and events.
