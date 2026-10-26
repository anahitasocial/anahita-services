# Running Anahita on k3s and on EKS

*Roadmap row 93, planned 2026-10-22. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

**Status:** planned 2026-10-22. Not started.

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

## To decide

Each with a recommendation:

1. **Ingress controller.** The manifests ask for ingress-nginx (`nginx`
   class, in `issuer.yml` and `ingress.yml`). k3s comes with Traefik.
   ingress-nginx was announced for retirement in November 2025, with
   maintenance ending in March 2026 (to confirm when this row starts).
   Recommended: a `k3s` overlay using Traefik; for the other overlays,
   decide between Traefik and the Gateway API in this row, since `What you
   need` should not ask for a retired project.
2. **Object storage on one server.** Recommended: the provider's own
   S3-compatible store where there is one, and a Garage component
   (`k8s/components/garage`) for a server with none, since docker-compose
   already runs Garage for development.
3. **Backups.** Deploying says nothing backs up the in-cluster MySQL.
   Recommended: a component with a CronJob that dumps the database to the
   bucket nightly and keeps a set number, and the steps to restore one; plus
   the k3s datastore's own snapshot, and the provider's disk snapshots where
   there are any (on AWS, EBS snapshots through Data Lifecycle Manager). The
   manual has a restore tried before a site opens, since nothing else proves
   a backup.

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
4. **Where to try the real server.** Recommended: a small server at a
   provider other than AWS, to prove the "any provider" part, and EKS on
   AWS.
5. **Visitors' addresses.** auth-service reads `X-Forwarded-For`. Check that
   Traefik behind k3s's built-in load balancer (ServiceLB) passes the real
   address, and say how if not.

## Design

### Overlays

- `k8s/overlays/k3s`: `generic` with Traefik's ingress class, the Garage
  and backup components, and the defaults for one server. k3s's
  `local-path` StorageClass is the default, so the volumes need nothing.
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

## Phases

1. Decide the five points above.
2. The `k3s` overlay, the Garage and backup components, the k3d targets;
   checked with k3d.
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
