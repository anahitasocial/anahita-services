# Permissions

[Documentation](README.md) › Permissions

Who can post, comment, like and add followers on a profile, and who can edit or
delete a comment. It covers the defaults an installation ships with, how to
change them, and what profile owners can change for themselves.

## Contents

- [Two separate settings](#two-separate-settings)
- [Access levels](#access-levels)
- [Who is always allowed](#who-is-always-allowed)
- [Posting](#posting)
- [Comments](#comments)
- [Likes](#likes)
- [Adding followers to a group](#adding-followers-to-a-group)
- [Reposts](#reposts)
- [The defaults](#the-defaults)
- [What profile owners can change](#what-profile-owners-can-change)
- [Accepted values](#accepted-values)

## Two separate settings

Every profile, whether a person or a group, has two settings that are easy to
confuse:

| Setting | Question it answers | Where it is set |
| --- | --- | --- |
| **Access** | Who can *see* this profile and its posts | Settings › Access |
| **Permissions** | Who can *add* to it: posts, comments, likes, followers | Settings › Permissions |

Permissions never widen access. Anybody refused by the profile's access is
refused everything below, whatever the permissions say.

## Access levels

A permission is one of these levels. Each one is measured against the profile
the post is on, not against the person who wrote the post.

| Level | Admits | Shown as |
| --- | --- | --- |
| `registered` | Anybody signed in | Anyone signed in |
| `followers` | People who follow the profile | Followers |
| `leaders` | People the profile follows (people only) | Leaders |
| `mutuals` | People who follow the profile and are followed back (people only) | Mutuals |
| `admins` | The profile's administrators. For a person, that is the person | Admins / Only me |

A profile's administrators meet every level. A group follows nobody, so
`leaders` and `mutuals` are only offered for people.

## Who is always allowed

These people are allowed whatever a profile has set:

- **Site administrators and super administrators**
- **The profile's owner**: the person, or for a group its administrators

The one exception is editing somebody else's comment, which nobody but its
author can do (see [Comments](#comments)).

## Posting

Who can create a note, article, topic or photo on somebody's profile.

- The owner and the profile's administrators always can.
- Everybody else has to be able to see the profile and meet the level set for
  that kind of post.
- `registered` is not accepted for posts. Nobody can open their profile to
  posts from anybody who happens to be signed in.

Default: `followers`.

## Comments

### Adding a comment

Everybody who is not always allowed has to be able to **read the post** first.
After that, it depends on the profile's access:

| Profile access | Who can comment |
| --- | --- |
| `public` or `registered` | The profile's comment setting: `registered` or `followers` |
| `followers` | Followers only, whatever the setting says |
| `leaders`, `mutuals`, `admins`, `myself` | Anybody who can see the profile |

The comment setting only matters on a public or registered profile, because a
narrower profile already limits who can see its posts. A followers-only profile
is spelled out as followers only, even though people the profile follows can
also read it.

These people can always comment, on a post they can see:

- the profile's owner and administrators
- the post's author
- site administrators and super administrators

Default: `registered`.

### Editing a comment

**Only the comment's author**, and only while they can still read the post.

Site administrators, super administrators and the profile's owner cannot edit
somebody else's comment. An edited comment still shows its author's name, so an
edit by anybody else would put words under that name with no record of who
wrote them. Moderators remove comments; they do not rewrite them.

### Deleting a comment

- the comment's author, even after they have lost sight of the post
- the profile the post is on: the person, or for a group its administrators
- site administrators and super administrators

The author of the *post* cannot delete other people's comments under it unless
the profile is theirs. In a group, that is the group's administrators.

## Likes

Who can like a post or a comment on a profile. The viewer has to be able to
read the post, then meet the level set for likes.

Default: `followers`.

## Adding followers to a group

Who can add somebody else to a group as a follower: `followers` or `admins`.
Site administrators always can.

Following a group yourself is not governed by this setting. Anybody who can
see the group can follow it, as with a person. This setting has no effect on
people, because nobody adds followers to a person.

Default: `admins`.

## Reposts

Reposts have no setting under Permissions and cannot be restricted per
profile.

## The defaults

The defaults live in `graph-grpc-defaults/actor_features/`:

| File | Applies to |
| --- | --- |
| `default.json` | Every profile, as the base |
| `person.json` | People, layered over `default.json` |
| `group.json` | Groups, layered over `default.json` |

Each service lists its `add_permissions` as an entity and a level:

```json
{
    "service": "text-service",
    "add_permissions": [
        { "entity": "note", "access": "followers" },
        { "entity": "article", "access": "followers" }
    ],
    "include_common_permissions": ["content_creator"]
}
```

Comments and likes are shared by every service that has posts, so they are set
once in `default.json` under `common_permissions` and pulled in with
`include_common_permissions`:

```json
"common_permissions": {
    "content_creator": [
        { "entity": "comment", "access": "registered" },
        { "entity": "like", "access": "followers" }
    ]
}
```

### Changing them

Edit the files and the change takes effect on the next request. graph-grpc
reads the directory named by `ACTOR_FEATURES_PATH` every time it reads a
profile, so no restart is needed.

`docker-compose.yml` mounts `graph-grpc-defaults/actor_features` at
`/defaults/actor_features`, which is where `ACTOR_FEATURES_PATH` points by
default. The graph-grpc image also carries a copy made at build time, at
`/app/defaults/actor_features`. An installation that runs the image without
the mount has to set `ACTOR_FEATURES_PATH` to that path, and rebuild the image
to pick up a change.

Only use the values listed under [Accepted values](#accepted-values). A value
the permission checks do not recognise refuses everybody except the owner, the
profile's administrators and site administrators, and nothing reports it.

## What profile owners can change

A profile's owner and its administrators can change the levels for their own
profile under **Settings › Permissions**. Site administrators can too, for any
profile.

- A profile that has never saved this tab follows the defaults, including any
  later change to them.
- Saving stores the profile's **whole** feature list, not only the values that
  changed. From then on, later changes to the defaults do not reach that
  profile.
- **Restore defaults** clears the profile's saved settings, and the defaults
  apply again.

On a profile whose access is narrower than `registered`, the comment row
explains who can comment instead of offering a choice. See
[Adding a comment](#adding-a-comment).

## Accepted values

The server refuses to save anything else. These are the values the permission
checks enforce.

| Entity | People | Groups |
| --- | --- | --- |
| Posts (`note`, `article`, `topic`, `photo`) | `followers`, `leaders`, `mutuals`, `admins` | `followers`, `admins` |
| `comment` | `registered`, `followers` | `registered`, `followers` |
| `like` | `registered`, `followers`, `leaders`, `mutuals`, `admins` | `registered`, `followers`, `admins` |
| `follower` | `admins` (not enforced) | `followers`, `admins` |
