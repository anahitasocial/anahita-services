# Running Anahita on k3s and on EKS

*Roadmap row 93, planned 2026-10-22. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** planned 2026-10-22. Phases 1 and 2 done on 2026-10-26; phase 3 next.

## Context

[Deploying](../deploying.md) explains how to run Anahita on any Kubernetes
cluster, with two overlays: `generic`, for any cluster and any
S3-compatible store, and `aws`, for EKS with S3, RDS and ElastiCache. It does
not say which cluster to choose, how big, or what it costs, and the `aws`
overlay has never been run on EKS.

The people Anahita is for (schools, newsrooms, libraries, research groups,
non-profits) are often small, and leaving Meta and X is easier when running
their own site is affordable. A managed cluster has a fixed cost that does
not shrink with the site. On AWS, in Canada (Central), October 2026, roughly
and before tax, a site for about 50 people:

| | EKS | k3s on one server |
| --- | --- | --- |
| Control plane | 73 | 0, on the server |
| One server, 2 vCPU and 8 GB (t4g.large) | 54 | 54 |
| Load balancer and its addresses | 27 | 0: the server's own address |
| Disks, bucket, DNS, the server's address | 9 | 11 |
| **USD a month** | **≈ 165** | **≈ 65** |

[k3s](https://k3s.io/) is a certified Kubernetes in a single binary, made
for one server or a few. It runs the same manifests, at any provider that
sells a Linux server, or on a machine of the organisation's own. It grows by
adding servers, and a site that outgrows it moves to a managed cluster by
applying the same overlay there.

What Anahita needs, from the manifests: 28 Deployments, MySQL, Redis and
RabbitMQ, about 33 pods. Most ask for 25m CPU and 32Mi; MySQL and
video-worker for 512Mi, video-worker for 500m while it transcodes. About 2.5
to 3 GB and 1.5 vCPU in use, with the cluster's own pods. On EKS that is
more pods than one node takes unless prefix delegation is on.

The user asked on 2026-10-22 for a manual for both, so that people can host
Anahita affordably and with other cloud companies.

## Decisions

Settled with the user on 2026-10-22:

1. **Row 93 becomes this:** a manual for running Anahita on k3s, at any
   provider, and on EKS; it was "a low-cost EKS cluster" for one site.
2. **Both, side by side:** when to choose which, sizes, monthly costs, and
   the steps for each.
3. **Tried before it is published:** first with k3d (k3s in Docker) on a
   developer's machine, then once on a real rented server with a domain and
   Let's Encrypt, deleted afterwards. EKS is tried the same way, for as long
   as it takes.
4. **Nothing about any one site** in this repository: no hostnames, sizes
   or providers chosen for a particular deployment. A site's own overlay,
   secrets and notes live with that site.

## Settled in phase 1

Each as recommended, settled with the user on 2026-10-26 unless said:

1. **Ingress controller.** The manifests ask for ingress-nginx (`nginx`
   class, in `issuer.yml` and `ingress.yml`). k3s comes with Traefik.
   ingress-nginx was announced for retirement in November 2025, with
   maintenance ending in March 2026 (to confirm when this row starts).
   Its maintenance did end in March 2026 (Kubernetes' statement, January
   2026). **Settled:** a `traefik` component, which the `k3s` overlay uses;
   the manifests keep the standard Ingress, and each overlay names its
   class: `generic` any maintained controller, Traefik as the example;
   `aws` the AWS Load Balancer Controller (phase 4). The Gateway API later.
2. **Object storage on one server.** Recommended: the provider's own
   S3-compatible store where there is one, and a Garage component
   (`k8s/components/garage`) for a server with none, since docker-compose
   already runs Garage for development. **Settled:** `k8s/components/garage`,
   in the `k3s` overlay by default, removed by one line where there is a
   bucket.
3. **Backups.** Deploying says nothing backs up the in-cluster MySQL.
   Recommended: a component with a CronJob that dumps the database to the
   bucket nightly and keeps a set number, and the steps to restore one; plus
   the k3s datastore's own snapshot, and the provider's disk snapshots where
   there are any (on AWS, EBS snapshots through Data Lifecycle Manager). The
   manual has a restore tried before a site opens, since nothing else proves
   a backup. **Settled:** `k8s/components/mysql-backup`, at 03:30 UTC,
   keeping 14 dumps under `backups/mysql/`. k3s's own datastore on one
   server is a file on the disk the snapshots cover.

**Settled on 2026-10-26 (the user):** on one server, **MySQL runs in the
cluster**, not as a managed database. A small site's database is small:
anahita.io's, after 13 years, was 140 MB, and a hundred active people add
tens of MB a year, so it stays in memory, and one MySQL handles far more
than a site that size asks of it. A managed database (RDS: about USD 15 to 20
a month for the smallest, single-AZ; twice that Multi-AZ) would add a quarter
or more to the bill for backups the CronJob gives. Lower costs are the point:
every site run from this manual pays them. The manual says when to move to a
managed one, in "Growing": thousands of active people, a database of several
GB that no longer fits in memory, or a database CPU that runs hot for long
stretches. The move is a dump, a restore and `MYSQL_HOST`.
4. **Where to try the real server.** k3d on a developer's machine first;
   then the first real site's own server is the trial (the user,
   2026-10-26), not a second provider.
5. **Visitors' addresses.** auth-service reads `X-Forwarded-For`. Check that
   Traefik behind k3s's built-in load balancer (ServiceLB) passes the real
   address, and say how if not. Checked in k3d and on the real server.
6. **No cluster but the one meant** (the user, 2026-10-26): the k3d targets
   never write `~/.kube/config` and name their own kubeconfig in every
   call; `make k8s-start` keeps kubectl's context (`--keep-context`); both
   stop if the current context changed.

