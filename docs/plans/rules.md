# Rules, promises and removing spam

*Roadmap row 102, planned 2026-10-19. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** parts 1 to 5 and 2b done 2026-10-19; part 6 next. Branch
`issues/rules` in both repositories, from `issues/thread-composer`.

**Parts, in order:**

1. `site.json` (package `anahita-libs/about`, `auth-defaults/site.json`) and
   `GET /site` — done 2026-10-19. The approved rules, the promises and the
   suggestions in all five languages; read again when it changes.
2. The site's rules ticked at sign-in, beside the Terms, recorded, and asked
   again when the version is raised; `SITE_RULES` — done 2026-10-19.
   Recorded as the Terms are (`auth_accounts.rules_accepted_*`, migration
   `20261122000003`), not in a table of their own: the site's agreement is
   the Terms' kind; groups' get theirs in part 5. `PATCH /agreements/rules`
   takes only the current version. Tried in a browser at both sizes. The
   Terms' versions are left in the web app for now: their text is there
   too, and moving one without the other would split a document from its
   version (to settle with the user).
2b. **The site's rules at sign-up, on the server** (the user, 2026-10-19:
   the first super administrator should be reminded "on the server side
   when signup happens"): every page auth-service renders to make an
   account — the first super administrator's set-password page, open
   sign-up, an invitation's acceptance, a sign-up request — shows the rules
   as a plain checklist, all at once with a box each and no animation ("we
   can be blunt and raw on the ux", the user), and the account is made with the
   agreement and its version recorded; without the current version ticked
   the server refuses. The first super administrator sees them whatever
   `SITE_RULES` says, and the boot log and the set-password email say them
   too. The web app's `/rules` then comes only when the version is raised.
   — done 2026-10-19: `web/templates/rules.html` on the sign-up, invitation
   and set-password pages (`site_rules_signup.go`); a request's version kept
   in `signup_requests.rules_accepted_version` (migration
   `20261122000005`) and recorded on approval; the boot log prints the rules.
   Tried on dev: a sign-up without the boxes refused, one with them made with
   the rules recorded. The email with the link does not carry them: the page
   it opens does. **Changed the same day** (the user: "for the first time
   installation I want the super admins agree not to use Anahita for those
   purposes ... hard code those in the code ... those are my rules as the
   creator"): the first super administrator makes a pledge written in the
   code, "I won't use Anahita to:" and five items, in place of the site's
   rules, whatever `SITE_RULES` says; members keep the site's rules as they
   are. `SITE_RULES=off` now asks nobody in the web app.
3. The promises on the About page — done 2026-10-19: first on the page,
   under "Our promises to you", with "Read our rules" leading to `/rules`
   (anahita-web `containers/about/Promises.jsx`). The site's rules have a
   page of their own, `/rules`, after the Terms (the user, the same day),
   each zooming forward once the one before is ticked.
4. Remove as spam — done 2026-10-19: `POST /people/:id/spam` and
   `POST /groups/:id/spam` in actor-service, the purge with a `reason`
   (`removed-as-spam` logged with who decided); the case action
   `removed_as_spam`; in the web app, in the profile's menu and on a
   report's case. No proof of identity: asked once more instead. Only for
   an account made in the last 30 days (`SPAM_REMOVAL_MAX_DAYS`; the user,
   the same day), refused `too_old` otherwise. Asked for with it: the days a
   deleted profile can be brought back made a setting too,
   `ACCOUNT_GRACE_DAYS`, 30 unless said, one number for the sweeper, the
   dates told, NodeInfo and the promise. Tried on
   dev: refused for a registered person (403) and for oneself (409); three
   throwaway accounts removed, one from a case, one from a profile.
5. A group's and an event's rules: written, ticked before following or
   answering, shown in the About card, agreed again after a change. In three:
   - 5a, written and shown — done 2026-10-19: `profile_rules`,
     `profile_rule_versions`, `profile_rule_agreements`; `ReadRules`,
     `SetRules`, `AgreeRules`; `GET|PUT /groups|events/:id/rules`,
     `POST .../rules/agree`; a Rules tab in the settings, with the site's
     suggestions; the rules in the About card.
   - 5b, ticked before following, asking to follow or answering, one at a
     time, the server refusing without the current version — done
     2026-10-19: graph-grpc refuses a follow or a request for oneself, an
     invitation's acceptance and an event's going or maybe with
     `rules_not_agreed` (FailedPrecondition; 409 from socialgraph-service
     and actor-service), administrators excepted; the web app asks first,
     in a dialog. Tried as Worf on a phone: the follow went through once
     the four rules were ticked.
   - 5c, agreed again after a change, before the next post there — done
     2026-10-19: `Medium.Add` and `Medium.AddThread` refuse a new post on a
     group or an event with rules not agreed to at their version (not a
     reply; not its administrators); 409 from text-service and
     photo-service; both composers ask first, saying the rules changed when
     they did. A video post from another client is refused by the worker,
     after its upload: the web app asks before. Tried as Worf: refused,
     agreed, posted; through the composer in a browser too.
6. Reporting against a rule.

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

Asked for the same day, and planned here with the rules, since all three say
what a site is and what it keeps to:

4. **Promises on every site's About page,** of what Anahita itself keeps to,
   in short and plain words, so that people read them (the user).
5. **Removing an account as spam at once,** without the 30 days a deleted
   account waits: "by the time we get to it, it might be too late" (the
   user).

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

6. **A group with rules takes a follow from another server as a request**
   (the user): somebody elsewhere cannot tick a checklist. Groups and events
   do not federate yet (row 73), so this is built when they do; recorded here
   so that it is.
7. **Rules changed, agreed again,** for the site and for groups: before
   their next post there (the user). For an event, changed rules are shown,
   not asked again.
8. **The site's starting list is drafted here** for the user to approve (the
   user), from what progressive servers on the fediverse share: the
   Mastodon Server Covenant asks servers for "active moderation against
   racism, sexism, homophobia and transphobia".

### The site's starting rules (approved by the user, 2026-10-19)

Short, one tick each:

1. **No hate.** Don't attack or put down people for their race, ethnicity,
   religion, gender, sexuality, disability or where they come from. No hate
   groups.
2. **Keep children safe.** Never post or share anything that sexualises or
   harms a child.
3. **No harassment.** Don't threaten, bully, pile on or stalk anyone. Never
   share someone's private details or intimate images without their
   consent.
4. **No violent extremism.** Don't promote fascism, white supremacy, or any
   far-right or violent movement.
5. **Stick to the facts on health and science.** Don't spread anti-vaccine or
   anti-science misinformation that puts people at risk.
6. **Moderators decide.** The people who run this site can remove anything
   that breaks these rules, and things these rules don't list.

### The promises (settled 2026-10-19)

On the About page of every Anahita site, under **Our promises to you**, as
the user chose them from what is built:

1. **Changed your mind?** You have 30 days to undo deleting your account.
   Just sign in again.
2. **Your data is yours.** Download all of it, any time, from your settings.
3. **Keep your account safe.** Turn on two-factor sign-in in your settings.

Only what the software keeps by itself: an operator's own promises (such as
moderation and daily backups on banu.io) are theirs to add. The Mastodon
Server Covenant's four commitments (active moderation, daily backups, a
second person with emergency access, three months' notice before closing)
go in `docs/deploying.md` as good practice for operators, not as a
requirement: the user can keep two of them, and a school or a small group
may keep fewer.

