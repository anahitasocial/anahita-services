# Documentation

Documentation for anahita-services, the Go microservices behind Anahita.

## Where to begin

Ways in, by what you are doing. Each is a short page of links, in the
order to read them.

| To | Read |
| --- | --- |
| Understand the ideas behind it | [Why Anahita](why-anahita.md) |
| Put an instance on the web | [Setting up](setting-up.md) |
| Change how it looks, what it says and what it allows | [Customising](customising.md) |
| Run it from day to day: upgrades, storage, accounts, reports | [Operating](operating.md) |
| Write an app or a service against it | [Developing](developing.md) |
| Ask an AI agent about Anahita, from the docs | [Anahita MCP](ai-agents.md) |
| Call the services from your own, over gRPC | [Services over gRPC](grpc.md) |
| Know what the licences allow | [Licensing](licensing.md) |

Changing Anahita's own code? Begin with [AGENTS.md](../AGENTS.md).

## Contents

The pages, grouped by what they are about, each group in the order to read it.

### Start here

1. [Getting started](getting-started.md) — running the stack locally
   1. [Requirements](getting-started.md#requirements)
   2. [Configure](getting-started.md#configure)
   3. [Starting](getting-started.md#starting)
   4. [First run](getting-started.md#first-run)
   5. [Developing](getting-started.md#developing)
   6. [Tests](getting-started.md#tests)
   7. [Other targets](getting-started.md#other-targets)
   8. [With an AI agent](getting-started.md#with-an-ai-agent)
2. [Setting up](setting-up.md) — putting an instance on the web, step by step
3. [Configuration](configuration.md) — every environment variable
   1. [Shared](configuration.md#shared)
   2. [Client addresses](configuration.md#client-addresses)
   3. [Internal calls](configuration.md#internal-calls)
   4. [Site identity](configuration.md#site-identity)
   5. [Registration](configuration.md#registration)
   6. [Privacy of the installation](configuration.md#privacy-of-the-installation)
   7. [First super administrator](configuration.md#first-super-administrator)
   8. [OAuth signing and encryption](configuration.md#oauth-signing-and-encryption)
   9. [Storage](configuration.md#storage)
   10. [Locations](configuration.md#locations)
   11. [Notifications](configuration.md#notifications)
   12. [Trending](configuration.md#trending)
   13. [Mail](configuration.md#mail)
   14. [Abuse reports](configuration.md#abuse-reports)
   15. [Account cleanup](configuration.md#account-cleanup)
   16. [The web shell](configuration.md#the-web-shell)
   17. [Video](configuration.md#video)
   18. [Download your data](configuration.md#download-your-data)
4. [Deploying](deploying.md) — a live site on Kubernetes, on any provider
   1. [What you need](deploying.md#what-you-need)
   2. [How the manifests are laid out](deploying.md#how-the-manifests-are-laid-out)
   3. [Hostnames](deploying.md#hostnames)
   4. [First deployment](deploying.md#first-deployment)
   5. [Certificates between the services](deploying.md#certificates-between-the-services)
   6. [Object storage](deploying.md#object-storage)
   7. [Database, Redis and RabbitMQ](deploying.md#database-redis-and-rabbitmq)
   8. [On AWS](deploying.md#on-aws)
   9. [Joining the fediverse](deploying.md#joining-the-fediverse)
   10. [Upgrading](deploying.md#upgrading)
   11. [Running more than one copy of a service](deploying.md#running-more-than-one-copy-of-a-service)
   12. [Trying it on your own machine](deploying.md#trying-it-on-your-own-machine)
   13. [One machine without Kubernetes](deploying.md#one-machine-without-kubernetes)
5. [Releases](releases.md) — Embryo and Birth, and what a version number says
   - [Version numbers](releases.md#version-numbers)
   - [Making one](releases.md#making-one)
   - [What is rebuilt](releases.md#what-is-rebuilt)
6. [Licensing](licensing.md) — what the licences allow, for running a site and for building on it

### AI agents

1. [Anahita MCP](ai-agents.md) — the docs and the API reference in Claude, Gemini, Cursor or VS Code, over MCP
   1. [Adding it](ai-agents.md#adding-it)
   2. [What it can do](ai-agents.md#what-it-can-do)
   3. [What it keeps](ai-agents.md#what-it-keeps)
   4. [Without MCP](ai-agents.md#without-mcp)
   5. [Your own copy](ai-agents.md#your-own-copy)

### Concepts

1. [Why Anahita](why-anahita.md) — the choices behind it: software for a community, the principles, what it does not do, and why two networks
2. [The graph](the-graph.md) — nodes and edges: how their types are named, every kind there is, and who may see a node
3. [How things are found](how-things-are-found.md) — the home page, custom feeds, search, suggestions and trending: what each orders by, and what is never used
4. [Features](features.md) — what Anahita does, by the 128 questions FediDB compares fediverse software by, and what is not built yet

### Run a site

1. [Operating](operating.md) — from day to day: upgrades, storage, accounts, reports
2. [Customising](customising.md) — how it looks, what it says and what it allows
3. [Registration and agreements](registration.md) — how people join
   1. [Where it happens](registration.md#where-it-happens)
   2. [Registration modes](registration.md#registration-modes)
   3. [What signup asks for](registration.md#what-signup-asks-for)
   4. [Email verification](registration.md#email-verification)
   5. [Invitations](registration.md#invitations)
   6. [The approval queue](registration.md#the-approval-queue)
   7. [Protection against bots](registration.md#protection-against-bots)
   8. [Terms of Service and Privacy Policy](registration.md#terms-of-service-and-privacy-policy)
   9. [Onboarding](registration.md#onboarding)
   10. [NodeInfo](registration.md#nodeinfo)
   11. [The first super administrator](registration.md#the-first-super-administrator)
   12. [Where the code is](registration.md#where-the-code-is)
4. [Privacy](privacy.md) — who can see a post, a profile and the site, and the settings an installation makes
   1. [The audiences](privacy.md#the-audiences)
   2. [Who can see a post](privacy.md#who-can-see-a-post)
   3. [Who can see a profile or a group](privacy.md#who-can-see-a-profile-or-a-group)
   4. [What the installation allows](privacy.md#what-the-installation-allows)
   5. [Setting them](privacy.md#setting-them)
   6. [Checking them](privacy.md#checking-them)
   7. [What the web app does](privacy.md#what-the-web-app-does)
   8. [Where it is enforced, and how it is tested](privacy.md#where-it-is-enforced-and-how-it-is-tested)
   9. [Federation](privacy.md#federation)
5. [Abuse reports](abuse-reports.md) — reporting, the reasons list, and what administrators do
   1. [Reports and cases](abuse-reports.md#reports-and-cases)
   2. [Reporting something](abuse-reports.md#reporting-something)
   3. [The reasons list](abuse-reports.md#the-reasons-list)
   4. [Using your own reasons](abuse-reports.md#using-your-own-reasons)
   5. [Breaking a rule](abuse-reports.md#breaking-a-rule)
   6. [A group's own reports](abuse-reports.md#a-groups-own-reports)
   7. [What administrators do](abuse-reports.md#what-administrators-do)
   8. [When the reported thing is deleted](abuse-reports.md#when-the-reported-thing-is-deleted)
   9. [Who sees what](abuse-reports.md#who-sees-what)
   10. [Where the code is](abuse-reports.md#where-the-code-is)
6. [Account lifecycle](account-lifecycle.md) — disabling, archiving, deleting, purging, and clearing out unused accounts
   1. [The four states](account-lifecycle.md#the-four-states)
   2. [Deleting and the 30 days](account-lifecycle.md#deleting-and-the-30-days)
   3. [What a purge removes](account-lifecycle.md#what-a-purge-removes)
   4. [What a purge leaves](account-lifecycle.md#what-a-purge-leaves)
   5. [Purging now](account-lifecycle.md#purging-now)
   6. [The accounts list](account-lifecycle.md#the-accounts-list)
   7. [The sweeper](account-lifecycle.md#the-sweeper)
   8. [Groups left without an administrator](account-lifecycle.md#groups-left-without-an-administrator)
   9. [Where the code is](account-lifecycle.md#where-the-code-is)

### Profiles

What every profile has, then each kind's own. The API reference has the same
five, under Profiles.

1. [Profiles](profiles.md) — what every person, group, event and website has alike, and each kind's address
   1. [What every profile has](profiles.md#what-every-profile-has)
   2. [Its address](profiles.md#its-address)
   3. [Its admins](profiles.md#its-admins)
   4. [The API](profiles.md#the-api)
   5. [Where the code is](profiles.md#where-the-code-is)
2. [People](people.md) — somebody with an account: what only a person has
   1. [Their account](people.md#their-account)
   2. [What only a person has](people.md#what-only-a-person-has)
   3. [The API](people.md#the-api)
   4. [Where the code is](people.md#where-the-code-is)
3. [Groups](groups.md) — made by members, looked after by admins, followed by people
   1. [Making one](groups.md#making-one)
   2. [What only a group has](groups.md#what-only-a-group-has)
   3. [The API](groups.md#the-api)
   4. [Where the code is](groups.md#where-the-code-is)
4. [Events](events.md) — a group with a time and a place: going or maybe, what is on, reminders
   1. [Who may do what](events.md#who-may-do-what)
   2. [What is on](events.md#what-is-on)
   3. [Reminders and when it is over](events.md#reminders-and-when-it-is-over)
   4. [The API](events.md#the-api)
   5. [Where the code is](events.md#where-the-code-is)
5. [Websites](websites.md) — a website's feed as a profile, under Fediverse and Bluesky below

### Accounts and the graph

1. [Signing in](signing-in.md) — passwords, passkeys, emailed codes and their limits
   1. [The ways in](signing-in.md#the-ways-in)
   2. [Signing in with a code](signing-in.md#signing-in-with-a-code)
   3. [What a code can and cannot do](signing-in.md#what-a-code-can-and-cannot-do)
   4. [Limits](signing-in.md#limits)
   5. [How codes are kept](signing-in.md#how-codes-are-kept)
   6. [Resetting a forgotten password](signing-in.md#resetting-a-forgotten-password)
   7. [Changing an email address](signing-in.md#changing-an-email-address)
   8. [Adding text messages later](signing-in.md#adding-text-messages-later)
   9. [Where the code is](signing-in.md#where-the-code-is)
2. [Permissions](permissions.md) — who can post, reply, like and add followers
   1. [Two separate settings](permissions.md#two-separate-settings)
   2. [Access levels](permissions.md#access-levels)
   3. [A members-only installation](permissions.md#a-members-only-installation)
   4. [Who is always allowed](permissions.md#who-is-always-allowed)
   5. [Seeing posts](permissions.md#seeing-posts)
   6. [Choosing who can see a post](permissions.md#choosing-who-can-see-a-post)
   7. [Posting](permissions.md#posting)
   8. [Replies](permissions.md#replies)
   9. [Likes](permissions.md#likes)
   10. [Following](permissions.md#following)
   11. [Invitations](permissions.md#invitations)
   12. [Events](permissions.md#events)
   13. [Reposts](permissions.md#reposts)
   14. [Mentions](permissions.md#mentions)
   15. [People in pictures and videos](permissions.md#people-in-pictures-and-videos)
   16. [Quotes](permissions.md#quotes)
   17. [Saved posts](permissions.md#saved-posts)
   18. [Pinned posts](permissions.md#pinned-posts)
   19. [Downloading photos and videos](permissions.md#downloading-photos-and-videos)
   20. [Custom feeds](permissions.md#custom-feeds)
   21. [Suggestions](permissions.md#suggestions)
   22. [Reporting](permissions.md#reporting)
   23. [Removing accounts](permissions.md#removing-accounts)
   24. [The defaults](permissions.md#the-defaults)
   25. [What profile owners can change](permissions.md#what-profile-owners-can-change)
   26. [Accepted values](permissions.md#accepted-values)
   27. [Where the code is](permissions.md#where-the-code-is)
3. [Notifications](notifications.md) — what people are told in the app, never by email: the kinds, who is told and who never is, and how a notification goes with what it tells of
   1. [No email](notifications.md#no-email)
   2. [What a notification is](notifications.md#what-a-notification-is)
   3. [The kinds](notifications.md#the-kinds)
   4. [Only what its reader may see](notifications.md#only-what-its-reader-may-see)
   5. [Who is never told](notifications.md#who-is-never-told)
   6. [When what it tells of goes](notifications.md#when-what-it-tells-of-goes)
   7. [Choosing what one is told about](notifications.md#choosing-what-one-is-told-about)
   8. [Several as one](notifications.md#several-as-one)
   9. [What can be done from one](notifications.md#what-can-be-done-from-one)
   10. [Read, and how long they are kept](notifications.md#read-and-how-long-they-are-kept)
   11. [Delivery](notifications.md#delivery)
   12. [The API](notifications.md#the-api)
4. [Recommendations](recommendations.md) — suggestions of who to follow: the contract, the reasons, who is never suggested, and running without it
   1. [The contract](recommendations.md#the-contract)
   2. [Why somebody is suggested](recommendations.md#why-somebody-is-suggested)
   3. [Who is never suggested](recommendations.md#who-is-never-suggested)
   4. [What each person chooses](recommendations.md#what-each-person-chooses)
   5. [Not interested](recommendations.md#not-interested)
   6. [Running it, and not running it](recommendations.md#running-it-and-not-running-it)
   7. [Where the code is](recommendations.md#where-the-code-is)
   8. [What is not built](recommendations.md#what-is-not-built)
5. [Download your data](data-export.md) — a person's copy of everything they put here: asking for it, what is in it, and how long it is kept
   1. [The parts](data-export.md#the-parts)
   2. [Asking for one](data-export.md#asking-for-one)
   3. [What is in an archive](data-export.md#what-is-in-an-archive)
   4. [What is not](data-export.md#what-is-not)
   5. [How it is made](data-export.md#how-it-is-made)
   6. [Keeping and deleting](data-export.md#keeping-and-deleting)
   7. [Endpoints](data-export.md#endpoints)
   8. [Settings](data-export.md#settings)
   9. [Running it, and not running it](data-export.md#running-it-and-not-running-it)
   10. [Where the code is](data-export.md#where-the-code-is)
   11. [What is not built](data-export.md#what-is-not-built)

### Posts

1. [Photo posts](photos.md) — several images in one post: uploading, storing, changing and removing them
   1. [One post, several images](photos.md#one-post-several-images)
   2. [Posting](photos.md#posting)
   3. [What is stored](photos.md#what-is-stored)
   4. [Reading](photos.md#reading)
   5. [Changing a post's images](photos.md#changing-a-posts-images)
   6. [Downloading](photos.md#downloading)
   7. [Removal](photos.md#removal)
   8. [Limits](photos.md#limits)
   9. [Where the code is](photos.md#where-the-code-is)
   10. [What is not built](photos.md#what-is-not-built)
2. [Videos](videos.md) — a video sent straight to storage, the worker that makes a post of it, and how it is played
   1. [The parts](videos.md#the-parts)
   2. [Sending a video](videos.md#sending-a-video)
   3. [What the worker does](videos.md#what-the-worker-does)
   4. [What a file has to be](videos.md#what-a-file-has-to-be)
   5. [The post](videos.md#the-post)
   6. [Playing it](videos.md#playing-it)
   7. [Sizes](videos.md#sizes)
   8. [GIFs and animated PNGs](videos.md#gifs-and-animated-pngs)
   9. [Downloading](videos.md#downloading)
   10. [Endpoints](videos.md#endpoints)
   11. [Settings](videos.md#settings)
   12. [Storage](videos.md#storage)
   13. [Running it, and not running it](videos.md#running-it-and-not-running-it)
   14. [Where the code is](videos.md#where-the-code-is)
   15. [What is not built](videos.md#what-is-not-built)
3. [Link previews](link-previews.md) — the card a note carries for a link: fetching a page safely, the picture, and how long a preview is kept
   1. [Two halves](link-previews.md#two-halves)
   2. [Fetching a page safely](link-previews.md#fetching-a-page-safely)
   3. [What is read from the page](link-previews.md#what-is-read-from-the-page)
   4. [The picture](link-previews.md#the-picture)
   5. [How long a preview is kept](link-previews.md#how-long-a-preview-is-kept)
   6. [Endpoints](link-previews.md#endpoints)
   7. [The card on a note](link-previews.md#the-card-on-a-note)
   8. [Running it, and not running it](link-previews.md#running-it-and-not-running-it)
   9. [Cards in long writing](link-previews.md#cards-in-long-writing)
   10. [Where the code is](link-previews.md#where-the-code-is)
   11. [What is not built](link-previews.md#what-is-not-built)
4. [Hashtags and locations](hashtags.md) — how a post gets its hashtags, at most ten hashtags and five locations, and the hashtags offered while writing
   1. [How a post gets its hashtags](hashtags.md#how-a-post-gets-its-hashtags)
   2. [At most ten hashtags](hashtags.md#at-most-ten-hashtags)
   3. [At most five locations](hashtags.md#at-most-five-locations)
   4. [Suggestions while writing](hashtags.md#suggestions-while-writing)
   5. [A hashtag's and a location's posts](hashtags.md#a-hashtags-and-a-locations-posts)
   6. [What the web app does](hashtags.md#what-the-web-app-does)
   7. [Where the code is](hashtags.md#where-the-code-is)
5. [The language of a post](post-language.md) — what is stored, how it is chosen, and what uses it
   1. [What it is for](post-language.md#what-it-is-for)
   2. [The tags](post-language.md#the-tags)
   3. [On a post](post-language.md#on-a-post)
   4. [On a person](post-language.md#on-a-person)
   5. [What the web app does](post-language.md#what-the-web-app-does)
   6. [Where the code is](post-language.md#where-the-code-is)
   7. [What is not built](post-language.md#what-is-not-built)
6. [Custom feeds](feeds.md) — feeds people build: what one is made of, who sees it, and what they see in it
   1. [The home page](feeds.md#the-home-page)
   2. [A feed is a node](feeds.md#a-feed-is-a-node)
   3. [The definition](feeds.md#the-definition)
   4. [Limits](feeds.md#limits)
   5. [Who sees a feed, and what they see in it](feeds.md#who-sees-a-feed-and-what-they-see-in-it)
   6. [Reading a feed](feeds.md#reading-a-feed)
   7. [Keeping feeds on the home page](feeds.md#keeping-feeds-on-the-home-page)
   8. [In search](feeds.md#in-search)
   9. [Discover](feeds.md#discover)
   10. [Endpoints](feeds.md#endpoints)
   11. [Where the code is](feeds.md#where-the-code-is)
   12. [What is not built](feeds.md#what-is-not-built)
7. [Search](search.md) — the order results come in, old and useful counting as much as new, and how an operator tunes it
   1. [Two orders](search.md#two-orders)
   2. [Most relevant](search.md#most-relevant)
   3. [Old and useful counts as much as new](search.md#old-and-useful-counts-as-much-as-new)
   4. [Tuning it](search.md#tuning-it)
   5. [Each person's choices](search.md#each-persons-choices)
   6. [What is found at all](search.md#what-is-found-at-all)
   7. [Where the code is](search.md#where-the-code-is)
   8. [What is not built](search.md#what-is-not-built)

### Fediverse and Bluesky

1. [Federation](federation.md) — taking part in the fediverse and in Bluesky: turning it on, who is on the fediverse, what they are called and what is told about them, reading Bluesky and writing to it as a connected account, and who owns what
   1. [What it is](federation.md#what-it-is)
   2. [A server on one network, an app on the other](federation.md#a-server-on-one-network-an-app-on-the-other)
   3. [Setting it up](federation.md#setting-it-up)
   4. [Keeping it running](federation.md#keeping-it-running)
   5. [When something does not work](federation.md#when-something-does-not-work)
   6. [Who is on the fediverse](federation.md#who-is-on-the-fediverse)
   7. [What a person is called](federation.md#what-a-person-is-called)
   8. [What other servers are told](federation.md#what-other-servers-are-told)
   9. [Looking somebody up on another server](federation.md#looking-somebody-up-on-another-server)
   10. [Being followed from another server](federation.md#being-followed-from-another-server)
   11. [Following somebody on another server](federation.md#following-somebody-on-another-server)
   12. [Posts going out](federation.md#posts-going-out)
   13. [Posts coming in](federation.md#posts-coming-in)
   14. [What can be done with what came from elsewhere](federation.md#what-can-be-done-with-what-came-from-elsewhere)
   15. [The inbox](federation.md#the-inbox)
   16. [What is sent, and when it does not arrive](federation.md#what-is-sent-and-when-it-does-not-arrive)
   17. [Signing keys and signed requests](federation.md#signing-keys-and-signed-requests)
   18. [The API](federation.md#the-api)
   19. [Leaving, and what follows a person's changes](federation.md#leaving-and-what-follows-a-persons-changes)
   20. [Servers that are refused](federation.md#servers-that-are-refused)
   21. [Replies](federation.md#replies)
   22. [Likes and reposts](federation.md#likes-and-reposts)
   23. [Pictures that are removed](federation.md#pictures-that-are-removed)
   24. [Posts and people that are removed](federation.md#posts-and-people-that-are-removed)
   25. [Bluesky](federation.md#bluesky)
   26. [Where the code is](federation.md#where-the-code-is)
2. [Websites](websites.md) — a website's feed as a profile to follow: connecting a site, proving its domain, its feed read and posted as notes, replies as the website, and on the fediverse
   1. [Who does what](websites.md#who-does-what)
   2. [Connecting a website](websites.md#connecting-a-website)
   3. [Proving a domain](websites.md#proving-a-domain)
   4. [Reading the feed](websites.md#reading-the-feed)
   5. [Its posts](websites.md#its-posts)
   6. [Replies as the website](websites.md#replies-as-the-website)
   7. [On the fediverse](websites.md#on-the-fediverse)
   8. [WebSub](websites.md#websub)
   9. [Settings](websites.md#settings)
   10. [Where the code is](websites.md#where-the-code-is)
3. [The federation lab](federation-lab.md) — trying federation on one machine: names, certificates, and another server to talk to
   1. [Why it is needed](federation-lab.md#why-it-is-needed)
   2. [What is in it](federation-lab.md#what-is-in-it)
   3. [Starting it](federation-lab.md#starting-it)
   4. [Opening it in a browser](federation-lab.md#opening-it-in-a-browser)
   5. [Somebody to talk to](federation-lab.md#somebody-to-talk-to)
   6. [Having GoToSocial look somebody up](federation-lab.md#having-gotosocial-look-somebody-up)
   7. [Turning the fediverse on and off for somebody](federation-lab.md#turning-the-fediverse-on-and-off-for-somebody)
   8. [Following somebody here from GoToSocial](federation-lab.md#following-somebody-here-from-gotosocial)
   9. [Following somebody on GoToSocial from here](federation-lab.md#following-somebody-on-gotosocial-from-here)
   10. [Posts, both ways](federation-lab.md#posts-both-ways)
   11. [Looking somebody on GoToSocial up from here](federation-lab.md#looking-somebody-on-gotosocial-up-from-here)
   12. [Asking as another server would](federation-lab.md#asking-as-another-server-would)
   13. [Stopping and emptying it](federation-lab.md#stopping-and-emptying-it)
   14. [Real servers, from the lab](federation-lab.md#real-servers-from-the-lab)
   15. [What it is not](federation-lab.md#what-it-is-not)
   16. [Where the code is](federation-lab.md#where-the-code-is)

### Build on Anahita

1. [Developing](developing.md) — writing an app or a service against it
2. [The API](api.md) — what is true of every route, and the reference made from the code (in [`api/`](api/))
3. [OAuth scopes and clients](oauth.md)
   - [Scopes](oauth.md#scopes)
   - [Seeding clients](oauth.md#seeding-clients)
   - [Where the code is](oauth.md#where-the-code-is)
4. [Services over gRPC](grpc.md) — calling the services from your own, in five languages
5. [Architecture](architecture.md) — services, routing and the shared library
   1. [Components](architecture.md#components)
   2. [Services](architecture.md#services)
   3. [Routing](architecture.md#routing)
   4. [One service for every kind of actor](architecture.md#one-service-for-every-kind-of-actor)
   5. [Naming images](architecture.md#naming-images)
   6. [anahita-libs](architecture.md#anahita-libs)
   7. [What a node is called elsewhere](architecture.md#what-a-node-is-called-elsewhere)
   8. [Naming dates and times](architecture.md#naming-dates-and-times)
6. [The web shell](web-shell.md) — sending the web app with each page's own title and picture, for links shared elsewhere
   1. [What it sends](web-shell.md#what-it-sends)
   2. [Which pages are described](web-shell.md#which-pages-are-described)
   3. [What is never described](web-shell.md#what-is-never-described)
   4. [Pictures](web-shell.md#pictures)
   5. [What other servers ask this host](web-shell.md#what-other-servers-ask-this-host)
   6. [Running it](web-shell.md#running-it)
   7. [Settings](web-shell.md#settings)
   8. [Not running it](web-shell.md#not-running-it)
   9. [Where the code is](web-shell.md#where-the-code-is)
   10. [What is not built](web-shell.md#what-is-not-built)
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
| [Notifications: right, then worth acting on](plans/done/notifications.md) | 79, 80 | done 2026-10-19 |
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
| [Documentation agents can use, and an MCP server](plans/done/docs-for-agents.md) | 39 | done 2026-10-27: the MCP server is anahita-mcp |
| [Anahita Recipes: a site ready to run, for each purpose](plans/recipes.md) | 40 | planned again 2026-11-07 by purpose: `site/` with `classic` and `from-php` first, in `anahita-recipes`; anahita-web's roadmap row 30 |
| [Running Anahita on k3s and on EKS](plans/k3s-and-eks.md) | 76 | planned 2026-10-22: a manual for both, with sizes and costs, tried on k3d, a rented server and EKS |
| [Sweeping away unused hashtags and locations](plans/done/tag-sweep.md) | 88 | done 2026-10-27: the sweeper removes a hashtag or location no post carries, a day old, but a location a feed names; `sweeper -tags-only` |
| [Sweeping away stored files nothing names](plans/done/file-sweep.md) | 89 | done 2026-11-07: a nightly sweep through file-grpc-service's `Browse` removes what no row names under `assets/v1/`, a day old, kept unless sure; dry until FILE_SWEEP=on |
| [BREAD names in the services' code](plans/done/bread-names.md) | 127 | done 2026-11-07: thirty functions named List, Remove or Update renamed Browse, Delete and Edit; five gRPC methods under both names for one release |
| [Threads that read as threads](plans/done/threads.md) | 126 | done 2026-11-07: a thread's next note and its length in a list, counted when a page is read; the writer's notes first on its page; the composer's count, Add to thread at the limit, and Split into a thread |
| [Refusing servers from a file](plans/domain-block-import.md) | 91 | planned 2026-10-27: a CSV of servers to refuse, Mastodon's own shape or one name a line, looked at before it is applied; the list downloaded as one too |
| [A second profile posting to a Bluesky account](plans/bluesky-posting-only.md) | 92 | planned 2026-10-28: a profile the same person looks after (a blog, a group) connects their Bluesky account to send its posts there, posting only: nothing from Bluesky is kept as it, and the account stays the first profile's |
| [Content warnings](plans/content-warnings.md) | 122 | planned 2026-11-03, Made with AI added 2026-11-07: six kinds, an author marks a post and each reader chooses Show, Warn or Hide; both networks in and out; moderators add one from a report |
| [Direct messages, and being found on the fediverse](plans/direct-messages.md) | 123 | planned 2026-11-03: listed on FediDB, FediDB's 128 questions answered in features.md, then messages here and with the fediverse as private mentions, with consent first |
| [Moving: who you follow, your followers, and your posts](plans/moving.md) | 124 | planned 2026-11-03: following out and in as Mastodon's CSV, heeding others' moves, moving oneself between Anahitas and with Mastodon with a signpost left behind, then posts between Anahitas |
| [Asking to follow on the fediverse](plans/remote-follow-requests.md) | 130 | planned 2026-11-07: following a profile elsewhere that approves its followers is a request, "Request to follow" then "Requested", until their server accepts |
| [Content Credentials](plans/content-credentials.md) | 131 | planned 2026-11-07: C2PA manifests checked on upload by a small `credentials-service` with the C2PA project's own library; a mark where one checks out, nothing where there is none |
| [Deleting one's old posts](plans/old-posts.md) | 132 | planned 2026-11-07: each person's own choice, off for everybody, with what is kept; run in the hourly sweeper |
| [Testing and scaling Anahita](plans/scaling.md) | 141 | planned 2026-11-07: metrics, a load test of 100,000 and 300,000 people, durable shared queues, a writer and readers, search on an index; feeds written ahead and caches only if the test calls for them |
| [Dependency alerts](plans/done/dependency-alerts.md) | 93 | done 2026-11-01, `v1.0.9`: the 744 alerts were 12 packages; Go 1.27.3, gRPC 1.85.0 and the rest in every module, and security updates now one pull request |
| [Refusing an account from another network](plans/done/refused-accounts.md) | 94 | done 2026-10-29: administrators refuse one fediverse or Bluesky account for the site, by its address or DID, and what is here of it is removed; listed in Site settings → Fediverse → Accounts |
| [Animated images: GIF, APNG and animated WebP, through the video service](plans/done/animated-images.md) | 41 | done 2026-10-14: GIFs and animated PNGs (not WebP), and video posts going to other servers and Bluesky |
| [A command-line client for Anahita](plans/cli-client.md) | 42 | recorded 2026-10-12 for later; waits for the API reference |
| [Translating posts](plans/translation.md) | 43 | recorded 2026-10-12 for later: LibreTranslate and DeepL behind one interface |
| [Anahita AI](plans/anahita-ai.md) | 95 | in progress from 2026-10-29: a service of its own, the model the installation's choice; alt-text drafts first, then statistics and Ask for administrators |
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
