# Deploying

[Documentation](README.md) › Deploying

How to run a live site on Kubernetes, on any provider. For development on
your own machine, use docker-compose instead: see
[Getting started](getting-started.md).

## Contents

- [What you need](#what-you-need)
- [How the manifests are laid out](#how-the-manifests-are-laid-out)
- [Hostnames](#hostnames)
- [First deployment](#first-deployment)
- [Certificates between the services](#certificates-between-the-services)
- [Object storage](#object-storage)
- [Database, Redis and RabbitMQ](#database-redis-and-rabbitmq)
- [On AWS](#on-aws)
- [Joining the fediverse](#joining-the-fediverse)
- [Images](#images)
- [Upgrading](#upgrading)
- [Running more than one copy of a service](#running-more-than-one-copy-of-a-service)
- [Trying it on your own machine](#trying-it-on-your-own-machine)
- [One machine without Kubernetes](#one-machine-without-kubernetes)

## What you need

| What | Notes |
| --- | --- |
| A Kubernetes cluster | Any provider's, or your own. A default StorageClass, unless the database, Redis and RabbitMQ all run elsewhere |
| An ingress controller | A maintained one. The base names the `nginx` class; the `traefik` component changes it to Traefik, which comes with k3s. ingress-nginx itself was retired in March 2026 |
| [cert-manager](https://cert-manager.io/) | Gets and renews the Let's Encrypt certificate, and issues the certificates the services use between themselves |
| Two DNS names | `api.` and `auth.` under your domain, both pointing at the ingress controller's address |
| A bucket | On any S3-compatible object store. Private. See [Object storage](#object-storage) |
| An SMTP account | Any provider |
| The images | Published to GitHub's registry for each version: see [Images](#images). Or your own builds: `make build_dockerfiles` builds them all |

Nothing here depends on one cloud. AWS is one option among several, and its
overlay only differs in how it signs in to S3 and where the database lives.

## How the manifests are laid out

~~~
k8s/
  base/                 every Anahita service, and nothing a site decides
    services/           one file per HTTP service
    grpc/               file, graph and mail
    jobs/               the migration job and the account sweeper
    gateway.yml         nginx: routing, CORS, cookie-to-token
    ingress.yml         the way in; sends everything to the gateway
    grpc-tls.yml        certificates for the calls between the services
    config.env          how the services find each other
  components/           optional, each added by an overlay
    mysql/ redis/ rabbitmq/   the backing services in the cluster
    mysql-backup/       a nightly dump of that MySQL to the bucket
    garage/             an S3-compatible store in the cluster
    traefik/            the Ingresses through Traefik
    mailpit/            catches every message, sends none on
  overlays/
    generic/            any cluster, any S3-compatible store
    k3s/                one server running k3s, at any provider
    aws/                EKS with S3, RDS and ElastiCache
    local/              minikube, for trying the manifests
    k3d/                k3d, for trying the k3s overlay's components
  k3s/
    traefik-config.yml  for a k3s cluster itself: keeps visitors' addresses
~~~

It is built with [Kustomize](https://kustomize.io/), which is part of
`kubectl`: `kubectl apply -k <overlay>`. The base is not applied on its own.
An overlay adds what a site decides: its hostnames, settings and secrets, and
which backing services run inside the cluster.

Each manifest gives its service the same variables `docker-compose.yml` does,
taken from two objects the overlay generates:

| Object | From | Holds |
| --- | --- | --- |
| ConfigMap `anahita-config` | `config.env` | Settings that are not secret |
| Secret `anahita-secrets` | `secrets.env` | Passwords and keys. Git ignores this file |

Every variable is described in [Configuration](configuration.md). The gateway
is the same nginx, with the same `nginx/nginx.conf`, that docker-compose runs,
so a path is routed the same way in both.

## Hostnames

With `ENV=prod`, auth-service works out its addresses from `DOMAIN_NAME`:

| Address | What is there |
| --- | --- |
| `https://DOMAIN_NAME` | The web app, hosted separately: see anahita-web's `docs/deploying.md`. Change it with `FRONTEND_URL` |
| `https://auth.DOMAIN_NAME` | The sign-in pages, and the address tokens are issued under (`OAUTH_ISSUER`) |
| `https://api.DOMAIN_NAME` | The API, which the web app is built to call (`REACT_APP_API_BASE_URL`) |

`auth.` and `api.` reach the same gateway. Three settings have to agree with
these, or signing in fails:

- `WEB_ORIGIN` in `config.env` is the web app's address. The gateway lets that
  one origin call the API with the session cookie.
- `OAUTH_ISSUER` in `config.env` is the `auth.` address.
- `oauth_seed_clients.json` holds the web app's `/oauth/callback` address.
  It is only read at first start. Afterwards, change it in the app under
  **Settings › OAuth clients**.

## First deployment

1. Copy `k8s/overlays/generic` (or `aws`) somewhere of your own, such as a
   private repository. If you move it, fix the `../../base` and
   `../../components` paths in its `kustomization.yml`.
2. Fill in `config.env`.
3. Copy `secrets.env.example` to `secrets.env` and replace every `CHANGE_ME`.
   `SECURE_COOKIE_KEY` and `CRYPT_LOCAL_KEY` are 32 characters each:
   `openssl rand -hex 16` makes one. Keep a copy of `CRYPT_LOCAL_KEY`
   somewhere safe. Without it nobody can sign in.
4. Put your hostnames in `ingress.yml`, your address in `issuer.yml`, and the
   web app's callback address in `oauth_seed_clients.json`.
5. In `kustomization.yml`, set the image tag to the release you are deploying.
6. Check what will be applied, then apply it:

   ~~~sh
   kubectl kustomize k8s/overlays/generic | less
   kubectl apply -k k8s/overlays/generic
   kubectl -n anahita get pods --watch
   ~~~

The migration job creates the schema on an empty database. Services that
start before it finishes restart until it has, which is expected.

auth-service creates the first super administrator from `SUPER_ADMIN_EMAIL`
and mails a link to set a password. The link is also in its log, which is the
way to it before mail works:

~~~sh
kubectl -n anahita logs deployment/auth-service | grep -i password
~~~

If you keep secrets in a tool such as Sealed Secrets or External Secrets,
drop the `secretGenerator` from `kustomization.yml` and have the tool create
a Secret called `anahita-secrets` with the same keys.

### Addresses behind a load balancer

auth-service reads a visitor's address from `X-Forwarded-For`, skipping
private addresses. That works when the ingress controller sees the visitor's
real address. Behind a cloud load balancer it often sees the balancer's
instead: turn on PROXY protocol, or `use-forwarded-headers`, in ingress-nginx
as your provider documents. The sign-in log showing `localhost`,
`development` for everybody is the sign that it is not set up.

On k3s, its built-in load balancer gives every connection the node's own
address unless told otherwise. Apply `k8s/k3s/traefik-config.yml` to the
cluster once, before the overlay:

~~~sh
kubectl apply -f k8s/k3s/traefik-config.yml
~~~

It sets Traefik's Service to `externalTrafficPolicy: Local`, which keeps the
visitor's address. Traefik drops an `X-Forwarded-For` sent by a visitor, so
it cannot be forged.

## Certificates between the services

The calls from the HTTP services to graph-grpc, file-grpc and mail-grpc use
mutual TLS, described under
[Internal calls](configuration.md#internal-calls). `k8s/base/grpc-tls.yml`
sets it up, and there is nothing to configure:

- cert-manager creates an authority for this installation alone, a
  certificate for each gRPC service, and one that every caller presents.
- Each is valid for 90 days and renewed 30 days before it ends. The services
  pick up a renewed certificate within a minute or two, without a restart.
- A pod waits to start until its certificate exists, which takes a few
  seconds on the first deployment.

To see them: `kubectl -n anahita get certificates`.

The certificates name the services as `base/config.env` dials them, such as
`graph-grpc-service`. If you change those addresses, change `dnsNames` in
`grpc-tls.yml` to match.

Any holder of a certificate from the authority may call any gRPC service.
There is no rule yet about which service may call which.

If a service mesh such as Linkerd already encrypts traffic between pods, set
`GRPC_TLS=off` in `config.env` and leave this to the mesh.

## Object storage

Uploads go to a bucket on any store that speaks the S3 API, set with the
`STORAGE_*` variables. The settings are described under
[Storage](configuration.md#storage).

| Store | `STORAGE_ENDPOINT` | `STORAGE_REGION` | `STORAGE_PATH_STYLE` | Checked |
| --- | --- | --- | --- | --- |
| AWS S3 | leave unset | e.g. `ca-central-1` | `false` | In use by the reference installation |
| Garage 2.1 | your Garage's S3 address | `garage`, or what `s3_region` says | `true` | Passes the storage test; the `garage` component, on k3d |
| MinIO | your MinIO's address | `us-east-1` unless configured | `true` | Not tested |
| OVHcloud | `https://s3.<region>.io.cloud.ovh.net` | e.g. `gra` | `false` | Not tested |
| Scaleway | `https://s3.<region>.scw.cloud` | e.g. `fr-par` | `false` | Not tested |
| Hetzner | `https://<location>.your-objectstorage.com` | e.g. `fsn1` | `false` | Not tested |

The rows marked not tested follow each provider's documentation. Before
relying on one, run the storage test against a bucket on it, as
[Tested stores](configuration.md#tested-stores) shows. It uploads, reads,
signs a link and deletes, in under a minute.

Where the provider sells no store, the `garage` component runs Garage in the
cluster, as the `k3s` overlay does. Its `garage-init` Job gives the node its
role, imports `STORAGE_ACCESS_KEY` and `STORAGE_SECRET_KEY`, makes
`STORAGE_BUCKET`, and lets `WEB_ORIGIN` `PUT` to it. Browsers fetch uploads
from it through signed links, so it has a public name of its own, `files.`,
set as `STORAGE_PUBLIC_ENDPOINT`. Its data is on one volume: back up the
server's disk.

Set `STORAGE_PUBLIC_ENDPOINT` when browsers reach the store at a different
address than the cluster does, which is the case for a store running inside
the cluster.

To move existing files from one store to another, copy the bucket with
[rclone](https://rclone.org/): `rclone sync old:bucket new:bucket`. The paths
inside the bucket stay the same, so nothing in the database changes.

### What is in the bucket

The bucket is private. Nothing in it is read except through an address the
file service signs for a few minutes.

Everything is under `assets/v1/`, in a folder named for the service that
looks after it.

| Folder | What | Written by |
| --- | --- | --- |
| `assets/v1/identity-image-service/avatars/` and `…/covers/` | Avatars and covers | `identity-image-service` |
| `assets/v1/photo-service/` | The images of photo posts | `photo-service` |
| `assets/v1/video-service/videos/` | Each video's sizes and poster | `video-worker` |
| `assets/v1/video-service/uploads/` | Videos as they were uploaded, until the worker has made them. Never read by a viewer | Browsers, through signed addresses |
| `assets/v1/export-service/` | People's copies of their own data, kept a week | `export-worker` |
| `assets/v1/link-preview-service/` | The pictures of link cards | `link-preview-service` |

Two rules on the bucket itself are worth setting:

- **CORS**, letting the web app's address `PUT`, where videos are posted:
  see [Videos](videos.md#storage).
- **A lifecycle rule that aborts unfinished multipart uploads** after a few
  days, which catches any upload the video worker could not abort itself.

On AWS, every setting is in the next section.

### A bucket on AWS S3

In the S3 console, in the region the rest of the site runs in, choose
**Create bucket**:

| Field | Choose | Why |
| --- | --- | --- |
| Bucket type | General purpose | Directory buckets (S3 Express One Zone) keep files in one data centre, for speed |
| Namespace and name | Global, a name with no dots, e.g. `example-org` | AWS's addresses for a bucket, and their certificates, do not take dots. The account regional namespace works too: its names end in the account ID and region, and no other account can ever take them |
| Copy settings from an existing bucket | No | |
| Object Ownership | ACLs disabled (recommended) | Then set `STORAGE_SEND_ACL=false`: a bucket with ACLs disabled accepts uploads that carry no ACL, and AWS documents the `private` ACL anahita-services sends by default on AWS as refused |
| Block Public Access | All four on | The bucket is private; files are read only through addresses file-grpc-service signs for a few minutes |
| Bucket Versioning | Enable | A file deleted or overwritten by mistake can be brought back, for as long as the lifecycle rule below keeps it |
| Tags | Optional, e.g. `project` | The bucket's cost shows on its own on the bill |
| Default encryption | SSE-S3 | file-grpc-service asks for it on every upload (`STORAGE_SSE`) |
| Object Lock | Off | Deleting a post, and purging a profile, delete its files |

Then, on the bucket:

1. **Permissions → Cross-origin resource sharing (CORS):** `PUT` from the
   web app's address, as [Videos](videos.md#storage) shows. List both
   addresses if the app answers at `www.` and without it.
2. **Management → Create lifecycle rule**, one rule for everything:
   1. **Name** `tidy-up`; **scope** all objects in the bucket, with its
      acknowledgement ticked; **object size** empty.
   2. **Actions:** tick only *Permanently delete noncurrent versions of
      objects* and *Delete expired object delete markers or incomplete
      multipart uploads*. Nothing under Transition or *Expire current
      versions*, which would delete the live files.
   3. **Noncurrent versions:** `30` days after objects become noncurrent;
      newer versions to retain empty. A file deleted or replaced can be
      brought back for 30 days, then it is gone; without this, everything a
      purge deletes would stay in the bucket as old versions.
   4. **Delete expired object delete markers** ticked; **delete incomplete
      multipart uploads** ticked, after `7` days.

No other rule is needed: the export worker deletes each archive itself after
`EXPORT_KEEP_DAYS`. No keys are needed on AWS either: see
[On AWS](#on-aws) for the role. Before the first deployment, run the storage
test against the new bucket ([Tested stores](configuration.md#tested-stores)).

### Moving a bucket to the v1 layout

Pictures are under `assets/v1/`. A bucket from before the first release has
them somewhere else, and `scripts/s3-migration` moves them:

| The bucket has | From | Run with |
| --- | --- | --- |
| `assets/public/n<id>/com_…/` | The PHP application, "v0". The file half of an upgrade from it; the database half is `migration-job/legacy/upgrade.sql` | `-from v0` |
| `assets/v2/`, with `avatar-service` and `cover-service` folders | This platform while it was being built | `-from v2` |

From `scripts/s3-migration`, with the same `STORAGE_*` settings the file
service has, or `AWS_PROFILE` for a profile on AWS:

~~~sh
# 1. What would be copied. Changes nothing.
STORAGE_BUCKET=my-bucket STORAGE_REGION=ca-central-1 go run . -from v2

# 2. Copy. The old files stay.
STORAGE_BUCKET=my-bucket STORAGE_REGION=ca-central-1 go run . -from v2 -apply

# 3. Deploy the services, and look at the site: avatars, covers, photos.

# 4. Remove the old files.
STORAGE_BUCKET=my-bucket STORAGE_REGION=ca-central-1 go run . -from v2 -apply -delete-old
~~~

**Copy before deploying.** Services that read the v1 layout find no
pictures in a bucket that has not been copied. Until step 4 both layouts
are in the bucket, so going back means deploying the services from before.

The store copies the files itself, two dozen at a time: tens of thousands
take minutes, and nothing is downloaded. Each step can be run again. An old
file is deleted only when its copy is in place and the same size, and a
file the script does not recognise is left and listed.

The script's own [README](../scripts/s3-migration/README.md) has every
option.

## Database, Redis and RabbitMQ

The `generic` overlay runs all three inside the cluster, from
`k8s/components`, each with one replica and a volume from the default
StorageClass. That is enough for a small site. **Nothing in `generic`
backs them up.** The `mysql-backup` component, which the `k3s` overlay has,
dumps the database at 03:30 UTC to `backups/mysql/` in the site's bucket and
keeps the newest 14; add it to `components:` in any overlay that runs the
`mysql` component. To take one now:

~~~sh
kubectl -n anahita create job --from=cronjob/mysql-backup mysql-backup-now
~~~

A dump is restored into an empty database with
`gunzip -c <dump>.sql.gz | mysql <database>`. Try one before a site opens:
nothing else proves a backup.

To use a managed service instead, remove its line from `components:` in
`kustomization.yml` and point the settings at it:

| Service | In `config.env` | In `secrets.env` |
| --- | --- | --- |
| MySQL 8 or later | `MYSQL_HOST` | `DSN`, `MYSQL_USER`, `MYSQL_PASSWORD` |
| Redis or Valkey | `REDIS_ADDR`, and `REDIS_TLS=true` if it requires TLS | `REDIS_PASSWORD` |
| RabbitMQ | | `AMQP_ADDR` |

The database account needs every right on its own database, including
creating and dropping stored procedures, which the upgrade from the PHP
application uses.

To put a volume on a StorageClass other than the default, add a patch to the
overlay:

~~~yaml
patches:
  - target:
      kind: StatefulSet
      name: mysql
    patch: |-
      - op: add
        path: /spec/volumeClaimTemplates/0/spec/storageClassName
        value: fast-ssd
~~~

## On AWS

`k8s/overlays/aws` differs from `generic` in three ways:

- **S3 without keys.** file-grpc-service runs as a service account tied to an
  IAM role (`service-accounts.yml`). Put your role's ARN there. The role needs
  `s3:GetObject`, `s3:PutObject` and `s3:DeleteObject` on the bucket's
  objects, and `s3:ListBucket` on the bucket.
- **RDS and ElastiCache** in place of the MySQL and Redis components.
- **SES** as the SMTP relay.

The key services are optional. To seal secrets and sign tokens with KMS, set
`CRYPT_BACKEND=kms`, `AWS_KMS_KEY_ID`, `OAUTH_SIGNER_BACKEND=kms` and
`OAUTH_KMS_KEY_ALIAS` in `config.env`, and give auth-service a service
account with a role that may use those keys, the same way.

This overlay has been checked to build. It has not been run on EKS.

## Joining the fediverse

Being found and followed from Mastodon and the like is its own piece of
setting up, with names that cannot be changed afterwards. It has a page of
its own, in the order to do it: [Federation](federation.md#setting-it-up).
Reading Bluesky, and writing to it for people who connect their own
account, needs much less: see [Bluesky](federation.md#bluesky).

## Images

Each version of anahita-services is published to GitHub's container
registry, for amd64 and arm64 under one name, when it is tagged
(`.github/workflows/images.yml`). A tagged version is a **Birth** release;
the main branch is always the **Embryo**, and is not published as images:
[Releases](releases.md).

~~~
ghcr.io/anahitasocial/<image>:<version>
~~~

`<image>` is the name of its file in `dockerfiles/`, except `nginx`, which
is published as `gateway`. Each also has `latest`, the newest Birth release, for
trying Anahita out; a site names the version it runs, in the `images:` of
its overlay, so that it changes only when its operator changes it. The tags
ending in `-amd64` and `-arm64` are the two halves the workflow joins, not
for use.

To publish a Birth release: `git tag v1.0.0 && git push origin v1.0.0`. The
workflow builds each architecture natively, on its own runner.

## Upgrading

~~~sh
kubectl -n anahita delete job migration-job --ignore-not-found
kubectl apply -k k8s/overlays/generic
~~~

A finished Job cannot be changed, which is why the old one is deleted first.
The new one applies whatever migrations have not run. Changing `config.env`
or `secrets.env` and applying again restarts the services that read them.

## Running more than one copy of a service

Every Deployment starts with one replica. The services that only answer
requests can be given more:

export, feed, identity-image, like, link-preview, location, photo, website,
search, socialgraph, text, video, web-shell, and the gateway.

The three workers, export-worker, video-worker and syndication-worker, take their
jobs from a table one at a time, so any number of copies share the work
without doing it twice.

These listen for events on the bus, and each copy receives every event, so
two copies would do the work twice (two notifications, two emails, two
posts sent to another server). Keep them at one until the event bus has
durable queues:

abuse-report, activitypub, actor, atproto, auth, hashtag, notification,
recommendation, file-grpc, graph-grpc, mail-grpc.

An event is acknowledged as it arrives and is never delivered twice, even
when its handler fails part way: what a handler wrote or sent is never
written or sent again. Its error is logged, a panic in it is caught, and
when the bus closes the channel the service says so and stops, to be
started again (anahita-libs/event/consumer.go). An event published while a
service is down is not heard by it.

## Trying it on your own machine

The `local` overlay runs the whole stack on
[minikube](https://minikube.sigs.k8s.io/), over plain HTTP with `ENV=dev`. It
is for checking the manifests, not for daily work.

~~~sh
make k8s-start     # a cluster with the ingress controller and cert-manager
make k8s-build     # builds every image and loads it into the cluster
make k8s-deploy    # applies k8s/overlays/local
kubectl --context minikube -n anahita get pods
~~~

`make k8s-start` starts minikube with `--keep-context`, so `kubectl` keeps
talking to the cluster it was, and stops if that changed. The `make k8s-*`
targets always name the minikube cluster, so they cannot touch another
one.

Uploads go to the Garage that docker-compose runs, so `make up` first if you
want them to work. To reach the API, either forward a port:

~~~sh
kubectl --context minikube -n anahita port-forward service/gateway 8080:80
curl http://localhost:8080/nodeinfo/2.1
~~~

or stop docker-compose, which uses port 80, and run `make k8s-tunnel` to
serve it at `http://localhost`. `make k8s-cleanup` deletes the cluster.

### With k3d

The `k3d` overlay runs the `k3s` overlay's components (Traefik, MySQL and
its backup, Redis, RabbitMQ, Garage) on [k3d](https://k3d.io/), which is
k3s in Docker, with a mailpit that catches every message. It needs k3d and
nothing else from docker-compose, which can keep running.

~~~sh
make k3d-start     # the cluster, with traefik-config.yml and cert-manager
make k3d-build     # builds every image and imports it into the cluster
make k3d-deploy    # applies k8s/overlays/k3d
kubectl --kubeconfig ~/.config/k3d/kubeconfig-anahita.yaml -n anahita get pods
~~~

The cluster's kubeconfig is its own file, `~/.config/k3d/kubeconfig-anahita.yaml`:
`~/.kube/config` is never written, `kubectl`'s current context is left
alone, and every target names that file, so none can reach another cluster.

| Address | What is there |
| --- | --- |
| `http://api.localhost:8088` | The API |
| `http://auth.localhost:8088` | The sign-in and sign-up pages |
| `http://files.localhost:8088` | Garage, where signed links to uploads point |

Sign-up codes and every other message are in mailpit (the `mailpit`
component), which a site can also use before it opens, so that nobody is
mailed by mistake:
`kubectl --kubeconfig ~/.config/k3d/kubeconfig-anahita.yaml -n anahita port-forward deploy/mailpit 8025`,
then `http://localhost:8025`. `make k3d-cleanup` deletes the cluster and its
kubeconfig.

## One machine without Kubernetes

There is no production docker-compose file. The one in the repository is for
development: it publishes every service's port and uses development
passwords.