### Removing an account as spam

1. **Remove as spam,** for a person or a group, by an administrator or a
   super administrator: from a report (Administration › Reports) and from
   the profile itself.
2. **At once and for good:** purged now, as a super administrator's purge
   is, with everything it wrote; not restorable, so promise 1 does not apply
   to it. Its sessions end.
3. **Never an administrator's or a super administrator's account,** nor
   one's own.
4. **Recorded:** `removed-as-spam` in the account's log, with who did it; a
   report about it closed as `actioned`, `removed_as_spam`.
5. **Asked once more** before it is done, saying it cannot be undone.

**Open:** keeping the address it signed up with from signing up again.
Proposed: not in this row.

### What a site says about itself, in one file (settled 2026-10-19)

The site's rules and its promises are the operator's, in a file like the
other defaults (the user: "the same way that we have <servicename>-defaults",
and "same as signup rules"): `auth-defaults/site.json`, read by auth-service
from `SITE_DEFAULTS_PATH` (`/app/defaults/site.json` in the image; mounted by
docker-compose, a ConfigMap on Kubernetes), read again when it changes, and
the defaults kept when it cannot be read, as `search_ranking.json` is.

```json
{
  "rules": {
    "version": "1",
    "items": [
      { "title": { "en-CA": "No hate.", "fr-FR": "…" }, "body": { "en-CA": "…" } }
    ]
  },
  "promises": [
    { "title": { "en-CA": "Changed your mind?" }, "body": { "en-CA": "You have {{days}} days to undo deleting your account. Just sign in again." } }
  ],
  "rule_suggestions": [
    { "title": { "en-CA": "Be kind and respectful" } }
  ]
}
```

