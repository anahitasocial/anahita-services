# Threads that read as threads (roadmap row 126)

Asked for by the user on 2026-11-07, of a thread they had posted on banu.io:
"there is no way for people to understand that this is a thread unless they
click on it. We also need char counters in our composer and only make the
+ Add to thread enabled once we reach a char limit (mastodon or bsky
whichever comes first). Also, do we have the option of just copy pasting a
long text and automatically breaking it into threads instead of the author
having to break their text into smaller cuts?" Not built. Planned for
later the same day, at the user's word, after row 125.

## Contents

1. [Settled](#settled)
2. [What there is now](#what-there-is-now)
3. [Part 1: a thread in a list](#part-1-a-thread-in-a-list)
4. [Part 2: a thread on its page](#part-2-a-thread-on-its-page)
5. [Part 3: the count, and when a note is full](#part-3-the-count-and-when-a-note-is-full)
6. [Part 4: long words split into a thread](#part-4-long-words-split-into-a-thread)
7. [Out of scope](#out-of-scope)
8. [Phases](#phases)

## Settled

With the user on 2026-11-07:

1. **In a list, the first note and the next,** joined by a line, as Bluesky
   and Threads show a thread, then one quiet link to the whole of it.
2. **On the post's page, the writer's notes flat,** one under the other at
   the same width, and other people's replies after them.
3. **The limit follows where the post goes:** Bluesky's 300 when it goes to
   Bluesky; Mastodon's 500 when it goes to the fediverse and not Bluesky;
   when it goes to neither, a note's own 8,000. "Why enforce that limit? Let
   them enjoy the extra room." A post goes to the fediverse or Bluesky only
   when it is a public note on the writer's own profile and they have turned
   that network on ([Federation](../federation.md)).
4. **"Add to thread" is offered once a note is full,** at that limit.
5. **Long words are split when the writer asks,** with one tap, never on
   their own: at paragraphs, then sentences, then words. The notes can be
   changed before Post. Still no numbering (row 101), and still up to ten
   notes.

## What there is now

1. **A thread is a chain of replies:** the first note, then each of the
   writer's notes answering the one before, written in one go (row 101,
   [thread-composer.md](done/thread-composer.md)) or one at a time later.
2. **In a list, only the first note shows,** with nothing to say more
   follows.
3. **On its page, the chain is drawn as any replies are,** each a level
   further in (anahita-web `containers/replies/Thread.jsx`,
   `utils/thread.js`), mixed with other people's replies.
4. **The composer counts nothing.** "Add to thread" is offered as soon as
   there are words (`Composer/Short.jsx`, `Composer/ThreadParts.jsx`). Past
   300, when the post goes to Bluesky, a line says Bluesky shows the first
   300 with a link to the rest.
5. **A post made public later goes out then.** A note written for fewer
   than everybody and made public afterwards is sent to the fediverse and
   Bluesky at that change, as a new post; made private again, both are told
   to take it back (activitypub-service `tellOf`, atproto-service
   `localPosted`). Checked 2026-11-07. Such a note may be longer than the
   network shows: it goes with a link to the rest here, as now.

## Part 1: a thread in a list

1. **The services say a note opens a thread:** a note in a list carries
   `thread`, the number of notes in its writer's chain, the first counted,
   and `thread_next`, the second note, shaped as any note is. Nothing for a
   chain of one.
2. **Counted on each read, not kept** (changed while building it,
   2026-11-07): one query for a page follows each note's chain down
   `parent_id`, and one more reads the second notes. Kept on the first note,
   every write that adds, deletes, hides or purges a reply, in several
   services, would have had to keep it in step, and a migration to begin
   it; counted, it cannot be wrong. The chain is the writer's earliest
   reply to each note, as the page draws it; a later reply of theirs to the
   same note is a reply like anybody's.
3. **Only the writer's chain:** a note by somebody else, or a reply by the
   writer to somebody else's reply, ends nothing and adds nothing.
4. **The card** (anahita-web `containers/feed/components/Default.jsx`):
   the first note as now, then the next, its photo beside it joined by a
   line to the first's, its words cut to a few lines; then "Show the thread"
   with the number of notes, leading to the post's page. The actions are the
   first note's.
5. **Notes from elsewhere** are left as they are for now: their chains are
   not all here.

## Part 2: a thread on its page

1. **The writer's chain first, flat:** each note at the same width, joined
   to the one before by a line, with its own actions; `utils/thread.js`
   picks it out of the thread it already reads whole.
2. **Then everybody else's replies,** as now, nested, each under the note it
   answers, with the note named when it is not the first.
3. **A reply's own page** (`ReplyContext.jsx`) is unchanged.

## Part 3: the count, and when a note is full

1. **The limit**, as [settled](#settled): 300, 500 or 8,000, from where the
   post goes as the composer knows it (`goesToBluesky`, and
   `useOnFediverse` for the fediverse). It changes as the audience is
   changed. In the reply form, where the reply goes.
2. **The count** is quiet: under each note's words, at the end of the line
   that is already there, "212 / 300", in the caption's grey. Its room is
   kept, so nothing moves as it appears. Counted as the composer counts now,
   letters and not bytes, Markdown's marks included.
3. **Past the limit** a note may still go on, up to 8,000: the line says
   what the network shows, as Bluesky's line does now, with Mastodon's
   added: Mastodon shows a longer note from elsewhere whole, so for the
   fediverse the 500 is what its people write, not a cut.
4. **"Add to thread"** is offered once the last note has reached the limit;
   before that it is shown, not offered, and the count says how far there is
   to go.
5. **The numbers** in `constants/limits.js`: `BLUESKY_POST_MAX`, and a new
   `FEDIVERSE_POST_MAX`, 500, with the services' `constants` to match.

## Part 4: long words split into a thread

1. **Offered past the limit:** in the line under the words, "Split into a
   thread", once the words are longer than the limit and would make ten
   notes or fewer. Longer than that, the line says so, and that an article
   takes it whole.
2. **Where it breaks:** at the last paragraph that fits, then the last
   sentence, then the last space; never inside a link, a hashtag, a mention
   or a Markdown mark. A word longer than a note is cut.
3. **What it makes:** the first note keeps the post's audience, locations
   and card, as row 101 does; the rest become the notes after it, each a
   box that can be changed, joined back with ✕, or split again.
4. **Undo:** one step, back to the words as they were pasted.
5. **In the browser only:** the server is sent a thread, as now.

## Out of scope

1. Numbering, here or on other networks (row 101).
2. Photos in a thread's notes (row 101, for later).
3. A chain from another server shown as a thread.

## Phases

1. **The services:** `thread` and `thread_next`, kept on the first note,
   the migration, tested against a database. One session.
2. **The web:** the card and the post's page. One session.
3. **The composer:** the count, "Add to thread" at the limit, and the split,
   in the composer and the reply form, tested in headless Chrome at both
   sizes. One session.
