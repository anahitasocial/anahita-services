# The federation lab

Trying federation on one machine: this installation and another server,
each at a name of its own, speaking to each other over https.

## Contents

- [Why it is needed](#why-it-is-needed)
- [What is in it](#what-is-in-it)
- [Starting it](#starting-it)
- [Opening it in a browser](#opening-it-in-a-browser)
- [Somebody to talk to](#somebody-to-talk-to)
- [Having GoToSocial look somebody up](#having-gotosocial-look-somebody-up)
- [Looking somebody on GoToSocial up from here](#looking-somebody-on-gotosocial-up-from-here)
- [Asking as another server would](#asking-as-another-server-would)
- [Stopping and emptying it](#stopping-and-emptying-it)
- [What it is not](#what-it-is-not)
- [Where the code is](#where-the-code-is)

## Why it is needed

Servers that federate find each other by name and speak only over https, and
a server keeps what it is told another is called. None of that can be tried
at `http://localhost`. The lab gives a development stack what a site on the
internet has: names, certificates, and a server it did not write to be
understood by.

The names end in `.test`, which is set aside for this and can never be a
real site. Nothing in the lab is reachable from outside your machine.

## What is in it

| Name | What answers | Container |
| --- | --- | --- |
| `https://api.anahita.test` | This installation's API, as other servers reach it | the gateway, `nginx` |
| `https://anahita.test` | The web app's host, where a handle such as `ana@anahita.test` is looked up | `web-shell`, when it is running |
| `https://gts.test` | [GoToSocial](https://gotosocial.org), a small server that speaks ActivityPub as Mastodon does | `gotosocial` |

While the lab is up, `activitypub-service` is this installation's part in
it: `make fedi_lab` starts it again as the site `anahita.test`, reached at
`https://api.anahita.test`, trusting the lab's authority and nothing else,
and allowed to fetch from the private addresses the lab's servers are at
(`ACTIVITYPUB_LAB=on`).
`make fedi_lab_down` puts it back as it was.

`fedi-lab-tls` is the front door: an nginx that holds the certificate and is
known on the Docker network by all three names, so each server reaches the
other by name, as two servers on the internet would. The certificate is
signed by an authority made for the lab (`scripts/fedi-lab-certs.sh`, in
`fedi-lab/certs/`, which Git ignores). GoToSocial is told to trust that
authority, and to fetch from the lab's private addresses, which it would
otherwise refuse as any careful server does.

## Starting it

With the stack running (`make up`):

~~~sh
make fedi_lab
~~~

It is not started by `make up`, and takes port 443 on your machine while it
runs.

## Opening it in a browser

Add the names to `/etc/hosts`:

~~~
127.0.0.1 anahita.test api.anahita.test gts.test
~~~

and, to be spared the warnings, trust `fedi-lab/certs/ca.crt` in your
browser or keychain. It signs nothing but the lab; remove it when you are
done with the lab.

## Somebody to talk to

GoToSocial starts with nobody on it. Make somebody:

~~~sh
docker compose --profile fedi-lab exec gotosocial \
  /gotosocial/gotosocial admin account create \
  --username alice --email alice@gts.test --password '<a long password>'
~~~

She is then `@alice@gts.test`, and can sign in at `https://gts.test` with
any Mastodon-compatible client.

## Having GoToSocial look somebody up

The real test of whether this installation is understood: ask the other
server to find somebody here by their handle. `fedi-lab/gts-token.sh` signs
a GoToSocial account in and prints a token of theirs:

~~~sh
run() { docker run --rm --network anahita-services_anahita-network \
  -v "$PWD/fedi-lab:/lab:ro" "$@"; }

TOKEN=$(run --entrypoint sh curlimages/curl /lab/gts-token.sh alice@gts.test '<her password>')

run curlimages/curl -s --cacert /lab/certs/ca.crt -H "Authorization: Bearer $TOKEN" \
  "https://gts.test/api/v2/search?q=@ana@anahita.test&resolve=true&type=accounts"
~~~

GoToSocial then asks `anahita.test` who that is, is sent on to the API,
fetches the person's description and their pictures, and answers with the
account as it understood it. Somebody who has not turned federation on is
not found. Until the web app has the switch, it is turned on for a person
with `PUT /activitypub/settings` ([Federation](federation.md#the-api)).

## Looking somebody on GoToSocial up from here

The other direction, and the test that what this installation signs is
accepted: GoToSocial answers for nobody without a signature it can check.

~~~sh
docker compose exec activitypub-service /app/app-binary lookup @alice@gts.test
~~~

It prints who was found, and keeps her as a person of another server in the
development database.

## Asking as another server would

From inside the network, trusting only the lab's authority:

~~~sh
docker run --rm --network anahita-services_anahita-network \
  -v "$PWD/fedi-lab/certs:/certs:ro" curlimages/curl -s --cacert /certs/ca.crt \
  "https://gts.test/.well-known/webfinger?resource=acct:alice@gts.test"

docker run --rm --network anahita-services_anahita-network \
  -v "$PWD/fedi-lab/certs:/certs:ro" curlimages/curl -s --cacert /certs/ca.crt \
  https://api.anahita.test/nodeinfo/2.1
~~~

A service of ours that is to speak to the lab has to trust the same
authority: mount `./fedi-lab/certs` into it and set `SSL_CERT_FILE` to the
`ca.crt` there, as the `gotosocial` entry in `docker-compose.yml` does.

## Stopping and emptying it

~~~sh
make fedi_lab_down     # stop it; what GoToSocial holds is kept
make fedi_lab_reset    # stop it and empty GoToSocial
~~~

**Empty it whenever this installation's identity changes** (a fresh
database, different keys). A server remembers who it was told somebody is,
and will refuse the same name with a different key.

## What it is not

- **Not the internet.** Mastodon itself, Threads and the rest each behave a
  little differently. Before a release, federation is tried against real
  servers through a tunnel to a throwaway name of its own, never one that
  will later be a real site's.
- **Not for Bluesky.** Reading from Bluesky needs nothing to be reachable;
  writing to it needs a public address.

## Where the code is

| What | Where |
| --- | --- |
| The profile, its two containers and its volume | `docker-compose.yml`, `fedi-lab` |
| The front door | `fedi-lab/nginx.conf` |
| The certificates | `scripts/fedi-lab-certs.sh` → `fedi-lab/certs/` |
| A token for a GoToSocial account | `fedi-lab/gts-token.sh` |
| `fedi_lab`, `fedi_lab_down`, `fedi_lab_reset` | `Makefile` |
