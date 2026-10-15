# Replies of any kind: a reply with pictures, a GIF or a video

*Roadmap item 53, raised by the user on 2026-10-14. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context

A reply here can only be words. The reply box makes a note, and nothing else can be posted into a thread, so nobody can answer with a picture or a GIF, which on Mastodon and Bluesky is ordinary. There a post is one thing that may carry words, pictures or a video, and a reply is a post like any other.

Anahita keeps its kinds of post apart (a note, a photo post, a video post, an article), each with its own service. Merging them into one kind was considered and set aside: it would touch nearly everything, and an article or an event does not fit "a post with attachments".

**The decision (user, 2026-10-14): nodes are nodes, and any media node may have a parent.** A reply with pictures is a photo post in the thread; a reply with a GIF or a video is a video post in the thread. The reply box stays simple: what a reply is open to, who may answer it and its language all follow the thread, as they do now.

## What exists

The core already allows it. Looked at on 2026-10-14:

- **The graph files any new post under its parent.** Given a parent, it places the post in the thread whatever the post's own kind (`graph-grpc-service/cmd/server/handlers/medium_thread.go`), taking the thread's root and leaving the post's own audience empty.
- **Threads, reply counts, the thread's audience and notifications go by the thread,** never by the kind: `root_id` in `data/repositories/replies.go` and `access_medium.go`; `RootID > 0` in notification-service.
- **Photo replies exist already.** A reply from Mastodon or Bluesky that carries pictures is kept as a photo post in its thread (`activitypub-service/cmd/api/handlers/inbox.go`, `atproto-service/cmd/api/handlers/posts.go`).
- **And their pictures are not shown.** The web app's reply list draws a reply's words and link card only (`src/containers/replies/ReplyItem.jsx`), and the thread as sent carries no pictures. A reply from Mastodon that is only a picture shows as an empty row. This is a bug today.

Where a reply is still a note only:

1. text-service's `POST /notes/:id/replies` is the only way to post one, and makes a note.
2. photo-service and video-service take no parent.
3. photo-service and video-service judge reading, changing and deleting by the post's own audience. A reply has none of its own (its thread's applies), so a photo reply opened at `/photos/:id` would be refused to all but its author. text-service already has the thread's rule.
4. The thread as sent has no pictures or video.
5. The web app sends every reply's change and deletion to `/notes/:id`, and draws only words.

## Design

**Posting.** photo-service's and video-service's ways of posting take an optional `parent_id`, passed to the graph, which does the rest. With a parent:
- what it is open to, who may answer it and its quote rule are not taken from the request: the thread decides, as for a note reply;
- its language may be given, and otherwise is the parent's;
- it is posted on its author's own profile, as a note reply is;
- the parent must be one the author may answer (`permissions.CanReply`, the rule text-service uses), checked before anything is stored. For a video, at the beginning of the upload, and again by the worker when it makes the post, since the thread may have closed meanwhile.

**The thread's rule in one place.** text-service's rule for reading, changing and deleting a reply (go by its thread; its author and the thread's owner may delete it) moves into the shared permissions in `anahita-libs`, and photo-service and video-service use it. One rule, one place.

**Threads carry their media.** A thread as sent gives each photo reply its pictures (the photo files, with alt text) and each video reply its video, as a post sent on its own does. Photo files are read for all a thread's photo replies at once.

**The reply box (web).** Beside the words, two buttons: a picture and a video or GIF.
- Pictures (up to four, with alt text, as in the photo composer) make the reply a photo post. Words are optional with pictures.
- A GIF or a video (one, the video composer's limits) makes the reply a video post. It is sent in parts as a video is, and shows in the thread as "preparing" until the clip is ready, when it takes its place.
- The box offers no audience, reply or quote setting: the thread has them. Language follows the parent, as now.
- Pictures and a video are not both on one reply.

**A reply in the thread (web).** `ReplyItem` draws a photo reply's pictures (the slides of a photo post, smaller) and a video reply's video (the looping clip for a GIF), under its words. Changing and deleting a reply goes to the service of its kind: words and alt text for a photo reply, words for a video reply.

**Between networks.** Nothing new is needed to send them: a photo or video reply goes to Mastodon as a note answering its parent, with its pictures or video, and to Bluesky as a reply with the embed, by the rules that already decide whether a reply goes. A video reply from Mastodon or Bluesky is not brought in yet, as no video is.

## Estimate

| Part | Time |
| --- | --- |
| The thread's rule into the shared permissions; photo-service and video-service take a parent and use it | 2–3 h |
| Threads carry pictures and video | 1–2 h |
| Reply box: pictures and a video or GIF; the thread drawing them; changing and deleting by kind | 4–5 h |
| Tests, and trying it against Mastodon and Bluesky | 2 h |
| Docs (`permissions.md`, `photos.md`, `videos.md`, `federation.md`, web `architecture.md`) | 1 h |
| **Total** | **about 10–13 h, 2–3 sessions** |

## Verification

- A reply with two pictures and no words shows them in the thread, on its own page, and in its author's replies; it is open to whom the thread is open to and to nobody else.
- A reply with a GIF shows "preparing", then loops in the thread.
- Somebody who may not answer a thread cannot post a picture or video reply to it, and a thread closed while a video reply was being prepared does not get it.
- A picture reply from Mastodon, which shows as an empty row today, shows its picture.
- A picture reply and a GIF reply reach Mastodon and Bluesky as replies with their media.

## Dependencies

- Notes as replies (item 7), photo posts with several pictures (item 17), the video service (item 16) and animated images (item 41): all done.

## Later: articles and topics as responses

An article or a topic can be a reply as well; nothing in the graph or the rules for threads stands in the way, only a way to post one (text-service's article and topic routes take no parent, and the reply box offers neither). Recorded with the user on 2026-10-14 for later: **a response to a post**, as on Medium. Somebody writes an article, or opens a topic, about a post somebody else made, from something like "Write a response" under it. It is in the post's thread like any reply, its own title and length kept, and it goes out as a reply too: to Mastodon as an `Article` answering the post, shown there with its title and a link; to Bluesky as a reply with the start of its text and a link.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-10-14 |
| Implementation | done 2026-10-14: photo and video replies in services and the web app, tried by the user against Mastodon and Bluesky the same day and working |
