# Articles and topics

Articles and topics are long writing: up to 8,000 characters, where a note
holds 500. Each is a post of its own, with a title, replies, and a place on
the profile it was written on. Use an article for writing a reader will come
back to, and a topic for a subject that others add to.

## Contents

1. [Length](#length)
2. [Articles](#articles)
3. [Topics](#topics)
4. [Drafts](#drafts)
5. [Where they show](#where-they-show)

## Length

1. **Notes, replies and the words of a photo, video or sound post:** 500
   characters. A link counts as 23, however long its address.
2. **Articles and topics:** 8,000 characters.

The limit is checked when a post is saved, and a longer one is refused
rather than cut.

## Articles

An article has a title and a body in Markdown, and can have a cover picture.
The title and the cover are what a reader sees first, in a feed and on the
profile, and the opening of the body is what a feed shows beneath them.

## Topics

A topic is long writing in the same way, with the same limit. It can be
replied to and pinned like an article.

## Drafts

An article or a topic can be kept as a draft and published when it is ready.
Nothing is said to anybody until it is published: no notification, and no
place in anybody's feed.

## Where they show

1. **On the profile** they were written on, in the Posts tab, beside notes.
2. **In feeds** that include them, with their title and opening.
3. **On their own page,** in full, with replies beneath.

## Where the code is

| What | Where |
| --- | --- |
| The limit for articles and topics | `anahita-libs/requests/media.go` (`Content`, 8,000) |
| The limit for notes | `anahita-libs/posts/length.go` (`MaxLength`, 500) |
| Creating an article or a topic | `text-service/cmd/api/handlers/article.go`, `topic.go` |
