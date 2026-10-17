# Photo posts

[Documentation](README.md) › Photo posts

A photo post holds up to four images. This page covers how they are uploaded,
stored, changed and removed.

Its routes are in the API reference: [photo-service](api/index.html#photo-service).

## Contents

- [One post, several images](#one-post-several-images)
- [Posting](#posting)
- [What is stored](#what-is-stored)
- [Reading](#reading)
- [Changing a post's images](#changing-a-posts-images)
- [Downloading](#downloading)
- [Removal](#removal)
- [Limits](#limits)
- [What is not built](#what-is-not-built)

## One post, several images

A post with several images is one post: one caption, one audience, one set of
likes and comments. It is not an album of separate posts. That is also how
Mastodon and Bluesky send them, as one post with attachments.

Four is the limit because it is what a stock Mastodon server lets its users
attach, so a post of four is shown whole there. Bluesky allows more. Whether
to raise it is left until what other servers do with a longer post has been
seen.
It is a constant (`constants.PhotoMaxFiles`), not a setting, and is published
in NodeInfo as `metadata.photoMaxFiles` so that a client does not have to
carry the number.

Each image can have a description of what it shows, up to 2,000 characters,
for people who cannot see it. One that comes with a post from another server
is kept up to 10,000, Mastodon's own limit.

## Posting

In two steps, so that no request carries more than one image however many the
post has. A **reply with pictures** is posted the same way, its images
uploaded for the person replying (their own id as `owner_id`), then
`POST /photos/:id/replies` with `{ body, language, uploads }`, where `:id` is
what it answers. Words are optional. It is a photo post in the thread: owned
by its author, with what it is open to, who may answer it and quoting all the
thread's; its language, unless given, is that of what it answers.

**1. Upload each image.** `POST /photos/:owner_id/uploads`, a form with the
image in `file`. `owner_id` is the profile the post will be on. Whoever may
not post there may not upload there. The answer:

```json
{ "upload_id": 31, "width": 2048, "height": 1365, "urls": { "small": { "url": "…" }, "…": {} } }
```

**2. Make the post from the uploads.** `POST /photos/:owner_id/` as JSON:

```json
{
  "name": "Herons",
  "body": "At the lagoon this morning.",
  "access": "followers",
  "language": "en",
  "uploads": [
    { "upload_id": 31, "alt_text": "A heron standing in shallow water" },
    { "upload_id": 30 }
  ]
}
```

The order of `uploads` is the order of the images. The first is the cover.

An upload can be used only by whoever uploaded it, and only on the profile it
was uploaded for: that profile is part of where the image is stored.

**One image in one request still works.** `POST /photos/:owner_id/` as a form
with the image in `file` (and optionally `alt_text`) makes a post with that
one image, as it did before a post could hold more.

When the images are refused the answer says why, and how many a post holds:

| Status | `error` | |
| --- | --- | --- |
| 422 | `files_required` | No image |
| 422 | `too_many_files` | More than four |
| 422 | `file_repeated` | The same image twice |
| 422 | `alt_text_too_long` | A description over 2,000 characters |
| 404 | `file_not_found` | An upload that is gone, or an image the post does not have |
| 403 | `upload_not_yours` | Somebody else's upload, or one made for another profile |
| 415 | `file_unsupported` | Not a JPEG or a PNG |
| 413 | `file_too_large` | Over 10 MB, or over 50 megapixels |
| 400 | `file_unreadable` | It could not be read as an image |

**An upload nobody uses is removed after a day,** with everything stored for
it, by the sweeper (`graph-grpc-service/sweeper/uploads.go`).

## What is stored

What arrives is never stored as it is. Each image is turned the right way up
and written out again at each size, so nothing a file carries besides its
picture reaches storage. What a file is, is decided from its bytes, not its
name.

Each size is measured by the picture's longest side, its height when it
stands up and its width when it lies down, as Bluesky and Pixelfed measure
them:

| Size | |
| --- | --- |
| `original` | The picture at its own size, up to 2,048 pixels on its longest side |
| `xxlarge`, `xlarge`, `large`, `medium`, `small` | 2,048, 1,920, 1,440, 1,080 and 640 pixels on the longest side |
| `square` | The middle of the picture, 640 pixels square |

An image is never enlarged: one smaller than a size is stored at its own
size under that size's name. A JPEG is written at quality 85. A PNG with
nothing see-through in it, a screenshot as often as not, is kept as a JPEG;
one that is see-through somewhere stays a PNG. Pictures copied from posts
on other servers are kept the same way. Photos kept before 2026-10-16 were
measured by their width, up to 2,560, and their files are as they were.

Images are stored under the profile the post is on, not under the post:
`assets/v1/photo-service/photo/<owner id>/<name>_<size>.<ext>`. That is what
lets an image be stored before its post exists.

Two tables, both owned by `graph-grpc-service`:

- **`photo_files`**: the images of each post, in order, with their
  descriptions and dimensions.
- **`photo_uploads`**: images uploaded and not in a post yet.

`nodes.filename` and `nodes.mimetype` stay, as a copy of the first image, so
everything that shows one image for a photo (search, notifications, hashtag
and location pages) goes on working without knowing about the list.

## Reading

A photo post carries its images as `files`, in order:

```json
"files": [
  { "id": "9f2c….jpg", "position": 0, "alt_text": "A heron standing in shallow water",
    "width": 2048, "height": 1365, "urls": { "medium": { "url": "…" }, "…": {} } }
]
```

`portrait_urls` is still there and is the first image's sizes.

`files` is sent by `photo-service` (a photo and a profile's photo list) and by
`feed-service` (feeds, including a photo that was reposted). The other
services that list posts send `portrait_urls` only, so a post found through
search shows its first image until it is opened.

A photo posted before this existed has one entry, with no description and no
dimensions.

**In preview mode** (`SITE_READ_ACCESS=preview`) a visitor is sent the first
image only, at its smallest size, and `files_held_back` says how many more
there are. See [Privacy](privacy.md).

## Changing a post's images

`PATCH /photos/:id/files`, by whoever may edit the post. The request is the
whole list as it is to be:

```json
{ "files": [
  { "id": "b41e….jpg", "alt_text": "Now first" },
  { "upload_id": 44 },
  { "id": "9f2c….jpg" }
] }
```

An image the post has is named by its `id`; a new one by its `upload_id`. One
left out is removed, and its stored copies are deleted. A description left out
is cleared. The rules and refusals are the same as when posting.

One list, and not separate requests to reorder, add and remove, so there is
one rule to explain and no order of changes to get wrong.

## Downloading

Anybody who can see a photo can save what their browser was shown. The
download is something else: a proper file, at the largest size kept, with a
sensible name, **where whoever posted it has said so**.

- **The switch.** `allow_download` on a photo post, off unless turned on.
  Sent when the post is made, and changed with
  `PATCH /photos/:id/download` `{ "allow": true }` by whoever may edit the
  post: its author, and the admins of the group it is on. Every photo
  posted before this existed is off: nobody is taken to have agreed.
- **What a new post starts with**, when its author does not say, is what
  the profile it is posted on says: a person's own choice for their
  profile, a group's for what is posted in the group
  (`PATCH /people/:id/download-default`, `PATCH /groups/:id/download-default`).
  That is where a post starts and no more. Changing it changes nothing
  already posted.
- **The download.** `GET /photos/:id/download?file=<position>` answers with
  a redirect to a signed address that makes a browser save the file. The
  first image when no position is given, and one image at a time. The file
  is the largest copy kept, which was encoded here from what was uploaded;
  it is named for the post's title, or `photo-<id>`.
- **Who may.** Somebody who can see the post, when the switch is on; and
  its author always, whatever the switch says. Somebody who cannot see the
  post is told it is not there (`404`), as for the post itself; somebody
  who can and may not download is told so (`403`). A visitor on a site in
  `preview` mode downloads nothing. 60 a minute for each person.
- **What a client is told.** A post that holds a file says
  `authorized.download` for the viewer, wherever it is sent; the post's
  own page also says `allow_download`, and a profile says its default.

Nothing is counted and nobody is told who downloaded what.

Videos work the same way, with `/videos/` in place of `/photos/`; see
[Videos](videos.md#downloading).

## Removal

- **Deleting a post** deletes every stored size of every one of its images.
- **Purging an account** does the same for every photo the account owned or
  posted. See [Account lifecycle](account-lifecycle.md).
- **Unused uploads** go after a day, as above.

## Limits

| | |
| --- | --- |
| Images in a post | 4 |
| One uploaded file | 10 MB, which is what the gateway lets one request carry (`client_max_body_size` in `nginx.conf`). The two services that take images, photo-service and identity-image-service, allow a request of 11M for it (`constants.ImageRequestBodyMaxSizeLimit`); until 2026-10-10 they used the general 2M, which refused anything larger before this limit was reached |
| Pixels in one image | 50 million |
| Formats | JPEG, PNG. An animated PNG is kept still, as its first picture; a GIF or an animated PNG that is to move is posted as a video, which loops (see [Videos](videos.md#gifs-and-animated-pngs)) |
| A description | 2,000 characters written here (what Bluesky's own app takes); 10,000 kept from another server (Mastodon's) |

Bluesky takes images of up to 2 MB (1 MB until 2025). A picture sent there is
made small enough before it goes. Every length people write is listed in
[The API](api.md#lengths).

## What is not built

- **Drag to reorder** in the web app. Images are moved with arrow buttons,
  which also work from the keyboard.
- **Images in the lists of search, hashtags, locations and likes.** They show
  the first image.
- **Albums, and collections of other people's posts.** Decided against; see
  [the plan](plans/done/photo-albums.md).
- **Photos and video in one post.** For the video service.
