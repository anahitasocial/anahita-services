# Permissions

[Documentation](README.md) › Permissions

Who can post, comment, like and add followers on a profile, and who can edit or
delete a comment. It covers the defaults an installation ships with, how to
change them, and what profile owners can change for themselves.

## Contents

- [Two separate settings](#two-separate-settings)
- [Access levels](#access-levels)
- [Who is always allowed](#who-is-always-allowed)
- [Seeing posts](#seeing-posts)
  - [Choosing who can see a post](#choosing-who-can-see-a-post)
- [Posting](#posting)
- [Comments](#comments)
- [Likes](#likes)
- [Adding followers to a group](#adding-followers-to-a-group)
- [Reposts](#reposts)
- [Reporting](#reporting)
- [Removing accounts](#removing-accounts)
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

## Seeing posts

A post has an access level of its own, as well as the one on the profile it is
on. Somebody sees a post when both admit them:

1. the **profile's** access admits them, and
2. the **post's** access admits them.

Both are measured against the profile the post is on. A followers-only post in
a group is for the group's followers, whoever wrote it.

Two people see a post without its own access being consulted:

- **The person whose profile it is on** sees everything on it.
- **The author** sees their own post for as long as they can see the profile
  it is on.

Nobody sees a post by, or on the profile of, somebody they have blocked or
been blocked by.

A post's access is always one of the levels above. There is no way to share a
post with named people only, and no private messaging: the private notes of
the PHP version were removed, not carried over.

The same rule applies on a profile, in the home feed, and under a hashtag or a
location. Search applies both access checks but not the two exceptions above,
so searching may not find your own restricted posts.

What depends on another post is judged by that post:

| Item | Seen by |
| --- | --- |
| A comment | Whoever can see the post it is under |
| A repost | Whoever can see the original post, and the reposter's profile |

So following somebody who reposts a followers-only post does not show it to
you. You see it only if you could see the original.

### Choosing who can see a post

The author chooses a post's audience when writing it, and whoever may edit the
post can change it afterwards. It is sent as `access` when the post is created
(`POST /notes/:owner_id/` and the same for articles, topics and photos) and
changed with `PATCH /…/:id/access`. Left out, a new post is `public`.

Which audiences a post may have depends on where it is:

| Where the post is | Audiences |
| --- | --- |
| Your own profile | `public`, `registered`, `followers`, `leaders`, `mutuals`, `myself` |
| Somebody else's profile | `public`, `registered`, `followers`, `leaders`, `mutuals` |
| A group, as a member | `public`, `registered`, `followers` |
| A group, as one of its administrators, or as a site administrator | `public`, `registered`, `followers`, `admins` |

Anything else is refused with 422 `access_not_allowed`, and the answer lists
the audiences that are allowed. Two of the gaps are deliberate:

- **No "only me" on somebody else's profile.** It would be a post only its
  author and that person can see, which is a private message. This platform
  does not have those.
- **No "administrators only" from a member who is not one.** That would be the
  same thing, addressed to a group's administrators.

As with every level, the audience is measured against the profile the post is
on: `followers` are that profile's followers, not the author's. A post is
never seen more widely than its profile, so on a followers-only group a
`public` post is still only seen by its members. The web app shows those wider
choices disabled, and does not offer `leaders` when writing a post.

An installation can take `public` out of all four rows, for everybody, with
`MIN_CONTENT_ACCESS=registered`; see
[Privacy of the installation](configuration.md#privacy-of-the-installation).
A post that is already public may keep that audience.

Every person and group in a response carries `authorized.audiences`: the
audiences a post the viewer writes there may have, from this same rule. The
web app's composer offers those and nothing else.

A post carries `authorized.audiences` too, for a viewer who may edit it: the
audiences it may be changed to. They can differ from the profile's list,
because "only me" depends on who wrote the post, not on who is changing it.

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

A repost is shown only to people who can see the original post. See
[Seeing posts](#seeing-posts).

## Reporting

Anybody signed in may report anything they can see, except themselves and
what they wrote. Something they cannot see cannot be reported, and answers as
if it did not exist.

Only administrators and super administrators may read what has been reported,
see who reported it, and decide a case. The person reported is not told. See
[Abuse reports](abuse-reports.md).

## Removing accounts

Disabling, archiving, deleting and purging a person or a group each have their
own rule, set in code and not changeable per profile:

| Action | Who may |
| --- | --- |
| Disable or enable | Site administrators and super administrators |
| Archive | The person themselves, a group's administrators, or a super administrator |
| Delete, with 30 days to restore | The person themselves, a group's administrators, or a site administrator |
| Purge, with no way back | Super administrators only |
| See the list of accounts and their activity | Super administrators only |
| Appoint an administrator to any group | Site administrators and super administrators |

What each one does is in [Account lifecycle](account-lifecycle.md).

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
