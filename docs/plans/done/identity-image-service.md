# One identity image service: avatars and covers merged

*Roadmap item 31, raised by the user on 2026-10-05 and done the same day. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md).*

## Context

Avatars were served by `avatar-service` and covers by `cover-service`. After people and groups were merged ([actor-service.md](actor-service.md)) the user asked whether these two should be merged as well, or avatars moved into actor-service, and what the right names are.

Measured: each had two routes, set and remove by node id; about 650 and 730 lines; the same steps in the same order. They differed in which permission they asked about and which column they wrote.

## Decisions

- **Merge the two into one service of their own,** not into actor-service. Covers belong to posts as well as to actors, so half would not fit there. And opening pictures people upload is the heaviest and riskiest thing done with them: kept apart, a hostile or huge file cannot slow the service every profile is read from.
- **The name is `identity-image-service`,** chosen by the user. An avatar says who something is; it is not decoration, which ruled out "artwork". "Identity media" was the user's first form of it; "image" was preferred because "media" already means posts in this codebase. "Image service" alone was too general, and "branding" too commercial.
- **Addresses do not change:** `/avatars/` and `/covers/`, with the same form fields. The web app is untouched.
- **Stored addresses do not change:** `assets/v2/avatar-service/…` and `assets/v2/cover-service/…` are where every avatar and cover already is.
- **"Avatar" stays as the word.** It is what Mastodon and Bluesky call the field.
- **`nodes.filename` keeps meaning "the file this node is",** for a photo and for a document if file sharing is built. The avatar is to move out of it to `avatar_filename`, and "portrait" is to be retired, with item 27 (one clean schema). Not now.

## What was built (2026-10-05)

- **`identity-image-service`**, a module named `identityimage`, made from cover-service. `avatar-service` and `cover-service` are gone.
- **One handler for both kinds.** What differs between an avatar and a cover is a small table (`kinds.go`): its name, its form field, which nodes may have one, which column it is. Setting and removing are written once.
- **One check for every uploaded image,** `images.Accept` in `anahita-libs/utils`, now used here and by photo-service: judged from the bytes, limited in bytes and in pixels before it is opened, turned upright.
- **A way to take an image off a node:** `ClearFile` on graph-grpc's node service.

**Found and fixed on the way.**
- **Removing an avatar or a cover did not remove it.** The stored picture was deleted, and the profile was then saved with an empty name, which an edit ignores: it leaves an image alone unless it is given a new one. So the profile went on naming a picture that was gone. Removal now clears the name through `ClearFile`, and clears it before the picture is deleted, so a failure between the two cannot leave that state.
- **Replacing one left the old one in storage for good.** The new name overwrote the old and nothing deleted the old picture. It is now deleted once the node names the new one.
- **Squares were squeezed.** An avatar's `square` size was the whole picture pressed into 160 by 160. It is now cut from the middle, as for photos. Small pictures are no longer enlarged to fill a large size.
- **A file over 2 MB (avatar) or 4 MB (cover) was refused,** which is most pictures straight off a phone. The limit was on the file as it arrived, though only reduced copies are stored. It is now 10 MB with a limit on pixels, as for photos.
- **A node that does not exist answered 500.** It answers 404.
- **Refusals now say why,** with the codes the photo service uses: `file_required`, `file_unsupported`, `file_too_large`, `file_unreadable`.

**Tests.** The service: every size stored and where; the square cut and not squeezed; nothing enlarged; replacing deletes the old picture, and only after the save; a failed save leaves nothing stored and keeps the old picture; what is not an image is refused and nothing stored; somebody else is refused; an avatar on a post is refused and a cover on one is not; removal clears the name first; if the name cannot be cleared the picture is kept; a missing node is 404. `ClearFile` against real MySQL, including that it never takes the photo off a photo post. `images.Accept`.

**Checked on the dev stack.** The service starts, the four routes answer 401 without a token through the gateway, and profiles still show their avatars.

**Not checked.** Setting or removing an avatar or a cover with a signed-in session, and nothing in a browser. The Kubernetes manifests were edited and not applied.

**To do when deploying.** Build and push `anahita/identity-image-service`; delete the `avatar-service` and `cover-service` Deployments and Services from a cluster that has them. graph-grpc has to be deployed with or before it, for `ClearFile`.

**Left for later.**
- Avatars and covers set through the Go services before this have squeezed squares. They are put right when the person sets a new one.
- Pictures orphaned by earlier replacements are still in storage. Nothing names them; finding them means listing storage against the names in use.
- Retiring "portrait", with item 27.

## Added the same day: `avatar_filename`

The user asked for the rename now and not with item 27, since the services had just been merged.

- **Migration `20261009000001_avatar_filename.sql`**, with `init.sql` and `legacy/upgrade.sql`: `nodes.avatar_filename`, placed after `filename`; every actor's `filename` moved into it and cleared. Nothing moves in storage.
- **`AvatarFilename`** on the node entity and in the protobuf, read and written by graph-grpc for the node itself and for the owner, author and editor read alongside a post. `Filename` on an actor is now always empty.
- **Every reader followed:** the avatar helpers in `anahita-libs`, identity-image-service, auth-service's account queries and its userinfo, and the list services that decide whether there is an avatar to presign (search, hashtags, likes, locations, abuse reports).
- **Tombstoning** an account clears the new column as well.

**Tests.** The migration was run on a throwaway MySQL: up, up again (nothing changes), and down (values back, column gone); the upgraded schema was compared with a fresh one, and the legacy path was run. graph-grpc and auth-service against real MySQL, including a new test that an avatar is stored in its own column, leaves `filename` empty, and is what a post's owner is read with.

**Deploying.** The migration and every service have to go out together: a service from before this reads avatars from `filename`, which the migration empties, and one from after reads a column that an unmigrated database does not have.

## Status

| Item | State |
| --- | --- |
| Plan and work | done 2026-10-05, on branch `identity-image-service` |