## Design

### Overlays

- `k8s/overlays/k3s`: `generic`'s files, with the `traefik`, `mysql`,
  `mysql-backup`, `redis`, `rabbitmq` and `garage` components, and a
  `files.` name for Garage. k3s's `local-path` StorageClass is the default,
  so the volumes need nothing.
- `k8s/overlays/k3d`: the same components with development settings, at
  `http://api.localhost:8088`, for `make k3d-deploy`.
- Three new components: `traefik` (the Ingress class), `garage` (Garage,
  and `garage-init`, which gives the node its role, imports the key, makes
  the bucket and its CORS rule through the admin API), `mysql-backup`.
- `k8s/overlays/aws`: as it is, corrected by whatever trying it on EKS turns
  up; prefix delegation, or the node count, said in the manual.

### The manual

`docs/deploying.md` stays the page for any cluster. Two pages beside it,
each in the order of doing it:

**On k3s** (`docs/deploying-k3s.md`):

1. Choosing a server: sizes for 50, 500 and 5,000 people, what each
   costs on AWS on the day it is written, and what to look for at other
   providers (vCPU, memory, disk, an IPv4 address, an S3-compatible store).
2. DNS: `api.` and `auth.` at the server's address.
3. Installing k3s, and cert-manager.
4. Object storage: the provider's, or Garage on the server.
5. The overlay: what to fill in, then `kubectl apply -k`.
6. Mail, the first administrator, and the web app.
7. Backups and restoring one.
8. Upgrading Anahita, k3s and the server.
9. Growing: a bigger server, a second one, managed MySQL (and the signs that it is time: see Backups above), then a managed
   cluster.

**On EKS** (`docs/deploying-eks.md`): what the "On AWS" section says now,
and the cluster itself: node sizes, spot or on-demand, prefix delegation,
the load balancer, IRSA, RDS and ElastiCache or in the cluster, costs.

### Trying it

- k3d: `make k3d-start`, `make k3d-deploy` and `make k3d-cleanup`, beside
  the minikube targets, so the `k3s` overlay is checked the way `local` is.
- A real server and a domain, following the manual word for word, as a
  newcomer would; every step that had to be changed is changed in the
  manual. Test names after Star Trek places. Delete the server, the bucket
  and the DNS records afterwards.

### Not in this row

- Recipes for kinds of sites (row 30, [recipes.md](recipes.md)): a recipe
  says what a site turns on, this row says where it runs.
- Any one site's deployment.

## What was built

| Where | What |
| --- | --- |
| `k8s/components/traefik` | The `anahita` Ingress's class changed to `traefik`, and ingress-nginx's annotation removed |
| `k8s/components/garage` | Garage 2.1 as a StatefulSet (one 50 Gi volume), its Service, a `garage` Ingress for `files.`, and `garage-init`, which readies it through the admin API v2 and sets the bucket's CORS rule with the AWS CLI |
| `k8s/components/mysql-backup` | The CronJob: `mysqldump --single-transaction --set-gtid-purged=OFF`, gzipped, sent with the AWS CLI, the newest 14 kept |
| `k8s/overlays/k3s` | `generic`'s files with the components, Garage at `files.`, Let's Encrypt through Traefik |
| `k8s/overlays/k3d` | The same components, development settings, a mailpit, names under `localhost` at port 8088 |
| `k8s/k3s/traefik-config.yml` | A HelmChartConfig for k3s's Traefik: `externalTrafficPolicy: Local` |
| `Makefile` | `k3d-start`, `k3d-build`, `k3d-deploy`, `k3d-cleanup`; `k8s-start` with `--keep-context`; both start targets stop if kubectl's current context changed |

## Different from the plan

- **Visitors' addresses** (point 5) needed a change: k3s's load balancer
  gave every connection the node's address (`10.42.0.1`), which would have
  made the per-address limits on sign-ups and password resets one limit
  for everybody. `traefik-config.yml` keeps the source address; k3d showed
  the connecting peer's after it, and a forged `X-Forwarded-For` dropped by
  Traefik.
- **Mail on k3d:** the `k3d` overlay has its own mailpit, so a person can
  sign up without docker-compose.

## Phases

1. Settle the points above. Done 2026-10-26.
2. The `k3s` overlay, the Garage and backup components, the k3d targets;
   checked with k3d. **Done 2026-10-26.** On k3d: every pod up; garage-init
   ran twice, the second time changing nothing; the API, the sign-in pages
   and Garage reached through Traefik; CORS letting only `WEB_ORIGIN` PUT; a
   person signed up (the code caught by mailpit), signed in, posted a photo
   and read it back through its signed `files.` link, while the same address
   unsigned was refused; a backup sent to the bucket and restored into an
   empty database, all 47 tables back. `~/.kube/config` was the same, byte
   for byte, after `make k3d-start`.
3. The k3s manual; tried on a real server.
4. The EKS manual; the `aws` overlay tried on EKS and corrected.

## Docs to change

| Page | Change |
| --- | --- |
| `docs/deploying.md` | Contents; What you need (ingress); How the manifests are laid out (`k3s`, the new components); Database, Redis and RabbitMQ (backups); On AWS points to the EKS page |
| `docs/deploying-k3s.md` | New |
| `docs/deploying-eks.md` | New |
| `docs/README.md` | The two pages in the docs table; this plan in the plans table |
| `docs/getting-started.md` | The k3d targets, if it lists the minikube ones |
| anahita-web `docs/deploying.md` | Where the web app goes when the API is on k3s |
| anahita-web `docs/plans/roadmap.md` | Row 93 |
