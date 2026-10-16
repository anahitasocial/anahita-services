# Abuse reports

[Documentation](README.md) › Abuse reports

How people report something, how administrators deal with it, and how an
installation sets the reasons people can choose from. It is all in
abuse-report-service, under `/abuse-reports`.

Its routes are in the API reference: [abuse-report-service](api/index.html#abuse-report-service).

## Contents

- [Reports and cases](#reports-and-cases)
- [Reporting something](#reporting-something)
- [The reasons list](#the-reasons-list)
- [Using your own reasons](#using-your-own-reasons)
- [What administrators do](#what-administrators-do)
- [When the reported thing is deleted](#when-the-reported-thing-is-deleted)
- [Who sees what](#who-sees-what)

## Reports and cases

Anything can be reported: a person, a group, a post, a reply, a hashtag or
a place.

A **report** is one person's complaint. A **case** is one reported thing and
the decision about it. Everybody who reports the same thing while its case is
open joins that case, so a post that twenty people report is one case for an
administrator to handle.

A case is `open` until somebody decides it, and then `actioned` (something was
done) or `dismissed` (nothing needed doing). If the same thing is reported
again after its case was closed, a new case opens, and the earlier one shows
as its history.

Reports are not nodes. They are kept in two tables of their own,
`abuse_cases` and `abuse_reports`, so that nothing which lists, searches,
federates or exports content can ever show one, and so that a case outlives
the thing it was about.

## Reporting something

`POST /abuse-reports` with `{ "target_id": 123, "reason_key": "spam", "details": "…" }`.

| Answer | Meaning |
| --- | --- |
| 201 | Reported |
| 200 | You had already reported it. Your report now has the new reason and details |
| 400 | The reason is not on the list or does not apply to this kind of thing, the reason needs an explanation and none was given, or it is your own |
| 404 | There is no such thing that you can see |
| 429 | More than 20 reports in the last hour |

- **Signed in only.**
- **You can only report what you can see.** Something hidden from you answers
  exactly as something that does not exist, so reporting cannot be used to
  find out what is hidden.
- **Not yourself, and not what you wrote.**
- **One report per person per case.** Reporting again changes yours.
- **Details** are optional, up to 1,000 characters, except where the reason
  requires them.

## The reasons list

`GET /abuse-reports/reasons?type=<node type>` returns the reasons that can be
given for that kind of thing, in the reader's language (`?lang=fr-FR`, or the
browser's `Accept-Language`).

The service carries a default list, in English and French:

Spam · Harassment or bullying · Racism · Sexism · Other hate · Violence or
threats · Self-harm or suicide · Sexual content shared without consent · Child
safety · Impersonation (people and groups only) · Private information ·
Misleading or false information · Intellectual property or copyright ·
Trademark · Illegal content or activity · Something else (an explanation is
required)

## Using your own reasons

What a community forbids is its own business. To replace the list:

1. Copy `abuse-report-service/data/reasons/reasons.json` and edit it.
2. Make the file available to the service: a volume in docker-compose, a
   ConfigMap on Kubernetes.
3. Set `REASONS_PATH` to where it is mounted.

The file is read again whenever it changes, so an edit takes effect without a
restart.

~~~json
{
  "reasons": [
    {
      "key": "hate_racism",
      "label": { "en-GB": "Racism", "fr-FR": "Racisme" },
      "description": {
        "en-GB": "Attacks or slurs based on race or ethnicity.",
        "fr-FR": "Attaques ou injures fondées sur la race ou l’origine ethnique."
      },
      "applies_to": ["node.actor.", "node.medium."],
      "requires_details": false,
      "atproto_reason": "com.atproto.moderation.defs#reasonRude"
    }
  ]
}
~~~

| Field | Meaning |
| --- | --- |
| `key` | What is stored on each report. Up to 64 characters, and unique. Treat it as permanent: reports that used a key you remove show the key in place of a label |
| `label` | The name, by language. `en-GB` is required |
| `description` | A sentence shown under the name. Optional |
| `applies_to` | The kinds of thing the reason can be given for, as type prefixes: `node.actor.` (people and groups), `node.medium.` (posts and replies), `node.tag.` (hashtags and places). Leave it out for everything |
| `requires_details` | `true` makes the reporter explain |
| `atproto_reason` | The AT Protocol reason this corresponds to. Not used yet; it is there for when reports are sent to other servers |

A file that cannot be used, because it is missing, is not valid JSON, has no
reasons, repeats a key or lacks an `en-GB` label, is not applied in part. The
built-in list is used until it is fixed, and the service's log says what is
wrong with it.

## What administrators do

In the web app, under **Administration › Reports**. The endpoints:

| Endpoint | Does |
| --- | --- |
| `GET /abuse-reports/cases?status=open&reason=spam` | Lists cases, the most recently reported first, 20 at a time (`limit`, `offset`). `status` is `open`, `actioned` or `dismissed`. `reason` is optional: with it, only the cases in which somebody gave that reason |
| `GET /abuse-reports/cases/summary` | `{ "open": n }`, for the number on the Administration menu |
| `GET /abuse-reports/cases/:id` | The case: what was reported and by whom, each report's reason and details, and the earlier cases the same thing has had |
| `POST /abuse-reports/cases/:id/resolve` | Closes it: `{ "status": "actioned" or "dismissed", "action": "…", "note": "…" }` |
| `POST /abuse-reports/cases/:id/reopen` | Opens a closed case again |

**The service records decisions. It does not carry them out.** Deleting a
post, disabling an account or archiving a group is done through the service
that owns it, under that service's own rules: an administrator who may not
archive a group cannot do it from here either. The app does the action first,
then records the case as `actioned` with what was done: `deleted`, `disabled`,
`archived`, `blocked` or `other`.

Two administrators deciding the same case at once get one decision recorded;
the second is told it was already decided (409). A closed case cannot be
reopened while a newer case for the same thing is open (409): use the newer
one.

Nobody is emailed about a report. Administrators see the number waiting on
the Administration menu.

## When the reported thing is deleted

There is nothing left to decide about something that no longer exists, so its
open case is closed as `actioned`, `deleted`, with nobody named as having
decided it.

- A **post or reply** is gone the moment it is deleted.
- A **person or group** is not: deleting one starts a 30-day wait during which
  it can be restored. Its case stays open until it is purged.

The service hears of this through an event. Events can be lost, so opening a
case also checks whether the thing still exists.

When a **person who filed reports** is purged, their reports are removed with
everything else they wrote. A case that other people also reported keeps going
with one report fewer. An open case that only they had reported is removed,
because no complaint is left to decide. A case already decided stays, as the
record of the decision.

This too arrives as an event, and nothing checks again if it is lost: the
reports then stay, naming an account that is only a reserved handle.

## Who sees what

| | Sees |
| --- | --- |
| The reporter | That their report was received. Nothing after that |
| The person reported | Nothing. They are not told of the report, or who made it |
| Administrators and super administrators | Every case, every report, and who made it |

The network address a report came from is stored with it and is not shown in
the app. It is blanked 90 days after the report was filed; the report itself
stays.

Filing a report needs the `account:write` scope. Reading cases needs
`actors:read` and deciding them `actors:write`, on top of being an
administrator.
