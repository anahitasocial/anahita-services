# The web shell

A link to a profile or a post, pasted into Mastodon, Bluesky, Slack or a
message, is shown with a title, a line of text and a picture. The site it
is pasted into gets those by fetching the page and reading the tags in its
`<head>`. It runs no JavaScript, and the web app is nothing but JavaScript:
every page of it is the same `index.html`, so every link looked the same.

`web-shell` is what sends the web app. For every page it sends that same
`index.html` with the page's own tags written in. Everybody gets the same
page: a browser starts the app from it, and a crawler reads the tags. Nobody
is told apart by what they say they are.

## Contents

- [What it sends](#what-it-sends)
- [Which pages are described](#which-pages-are-described)
- [What is never described](#what-is-never-described)
- [Pictures](#pictures)
- [What other servers ask this host](#what-other-servers-ask-this-host)
- [Running it](#running-it)
- [Settings](#settings)
- [Not running it](#not-running-it)
- [What is not built](#what-is-not-built)

## What it sends

| Asked for | Sent |
| --- | --- |
| A file of the build (`/static/…`, `/manifest.json`, `/favicon.ico`, `/statics/…`) | The file. Those under `/static/` are named by their contents and may be kept for a year; the rest for an hour |
| A file under `/static/` that is not there | `404`, and not the app: a missing script answered with a page fails in a way nobody can read |
| `/_shell/image/<name>` | A redirect to a page's picture; see [Pictures](#pictures) |
| Anything else | `index.html` with the page's tags, kept by browsers for a minute at most |

The tags are `<title>`, `description`, `og:title`, `og:description`,
`og:type`, `og:site_name`, `og:url`, `og:image`, `og:image:alt`,
`twitter:card` and `<link rel="canonical">`. Whatever the build had of these
is taken out first, so each is said once. Everything written is escaped: a
name or a post is somebody's text.

The site's own name and description, used for every page that has nothing
of its own to say, are read from the build's `index.html`, where
`REACT_APP_NAME` and `REACT_APP_DESCRIPTION` put them. They are not set
twice.

## Which pages are described

| Page | Title | Description | Picture |
| --- | --- | --- | --- |
| `/people/<alias>` and its tabs | The person's name | Their bio | Their avatar |
| `/groups/<id>`, `/events/<id>` and their tabs | Its name | What it says of itself | Its avatar |
| `/photos/<id>` | The photo's title | Its text | The photo |
| `/articles/<id>`, `/topics/<id>` | Its title | Its excerpt, or the start of its text | Its cover |
| `/notes/<id>` | Its writer's name | The start of the note | The picture of the link it carries a card for |
| `/hashtags/<tag>` | `#tag` | The site's | The site's |
| Any other | The site's name | The site's | `/statics/media/ogimage.jpg` |

Text is made plain, with Markdown and HTML taken out, and cut to 200
characters.

## What is never described

A tag is copied by every site the link is pasted into and cannot be taken
back, so the rule is stricter than who may read the page.

- The page is read as somebody who is **not signed in**, through the same
  calls and access rules as the API.
- What comes back is described only if it says of itself that it is
  `public`, **and** the profile it is on says the same, and it is not
  deleted, archived or disabled. A public note on a profile for registered
  people is not described.
- A reply is described only if the post at the top of its thread passes
  the same test.
- Settings and notification pages are nobody's to describe.
- With `SITE_READ_ACCESS=registered`, a site for members, no page is
  described and nothing is read at all.

Everything else is sent with what the site says of itself, exactly like a
page that does not exist. The answer does not tell the two apart.

What a page says is read again after a minute, so something made private
stops being described within that time. What another site already copied is
that site's to forget.

## Pictures

Stored pictures are read through signed addresses that stop working after a
few minutes, and a crawler keeps a picture's address for days. So a page's
picture is given as an address on the shell that does not change,
`/_shell/image/person-5` or `/_shell/image/post-170203`, which redirects to
a freshly signed one each time it is asked for.

The redirect is made by the same rules as the page: for what is not public
it answers `404`. A picture is never easier to reach than the page it is
the picture of.

## What other servers ask this host

Somebody here is known elsewhere as `ana@example.org`, so a server that
wants to find her asks `example.org`, the web app's host, where she is. The
shell knows nothing of people; the API does. So three addresses are answered
with a redirect to the same path on the API, with whatever was asked:

| Address | What it is for |
| --- | --- |
| `/.well-known/webfinger` | Finding somebody by their handle |
| `/.well-known/host-meta` | The older way of asking where WebFinger is |
| `/.well-known/nodeinfo` | What software this is and how many use it |

The API's address is `PUBLIC_API_URL`, or `https://api.DOMAIN_NAME` when
that is not set. activitypub-service answers WebFinger there.

## Running it

It needs the web app's build, the `build/` folder that `yarn build` makes
in anahita-web, and reaches `graph-grpc-service` and `file-grpc-service`.
It has no database access and takes no session: it reads as a visitor.

The web app and the API have different hosts (`www.example.org` and
`api.example.org`), because their paths overlap: `/people/alice` is a page
on one and a call on the other. The shell answers on the web app's host,
behind whatever ends TLS for it, and the gateway goes on answering on the
API's.

**docker-compose.** It is not started by `up`, having nothing to send
without a build:

```sh
make web_shell WEB_BUILD=/path/to/anahita-web/build
```

It listens on port 8102. `WEB_BUILD` copies the build into
`web-shell/web/`. To have it read anahita-web's folder in place, so a new
build of the app is picked up by a restart, set `WEB_BUILD_PATH` in `.env`
to that folder and run `make web_shell`.

**Kubernetes.** Build the image with the app in it, `make build_web_shell
WEB_BUILD=<folder>` then `docker build -f dockerfiles/web-shell.dockerfile`,
add `k8s/base/services/web-shell.yml` to the overlay's resources, and send
the web app's host to the `web-shell` Service in the Ingress.

`index.html` is read once, when it starts. Start it again after a new build.

## Settings

| Variable | Value |
| --- | --- |
| `PUBLIC_WEB_URL` | The address people reach the web app at, `https://www.example.org`. Written into `og:url`, the canonical link and the address of each picture. Left out, the host each request came to is used, with `https` when `X-Forwarded-Proto` says so |
| `PUBLIC_API_URL`, `DOMAIN_NAME`, `ENV` | Where the API is, for what other servers ask this host. See [Site identity](configuration.md#site-identity) |
| `WEB_BUILD_PATH` | docker-compose only: the folder mounted as the build. Defaults to `web-shell/web` |
| `WEB_BUILD_DIR` | Where the build is inside the container. Defaults to `/app/web` |
| `SITE_READ_ACCESS` | As for every service; see [Configuration](configuration.md#privacy-of-the-installation) |

## Not running it

The web app is a static site and can still be put on any static host, as
anahita-web's `docs/deploying.md` describes. It works the same; links to it
all show the site's own title and picture.

**A site that federates and does not run the shell** has to have its host
do what the shell does for other servers: redirect the three addresses
[above](#what-other-servers-ask-this-host) to the same paths on the API.
Without that, nobody here can be found by their handle.

## What is not built

- A place's name on `/locations/<id>`, and a custom feed's on `/feeds/<id>`.
- `og:image:width` and `height`.
- The `alternate` link to a page's ActivityPub form.
