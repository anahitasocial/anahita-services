# Imports, and moving a WordPress blog (roadmap row 142)

Asked for by the user on 2026-11-07: a WordPress plugin that lists one's
blog posts, sends the chosen ones to Anahita "with respect to their dates,
order, and without pushing them out to the fediverse", then gives a
redirect script "which I'll later use as a replacement for the wordpress
installation"; and, the same day, "why don't we add an import-service and use
them both [with export-service] for any export/import related scenarios that
don't go through conventional APIs", and then: "I'm open to your
suggestions regarding import/export services, just make it lean and clean."
Planned the same day. Not built.

## Contents

1. [Why](#why)
2. [What is settled](#what-is-settled)
3. [Imports in export-service](#imports-in-export-service)
4. [The WordPress plugin](#the-wordpress-plugin)
5. [The redirect script](#the-redirect-script)
6. [What else is imported this way](#what-else-is-imported-this-way)
7. [Out of scope](#out-of-scope)
8. [Phases](#phases)
9. [Documentation](#documentation)

## Why

Many people have a blog they would rather keep on a small site of their
own, with the people they know, than on WordPress with its upkeep. Moving it
by hand, post by post, loses the dates, the order and every link to it.

An import is not an ordinary post. It needs what no ordinary API should
give: a date in the past, nothing sent to other networks, nobody told. It
runs for minutes, must resume after a failure, must not bring the same post
twice, and must say what did not come. Kept in one service, beside
export-service, those powers are in one place, and each new kind of import
is a reader added to it, not a service of its own.

## What is settled

1. **Imports in export-service, not a service of their own** (lean, at the
   user's word): it and `export-worker` already have a job, its progress,
   resuming, files in storage and limits, and an import is the same work
   the other way. No new deployment. `/exports` and `/imports` side by side,
   for every import that is not an ordinary request: a WordPress blog,
   Mastodon's CSV, Anahita's own download, and later Mastodon's archive.
2. **Renamed for both directions when imports are built:**
   `import-export-service` and `import-export-worker` (the user, 2026-11-07),
   both names served for one release, as the BREAD names were.
3. **It never writes another service's tables.** It asks the service that
   owns the thing (text-service for an article, photo-service for a photo,
   the social graph for a follow), so the rules, checks and files stay
   where they are. The powers an import needs are an internal call only it
   may make.
4. **An import is the person's own,** made by them, on their own profile,
   and nothing they could not post by hand is posted for them.
5. **Not through it:** moving a PHP site works on the database itself and
   stays in the `from-php` recipe (rows 30 and 92); a list of servers an
   administrator refuses is activitypub-service's own setting (row 108).

## Imports in export-service

1. **A job:** what is imported, from where, by whom; how many done, how
   many left, what failed and why; resumed where it stopped after a
   restart. `GET /imports/:id` for its progress.
2. **Where each thing came from,** kept for good: the source's address or
   ID beside what it became here (`import_sources`). Importing again skips
   what came already, and the map from old addresses to new is what the
   redirect script is made from.
3. **The powers, as an internal call** (gRPC, export-service's certificate
   only): `published_at` in the past; `send: false`, so that nothing goes to
   the fediverse or Bluesky and no follower is notified; the post then
   stays out of the home feed as new, since the feed is in order of date.
4. **Pictures** fetched from the source and given to photo-service as any
   upload, so that they are checked, sized and have their location removed
   as any other.
5. **Limits:** one import a person at a time, a ceiling on a job's size
   that the operator sets, and the rate at which it asks the other services
   kept low, so that a large blog never slows the site.

## The WordPress plugin

Its own repository, `anahita-wordpress`, on anahitasocial, MIT, which
WordPress's plugin directory accepts as compatible with the GPL.

1. **Pairing:** the plugin asks for the Anahita site's address and shows a
   code; the person approves it on Anahita, signed in as themselves (the
   device flow auth-service has); the plugin is given access to imports
   only, kept in WordPress's options. One public app ID for every blog,
   seeded as the others are, so nothing is registered for each blog.
2. **Choosing:** every post, newest first, with its date and categories,
   filtered by year and category, each with a box to tick.
3. **Sending:** the chosen posts, oldest first, in the background, with a
   progress bar, resumed if the page is closed:
   1. each post an article, its date kept, sent nowhere, nobody told;
   2. its featured image the article's cover; the pictures in its text
      uploaded as photos and its links to them changed;
   3. its categories and tags hashtags;
   4. a post with a gallery and few words a photo post, the rest
      articles; asked once for the whole import.
4. **The map:** for each post, Anahita answers with its new address, and the
   plugin keeps it. Sending again skips what is there.

## The redirect script

From the plugin, **Download the redirect script:** an `index.php` with the
map in it, and the `.htaccess` that sends it every address that is not a
file, WordPress's own. It works wherever WordPress did, shared hosting too.
Each answer is permanent (301), so that search engines move a page's
standing with it:

1. a post's address, with and without its last `/`, and its `/?p=123`;
2. its pictures under `/wp-content/uploads/`, to their new addresses, once
   the files are gone;
3. a category's or a tag's page, to the hashtag's;
4. the feed, and anything else, to the person's profile on Anahita.

Then: a copy of the site and its database kept, WordPress's files taken
away, the two files put in their place. The domain goes on working, with
nothing left to update.

## What else is imported this way

1. **Row 124, part 1:** who one follows, blocks and mutes, from Mastodon's
   CSV; the same files out are made by export-service.
2. **Row 124, part 4:** one's posts from another Anahita's download.
3. **Later:** Mastodon's archive (`outbox.json`), decided apart.

## Out of scope

1. Comments on the blog: they belong to other people, who have not agreed
   to be here.
2. Pages (WordPress's "About" and the like): one's profile and its About
   card are for that.
3. Keeping the blog and Anahita in step after the move.

## Phases

1. **Imports in export-service** and its worker, the job, the sources kept, the
   internal call in text-service and photo-service: about two sessions.
2. **The plugin:** pairing, the list, sending, the map: about two to three
   sessions.
3. **The redirect script,** tried on a copy of a WordPress blog, then a
   real blog moved: about half a session.

About five sessions.

## Documentation

1. anahita-services: `docs/imports.md`, new (what can be imported, how,
   and what an import may do that a post may not); `docs/architecture.md`
   (the new service); `docs/configuration.md`; the API reference;
   `CHANGELOG.md`.
2. anahita-wordpress: its README, and the plugin directory's page.
3. anahita.io: a line for WordPress bloggers, and the plugin's link.