1. **Each text carries its translations;** a language left out falls back to
   English, so an operator may write in one language.
2. **The site's rules are not edited in the app:** the file is where they
   are. Raising `version` asks everybody to agree again; a correction that
   leaves it is not asked again.
3. **`{{days}}`** and any such number is filled from the configuration, so a
   promise never says what the site does not do.
4. **Served on one public route,** `GET /site`, read by the About page, the
   sign-up checklist and the group rules editor; NodeInfo says only whether
   rules are on (`SITE_RULES`).
5. **An operator's own promises** (banu.io's moderation and daily backups)
   are added to the file, with no change to the code.
6. **On the service side** (the user): the web app keeps only its own
   words (the headings, "I agree"); what a site says comes from `GET /site`.
   The Terms of Service and Privacy Policy versions, now in the web app
   (`src/statics/legal`), move into the file too, so the server knows the
   current version and refuses one that is not.

### A group's and an event's rules

1. **Written by its administrators,** each a short title and a line of
   explanation if wanted; **at most ten**, so that every line is read;
   in the order they choose, with the date they last changed.
2. **Suggestions to pick from and change** (`rule_suggestions` in the
   file), as Facebook offers a new group's administrators. To begin with:
   be kind and respectful; stay on topic; no spam or self-promotion; what is
   shared here stays here; no hate or bullying; content warnings for
   difficult topics; check facts before you share; credit people's work.
3. **The site's rules apply as well,** said in one line under the group's,
   so a group need not repeat them.

**Reporting against a rule** (the user, 2026-10-19: in this row, as its last
part): a post in a group can be reported as breaking one of the group's
rules, and the report reaches that group's administrators, as Reddit and
Mastodon do; the site's rules are offered as reasons in the same way.

## Design

What is there already (found 2026-10-19): the Terms of Service and Privacy
Policy are recorded as columns of `auth_accounts` (`tos_*`, `pp_*`), with the
version the web app holds (`src/statics/legal`); sign-up asks for none of it,
and every way in reaches the web app's `AgreementsGate` at the first sign-in,
the first super administrator too (`SeedSuperAdmin`). Following a group is a
follow edge (`SocialgraphFollow.Add`), or a request where the group takes
requests; answering an event is a follow edge with its answer in its meta
(`Events.Rsvp`).

1. **Data** (migration `20261122000003`, with `init.sql` and
   `legacy/upgrade.sql` in step): `rules` (profile, position, title, body)
   for groups and events, with a version on the profile that changes with
   any edit; and `rule_agreements` (person, profile, version, created_at),
   profile 0 for the site, whose rules are in `site.json`.
2. **The site's rules** at sign-in, in the gate after the Terms
   (`AgreementsGate`), so every way in is covered, as the Terms are.
   `SITE_RULES` (default on) in the configuration and in NodeInfo; off, only
   the first super administrator sees them, once.
3. **A group's and an event's rules:** edited by its administrators in a
   Rules tab of its settings; ticked in a dialog before + Follow, before a
   request to follow, and before an event's answer; checked by the server
   where each is made (`Follower.Add`, `FollowRequest.Add`, `Event.Rsvp`, an
   invitation's acceptance), which refuses without the current version
   agreed. Shown in the About card.
4. **Agreeing again** after a change, for the site and groups: the next post
   there is refused (409, `rules_changed`) until the new version is
   agreed, and the web app shows the checklist.
5. **The promises:** a section of the About page, in every base, with the
   30 days read from the configuration rather than written in.
6. **Remove as spam:** a route beside the purge, for administrators, with
   the checks above; a button in a report's case and in the profile's menu.
7. **Reporting against a rule,** last: a report can name a rule of the group
   it was posted in, or of the site (abuse-report-service); a group's
   administrators see the reports of their group's rules.

## Checking

Sign up by each way in; follow a group with rules and one without; answer
an event with rules; change a rule and post again; remove a test account as
spam and see it gone at once; the About page at both sizes.

## Docs to change

`docs/configuration.md` (`SITE_RULES`), `docs/api.md` and the reference,
`docs/permissions.md`, `docs/registration.md`, `docs/account-lifecycle.md`
and `docs/abuse-reports.md` (removing as spam), `docs/deploying.md` (the
first super administrator, and the covenant as good practice), anahita-web
`docs/` for sign-up, groups, events and the About page.
