# Documentation

Documentation for anahita-services, the Go microservices behind Anahita.

## Contents

1. [Getting started](getting-started.md) — running the stack locally
   - [Requirements](getting-started.md#requirements)
   - [Configure](getting-started.md#configure)
   - [Starting](getting-started.md#starting)
   - [First run](getting-started.md#first-run)
   - [Developing](getting-started.md#developing)
   - [Tests](getting-started.md#tests)
   - [Other targets](getting-started.md#other-targets)
2. [Configuration](configuration.md) — every environment variable
   - [Shared](configuration.md#shared)
   - [Client addresses](configuration.md#client-addresses)
   - [Internal calls](configuration.md#internal-calls)
   - [Site identity](configuration.md#site-identity)
   - [Registration](configuration.md#registration)
   - [Privacy of the installation](configuration.md#privacy-of-the-installation)
   - [First super administrator](configuration.md#first-super-administrator)
   - [OAuth signing and encryption](configuration.md#oauth-signing-and-encryption)
   - [Storage](configuration.md#storage)
   - [Places](configuration.md#places)
   - [Trending](configuration.md#trending)
   - [Mail](configuration.md#mail)
   - [Abuse reports](configuration.md#abuse-reports)
   - [Account cleanup](configuration.md#account-cleanup)
   - [The web shell](configuration.md#the-web-shell)
   - [Video](configuration.md#video)
   - [Download your data](configuration.md#download-your-data)
3. [Architecture](architecture.md) — services, routing and the shared library
   - [Components](architecture.md#components)
   - [Services](architecture.md#services)
   - [Routing](architecture.md#routing)
   - [One service for every kind of actor](architecture.md#one-service-for-every-kind-of-actor)
   - [Naming images](architecture.md#naming-images)
   - [anahita-libs](architecture.md#anahita-libs)
   - [What a node is called elsewhere](architecture.md#what-a-node-is-called-elsewhere)
   - [Naming dates and times](architecture.md#naming-dates-and-times)
   - [The API](api.md) — what is true of every route, and the reference made from the code ([`api/`](api/))
4. [OAuth scopes and clients](oauth.md)
   - [Scopes](oauth.md#scopes)
   - [Seeding clients](oauth.md#seeding-clients)
5. [Registration and agreements](registration.md) — how people join
   - [Where it happens](registration.md#where-it-happens)
   - [Registration modes](registration.md#registration-modes)
   - [What signup asks for](registration.md#what-signup-asks-for)
   - [Email verification](registration.md#email-verification)
   - [Invitations](registration.md#invitations)
   - [The approval queue](registration.md#the-approval-queue)
   - [Protection against bots](registration.md#protection-against-bots)
   - [Terms of Service and Privacy Policy](registration.md#terms-of-service-and-privacy-policy)
   - [Onboarding](registration.md#onboarding)
   - [NodeInfo](registration.md#nodeinfo)
   - [The first super administrator](registration.md#the-first-super-administrator)
6. [Permissions](permissions.md) — who can post, reply, like and add followers
   - [Two separate settings](permissions.md#two-separate-settings)
   - [Access levels](permissions.md#access-levels)
   - [A members-only installation](permissions.md#a-members-only-installation)
   - [Who is always allowed](permissions.md#who-is-always-allowed)
   - [Seeing posts](permissions.md#seeing-posts)
   - [Posting](permissions.md#posting)
   - [Choosing who can see a post](permissions.md#choosing-who-can-see-a-post)
   - [Replies](permissions.md#replies)
   - [Likes](permissions.md#likes)
   - [Following](permissions.md#following)
   - [Invitations](permissions.md#invitations)
   - [Events](permissions.md#events)
   - [Reposts](permissions.md#reposts)
   - [Mentions](permissions.md#mentions)
   - [Quotes](permissions.md#quotes)
   - [Saved posts](permissions.md#saved-posts)
   - [Pinned posts](permissions.md#pinned-posts)
   - [Downloading photos and videos](permissions.md#downloading-photos-and-videos)
   - [Custom feeds](permissions.md#custom-feeds)
   - [Suggestions](permissions.md#suggestions)
   - [Reporting](permissions.md#reporting)
   - [Removing accounts](permissions.md#removing-accounts)
   - [The defaults](permissions.md#the-defaults)
   - [What profile owners can change](permissions.md#what-profile-owners-can-change)
   - [Accepted values](permissions.md#accepted-values)
7. [Signing in](signing-in.md) — passwords, passkeys, emailed codes and their limits
   - [The ways in](signing-in.md#the-ways-in)
   - [Signing in with a code](signing-in.md#signing-in-with-a-code)
   - [What a code can and cannot do](signing-in.md#what-a-code-can-and-cannot-do)
   - [Limits](signing-in.md#limits)
   - [How codes are kept](signing-in.md#how-codes-are-kept)
   - [Resetting a forgotten password](signing-in.md#resetting-a-forgotten-password)
   - [Changing an email address](signing-in.md#changing-an-email-address)
   - [Adding text messages later](signing-in.md#adding-text-messages-later)
8. [Abuse reports](abuse-reports.md) — reporting, the reasons list, and what administrators do
   - [Reports and cases](abuse-reports.md#reports-and-cases)
   - [Reporting something](abuse-reports.md#reporting-something)
   - [The reasons list](abuse-reports.md#the-reasons-list)
   - [Using your own reasons](abuse-reports.md#using-your-own-reasons)
   - [What administrators do](abuse-reports.md#what-administrators-do)
   - [When the reported thing is deleted](abuse-reports.md#when-the-reported-thing-is-deleted)
   - [Who sees what](abuse-reports.md#who-sees-what)
9. [Privacy](privacy.md) — who can see a post, a profile and the site, and the settings an installation makes
   - [The audiences](privacy.md#the-audiences)
   - [Who can see a post](privacy.md#who-can-see-a-post)
   - [Who can see a profile or a group](privacy.md#who-can-see-a-profile-or-a-group)
   - [What the installation allows](privacy.md#what-the-installation-allows)
   - [Setting them](privacy.md#setting-them)
   - [Checking them](privacy.md#checking-them)
   - [What the web app does](privacy.md#what-the-web-app-does)
   - [Where it is enforced, and how it is tested](privacy.md#where-it-is-enforced-and-how-it-is-tested)
   - [Federation](privacy.md#federation)
10. [The language of a post](post-language.md) — what is stored, how it is chosen, and what uses it
   - [What it is for](post-language.md#what-it-is-for)
   - [The tags](post-language.md#the-tags)
   - [On a post](post-language.md#on-a-post)
   - [On a person](post-language.md#on-a-person)
   - [What the web app does](post-language.md#what-the-web-app-does)
   - [What is not built](post-language.md#what-is-not-built)
11. [Photo posts](photos.md) — several images in one post: uploading, storing, changing and removing them
   - [One post, several images](photos.md#one-post-several-images)
   - [Posting](photos.md#posting)
   - [What is stored](photos.md#what-is-stored)
   - [Reading](photos.md#reading)
   - [Changing a post's images](photos.md#changing-a-posts-images)
   - [Downloading](photos.md#downloading)
   - [Removal](photos.md#removal)
   - [Limits](photos.md#limits)
   - [What is not built](photos.md#what-is-not-built)
12. [Account lifecycle](account-lifecycle.md) — disabling, archiving, deleting, purging, and clearing out unused accounts
   - [The four states](account-lifecycle.md#the-four-states)
   - [Deleting and the 30 days](account-lifecycle.md#deleting-and-the-30-days)
   - [What a purge removes](account-lifecycle.md#what-a-purge-removes)
   - [What a purge leaves](account-lifecycle.md#what-a-purge-leaves)
   - [Purging now](account-lifecycle.md#purging-now)
   - [The accounts list](account-lifecycle.md#the-accounts-list)
   - [The sweeper](account-lifecycle.md#the-sweeper)
   - [Groups left without an administrator](account-lifecycle.md#groups-left-without-an-administrator)
13. [Deploying](deploying.md) — a live site on Kubernetes, on any provider
   - [What you need](deploying.md#what-you-need)
   - [How the manifests are laid out](deploying.md#how-the-manifests-are-laid-out)
   - [Hostnames](deploying.md#hostnames)
   - [First deployment](deploying.md#first-deployment)
   - [Certificates between the services](deploying.md#certificates-between-the-services)
   - [Object storage](deploying.md#object-storage)
     - [What is in the bucket](deploying.md#what-is-in-the-bucket)
     - [Moving a bucket to the v1 layout](deploying.md#moving-a-bucket-to-the-v1-layout)
   - [Database, Redis and RabbitMQ](deploying.md#database-redis-and-rabbitmq)
   - [On AWS](deploying.md#on-aws)
   - [Joining the fediverse](deploying.md#joining-the-fediverse)
   - [Upgrading](deploying.md#upgrading)
   - [Running more than one copy of a service](deploying.md#running-more-than-one-copy-of-a-service)
   - [Trying it on your own machine](deploying.md#trying-it-on-your-own-machine)
   - [One machine without Kubernetes](deploying.md#one-machine-without-kubernetes)

14. [Custom feeds](feeds.md) — feeds people build: what one is made of, who sees it, and what they see in it
   - [The home page](feeds.md#the-home-page)
   - [A feed is a node](feeds.md#a-feed-is-a-node)
   - [The definition](feeds.md#the-definition)
   - [Limits](feeds.md#limits)
   - [Who sees a feed, and what they see in it](feeds.md#who-sees-a-feed-and-what-they-see-in-it)
   - [Reading a feed](feeds.md#reading-a-feed)
   - [Keeping feeds on the home page](feeds.md#keeping-feeds-on-the-home-page)
   - [In search](feeds.md#in-search)
   - [Discover](feeds.md#discover)
   - [Endpoints](feeds.md#endpoints)
   - [Where the code is](feeds.md#where-the-code-is)
   - [What is not built](feeds.md#what-is-not-built)
15. [Recommendations](recommendations.md) — suggestions of who to follow: the contract, the reasons, who is never suggested, and running without it
   - [The contract](recommendations.md#the-contract)
   - [Why somebody is suggested](recommendations.md#why-somebody-is-suggested)
   - [Who is never suggested](recommendations.md#who-is-never-suggested)
   - [What each person chooses](recommendations.md#what-each-person-chooses)
   - [Not interested](recommendations.md#not-interested)
   - [Running it, and not running it](recommendations.md#running-it-and-not-running-it)
   - [Where the code is](recommendations.md#where-the-code-is)
   - [What is not built](recommendations.md#what-is-not-built)
16. [Link previews](link-previews.md) — the card a note carries for a link: fetching a page safely, the picture, and how long a preview is kept
   - [Two halves](link-previews.md#two-halves)
   - [Fetching a page safely](link-previews.md#fetching-a-page-safely)
   - [What is read from the page](link-previews.md#what-is-read-from-the-page)
   - [The picture](link-previews.md#the-picture)
   - [How long a preview is kept](link-previews.md#how-long-a-preview-is-kept)
   - [Endpoints](link-previews.md#endpoints)
   - [The card on a note](link-previews.md#the-card-on-a-note)
   - [Running it, and not running it](link-previews.md#running-it-and-not-running-it)
   - [What is not built](link-previews.md#what-is-not-built)
17. [The web shell](web-shell.md) — sending the web app with each page's own title and picture, for links shared elsewhere
   - [What it sends](web-shell.md#what-it-sends)
   - [Which pages are described](web-shell.md#which-pages-are-described)
   - [What is never described](web-shell.md#what-is-never-described)
   - [Pictures](web-shell.md#pictures)
   - [What other servers ask this host](web-shell.md#what-other-servers-ask-this-host)
   - [Running it](web-shell.md#running-it)
   - [Settings](web-shell.md#settings)
   - [Not running it](web-shell.md#not-running-it)
   - [What is not built](web-shell.md#what-is-not-built)
18. [Videos](videos.md) — a video sent straight to storage, the worker that makes a post of it, and how it is played
   - [The parts](videos.md#the-parts)
   - [Sending a video](videos.md#sending-a-video)
   - [What the worker does](videos.md#what-the-worker-does)
   - [What a file has to be](videos.md#what-a-file-has-to-be)
   - [The post](videos.md#the-post)
   - [Playing it](videos.md#playing-it)
   - [Sizes](videos.md#sizes)
   - [Downloading](videos.md#downloading)
   - [Endpoints](videos.md#endpoints)
   - [Settings](videos.md#settings)
   - [Storage](videos.md#storage)
   - [Running it, and not running it](videos.md#running-it-and-not-running-it)
   - [What is not built](videos.md#what-is-not-built)
19. [Download your data](data-export.md) — a person's copy of everything they put here: asking for it, what is in it, and how long it is kept
   - [The parts](data-export.md#the-parts)
   - [Asking for one](data-export.md#asking-for-one)
   - [What is in an archive](data-export.md#what-is-in-an-archive)
   - [What is not](data-export.md#what-is-not)
   - [How it is made](data-export.md#how-it-is-made)
   - [Keeping and deleting](data-export.md#keeping-and-deleting)
   - [Endpoints](data-export.md#endpoints)
   - [Settings](data-export.md#settings)
   - [Running it, and not running it](data-export.md#running-it-and-not-running-it)
   - [What is not built](data-export.md#what-is-not-built)
20. [Federation](federation.md) — taking part in the fediverse and in Bluesky: turning it on, who is on the fediverse, what they are called and what is told about them, reading Bluesky and writing to it as a connected account, and who owns what
   - [What it is](federation.md#what-it-is)
   - [A server on one network, an app on the other](federation.md#a-server-on-one-network-an-app-on-the-other)
   - [Setting it up](federation.md#setting-it-up)
   - [Keeping it running](federation.md#keeping-it-running)
   - [When something does not work](federation.md#when-something-does-not-work)
   - [Who is on the fediverse](federation.md#who-is-on-the-fediverse)
   - [What a person is called](federation.md#what-a-person-is-called)
   - [What other servers are told](federation.md#what-other-servers-are-told)
   - [Looking somebody up on another server](federation.md#looking-somebody-up-on-another-server)
   - [Being followed from another server](federation.md#being-followed-from-another-server)
   - [Following somebody on another server](federation.md#following-somebody-on-another-server)
   - [Posts going out](federation.md#posts-going-out)
   - [Posts coming in](federation.md#posts-coming-in)
   - [What can be done with what came from elsewhere](federation.md#what-can-be-done-with-what-came-from-elsewhere)
   - [The inbox](federation.md#the-inbox)
   - [What is sent, and when it does not arrive](federation.md#what-is-sent-and-when-it-does-not-arrive)
   - [Signing keys and signed requests](federation.md#signing-keys-and-signed-requests)
   - [The API](federation.md#the-api)
   - [Leaving, and what follows a person's changes](federation.md#leaving-and-what-follows-a-persons-changes)
   - [Servers that are refused](federation.md#servers-that-are-refused)
   - [Replies](federation.md#replies)
   - [Likes and reposts](federation.md#likes-and-reposts)
   - [Pictures that are removed](federation.md#pictures-that-are-removed)
   - [Posts and people that are removed](federation.md#posts-and-people-that-are-removed)
   - [Bluesky](federation.md#bluesky)
   - [Where the code is](federation.md#where-the-code-is)
21. [The federation lab](federation-lab.md) — trying federation on one machine: names, certificates, and another server to talk to
   - [Why it is needed](federation-lab.md#why-it-is-needed)
   - [What is in it](federation-lab.md#what-is-in-it)
   - [Starting it](federation-lab.md#starting-it)
   - [Opening it in a browser](federation-lab.md#opening-it-in-a-browser)
   - [Somebody to talk to](federation-lab.md#somebody-to-talk-to)
   - [Having GoToSocial look somebody up](federation-lab.md#having-gotosocial-look-somebody-up)
   - [Turning the fediverse on and off for somebody](federation-lab.md#turning-the-fediverse-on-and-off-for-somebody)
   - [Following somebody here from GoToSocial](federation-lab.md#following-somebody-here-from-gotosocial)
   - [Following somebody on GoToSocial from here](federation-lab.md#following-somebody-on-gotosocial-from-here)
   - [Posts, both ways](federation-lab.md#posts-both-ways)
   - [Looking somebody on GoToSocial up from here](federation-lab.md#looking-somebody-on-gotosocial-up-from-here)
   - [Asking as another server would](federation-lab.md#asking-as-another-server-would)
   - [Stopping and emptying it](federation-lab.md#stopping-and-emptying-it)
   - [Real servers, from the lab](federation-lab.md#real-servers-from-the-lab)
   - [What it is not](federation-lab.md#what-it-is-not)
   - [Where the code is](federation-lab.md#where-the-code-is)
22. [Search](search.md) — the order results come in, old and useful counting as much as new, and how an operator tunes it
   - [Two orders](search.md#two-orders)
   - [Most relevant](search.md#most-relevant)
   - [Old and useful counts as much as new](search.md#old-and-useful-counts-as-much-as-new)
   - [Tuning it](search.md#tuning-it)
   - [Each person's choices](search.md#each-persons-choices)
   - [What is found at all](search.md#what-is-found-at-all)
   - [Where the code is](search.md#where-the-code-is)
   - [What is not built](search.md#what-is-not-built)
23. [Hashtags and places](hashtags.md) — how a post gets its hashtags, at most ten hashtags and five places, and the hashtags offered while writing
   - [How a post gets its hashtags](hashtags.md#how-a-post-gets-its-hashtags)
   - [At most ten hashtags](hashtags.md#at-most-ten-hashtags)
   - [At most five places](hashtags.md#at-most-five-places)
   - [Suggestions while writing](hashtags.md#suggestions-while-writing)
   - [What the web app does](hashtags.md#what-the-web-app-does)
   - [Where the code is](hashtags.md#where-the-code-is)
24. [Websites](websites.md) — a website's feed as a profile to follow: connecting a site, proving its domain, its feed read and posted as notes, replies as the website, and on the fediverse
   - [Who does what](websites.md#who-does-what)
   - [Connecting a website](websites.md#connecting-a-website)
   - [Proving a domain](websites.md#proving-a-domain)
   - [Reading the feed](websites.md#reading-the-feed)
   - [Its posts](websites.md#its-posts)
   - [Replies as the website](websites.md#replies-as-the-website)
   - [On the fediverse](websites.md#on-the-fediverse)
   - [WebSub](websites.md#websub)
   - [Settings](websites.md#settings)
   - [Where the code is](websites.md#where-the-code-is)

## Plans

Design documents for work in progress and work already done, in
[`plans/`](plans/). They record why things were built the way they were, and
are not kept up to date once the work lands — the pages above are.

The order they are meant to be done in, with estimates and dependencies, is
the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md)
in anahita-web. Finished plans move to [`plans/done/`](plans/done/).

| Plan | Roadmap item | Status |
| --- | --- | --- |
| [Recommendation service](plans/done/recommendation-service.md) | 3 | built 2026-10-11 |
| [Sign in with a code; codes instead of links](plans/done/login-code.md) | 6, 6b | done 2026-10-03 (step-up by code left out by decision) |
| [Notes as replies](plans/done/notes-as-replies.md) | 7 | done 2026-10-05: every comment is a reply, a post's author chooses who can reply, and comment-service is removed |
| [Federation: ActivityPub and AT Protocol](plans/federation.md) | 8 | phase 0 done 2026-10-12; phase 1, ActivityPub, mostly built 2026-10-13: see [Federation](federation.md). AT Protocol built 2026-10-13 as connected accounts: everybody reads Bluesky, and whoever connects their own account posts, replies, likes, reposts and follows there. Hosting Bluesky identities on a server of the site's own is [plans/atproto-server.md](plans/atproto-server.md), to be decided |
| [Keeping what comes from other networks within bounds](plans/remote-content-limits.md) | 47 | built 2026-10-13 (`REMOTE_*`); videos wait until videos are brought in |
| [Anahita as an AT Protocol server](plans/atproto-server.md) | 51 | recorded 2026-10-14; to be decided in the future |
| [Link previews](plans/done/link-previews.md) | 10 | built 2026-10-11, with the web shell of its part B |
| [Downloading what a post holds](plans/done/downloads.md) | 37 | built 2026-10-11 |
| [Invitations](plans/done/actor-invitations.md) | 11 | done 2026-10-10 |
| [Events](plans/done/events.md) | 12 | done 2026-10-10 |
| [Feed generators (custom feeds)](plans/done/feed-generators.md) | 14 | done 2026-10-11 |
| [Video: video-service and a transcoding worker](plans/done/video-service.md) | 16 | built 2026-10-11 |
| [Creative Commons licences](plans/content-licences.md) | 19 | kept for later (2026-10-04); look again after items 16 and 17 |
| [Download your data](plans/done/data-export.md) | 21 | built 2026-10-11 |
| [Post insights](plans/post-insights.md) | 22, set aside: a post's activity was built instead | not started |
| [Profile tabs for replies and reposts](plans/done/profile-tabs-and-reposts.md) | 24 | done 2026-10-05 |
| [Quote posts](plans/done/quote-posts.md) | 25 | done 2026-10-05 |
| [Verified domains](plans/verified-domains.md) | 26 | not started |
| [Collections](plans/collections.md) | 34 | shape decided 2026-10-06; kept for later, not to be built yet |
| [Websites: a site's feed as something to follow](plans/done/website.md) | 30 | done 2026-10-17: syndication-service and syndication-worker, a verified domain's feed posted as notes, replies as the website, an Organization on the fediverse, its own Bluesky account ([Websites](websites.md)) |
| [One clean schema](plans/done/migration-baseline.md) | 27 | done 2026-10-17: the migrations folded into `init.sql` and `legacy/upgrade.sql`, the PHP upgrade fixed, `make schema_test`, NULL only where it means something ([The database](architecture.md#the-database)) |
| [The API reference, made from the code](plans/done/api-reference.md) | 38 | done 2026-10-15: every route described, `docs/api/index.html`, answers checked in the handler tests |
| [Documentation agents can use, and an MCP server](plans/docs-for-agents.md) | 39 | planned 2026-10-11; place in the order not decided |
| [Anahita Recipes: a starting configuration for each kind of site](plans/recipes.md) | 40 | planned 2026-10-11; place in the order not decided |
| [Animated images: GIF, APNG and animated WebP, through the video service](plans/done/animated-images.md) | 41 | done 2026-10-14: GIFs and animated PNGs (not WebP), and video posts going to other servers and Bluesky |
| [A command-line client for Anahita](plans/cli-client.md) | 42 | recorded 2026-10-12 for later; waits for the API reference |
| [Translating posts](plans/translation.md) | 43 | recorded 2026-10-12 for later: LibreTranslate and DeepL behind one interface |
| [Replies of any kind: pictures, a GIF or a video](plans/done/replies-of-any-kind.md) | 53 | done 2026-10-14 |
| [Hiding a reply from others, across servers](plans/done/hiding-replies-across-servers.md) | 44 | done 2026-10-15: a block hides what the blocked person left, for every reader, and is told to the fediverse; a reply hidden here is hidden on Bluesky; a person's block does not reach their groups (item 55 is blocking by a group) |
| [auth-service hardening](plans/auth-service-hardening.md) | 0b, 0c | 0b and 0c done 2026-10-03; the plan's "Not checked" list is still open |
| [Actor profile management](plans/actor-profile-management.md) | — | partly done; the rest is in item 8 |
| [Signup, registration modes, and agreements](plans/done/signup-and-onboarding.md) | — | done |
| [Onboarding](plans/done/onboarding.md) | — | done (phase 3 moved to the recommendation service) |
| [Dissolve `people_people`](plans/done/dissolve-people-people.md) | — | done |
| [Actor lifecycle states](plans/done/actor-lifecycle-states.md) | — | done (archiving media deferred) |
| [Feed access leak](plans/done/feed-access-leak.md) | 0 | done 2026-10-03 |
| [Remove unused legacy node columns](plans/done/remove-unused-node-columns.md) | 0d | done 2026-10-02 |
| [Remove private notes](plans/done/remove-private-notes.md) | 1 | done 2026-10-03 |
| [Abuse reports](plans/done/abuse-report-service.md) | 4b | done 2026-10-03 |
| [Complete hard delete and bulk cleanup](plans/done/actor-hard-delete.md) | 5 | done 2026-10-04 (the web part not yet tried in a browser) |
| [One name for timestamps: `_at`](plans/done/timestamp-names.md) | 28 | done 2026-10-04 |
| [One identity image service: avatars and covers merged](plans/done/identity-image-service.md) | 31 | done 2026-10-05 (nothing needing a signed-in session was tried) |
| [One actor service: people and groups merged](plans/done/actor-service.md) | 29 | done 2026-10-05 (nothing needing a signed-in session was tried) |
| [Several images per photo post; photo sets removed](plans/done/photo-albums.md) | 17, 18 | 18 done 2026-10-03; 17 done 2026-10-05 (posting and the web parts not yet tried in a browser) |
| [Language on posts](plans/done/content-language.md) | 20 | done 2026-10-05 in a smaller form: the author chooses, nothing is detected |
| [Instance privacy and the composer audience picker](plans/done/instance-privacy.md) | 13 | done 2026-10-05 (the web parts not yet tried in a browser) |
| [Cloud-neutral storage and deployment](plans/done/cloud-neutral.md) | 9 | done 2026-10-03 (non-AWS providers other than Garage untested) |
