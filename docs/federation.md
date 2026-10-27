# Federation

How people here take part in the fediverse (ActivityPub: Mastodon and the
servers like it) and in Bluesky (the AT Protocol), and what an installation
has to do for that to happen.

Its routes are in the API reference: [activitypub-service](api/index.html#activitypub-service) and [atproto-service](api/index.html#atproto-service).

**This is being built.** What is described here is what works today: a
person can choose to be found, people follow each other across servers
either way round, their public notes travel with the follow, a person can
leave again, a super administrator can refuse a server, and the web app has the
switch, the lookup, and shows people and posts from other servers.
Replies, likes and reposts cross both ways. Posts cross with their
pictures, and arrive with a card for the link in them.

Bluesky is read for everybody, and written to for whoever connects the
Bluesky account they already have. See [Bluesky](#bluesky).

## Contents

- [What it is](#what-it-is)
- [A server on one network, an app on the other](#a-server-on-one-network-an-app-on-the-other)
- [Setting it up](#setting-it-up)
- [Keeping it running](#keeping-it-running)
- [When something does not work](#when-something-does-not-work)
- [Who is on the fediverse](#who-is-on-the-fediverse)
- [What a person is called](#what-a-person-is-called)
- [What other servers are told](#what-other-servers-are-told)
- [Looking somebody up on another server](#looking-somebody-up-on-another-server)
- [Being followed from another server](#being-followed-from-another-server)
- [Following somebody on another server](#following-somebody-on-another-server)
- [Posts going out](#posts-going-out)
- [Posts coming in](#posts-coming-in)
- [What can be done with what came from elsewhere](#what-can-be-done-with-what-came-from-elsewhere)
- [The inbox](#the-inbox)
- [What is sent, and when it does not arrive](#what-is-sent-and-when-it-does-not-arrive)
- [Signing keys and signed requests](#signing-keys-and-signed-requests)
- [The API](#the-api)
- [Leaving, and what follows a person's changes](#leaving-and-what-follows-a-persons-changes)
- [Replies](#replies)
- [Likes and reposts](#likes-and-reposts)
- [Servers that are refused](#servers-that-are-refused)
- [Pictures that are removed](#pictures-that-are-removed)
- [Posts and people that are removed](#posts-and-people-that-are-removed)
- [Bluesky](#bluesky)
- [Where the code is](#where-the-code-is)

## What it is

Mastodon, GoToSocial, Pixelfed and many others speak one protocol,
ActivityPub, and somebody on any of them can follow somebody on any other.
`activitypub-service` makes this installation one of them. It is a service
of its own because it is the one that reads what the internet sends it, and
because an installation may not want it at all.

Bluesky speaks a different protocol, and is a different service:
`atproto-service`, which reads Bluesky for everybody here, and writes to it
for whoever connects the Bluesky account they already have. See
[Bluesky](#bluesky).

## A server on one network, an app on the other

Anahita takes part in the two networks in two different ways, and what
people own, and where, follows from that.

| | ActivityPub (Mastodon and the fediverse) | AT Protocol (Bluesky) |
| --- | --- | --- |
| Anahita is | a server, as a Mastodon server is | an app, as a Bluesky client is, with a small reader of its own |
| Somebody's identity there | `@name@your-site.example`, made by Anahita when they turn federation on | the Bluesky account they already have, which they connect |
| Who holds the keys that sign for them | Anahita, encrypted | their own Bluesky server; Anahita keeps only a token to post as them, encrypted |
| Needed to take part | turning it on in their settings | a Bluesky account of their own |
| Where the original of what they post lives | here | here, with a copy of their public posts in their Bluesky repository |

### On the fediverse: a server

Anahita is an ActivityPub server in its own right. Somebody who turns
federation on becomes `@name@your-site.example`: other servers find them by
WebFinger, read their profile from Anahita at an address on the site's
domain, and deliver follows, replies, likes and reposts to its inbox, each
request signed and checked. Anahita delivers their public posts, replies,
likes, reposts and follows to their followers' servers itself, signed with
a key it made for them.

Nobody needs an account anywhere else. The identity belongs to the site's
domain, as on any fediverse server: if the site goes, or changes its
domain, the identities go with it (moving an account to another server,
ActivityPub's `Move`, is not built). Other servers keep copies of what they
received, as they do of any federated post; deletions are sent to them, and
no server can be made to honour one.

### On Bluesky: an app

On the AT Protocol each person's posts are records in their own repository,
kept by their personal data server (PDS) and tied to their identity (a DID),
which they can move from one server to another. Anahita is not a PDS. It
hosts nobody's repository and gives nobody a Bluesky identity:

- **Writing, as a client.** For somebody who connects their account,
  Anahita signs in to their own PDS and writes their public notes and photo
  posts there, and their replies, likes, reposts and follows, as Bluesky's
  own app would. Their PDS signs the records and keeps them.
- **Reading, as a small indexer.** It listens to Jetstream for the accounts
  people here follow, reads profiles and posts from Bluesky's public
  AppView, and keeps what it finds in its own database.

Somebody with no Bluesky account is not on Bluesky through Anahita. Running
a PDS beside Anahita, so that members get an identity under the site's
domain, is written down for later (`docs/plans/atproto-server.md`).

### Who owns what

- **Everything posted here is kept here,** in the installation's database
  and storage, run by whoever runs the site. That is the fediverse's sense
  of owning: choosing one's host, or being it.
- **A copy of a connected person's public posts goes to Bluesky.** Each
  picture is uploaded, made small enough for Bluesky's limit of two
  megabytes,
  to their PDS: Bluesky's own hosting for most people, or their own server
  if they run one. Those copies are theirs in the AT Protocol's sense: in
  their repository, under their identity, moving with it. From there
  Bluesky's relay and its image and video services spread and cache them,
  and anybody reading the public firehose may keep a copy.
- **What a post names on Bluesky is a file, not a server.** A picture is
  named by its content's hash and the account it belongs to; the original
  is fetched from that account's PDS. Bluesky's app shows its own resized
  copies from its image servers (`cdn.bsky.app`) and plays videos from its
  video service, so most viewing comes from Bluesky's caches even when the
  original is on somebody's own server.
- **Deleting a post here deletes it from their Bluesky account,** and
  Bluesky takes it out of its app. Copies others already took from the
  public network cannot be called back, there or on the fediverse.
- **Disconnecting stops new copies and takes nothing back.** What was
  written to Bluesky stays there, and can no longer be removed from here.
- **Only what is public goes out,** on either network: followers-only and
  members-only posts, a site kept to its members, and anybody who did not
  turn federation on or connect an account send nothing.
- **What comes in is copied in,** pictures included, into the
  installation's own storage, so that a reader's browser never asks another
  server for them. Pictures from elsewhere are removed after a while unless
  somebody here holds to the post (see
  [Pictures that are removed](#pictures-that-are-removed)).
- **Each person can take a copy of everything,** with
  [Download your data](data-export.md), including an archive shaped as
  ActivityPub has it. Their Bluesky repository is not in it: they can
  export that from Bluesky itself.

### Groups, events and the rest

What only Anahita has (groups, events, topics, followers-only
posts, likes and follows between people here, custom feeds) stays here, on
either network. An app built on the AT Protocol may keep such things as
records of its own kinds in people's repositories; Anahita does not, since
a repository is public and much of this is not.

## Setting it up

For whoever runs the installation. Federation is off until all of this is
done, and after that it is still off **for every person** until they turn
it on for themselves: sharing with other servers is each person's own
choice, and nothing an operator sets makes it for them.

### 1. Choose the names, for good

Two names are told to other servers and kept by them for ever:

| | Example | What it is |
| --- | --- | --- |
| The handle domain | `example.org` | The part of `ana@example.org` after the `@`. The host people know the site by, which is the web app's |
| The API's address | `https://api.example.org` | Where other servers fetch people and posts, and send what they have for people here |

**Change either afterwards and everybody here becomes somebody new on
every other server, with no followers.** Settle them before the first
person turns federation on. Try things out under a name you will throw
away, never the one you mean to keep (see step 8).

### 2. Be a site that can federate

- **Reachable over https at those names,** with a certificate other
  servers trust. [Deploying](deploying.md) covers the ingress and
  cert-manager.
- **Open to read:** `SITE_READ_ACCESS=public`, which is the default. A site
  for members, or one that shows visitors a preview, does not federate:
  federating would send out what it keeps in ([Privacy](privacy.md)).

### 3. Settings

On a site whose web app is at `https://DOMAIN_NAME` and whose API is at
`https://api.DOMAIN_NAME`, with `ENV=prod`, **nothing has to be set**: the
names are worked out. Otherwise say them
([Configuration](configuration.md#site-identity)):

~~~
PUBLIC_DOMAIN=example.org
PUBLIC_WEB_URL=https://example.org
PUBLIC_API_URL=https://api.example.org
~~~

`activitypub-service` also needs what the other services have, all of it
already in `config.env` and `secrets.env`:

| Setting | Why |
| --- | --- |
| `DSN` | Its own tables |
| `GRAPH_SERVICE_ADDR` | People, posts and follows are the graph's |
| `FILE_SERVICE_ADDR` | The avatars of people on other servers are copied into this installation's storage |
| `LINK_PREVIEW_URL` | Optional. Where link-preview-service is (`http://link-preview-service`), for the card of a link in a post from another server. Left out, such posts have none |
| `AMQP_ADDR` | It hears there that somebody here followed, posted or changed their profile |
| `REDIS_ADDR` | Limits on lookups and on what one address may send |
| `SECURE_COOKIE_KEY`, `OAUTH_ISSUER` | Knowing who is signed in, for a person's own switch |
| `CRYPT_BACKEND`, `CRYPT_LOCAL_KEY` | What each person's signing key is kept under. **The same values auth-service has** |

And tell NodeInfo, which auth-service serves, that the installation speaks
ActivityPub, so that other servers and the directories that list servers
know:

~~~
FEDERATION_PROTOCOLS=activitypub
~~~

### 4. Run it

- **The migrations,** as for any upgrade: the migration job adds the
  service's tables.
- **The service.** On Kubernetes it is in `k8s/base`
  (`services/activitypub.yml`); with docker-compose it is started by
  `make up`. One copy is enough to begin with, and any number may run.
- **The gateway** already sends it what is its own: `/.well-known/webfinger`,
  `/.well-known/host-meta`, `/inbox`, `/ap/` and `/activitypub/`. Rebuild the
  gateway's image with the rest.

At its start the service says which it is:

~~~
on the fediverse as example.org, reached at https://api.example.org
this installation is not on the fediverse: …
~~~

### 5. Have the web app's host send servers on to the API

A server that meets `ana@example.org` asks `example.org`, the web app's
host, who that is. That host has to send the question on:

| Asked of the web app's host | Sent on to |
| --- | --- |
| `/.well-known/webfinger?…` | `https://api.example.org/.well-known/webfinger?…`, with the same question |
| `/.well-known/host-meta` | `https://api.example.org/.well-known/host-meta` |
| `/.well-known/nodeinfo` | `https://api.example.org/.well-known/nodeinfo` |

- **With the web shell,** this is done: it redirects all three
  ([The web shell](web-shell.md#what-other-servers-ask-this-host)).
- **On a static host,** add three redirects (302, keeping the query). In
  nginx:

  ~~~nginx
  location /.well-known/ {
      add_header Access-Control-Allow-Origin * always;
      return 302 https://api.example.org$request_uri;
  }
  ~~~

**Whatever is in front of the API must pass on the `Host` it was asked
for.** The host is part of what every request between servers is signed
over; a proxy that replaces it makes every signature fail, both ways. The
gateway and the manifests here pass it on.

### 6. Check it, before anybody is told

From anywhere on the internet:

~~~sh
# The installation itself, which signs what it asks of other servers.
curl -s "https://example.org/.well-known/webfinger?resource=acct:example.org@example.org" -L
curl -s -H "Accept: application/activity+json" https://api.example.org/ap/actor

# What it says it speaks.
curl -s https://api.example.org/nodeinfo/2.1 | grep protocols
~~~

The first answers with a `subject` and a link to `/ap/actor`; the second
with an `Application` that has a `publicKey`. And that it can reach and be
understood by another server, with any account you know of on one:

~~~sh
docker compose exec activitypub-service /app/app-binary lookup @someone@mastodon.example
kubectl exec deploy/activitypub-service -- /app/app-binary lookup @someone@mastodon.example
~~~

It prints who was found. This is the one command that tries everything at
once: names, certificates, the signature, and the other server's opinion
of all three.

### 7. The first person

Signed in, somebody turns it on under **Settings › Privacy › Fediverse**.
Their profile has to be open to everybody. Then, from an account on
another server, search for their handle, `@ana@example.org`, and follow
them. A public note they post arrives there within seconds.

### 8. Trying it without going live

- **On one machine:** [the federation lab](federation-lab.md) runs this
  installation and another server side by side, with names and
  certificates of their own. Nothing leaves the machine.
- **Against real servers, before a site is live:** a tunnel from your
  machine to a hostname you will never use for anything else, with the
  three `PUBLIC_` settings saying that name. Remember that the servers you
  try it against keep the people they met under that name. Turning
  federation off for each of them (below) before the tunnel is closed asks
  those servers to remove them.

## Keeping it running

- **Back up `actor_keys`, and the key it is sealed with.** A person's
  signing key is what makes them who they are to other servers. Lose the
  table, or `CRYPT_LOCAL_KEY`, and every person here is somebody whose
  followers' servers refuse what they send. Both are in the backups an
  installation cannot do without.
- **Watch what could not be sent:**

  ~~~sql
  SELECT target, attempts, last_error, created_at
  FROM activitypub_deliveries
  WHERE status = 'failed'
  ORDER BY created_at DESC;
  ~~~

  A few are ordinary: servers close. Many to one server means it is down,
  or refuses this installation. Rows that failed can be deleted when they
  have been looked at.
- **Refuse a server** that sends abuse: see
  [Servers that are refused](#servers-that-are-refused).
- **The service's log** says why anything that arrived was refused
  (`inbox: … refused: …`), and what a lookup could not do.
- **Clocks.** A request dated more than fifteen minutes from now is
  refused, here and everywhere else. A server whose clock has drifted
  federates with nobody.

## When something does not work

| What is seen | Likely why |
| --- | --- |
| Nobody here is found by their handle from another server | The web app's host does not send `/.well-known/webfinger` on to the API (step 5). Try the `curl` of step 6 |
| One person is not found | They have not turned it on; or their profile is not open to everybody; or the other server asked earlier and is remembering the answer |
| The Fediverse card says the site has no address other servers can reach | The `PUBLIC_` settings, or `DOMAIN_NAME` and `ENV`, say `localhost`, a private address, or `http` (step 3) |
| The card says the site keeps what is on it to its members | `SITE_READ_ACCESS` is not `public` (step 2) |
| A follow from another server stays "requested" there | This installation's answer did not arrive: look at `activitypub_deliveries`. Or their request was refused: look at the log |
| Everything arriving is refused with "the signature is not this key's" | A proxy in front of the API is replacing `Host`, or rewriting the path |
| Everything arriving is refused with "date is missing or too far from now" | This server's clock, or theirs |
| `lookup` from the command line answers `lookup_failed` | The other server could not be reached, or refused the signed request. The log line above it says which |
| A note did not go out | It is not one that does ([Posts going out](#posts-going-out)); or the bus was down when it was posted. Editing it sends it |

## Who is on the fediverse

Nobody, until they say so. What is sent to another server cannot be called
back: it is copied by servers this one has never heard of.

A person is on the fediverse when all of these are so:

- they turned it on for themselves;
- they are a person, made here, enabled, and not on their way to being
  deleted;
- their profile is open to everybody (`public`): not to members only, and
  not to followers;
- the graph agrees that a visitor may see them.

The moment one of them stops being so, they are not found: every address of
theirs answers `404`, in the same words as an address nobody has. A server
is never told *why* somebody is not there.

A **website**, a site's feed connected as a profile ([Websites](websites.md)),
is on the fediverse once one of its administrators turns it on, off until
then as a person is, and only while it is enabled and not on its way to
being deleted, on a site that federates. It is an `Organization`, found by
its handle as a person is.

Groups and events are not on the fediverse yet.

## What a person is called

| | Example | Made of |
| --- | --- | --- |
| Their handle | `ana@example.org` | Their username and `PUBLIC_DOMAIN` |
| Their address | `https://api.example.org/ap/users/0962789f-…` | `PUBLIC_API_URL` and the node's UUID |

The handle is what people type. The address is what servers keep, and it is
made of the UUID so that it never changes: a username can be changed, and a
person who changed theirs must still be the same person to everybody who
follows them.

## What other servers are told

No more than the person's public profile already shows a visitor.

| Address | Answers |
| --- | --- |
| `/.well-known/webfinger?resource=acct:ana@example.org` | Where the person is: their address, and their page, `https://example.org/@ana`, which is also their `url`, the link Mastodon shows for their profile. Also answered for their address and for their page's, and for the older `/people/ana` |
| `/.well-known/host-meta` | Where WebFinger is, for servers that ask the older way |
| `/ap/users/<uuid>` | Who they are: name, username, what they say of themselves as HTML that is safe to show, their avatar and cover at [addresses that stay](architecture.md#what-a-node-is-called-elsewhere), when they joined, whether a follow waits to be approved, and the public half of their signing key |
| `/ap/users/<uuid>/followers`, `/following` | How many, and never who |
| `/ap/users/<uuid>/outbox` | How many posts, and not the posts |
| `/ap/notes/<uuid>` | One public post, as other servers read it |
| `/ap/actor` | The installation itself, which signs what it asks of other servers in nobody's name. It has a key and nothing else: nobody follows it and it posts nothing |

WebFinger answers for a handle on the API's host as well as on the handle's
own domain (`ana@api.example.org` as well as `ana@example.org`), and says in
its answer which is the real one. A server that first met somebody by their
address asks the host in that address who they are.

## Looking somebody up on another server

A person here types a handle, `@alice@mastodon.example`, or pastes the
address of a profile. The service asks that server who it is and keeps them
as a person of another server: a node of type
`node.actor.activitypub-service.person.v1`, with their address as its `uri`
([architecture](architecture.md#what-a-node-is-called-elsewhere)), and a row
in `activitypub_actors` with where to write to them and the key they sign
with.

**What another server says is believed only about itself.** A description is
kept only when the address it gives for itself is on the server it was
fetched from, its key is its own, and its inbox is an https address. It is
never believed about this installation. What somebody says of themselves
arrives as HTML and is kept as Markdown with nothing in it that runs.

**It goes only where it is safe to.** The service is told where to go by
people and by other servers, so it fetches through the same guarded client
as link previews (`anahita-libs/safehttp`): https only, public addresses
only, decided at the moment of connecting, a megabyte at most.

**Their pictures are copied here.** Their avatar, and the cover across the
top of their profile (what Mastodon calls the header, `image` in their
description), are each fetched once, checked to be
a picture and nothing else, written again at this installation's own sizes,
and kept in its storage like anybody's (`avatar.go`). They are shown from
there, never from the server it came from: a picture loaded straight from
another server tells that server who looked at whom, for every reader. Each
is fetched again only when their server gives a different address for it
(`activitypub_actors.icon_url`, `header_url`), and both are removed from
storage when the person is.

A lookup is limited to 30 in ten minutes for each person.

## Being followed from another server

Somebody on another server follows somebody here by sending a `Follow` to
the inbox.

| The person here | What happens |
| --- | --- |
| Is on the fediverse | They are followed at once: an ordinary follow in the graph, from the node kept for the person elsewhere. The other server is sent an `Accept` |
| Has blocked whoever asks | Nothing is made, and the other server is sent a `Reject` |
| Is not on the fediverse | Nothing, and nothing is said |

**Nobody on the fediverse approves their followers.** Only a profile open
to everybody is on it, and such a profile is followed by whoever chooses
to, from here or from elsewhere. Letting people *ask* to follow is for a
profile that is not open to everybody, and such a profile does not
federate. So a person's description always says a follow does not wait.

Taking a follow back (`Undo` of the `Follow`) removes it, or the request.
Only whoever followed can take their own follow back.

What the other server calls each follow is kept in `activitypub_follows`,
because accepting it or undoing it means quoting it back.

A person followed from elsewhere is told as they are of any follower, and
the notification leads to the follower's profile here.

## Following somebody on another server

Somebody here follows a person kept from another server the way they follow
anybody: the same button, the same request to `socialgraph-service`.
Nothing there was changed. `activitypub-service` hears on the bus that it
happened and asks the other server, with a signed `Follow`.

| Then | What happens here |
| --- | --- |
| The other server says yes (`Accept`) | The follow stands. It is recorded as accepted |
| It says no (`Reject`), then or later | The follow here is taken away |
| It says nothing yet (the person there approves their followers) | The follow is there and recorded as pending. What that person posts will not arrive until they say yes |
| The person here unfollows | The other server is sent an `Undo`, quoting the follow |

- **Only somebody who is on the fediverse can follow across it.** The other
  server has to be able to find out who is asking. A follow of somebody
  elsewhere by a person who is not, is taken away again as soon as it is
  heard of; the web app is not to offer it.
- **An answer is matched to the question.** A yes or a no does something
  only when it is from the person who was asked, about a follow this
  installation is waiting on, by the name that follow was sent under. Nobody
  can be made to follow somebody by being told they were accepted.
- **The bus can lose a message.** A follow the other server was never asked
  about stays as it is here and carries nothing, until
  [the sweep](#what-the-bus-lost) finds it, within a few minutes, and asks.

## Posts going out

What is sent to another server cannot be called back, so what is sent is
narrow, and is asked of the post as it is at that moment:

- a **note, a photo post, a video post or an article**, written here, by
  somebody who is on the fediverse. A photo post is sent as a note with its
  pictures, a video post as a note with its video, and an article as an
  `Article`;
- on **their own profile**: not in a group, not on somebody else's;
- **open to everybody**: not to members, not to followers;
- or **a reply**, by [the rule for replies](#replies);
- and a visitor may see it, by the graph's own reckoning.

Such a note goes to every server that has a follower of its author, once
for each server. It is sent as HTML made from its Markdown, with nothing in
it that runs; the people it names and its first ten hashtags are links, and
are listed beside the text; its language is said when its author said one. It can also
be fetched, at `/ap/notes/<uuid>`.

| Then | Other servers are sent |
| --- | --- |
| The note is changed | An `Update`, with the note as it is now |
| A location is added to it or taken off, from its page | An `Update`, said to be changed then |
| It stops being one that goes out (made for followers only, say) | A `Delete`: it is taken back |
| It is deleted | A `Delete` |

A note that never went out is deleted at nobody. Which notes went out is
kept in `activitypub_objects`: a deleted note's node is gone by the time
anybody hears of it, and that table is what still knows what it was called.

**A photo post goes with its pictures.** Each is listed with the note at an
address of this installation's, `/ap/notes/<uuid>/pictures/<n>`, with what
its author said it shows and how big it is. A server that asks is sent on
to the largest copy kept, at a signed address in storage good for a short
while, and makes its own copy and its own smaller sizes. The address gives
a picture only while its post is one that goes out, which is asked each
time: a post closed to its followers stops giving its pictures to anybody.
A photo post is heard of a moment before its pictures are listed, and is
not sent without them: it is looked at again after two seconds, and failing
that is found by the sweep.

**A video post goes with its video**, a GIF made into a clip included. Its
title is the first line of the note, since a note has none of its own. The
video is listed at `/ap/notes/<uuid>/video`, the largest size no taller
than 720 and no heavier than 40 MB (`ap.VideoMax`, which Mastodon and
GoToSocial take of a video from elsewhere unless told otherwise), with its
poster as its `icon` at `/ap/notes/<uuid>/poster`. Both send a server that
asks on to storage, as a picture's address does, and only while the post
goes out. A video with no size that light goes as its words, linking to
its page. Mastodon shows a video with no sound as a GIF, silently and on a
loop, so a clip made of a GIF is one there too, with nothing said to make
it so.

**A post goes with its locations,** the ones its author chose for it (the
user, 2026-10-19: choosing one is their consent, for that post), and goes
no further than the post does:

| Where | What |
| --- | --- |
| `location` | The first location chosen, a `Place` with its `name`, `latitude`, `longitude` and its page here as `url`, as Pixelfed sends and reads one: ActivityStreams allows several, but servers that read it expect one |
| `tag` | Every location, the first included, as a `Place` with its point and its page as `href`, beside the mentions and hashtags |

The composer sends the locations with the post (`location_ids`, up to five,
on adding a note, a photo post, an article or a topic), and the graph ties
them as it makes the post, before it is announced, so the `Create` carries
them. Mastodon shows neither; Bluesky has no field for one, and nothing goes
there. Nobody's own position is ever sent (roadmap row 94). See
[Hashtags and locations](hashtags.md).

**A photo or a video goes with who is in it** (roadmap row 103): each
person here tagged in it, their tag approved, as a `Mention` in its `tag`,
`@alias@domain` with their address here, beside whoever its words name.
Only somebody whose profile is on the fediverse, as for a name in the
words; a tag tells another server of nobody who has not chosen to be known
there. A tag approved or taken off sends the post again as an `Update`.
Bluesky has no tags, and nothing goes there.

A **website's notes**, the items of its feed and its replies, go out as a
person's notes do, the website's, to its followers ([Websites](websites.md)).

**An article goes as an `Article`** at the same address a note has
(`/ap/notes/<uuid>`), under the same rules:

| Of the `Article` | From the article |
| --- | --- |
| `name` | Its title |
| `summary` | Its first paragraph of words, as plain text, cut at a word near 300 characters; a heading, a picture, a quotation, a list or an address on a line of its own is passed over |
| `content` | All of it, as HTML made from its Markdown, as a note's is |
| `url` | Its page here, `/articles/<id>` |
| `image` | Its cover, at `/covers/<uuid>/large` |

Mastodon shows an `Article` as its title and a link to its page here;
WriteFreely, Ghost, Friendica and the like show the article itself.
Replies, likes and reposts from elsewhere are taken on it, its changes and
its deletion follow, as for a note. A post the article shows by its
address on a line of its own stays a link.

**Not sent:** topics (the user, 2026-10-17: they stay here); anything in a group;
a quoted post (the note that quotes goes, without what it quotes);
the card for a link. Somebody who starts following is sent what is posted
from then on, and nothing from before.

## Posts coming in

A note from another server is kept when all of these are so:

- **somebody here follows whoever wrote it.** This installation is not a
  copy of the fediverse: what nobody here asked for is not stored;
- it is **its sender's own**, at an address on their own server;
- it is **addressed to everybody**;
- it has some text. A note that **answers another** is kept by
  [the rule for replies](#replies).

It is kept as an ordinary note, owned by the node kept for its author, with
its address as its `uri`. Its HTML is kept as Markdown with nothing in it
that runs, at most 20,000 characters, dated when its author says it was
written, in the language it says. Because it is an ordinary note on the
profile of somebody followed, it is in the feed of whoever follows them
with nothing changed in the feeds.

**With pictures, it is a photo post,** which is what a post with pictures
is here: its words are the caption and its pictures, up to what a photo post
holds here (`PHOTO_MAX_FILES`), are its images,
each with what its author said it shows. So it is drawn, opened and stepped
through like any photo post, and is in its author's photos. The pictures
are copied into this installation's storage (`pictures.go`) and never shown
from where they came from; each is checked to be a picture and nothing
else, held to the limits an upload is held to, and written out again at
this installation's own sizes. They are fetched after the post is kept and
not waited for, so a post is there a moment before its pictures are.
Told of again, its pictures are not fetched again; taken away where it was
written, what is stored of it is removed.

- **A post its author marked as sensitive is kept without its pictures.**
  Nothing here puts a picture behind a warning yet.
- **What a post was first kept as, it stays:** a note that later gains a
  picture is still a note.
- **Only pictures.** Video and sound that come with a post are not kept.

**A custom emoji** in a post, or in somebody's name or what they say of
themselves, is a word between colons with a picture listed beside it. The
picture is not shown yet, so the word is taken out
(`federation.WithoutCustomEmoji`): only a word that is listed as one.

**A link in it is given a card,** as a link in a note written here is:
what the page is called, what it says of itself and its picture, read by
this installation and kept in its own storage. `activitypub-service` asks
link-preview-service for it at an address the gateway sends nobody to
(`GET /internal/link-previews`, `LINK_PREVIEW_URL`), and keeps the answer
in the note's meta where a note keeps its card. Nothing the other server
says of the link is used, and no reader is sent to another site for a
picture. The card is of the first link that is to a page, not of somebody's
name or a hashtag. A post is kept without one where the page says nothing
of itself, or does not answer within six seconds.

A post that was kept before something about keeping posts was put right
can be read again from its own server:

~~~sh
docker compose exec activitypub-service /app/app-binary reread https://mastodon.example/users/alice/statuses/1
~~~

**Its locations are tagged here** (roadmap row 82, part 4), unless the
installation says `FEDERATION_LOCATIONS=off`. Every `Place` it names, its
`location` (Pixelfed sends one with a post its author placed) and the
`Place`s in its `tag`, at most five, is matched to a location here: one of
the same name within 25 km, since Pixelfed's places are mostly cities whose
point differs from server to server; else any within 100 m. When none
matches, one is made, with its name and its point kept to four decimals
(about 11 m), put down to the post's author as a hashtag is to whoever first
used it. The post then shows among that location's posts, in trending and
nearby. When it is edited there, the locations it no longer names are taken
off. A place's own address (places.pub gives OpenStreetMap places stable
ones) is not kept yet: the graph keeps addresses only for profiles and
posts from elsewhere. Nobody's own position is read: `Arrive` and other
check-ins are not (row 94).

**Its hashtags are this installation's.** A hashtag arrives as a link to
its page on the server the note was written on. It is kept as the hashtag
alone (`federation.PostToMarkdown`), and the note is then given its
hashtags by hashtag-service exactly as a note written here is: one that is
not here yet is made, and the note is found under it with every other post
that has it. So `#gardening` in a note from elsewhere leads to this
installation's own page for it, as it does on Mastodon. Only its first ten
different hashtags are filed, as for a post written here ([Hashtags and
locations](hashtags.md#at-most-ten-hashtags)); the rest stay words. A hashtag in letters this installation does not take for one (see
`entities.HashtagRegexPattern`) is left the link it came as; so is one in
what somebody says of themselves.

hashtag-service is told by an event of its own,
`federation.medium.text-service.note.added|edited.v1`
(`anahita-libs/event/remote.go`), and not by the one a note written here
is announced with. Everything that listens for those would take the note
for one written here: whoever it names by `@name` would be told they were
mentioned, though the name is somebody's on another server, and its
author's followers would be notified as if by somebody here.

Changed there, it is changed here; taken away there, it is taken away here,
and only by its author: the graph removes a note from another server only
in the name of whoever it belongs to
(`graph-grpc-service/data/repositories/nodes_remote.go`).

**What they posted before they were followed.** A server sends a follower
what is posted from then on, so somebody followed today would have an
empty profile here until they next post. When somebody is first followed,
their most recent public notes, up to twenty, are read once from the list
their server keeps (`backfill.go`), each held to the same rule as a note
that is sent. Only ever from their own server, and in no more than
twenty-five requests whatever it answers.

**Not kept:** video and sound that come with a post, and polls.

## Replies

A reply here is a post with a parent and a root, its author's own, seen by
whoever may see the root: a note, or a photo or video post when it carries
pictures or a GIF or a video. Between servers it is a `Note` with
`inReplyTo`, with its pictures or its video attached as a photo or video
post's are; to Bluesky, a reply with its embed. A reply of any kind may
answer a note, a photo post or a video post (tried against Mastodon and
Bluesky on 2026-10-14).
Nothing about replies was changed for this: what is added is which of them
cross, and how.

### Going out

A reply written here is sent when its author is on the fediverse, it is
not hidden or removed, the root of its thread is open to everybody, and
**what it answers is something other servers can see**: a note that came
from another server, or a note here that itself went out. So a thread of
somebody's own notes goes out whole, each after the one before, and a reply
under a note that stayed here stays here.

**A thread written in one go** (`POST /notes/…/thread`, roadmap row 101) is
heard of in no set order: its notes are announced together. A note whose
part before it, by the same writer and made within a minute of it
(`federation.WrittenTogether`), has not gone out yet sends that part first,
so the thread goes out at once and in order, on the fediverse and on
Bluesky, not at the next sweep. An older note of the writer's that never
went out still does not go out because of a reply. On Bluesky, the part
before is read from the writer's own server (`com.atproto.repo.getRecord`)
rather than the AppView, which may not have seen it yet; a part written
before the one it answers is tried again shortly, and not given up on.

It is sent with `inReplyTo` set to what it answers, by the address that
has between servers, to its author's followers and to whoever wrote what
it answers when they are on another server. They are named beside the text
as mentioned (a `Mention`) and as somebody it is for, which is what has
their server tell them; their name is not written into the reply. Where it
was sent beyond the followers is kept (`activitypub_objects.reply_inbox`),
so that a change or a deletion goes there too, after the reply is gone.

### Coming in

A note from another server that answers something is kept when what it
answers is known here (a note here that went out, or a note kept from
elsewhere), it is its sender's own and addressed to everybody, the thread's
root is open to everybody, and:

- **under a thread that began here**, its author may reply by the thread's
  own rule, the one `permissions.CanReply` holds everybody to: whoever the
  root's author lets reply (anyone, nobody, their followers, the people
  they follow, the people mentioned), measured by the follows between them
  in the graph, and the profile's rule for who may comment, where somebody
  elsewhere counts as a registered reader. Somebody the thread's owner or
  author has blocked is refused. Whoever wrote the reply need not be known
  already: they are looked up and kept;
- **under a thread that began elsewhere**, somebody here follows its
  author, as for any note.

Mastodon begins a reply with the name of whoever is answered, as a link to
their profile. Where that is somebody of this installation the name alone
is kept (`@ana`), which the web app shows as any mention; a link that reads
as one name and leads to another is left as it came.

It is kept under its parent like any reply, counted, and seen by whoever
may see the root and nobody more: the graph gives it its root and takes no
word from outside for which thread it is in. Whoever it answers here is
notified, as for a reply written here (notification-service hears the
event for a note kept from elsewhere). A reply to something not known here
is dropped; the rest of a thread is not fetched from its server.

Taken away where it was written, it is taken away here, the thread is
counted again (one that people here have answered is emptied and kept, as a
reply here is when it is removed after being answered: what was said under
it is theirs, and nothing sent from outside removes what was made here), and the news of it goes with it: the service says on the bus
that it is gone (`federation.medium.text-service.note.deleted.v1`), and
notification-service removes what it told anybody of the reply.

### Answering from here

Somebody here may reply to a note from another server that they can see.
Who its author lets reply is its own server's to say and to hold to.
Written by somebody on the fediverse, the reply is sent; written by
somebody who is not, it stays here, as a conversation among the people of
this installation, and the web app says so before it is posted.

## Likes and reposts

Both are made here as they always were, through like-service and
feed-service, and said on the bus. `activitypub-service` hears that and
tells other servers; and what other servers send is kept through the same
graph calls those services use, so a like or a repost from elsewhere is
counted, shown and told to its author like any other.

| | Going out | Coming in |
| --- | --- | --- |
| **A like** (`Like`) | Somebody on the fediverse likes a note or a reply that came from another server: its author's server is told. Taken back with an `Undo` | Of a note or a reply here that went out: kept as a like by the person kept for whoever sent it, and the author told. `Undo` takes it away |
| **A repost** (`Announce`, Mastodon's boost) | Somebody on the fediverse reposts a note other servers can see, one from elsewhere or one here that went out: their followers' servers are told, and the note's author's when it is from elsewhere. Taken back with an `Undo` | Of a note here that went out, by anybody: kept as a repost and the author told. Of a note from elsewhere, only by somebody followed from here: the note is read from its own server and kept with whoever wrote it, if it is not here already, and the repost is on the reposter's profile and in the feeds of who follows them |

- **By somebody who is not on the fediverse,** a like or a repost of
  something from elsewhere stays here, as their reply does.
- **Somebody the author has blocked** neither likes nor reposts what the
  author wrote; a server that is refused is not listened to at all.
- **A like is named from who liked and what** (`/ap/activities/like-…`), so
  taking it back names it again with nothing kept. A repost that went out
  is on record like a note (`activitypub_objects`, `kind` `repost`), with
  where it went, because the repost is gone by the time its taking back is
  heard of.
- **Taken back elsewhere, the news of it goes here:** a repost of
  something here that is undone on its own server is removed, and what its
  author was told of it with it.
- **Nobody elsewhere is notified here:** whoever wrote something from
  another server is told by their own server.
- **Not covered by the sweep:** a like or a repost whose word the bus lost
  is not found again. Doing it again puts it right.

## What can be done with what came from elsewhere

Somebody from another server, and what they post, are looked at here and
followed, and little else. The rules are the installation's own, in
`anahita-libs/permissions`, so every service keeps them:

| | Allowed? |
| --- | --- |
| Reading their profile, at `GET /people/<handle>` with the whole handle (`alice@mastodon.example`) | As for any public profile. It answers with `uri` and `origin_domain`, which nothing made here has |
| Following and unfollowing them, blocking them | Yes, through the same routes as for anybody. A block ends every follow between the two, both ways, so that nothing more is delivered to their server, and their server is sent a `Block` by somebody on the fediverse, which Mastodon and what speaks to it keep to: they stop seeing the blocker's posts and answering them. Unblocking sends `Undo` of it; the follows are not made again. The sweep forgets any follow left across a block |
| Changing their profile, their settings, their pictures | Nobody, an administrator included: every route that changes a person reads them as a person of this installation, and does not find them |
| Posting on their profile | Nobody |
| Changing a post of theirs | Nobody. An administrator may remove it from this installation |
| Replying to a post of theirs | Yes: see [Replies](#replies) |
| Liking a post or a reply of theirs | Yes: see [Likes and reposts](#likes-and-reposts) |
| Reposting a post of theirs | Yes, likewise |
| Saving a post of theirs | Yes; it stays here |

## The inbox

`POST /inbox`, and `POST /ap/users/<uuid>/inbox` for each person: the same
door. It is the one place in the installation where anybody on the internet
may send anything, so nothing that arrives is acted on, or read for what it
says, until it is known who sent it:

1. **It must be signed,** and the signature must check against the key of
   whoever it names, read from that person's own server (or kept from the
   last time, and read again if the kept one no longer checks).
2. **Whoever the message says did the thing must be whoever signed it.**
   Nobody is believed about what somebody else did.
3. **Only then is it read,** and only what this installation understands is
   done: `Follow` and its `Undo`; `Accept` or `Reject` of a follow asked
   for from here; `Create`, `Update` and `Delete` of a note; `Like` and
   `Announce` (a repost) and their `Undo`; `Update` and `Delete` of the
   person themselves. Everything else is answered `202` and ignored.

A message of more than a megabyte is refused. What cannot be checked is
answered `401`, and nothing is kept of whoever sent it.

## What is sent, and when it does not arrive

What is to be sent to another server is written to a table first
(`activitypub_deliveries`) and sent from there, signed by the person it is
sent for. If the other server is down it is tried again after 1, 4, 16, 64
and 256 minutes and then every six hours, for two days; if the other server
says the inbox is gone, or that what was sent is wrong, it is given up at
once. What is given up is kept in the table with the reason.

~~~sql
SELECT target, attempts, last_error, created_at
FROM activitypub_deliveries
WHERE status = 'failed'
ORDER BY created_at DESC;
~~~

The sending runs inside `activitypub-service`. Any number of copies of the
service may run: a row is handed to one of them at a time.

### What the bus lost

The service is told of what happens here (a follow, a post, a profile
closed) on a bus that can drop a message, and what it was not told of,
other servers were not told of either. So every five minutes, and a minute
after it starts, it looks for itself (`cmd/api/handlers/sweep.go`,
`data/repositories/missed.go`) at where the graph and its own records
differ:

| Found | Done |
| --- | --- |
| A follow of somebody elsewhere that their server was never asked about | It is asked. Or the follow is taken away, where it is one that cannot be: by somebody not on the fediverse, or from the lab to a real server |
| A follow their server was asked about that is no longer in the graph | Their server is told it is over |
| A public note by somebody on the fediverse, written in the last two days and since they joined, that was never sent | It is sent to their followers |
| A note that was sent and is gone here, disabled, or no longer open to everybody | Servers that have it are told to drop it |
| A note other servers have that was changed here since they were last sent it | They are sent the change |
| Somebody whose profile was changed since their followers' servers were last sent it | They are sent it as it is now |
| Somebody on the fediverse whose profile is no longer open to everybody, or is disabled or gone | They leave, as they would have |

Each is handed to what would have handled the event, which decides again
from the thing as it is now and does its work once however often it is
asked. So a sweep that finds nothing new sends nothing, every copy of the
service may sweep at once, and an event that was only late does no harm.
Something is not taken for lost until two minutes have passed, and at most
200 of each kind are put right in one sweep. A line in the log says what a
sweep put right; one that found nothing says nothing.

**A change is found by what is on record of what was last sent:**
`activitypub_objects.told_updated_at` for a note and
`activitypub_accounts.told_updated_at` for a person, each the thing's own
`updated_at` when other servers were sent it. A change made later than that
was never told. A row from before these were kept has none, and is compared
with when the note was made, or when the person joined the fediverse.

A note that went unsent for more than two days is not sent late, since
followers would be shown it as new.

## Signing keys and signed requests

Whatever is sent in somebody's name is signed with a key of theirs, and the
other server checks it against the public half, which it reads from their
description. What the installation asks for in nobody's name, such as a
lookup, it signs as itself (`/ap/actor`): many servers answer nobody who
does not say who is asking.

The signature is over the request's target, its host, its date and, when it
has a body, a digest of the body (`activitypub-service/httpsig`). Each of
the four is one an attack goes through without, and each has a test. A
request dated more than fifteen minutes from now is refused.

**Behind a proxy, the `Host` the service is given must be the one that was
asked for.** The host is part of what is signed. The gateway and the
manifests here pass it on; a proxy put in front that does not will make
every signature fail, in both directions.

- A key is made when a person turns federation on (RSA, 2048 bits, which is
  what every server understands), and for anybody on the fediverse who has
  none the first time one is asked for: a website's, for one.
- The private half is kept in `actor_keys`, encrypted by the means
  auth-service keeps its secrets: `CRYPT_BACKEND` and `CRYPT_LOCAL_KEY`
  ([Configuration](configuration.md)). **Losing that key loses every
  person's signing key,** and with it their followers elsewhere: it is in
  the backups an installation cannot do without.
- Everybody is told the same key. A server that was told one will refuse
  what is signed with another.

## The API

Signed in, for the person themselves:

| Request | Does |
| --- | --- |
| `GET /activitypub/settings` | Whether they are on the fediverse, whether they could be, and what they are or would be called. An administrator may ask it of somebody else, with `?person_id=`, to read and not to change |
| `PUT /activitypub/settings` `{ "enabled": true }` | Turns it on or off |
| `GET /activitypub/lookup?q=@alice@mastodon.example` | Finds somebody on another server, by handle or by the address of their profile, and keeps them |

~~~json
{
  "available": true,
  "enabled": false,
  "handle": "ana@example.org",
  "actor": "https://api.example.org/ap/users/0962789f-aeca-4850-ac9b-41129e3b6f46"
}
~~~

When `available` is false, `reason` says why:

| `reason` | Means |
| --- | --- |
| `site_private` | The site is for members, or shows visitors a preview |
| `site_unreachable` | The site has no address another server could reach |
| `profile_not_public` | Their profile is not open to everybody |

Turning it on when it is not available is refused with `409` and the reason.
Turning it off is always allowed.

A lookup answers with who was found:

~~~json
{
  "id": 170292,
  "type": "node.actor.activitypub-service.person.v1",
  "name": "Alice",
  "alias": "alice@mastodon.example",
  "body": "Grows **beans**",
  "uri": "https://mastodon.example/users/alice",
  "origin_domain": "mastodon.example",
  "url": "https://mastodon.example/@alice",
  "locked": true
}
~~~

or with why not: `400 lookup_unreadable` (not a handle or an https address),
`404 lookup_is_here` (somebody on this installation), `404 lookup_not_found`,
`404 lookup_not_a_person` (a group, a post), `502 lookup_failed` (the other
server could not be reached or understood), `429 too_many_lookups`.

**From the command line,** for whoever runs the service and wants to know
whether it and another server can reach and understand each other:

~~~sh
docker compose exec activitypub-service /app/app-binary lookup @alice@mastodon.example
~~~

## Leaving, and what follows a person's changes

**Leaving.** A person leaves the fediverse when they turn it off, and also
when they stop being somebody who may be on it: their profile is no longer
open to everybody, or they are disabled, archived, deleted or purged.
Either way:

- every server that knows them (the ones with a follower of theirs, and
  the ones with somebody they follow) is sent a `Delete` of the person,
  which is the one message the fediverse has for "forget them";
- here, the follows that crossed between servers are removed, both ways
  round, and the record of what they sent;
- from that moment they are not found.

**Leaving is for good, as far as other servers go.** A server told that
somebody is deleted removes them, their posts and their followers, and
remembers that it did. In the lab, GoToSocial marked the account suspended
and did not take the same person back when they turned federation on
again. So turning it on again makes a person findable here, and whether
any server they met before will know them is that server's choice.

This is what the fediverse itself does: a `Delete` of the person is how
Mastodon and the rest say an account is gone, and there is no gentler
message that means "stop showing them". Staying silent instead would leave
a person's profile and posts on other servers, in public, for good.

**So that nobody leaves by accident,** the web app asks before the switch
is turned off, and the card where a profile's audience is chosen warns as
soon as anything but "everybody" is picked by somebody who is on the
fediverse, and asks once more before saving.

**A profile changed.** When somebody on the fediverse changes their name,
what they say of themselves or their pictures, their followers' servers are
sent them as they are now (`Update`).

**From the other side.** A profile changed elsewhere is read again from
its own server, and not taken from the message that says so. An account
closed elsewhere is removed here: the person, what they posted, and every
follow between them and anybody here. Only they can say so of themselves,
and a stranger saying they are gone is taken without their server being
asked anything: a large server announces every closed account to every
server it knows.

## Servers that are refused

A list kept by super administrators (`federation_domain_blocks`). A server
on it:

- is not listened to: what it sends is answered `403` before its key is
  even fetched;
- is sent nothing, including what was queued for it before it was refused;
- cannot have anybody on it looked up.

A name covers every name under it: `example.net` refuses
`social.example.net` too.

**It does not remove what already came from that server.** People from it
who are kept here stay, with their posts and the follows between them and
people here; nothing more arrives and nothing more is sent. They are
removed one by one, as any post or person is.

| Request | Does |
| --- | --- |
| `GET /activitypub/blocks?q=&limit=&start=` | A page of the list, by name, with `pagination.total`. `q` narrows it to the names and reasons it is found in; `limit` is 50 unless asked, 200 at most |
| `POST /activitypub/blocks` `{ "domain": "spam.example", "reason": "…" }` | Refuses a server, or changes why. The domain may be typed as a name, an address or a handle |
| `DELETE /activitypub/blocks/spam.example` | Stops refusing it |

**Super administrators only,** like the rest of what is set for the whole
installation: refusing a server is a choice made for everybody on it, and
an administrator who is not a super administrator is answered `403`. The
reason is for the super administrators who come after, and is never sent to
the server refused. A change takes up to half a minute to take hold in
every copy of the service.

In the web app the list is in **Site settings, under Fediverse**: the
servers refused with why, a search by name or reason, a button beside each
to stop refusing it, and a dialog to refuse another. It is read a page at a
time as it is scrolled, so a list of thousands is no heavier than a list of
ten.

**How much one address may send** is limited too: 600 messages a minute to
the inbox, counted before anything is read, and answered `429` beyond
that.

## Pictures that are removed

A picture is nearly all of what a post from another server costs to keep:
it is stored at every size a photo is. So the pictures of a post from
elsewhere are removed from storage once the post has been left alone for
`REMOTE_MEDIA_DAYS`, thirty unless said, and copied here again when
somebody asks to see them. It is what Mastodon and GoToSocial do with what
they hold of other servers, and it is on as the installation comes, since
it loses nothing that is still there to be had. `0` keeps pictures for
good.

- **What stays.** The post, with its words, and what each picture showed.
  It says in its meta (`media_removed`) how many pictures it had, and the
  API sends that with the post: `media_removed: { count, alts, gone }`.
- **"Left alone"** is by when the post was last written here: when it
  arrived, when its author changed it, and when its pictures were last
  copied again. Pictures that were asked for are kept as long again.
- **Never** the pictures of a post somebody here liked, saved, answered,
  reposted or quoted, nor of one an open report is about. And nothing
  written here is ever looked at: only what has an address on another
  server.
- **Asked for again:** `POST /activitypub/posts/:id/pictures` or
  `POST /atproto/posts/:id/pictures`, by anybody signed in who may see the
  post, sixty times in ten minutes. The post is read again from where it
  came from and its pictures are copied as they were the first time. In
  the web app it is a button where the pictures were, and it is asked at
  once on the post's own page.
- **No longer to be had.** When the post was taken away where it was
  written, or what it is read from says it has no pictures now, the answer
  is `410 media_gone`, and the post says so from then on (`gone`), keeping
  what its pictures showed.
- **Each service looks after what came through it,** every hour, two
  hundred posts at a time, oldest first: an installation that runs one of
  the two is covered by that one.

Avatars and covers of people elsewhere are small, one of each, and are
kept while the person is. The picture of a link's card is the link
preview service's own.

A cover a server shows for somebody who set none (Mastodon's grey pattern
at `.../headers/original/missing.png`, NodeBB's
`.../assets/images/cover-default.png`) is not copied: such a profile has no
cover here.

## Posts and people that are removed

The words of a post cost very little, and are kept far longer than its
pictures. What goes is what nobody here has a reason to keep.

| Setting | What goes | As it comes |
| --- | --- | --- |
| `REMOTE_UNFOLLOWED_DAYS` | The posts of somebody nobody here follows any more, once they have been left alone this many days | 30 |
| `REMOTE_POSTS_DAYS` | Any post from elsewhere left alone this many days, whoever follows its author | Off |
| `REMOTE_POSTS_KEPT` | All but the newest so many posts of one person elsewhere | Off |

The first is on because it removes only what nobody is reading: somebody
looked up once and never followed, or unfollowed a month ago. The other
two are for an installation that has to hold its database to a size, and
are off because **what they remove is not got back by asking**: a post
that went is read again only if its author is followed anew, and then
only their latest. `0` turns any of the three off.

- **Never** a post somebody here liked or saved; one that anything stands
  under or on, here or elsewhere (an answer, a repost, a quote); one in a
  thread somebody here wrote in; one an open report is about.
- **Never an answer.** Only a post that begins a thread is removed by a
  rule. An answer from elsewhere is here because it is part of a
  conversation somebody here is in.
- **Never anything written here.** Only what has an address on another
  server is looked at, and only what is owned by somebody of another
  server. The operator's settings do not clip what the site's own people
  wrote, at any age or number.
- **"Left alone"** is as for pictures: by when the post was last written
  here.
- **It goes as if its author had taken it away:** with what is stored of
  its pictures, and said on the bus, so that its hashtags and any news of
  it go too.
- **Then the person.** Somebody from elsewhere is removed, with their
  avatar and cover, when all of these hold:
  - no post of theirs is left here;
  - nobody here follows them, and they follow nobody here;
  - nobody here has blocked them, and nothing ties them to anything here
    (no like, no mention);
  - no open report is about them;
  - they have been left alone for `REMOTE_UNFOLLOWED_DAYS`.

  Looked up again, they are read again. With that setting at `0` nobody
  is removed.
- **Each service looks after what came through it,** every hour, two
  hundred posts and two hundred people at a time, the longest left alone
  first.

Not built: the same for videos, which are not brought in from elsewhere
yet (`docs/plans/remote-content-limits.md`).

## Bluesky

`atproto-service` lets somebody here look up an account on Bluesky and
follow it. Its posts are then kept here as they are written, and are in the
feed of whoever follows it, like the posts of anybody else they follow.

**Everybody can read Bluesky; whoever connects their own Bluesky account
can also post, reply, like, repost and follow there.** Nobody is given a
name on Bluesky by this installation. Somebody
who already has an account there can connect it, and from then on their
public notes are posted there, and a like, a repost or a reply they give
here to something from Bluesky is made there too, as that account: see [Answering on Bluesky](#answering-on-bluesky).
A website's administrators connect the site's own account the same way,
and its notes are posted there ([Websites](websites.md#on-bluesky)).
For everybody else such things stay here, which the web app says under the
reply being written, and their follow of somebody on Bluesky is kept here
without that account being told.

So it asks nothing of the installation but a way out to the internet: no
public name, no certificate. It works on a developer's machine as it is.
**Only a site open to read writes to Bluesky** (`SITE_READ_ACCESS` left
out or `public`), and only for somebody whose profile anybody may see. On
a site kept to its members, or one that shows visitors a preview,
connecting an account is not offered and nothing leaves; reading still
works.

### Running it

`make atproto`, or the `atproto-service` entry in `docker-compose.yml`; in
Kubernetes, `k8s/base/services/atproto.yml`. An installation that leaves it
out follows nobody on Bluesky: the web app's lookup finds no service to
ask, and offers nothing.

| Variable | Value |
| --- | --- |
| `ATPROTO_APPVIEW_URL` | Optional. The reader that is asked who an account is and what it posted. Left out, Bluesky's own, `https://public.api.bsky.app`, which asks for no account |
| `ATPROTO_JETSTREAM_URL` | Optional. Where posts are heard as they are written: one address or several, with commas between. Left out, the four Bluesky runs (`wss://jetstream1.us-east.bsky.network/subscribe` and its three siblings). More than one is better than one: see [Hearing what is posted](#hearing-what-is-posted) |
| `ATPROTO_CDN_URL` | Optional. Where pictures are fetched from, to be copied here. Left out, `https://cdn.bsky.app` |
| `LINK_PREVIEW_URL` | Where `link-preview-service` is, for the card of a link in a post. Left out, such posts have no card |
| `ATPROTO_PLC_URL` | Optional. The directory a DID is looked up in. Left out, Bluesky's, `https://plc.directory` |
| `CRYPT_BACKEND`, `CRYPT_LOCAL_KEY` (or `AWS_KMS_KEY_ID`) | The same as auth-service's. The token of each connected account is kept encrypted under them; without them nobody can connect an account and nothing is written to Bluesky |
| `SITE_READ_ACCESS` | The same as every service's. Anything but `public` (or left out) turns writing to Bluesky off |
| `FEDERATION_PROTOCOLS` | Read by auth-service, not this service: add `atproto` (`activitypub,atproto`) so that NodeInfo says the site is on Bluesky and the web app's front page says so |

The rest is what every service has: the database, the graph and file
services, the bus, and the settings for checking who is signed in.

`app-binary lookup alice.bsky.social`, run in the service's container,
looks somebody up with nobody signed in and prints who was found: the
first thing to try when Bluesky seems not to be reached.

### Looking somebody up

`GET /atproto/lookup?q=`, signed in. `q` is a handle with or without its
`@` (`alice.bsky.social`), the address of a profile
(`https://bsky.app/profile/…`), or an account's lasting name
(`did:plc:…`). The answer is the same as for somebody on the fediverse:
the person, kept here, to be opened and followed. Thirty lookups in ten
minutes for each person.

In the web app it is the same box as for the fediverse, on the search
page. How a handle is written says which network is asked:
`@alice@mastodon.example` has its server after a second `@`, and
`alice.bsky.social` is a name of its own.

| Refused with | When |
| --- | --- |
| `400 lookup_unreadable` | What was typed is not a handle, a DID or the address of a profile |
| `404 lookup_is_here` | The handle is this installation's own name, or one under it |
| `404 lookup_not_found` | Nobody is there: never made, closed, taken down, or put away by its owner |
| `404 lookup_refused` | The handle is under a name on the list of [servers that are refused](#servers-that-are-refused), or `bsky.app` is on it, which refuses the whole of Bluesky |
| `502 lookup_failed` | Bluesky could not be reached or understood |
| `429 too_many_lookups` | The limit |

### Who they are here

A node of type `node.actor.atproto-service.person.v1`, called by their
handle, with what they say of themselves, their avatar and their banner.
Both pictures are copied into this installation's storage, as for somebody
on the fediverse. Their address between servers is `at://` and their DID,
which does not change when their handle does; `origin_domain` is
`bsky.app`, where "View on bsky.app" leads.

A handle can pass from one account to another. When it does, the new
owner of the handle is shown here by their DID, written so it cannot be
mistaken for a handle (`did-plc-….invalid`), until the old account is no
longer kept here; then the new owner gets the handle. An account whose
handle no longer leads to it is shown by its DID in the same way.

Their profile is at `/people/alice.bsky.social`: a username here can have
no dot in it, so a name with one is somebody on Bluesky.

### Their posts

A post is kept only when somebody here follows whoever wrote it.
Everything on Bluesky is open to everybody, so there is no asking who a
post is for.

- **A reply is kept when what it answers is kept here,** and is put under
  it: somebody followed saying a long thing in several posts, or answering
  somebody else who is followed. A reply to something that is not kept
  here is left, since it would stand without what it answers. Replies by
  people nobody here follows are not heard at all.
- **A repost by somebody followed is kept as their repost,** and is in
  the feed as anybody's repost is. What was reposted is usually by
  somebody nobody here follows: it is read from Bluesky, with whoever
  wrote it, and kept for the repost's sake. It is the one way a post
  arrives without its author being followed. A repost taken back goes
  (`atproto_reposts` remembers which post each was of, since taking one
  back says only its name).

It is kept the way a post from the fediverse is:

- **Its words** become Markdown. Links stay links, a name leads to that
  account's page on Bluesky, and a hashtag is a hashtag of this
  installation, made if it is new.
- **With pictures it is a photo post,** up to four (what Bluesky takes),
  each with what its
  author said it shows, copied into this installation's storage. What its
  author marked as not to be shown plainly (nudity, graphic media) is kept
  for its words, without its pictures.
- **A link it carries is given a card** read by this installation itself,
  whether or not the address is in its words.
- **A post it quotes is led to,** with a line at its end; the quoted post
  is not kept.
- **A video is not kept.** A post with words and a video is kept for its
  words; one that is only a video is not kept at all.

When somebody is first followed, their twenty most recent posts are read,
with their answers to themselves, once, so that their profile here is not empty until they next post. A post
taken away on Bluesky goes from here, with what was stored of it. An
account that is closed goes too, with everything of theirs; one that is
only put away for a while is left as it is.

### Hearing what is posted

One connection is held open to Jetstream, Bluesky's feed of everything
done on the network, and told which accounts are wanted: those somebody
here follows, ten thousand at most. With nobody followed, no connection is
made.

- **One copy listens.** Any number of copies of the service may run, and
  one of them holds the turn (a lock in the database). When it stops,
  another takes over within half a minute.
- **Nothing is missed while nobody listens.** The time of the last thing
  dealt with is kept (`atproto_cursor`), and a connection that begins
  again asks for what came after it. Jetstream keeps what was said for a
  limited time; a service that was down for longer than that loses what
  was posted in between.
- **No one Jetstream is relied on.** One of them can fall behind, or stop
  passing things on, while still answering: on 2026-10-13 one of Bluesky's
  four had nothing of the last hour and the other three had all of it,
  and that cannot be told from a quiet evening by listening. So each is
  listened to for ten minutes and then the next, and each begins far
  enough back to hear again what the last three were asked for. What one
  of them kept back arrives from the next, some minutes late. With one
  address only, there is no such cover.
- **A follow that the bus lost** is found within a minute: the service
  asks the graph who is followed, and does not rely on having been told.

### Answering on Bluesky

Somebody here who has a Bluesky account connects it in their settings,
under Privacy, with their handle and **an app password**: a password
Bluesky lets an account make for one app and take back without touching
its own. The card says to use one and where to make it.

- **What is kept.** The password is sent once to the server that holds
  the account, found from the account's lasting name, and exchanged there
  for two tokens. It is not kept. The longer-lived token is kept,
  encrypted under the same key as auth-service's secrets
  (`CRYPT_LOCAL_KEY`); the short one is held in memory. An installation
  with no such key reads Bluesky and writes nothing: nobody is offered the
  card.
- **What goes.** A like or a repost of a post that came from Bluesky; a
  reply under one, or under a reply that itself went there; and a note, a
  photo or video post or an article of the person's own that everybody may
  read, on their own profile. A topic does not go. Connecting an account is saying that such posts are
  posted there, as being on the fediverse is. A post on somebody else's
  profile or a group's does not go.
- **Videos.** A video post goes with its video, a GIF made into a clip
  included, and its title as its first line: the largest size no taller
  than 720 and no heavier than 50 MB, read from storage at an address only
  the services reach and given to the account's server
  (`app.bsky.embed.video`), which Bluesky makes ready to play once the post
  is written. That is the simpler of the two ways Bluesky's documents
  describe; the other, through Bluesky's video service, has it ready
  before the post appears. A video longer than three minutes or heavier
  than that goes as a card that leads to its page here, with its poster;
  so does one Bluesky would not take. An account Bluesky hosts must have
  confirmed its email to post videos, and has a daily limit.
- **Articles.** An article goes as its title, as the first line, then its
  words, cut to what a post holds, with a card that leads to the article
  here: its title, its opening under it, and its cover as its picture,
  given to the account's server as a photo post's pictures are.
- **Pictures and cards.** A photo post goes with up to four pictures and
  what each shows; one with more goes with the first four, and its words
  end with the address of the post here, where they all are. A note goes with the card of the link in it, picture
  and all: Bluesky draws no card of its own from an address in a post, so
  the card is this installation's, as it was kept when the note was made.
  Each picture is given to the account's server first. Bluesky takes two
  megabytes for each: one that weighs more is made smaller, and written
  again as a JPEG when that is not enough. Words become
  what Bluesky takes: Markdown's marks are taken out, links and the first
  ten hashtags are marked as such, and what is longer than three hundred letters is cut
  with a mark to say so; a note so cut carries a card of itself here,
  where the rest is. A post with pictures or a video has no room for a
  card, so when it is cut its words end instead with the address of the
  post here, on a line of its own. A post or a reply that is changed here afterwards
  is not changed there: Bluesky has no changing a post, and taking it down
  to post it again would lose its likes, reposts and replies. Mastodon and
  other servers are sent the change.
- **Following.** Somebody connected who follows an account on Bluesky from
  here follows it there too, as their account: it is the only kind of
  follow Bluesky has, so whoever is followed is told and sees them among
  their followers. An account that follows them already is left as it is,
  and nothing is written. Unfollowing here takes back only a follow that
  was written from here: one made on Bluesky itself stays until it is
  removed there. Whoever has not connected an account follows here and
  nowhere else, and nobody is told.
- **Blocking.** Somebody connected who blocks an account on Bluesky from
  here blocks it on Bluesky too: a block (`app.bsky.graph.block`) is
  written to their account, as Bluesky's own app writes one, and taken back
  when they unblock it here. A block made in Bluesky's app is not this
  installation's to take back. Written whatever their profile here is open
  to: a block keeps somebody away and tells nobody else.
- **Hiding a reply.** Somebody connected who hides a reply from Bluesky
  under a post of theirs that went there, or a reply of somebody's here
  that went there, hides it on Bluesky too: it is added to the post's
  threadgate (`app.bsky.feed.threadgate`, named as the post is), the list
  of hidden replies every Bluesky app reads and tucks away. Shown again
  here, it is taken out. Only that reply is changed: replies hidden in
  Bluesky's own app, and who may reply when that was set there, are left
  as they are. Whether it is hidden is read when the threadgate is
  written, so hiding and showing again quickly writes what is so at the
  end. A threadgate holds 300 hidden replies; past that the oldest go.
  Deleting the post here removes its threadgate with it. A reply hidden in
  Bluesky's own app is not hidden here yet.
- **What comes back.** Each connected account is asked, once a minute,
  what it was told of on Bluesky, as its own app would show it: who liked
  or reposted something written from here, who answered it, and who
  followed the account. Each is kept as the same thing from the fediverse
  is: whoever did it becomes somebody of another server here, though
  nobody here follows them, and their like, repost, reply or follow is
  theirs. The person here is told as of anybody's. Somebody they have
  blocked does none of it. What happened before the account was connected
  is not gone back for.
- **What is taken back there.** An account is told when somebody likes,
  reposts, answers or follows, and not when they think better of it. So
  every five minutes, and once soon after the service starts, what is
  kept here of such things is looked for on Bluesky again: who likes and
  has reposted each post that went out from here, whether each answer is
  still there, and whether each follower still follows. What is no longer
  so is removed here, with the news of it. Nothing is removed on a doubt:
  not when Bluesky could not be asked, not from a list too long to read to
  its end (more than five hundred likes), and not what arrived in the last
  three minutes.
- **What is taken back.** Taking a like or a repost back here, or deleting
  the reply, removes it there.
- **It arrives.** What is to be written waits in a table
  (`atproto_writes`) and is tried until it is done, for two days, as what
  goes to the fediverse is.
- **It is not kept twice.** Bluesky tells of everything an account does,
  and whoever follows their own account here would be told of their own
  reply. Each thing written is remembered by the name it was written
  under (`atproto_sent`) and is known for this installation's own.
- **When it stops working.** An app password that is removed on Bluesky
  ends the connection. The first thing that cannot be written marks the
  account as needing to be connected again, which the card then says.
- **One person, one profile.** A Bluesky account that somebody here
  connected is that person. It is not also kept as somebody of another
  server: what was kept of it before it was connected (its profile here,
  its posts, and the follows of it) is removed when it is connected, and
  looking it up afterwards finds the person here.
- **What they post on Bluesky itself is theirs here.** A post made with
  the connected account in Bluesky's own app is kept here as a post of
  the person's own, on their profile, open to everybody as it is there,
  with its pictures and the card of its link. It is a post like any they
  wrote here: it is given its hashtags, and goes to the fediverse if they
  are on it. It is not sent back to Bluesky; a like or an answer given to
  it here reaches it there; deleted there, it goes from here. Only a post
  that begins a thread, and only from when the account was connected.
- **Disconnecting** forgets the token. What was written stays on Bluesky,
  and can then be removed only there.

| Request | Does |
| --- | --- |
| `GET /atproto/settings` | Whether the viewer has connected an account: `connected`, `handle`, `did`, `broken`. `404 bluesky_writing_off` where the installation does not write to Bluesky |
| `PUT /atproto/settings` `{ "handle": "…", "app_password": "…" }` | Connects it. `404 bluesky_not_found`, `403 bluesky_denied` for a password that is not taken, `409 bluesky_taken` when somebody else here connected that account, `502 bluesky_failed`. Ten tries in ten minutes |
| `DELETE /atproto/settings` | Disconnects |
| `GET /atproto/settings/import-follows` | What that would do, before it is done: `following`, how many accounts the viewer follows on Bluesky; `most`, the most followed here in one go; `posts_of_each`, how many posts of each are read |
| `POST /atproto/settings/import-follows` `{ "limit": 25 }` | Follows here the accounts the viewer's Bluesky account follows already, the ones followed most lately first: `limit` of them, 25 when none is said and 500 at most, leaving out whoever is refused or blocked. Answered `202` at once and carried on with afterwards: each account is kept and has its recent posts read, which for a long list takes minutes. Nothing is written to Bluesky. Three times an hour |

`ATPROTO_PLC_URL` is where the server that holds an account is looked up:
left out, `https://plc.directory`.

### What is not there yet

Likes given on Bluesky to
posts that came from there are not read, nor how many a post has; replies
by people nobody here follows are not read; videos are not kept; a reply
hidden in Bluesky's app is not hidden here.
The list of [servers that are refused](#servers-that-are-refused) is read
by this service and still edited through `activitypub-service`, so an
installation that runs this service alone cannot change it from the web
app.

## Where the code is

| What | Where |
| --- | --- |
| Bluesky | `atproto-service/`: `bsky/` (the reader, a post's words, Jetstream), `cmd/api/handlers/` (`lookup.go`, `posts.go`, `reposts.go`, `backfill.go`, `stream.go`, `events.go`, `own.go` (one's own Bluesky posts brought in), `import.go` (importing follows), `reactions.go` and `verify.go` (what people on Bluesky do in return, and what they took back), `media.go` and `retention.go` (pictures and posts removed after a while); writing: `settings.go`, `out.go`, `hidden.go` (hidden replies, by threadgate), `bsky/session.go`, `bsky/outgoing.go`), tables `atproto_actors`, `atproto_cursor`, `atproto_reposts`, `atproto_accounts`, `atproto_sent`, `atproto_writes` |
| What both protocols share | `anahita-libs/federation/`: pictures (`pictures.go`), removing and restoring them (`media.go`), cards (`cards.go`), refused servers (`blocks.go`), a post's words (`content.go`) |
| The service | `activitypub-service/` |
| Who is on the fediverse | `cmd/api/handlers/handlers.go`: `siteFederates`, `mayFederate`, `federating` |
| WebFinger and host-meta | `cmd/api/handlers/discovery.go` |
| A person's description | `cmd/api/handlers/actor.go`, `ap/actor.go` |
| The addresses of things | `ap/addresses.go` |
| Keys | `keys/`, `cmd/api/handlers/keys.go`, table `actor_keys` |
| Signing and checking requests | `httpsig/` |
| Reading from other servers | `fetch/`, over `anahita-libs/safehttp` |
| Who somebody elsewhere is, and what is believed of it | `remote/` |
| What the bus lost | `cmd/api/handlers/sweep.go`, `data/repositories/missed.go` |
| Replies | `cmd/api/handlers/replies_in.go`, `notes.go` (`answers`) |
| Likes and reposts | `cmd/api/handlers/likes.go`, `reposts.go` |
| Pictures and cards of posts from elsewhere | `cmd/api/handlers/pictures.go`, `cards.go`, over `anahita-libs/federation` |
| Looking somebody up | `cmd/api/handlers/lookup.go`, table `activitypub_actors` |
| Their picture, and their earlier posts | `cmd/api/handlers/avatar.go`, `backfill.go` |
| The inbox, and being followed | `cmd/api/handlers/inbox.go`, table `activitypub_follows` |
| Following from here | `cmd/api/handlers/events.go`, listening for `edge.follow.*` on the bus (`AMQP_ADDR`) |
| Posts going out | `cmd/api/handlers/notes.go`, `ap/note.go`, listening for `node.medium.*`, table `activitypub_objects` |
| Posts coming in | `cmd/api/handlers/inbox.go`: `received`, `withdrawn` |
| Leaving, and profile changes | `cmd/api/handlers/leaving.go`; from the other side, `inbox.go`: `forget`, `refreshed`; the graph's `DeleteRemoteActor` |
| Servers that are refused | `cmd/api/handlers/blocks.go`, `data/repositories/blocks.go`, table `federation_domain_blocks` |
| What NodeInfo says is spoken | `auth-service/cmd/api/handlers/node_info.go`: `federationProtocols` (`FEDERATION_PROTOCOLS`) |
| What may be done with what came from elsewhere | `anahita-libs/permissions` (`remote_test.go` holds the rule); `actor-service/cmd/api/handlers/person.go`: `personTypeOf` |
| What is sent | `cmd/api/handlers/deliver.go`, `ap/activity.go`, table `activitypub_deliveries`, over `anahita-libs/federation` (`queue.go`) |
| The installation's own actor | `cmd/api/handlers/instance.go` |
| A person's choice | `cmd/api/handlers/settings.go`, table `activitypub_accounts` |
| The gateway's routes | `nginx/nginx.conf`: `/.well-known/webfinger`, `/.well-known/host-meta`, `/inbox`, `/ap/`, `/activitypub/` |
| `ACTIVITYPUB_LAB` | Lets the service fetch from private addresses. For [the lab](federation-lab.md) only; refused with `ENV=prod` |
| What it shares with the Bluesky service to come | `anahita-libs/federation`, `anahita-libs/site` |
