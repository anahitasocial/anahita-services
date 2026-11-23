# A profile's link card (roadmap row 172)

Asked for by the user on 2026-11-23: a profile shared as a link shows a
small square, its profile photo alone, beside the words, or nothing when it
has no photo. It is to show a card instead, at the shape link previews show
best.

## What it shows

1200 × 630, flat, in the site's own colours:

1. The cover photo across the top, or the site's primary colour where there
   is none.
2. The profile photo in a circle, over the cover's lower edge, or the
   initials' circle the web app draws where there is none.
3. The handle, `@rastin@banu.io`.

The name and the description are not on it (the user, 2026-11-23). A name
can be in any script, and a card drawn on the server has only the fonts it
is given, not the device's, which draw every script in the web app; the
fonts for Chinese, Japanese and Korean alone are 15 to 20 MB. A handle is
Latin only: a person's username is `a`–`z`, `0`–`9`, `-` and `_`
(`validateUsername`), and a group's, event's or channel's is made from its
name by `slug.Make`, which writes it in Latin letters. So one small Latin
font is enough, and the name is still in the preview's title, which the
reader's device draws.

Nothing that changes often, and no counts: the app shows no profile
analytics, and a card does not either.

## How

1. **web-shell draws it.** It already chooses each page's picture and
   sends it through `/_shell/image/<name>`, a redirect to the stored file,
   signed each time. For a public profile, the picture becomes the card:
   `twitter:card` `summary_large_image`, its `og:image:alt` the handle.
2. **Drawn once, kept in storage**, under `cards/<uuid>/<key>.png`, the key
   made of what is on it: the profile photo's file, the cover's and the
   handle. Asked for and not there, it is drawn, with `golang.org/x/image`,
   and added through file-grpc-service; after that it is only signed.
   A new photo, cover or handle is a new key, so a new address, and link
   previews fetch it again, as profile photos are since row 163.
3. **The one before is deleted** when a new one is drawn, so a profile
   keeps one card.
4. **For profiles anybody may see**, as web-shell describes today: a person,
   a group, an event, a channel. Nothing on a site for members only; a
   page that is not a profile keeps the picture it has.
5. **The font** is a Latin one under the SIL Open Font License, built into
   web-shell's image, a few hundred kilobytes.

## Not in it

Cards for posts, which keep their own pictures; a card for a profile
elsewhere, which its own server describes; and any choice of layout for a
site. The colours are the site's, the layout is the same everywhere.

## Checking it

Tests of the key (a new photo, cover or handle is a new card), of a card
drawn once and then only signed, and of what is never given one. On the
dev stack, then on banu.io: how Mastodon, Bluesky, Signal and iMessage
show a profile's link, and the cards looked at at their real size.

## Status

1. Planned 2026-11-23.
