# Deploying on EKS

[Documentation](README.md) › [Deploying](deploying.md) › On EKS

An example of Anahita on Amazon EKS, from an empty AWS account to the first
person signed in, with the database in RDS, Valkey in ElastiCache, files in
S3 and mail through SES.

> **This is an example.** Its names, its region, its Kubernetes version, the
> size and number of its nodes and the size of its database are a starting
> point, not a recommendation: put in your own. It has not been run end to
> end on EKS. The files it uses are checked to parse, and the `aws` overlay
> to build.

EKS is for a site that has outgrown one server, or for an organisation that
runs its services on AWS already. A new site costs much less on one server
with k3s: start from a recipe in
[Anahita Recipes](https://github.com/anahitasocial/anahita-recipes).

## Contents

1. [What it costs](#what-it-costs)
2. [What you need](#what-you-need)
3. [The files](#the-files)
4. [The cluster](#the-cluster)
5. [The bucket](#the-bucket)
6. [The database and Valkey](#the-database-and-valkey)
7. [Mail](#mail)
8. [Traefik and cert-manager](#traefik-and-cert-manager)
9. [The site](#the-site)
10. [Backups](#backups)
11. [Upgrading](#upgrading)
12. [Deleting it all](#deleting-it-all)

## What it costs

What this example runs, a month, at on-demand prices in `ca-central-1` in
November 2026, before tax and data transfer. Prices change: check AWS's
pricing pages on the day.

| What | Size | About, USD a month |
| --- | --- | --- |
| The EKS cluster | one, in standard support | 73 |
| Nodes | two `t3.medium` (2 vCPUs, 4 GB), 30 GB of gp3 each | 75 |
| Network Load Balancer | one | 20 |
| Public IPv4 addresses | two nodes and the load balancer's two | 15 |
| RDS for MySQL | `db.t4g.micro`, 20 GB, one zone | 17 |
| ElastiCache | `cache.t4g.micro`, one node | 13 |
| S3, SES | by use | a few |
| **In all** | | **about 215** |

The cluster's fee, the load balancer and the addresses are paid however few
people use the site. Spot nodes (in `cluster.yml`) cost about a third as
much, and may be taken back with two minutes' notice; Anahita's services
start again on the other node. A bigger site grows the nodes and the
database first.

## What you need

1. **An AWS account,** and a profile on your own computer that may make
   clusters, roles, databases and buckets in it.
2. **A domain,** and access to its DNS records.
3. **On your own computer:** the AWS CLI, `eksctl` and `kubectl`.
4. **A checkout of anahita-services** at the release you will run, so that
   the manifests and the images agree.

Every `aws` and `eksctl` command below names its profile and region, and
every `kubectl` its kubeconfig, so that nothing lands in another account or
cluster. Replace `<your-profile>` and `<your-site>`.

## The files

| File | What it is |
| --- | --- |
| `k8s/eks/cluster.yml` | The cluster, for eksctl: the network, the nodes, the add-ons, and the role file-grpc-service signs in to S3 with |
| `k8s/eks/storage-class.yml` | gp3 as the cluster's default disk, which EKS does not set |
| `k8s/eks/traefik.yml` | Traefik and its Network Load Balancer, the way into the cluster |
| `k8s/overlays/aws` | The site: S3, RDS, ElastiCache and SES, Traefik's Ingress class, RabbitMQ in the cluster |

Copy `k8s/overlays/aws` somewhere of your own, such as a private repository,
as [First deployment](deploying.md#first-deployment) says, and keep your
copies of the three `k8s/eks` files beside it.

## The cluster

In `cluster.yml`, change the bucket's name (`example-anahita`, twice) to
yours, and anything else of yours: the cluster's name, the region, the
version, the nodes. Then:

~~~sh
eksctl --profile <your-profile> create cluster -f k8s/eks/cluster.yml --write-kubeconfig=false
eksctl --profile <your-profile> utils write-kubeconfig --cluster anahita --region ca-central-1 \
  --kubeconfig ~/.kube/<your-site>.yaml
kubectl --kubeconfig ~/.kube/<your-site>.yaml get nodes
~~~

It takes about twenty minutes. eksctl makes the network (two public and two
private subnets, and no NAT gateway), the node group, the add-ons, and the
role `anahita-files`. Note the role's ARN for the overlay:

~~~sh
aws --profile <your-profile> iam get-role --role-name anahita-files --query Role.Arn --output text
~~~

## The bucket

Make the bucket as [A bucket on AWS S3](deploying.md#a-bucket-on-aws-s3)
says, in the cluster's region, with the name `cluster.yml` gives it. No keys
are needed: file-grpc-service uses the role.

## The database and Valkey

Both go in the cluster's network, in its two **private** subnets, reached
only from the nodes. eksctl named the network `eksctl-anahita-cluster/VPC`,
and the nodes' security group `eksctl-anahita-cluster-ClusterSharedNodeSecurityGroup-…`.

1. **A security group** for the two, in that network, letting in TCP 3306
   and 6379 from the nodes' security group, and nothing else.
2. **RDS for MySQL:**

   | Setting | Choose |
   | --- | --- |
   | Engine | MySQL 8.4 |
   | Size | `db.t4g.micro`, 20 GB of gp3; more for a bigger site |
   | Network | The cluster's network, a subnet group of its two private subnets, no public access, the security group above |
   | Database | `anahita`, with its own user, which needs every right on it ([Database, Valkey and RabbitMQ](deploying.md#database-valkey-and-rabbitmq)) |
   | Backups | Automated, kept 7 days or more |
   | Character set | `utf8mb4`, the default |

3. **ElastiCache:**

   | Setting | Choose |
   | --- | --- |
   | Engine | Valkey (Redis OSS works too, and costs more) |
   | Size | `cache.t4g.micro`, one node, no cluster mode |
   | Network | A subnet group of the two private subnets, the security group above |
   | Encryption in transit | On: `VALKEY_TLS=true` in `config.env` |

Put the database's address in `MYSQL_HOST` and `DSN`, and the cache's
primary address in `VALKEY_ADDR`.

## Mail

SES sends the site's mail. In the cluster's region:

1. Verify the domain, with the DNS records SES gives (DKIM among them).
2. Ask for production access: until then SES sends only to addresses
   verified in it.
3. Make SMTP credentials, and put them in `SMTP_USER` and `SMTP_PASSWORD`.

`config.env` already names SES's relay for `ca-central-1`; change the region
in `SMTP_HOST` if yours is another.

## Traefik and cert-manager

Once for the cluster, before the site:

~~~sh
kubectl --kubeconfig ~/.kube/<your-site>.yaml apply -f k8s/eks/storage-class.yml
kubectl --kubeconfig ~/.kube/<your-site>.yaml apply -f k8s/eks/traefik.yml
kubectl --kubeconfig ~/.kube/<your-site>.yaml apply -f \
  https://github.com/cert-manager/cert-manager/releases/download/v1.21.3/cert-manager.yaml
~~~

No Helm: each is a plain manifest, pinned to a version. EKS makes the
Network Load Balancer for Traefik's Service by itself; its name appears
after a minute or two:

~~~sh
kubectl --kubeconfig ~/.kube/<your-site>.yaml -n traefik get service traefik
~~~

Point `api.` and `auth.` of your domain at that name: a CNAME, or an alias
record in Route 53. The web app, at the domain itself, is hosted elsewhere
(anahita-web's `docs/deploying.md`).

The load balancer passes on each visitor's own address
(`externalTrafficPolicy: Local`), so the limits on sign-ups and password
resets apply to each visitor, not to everybody at once.

## The site

Fill in your copy of the `aws` overlay as
[First deployment](deploying.md#first-deployment) says, with these on EKS:

1. `service-accounts.yml`: the ARN of `anahita-files`.
2. `config.env`: the bucket and region; `MYSQL_HOST`, `VALKEY_ADDR` and
   `VALKEY_TLS=true`; SES's `SMTP_HOST`.
3. `secrets.env`: `DSN` and the database's password; `SMTP_USER` and
   `SMTP_PASSWORD`; the rest as the example says.
4. `ingress.yml` and `issuer.yml`: your names and your address.
5. `kustomization.yml`: the release you run, on every image.

Then apply it, and sign in as
[First deployment](deploying.md#first-deployment) says:

~~~sh
kubectl kustomize <your-overlay> | less
kubectl --kubeconfig ~/.kube/<your-site>.yaml apply -k <your-overlay>
kubectl --kubeconfig ~/.kube/<your-site>.yaml -n anahita get pods --watch
~~~

## Backups

1. **The database:** RDS's automated backups, restored to any second within
   the days kept. Restore one into a new instance before the site opens:
   nothing else proves a backup.
2. **Files:** the bucket's versioning and lifecycle rule, as
   [A bucket on AWS S3](deploying.md#a-bucket-on-aws-s3) sets them.
3. **`secrets.env`,** `CRYPT_LOCAL_KEY` above all: a copy somewhere other than
   the cluster and your computer. Without it nobody can sign in.

RabbitMQ's volume holds only work in progress, and needs no backup.

## Upgrading

1. **Anahita:** as in [Upgrading](deploying.md#upgrading), with your overlay.
2. **Kubernetes:** one version at a time, the cluster first, then the nodes:

   ~~~sh
   eksctl --profile <your-profile> upgrade cluster -f k8s/eks/cluster.yml --approve
   eksctl --profile <your-profile> upgrade nodegroup --cluster anahita --region ca-central-1 --name anahita-1
   ~~~

   Then the add-ons, from the console or `eksctl update addon`.
3. **Other nodes** (another size, or spot): add a second node group to
   `cluster.yml`, `anahita-2`, and make it with
   `eksctl --profile <your-profile> create nodegroup -f k8s/eks/cluster.yml --include anahita-2`.
   Once its nodes are ready, delete the old group, which moves its pods
   first: `eksctl --profile <your-profile> delete nodegroup --cluster anahita --region ca-central-1 --name anahita-1`.
   Take `anahita-1` out of `cluster.yml`.

## Deleting it all

In this order, so that nothing is left to pay for. The database and the
cache are in the cluster's network, which cannot be deleted while they are
there.

1. The site: `kubectl --kubeconfig ~/.kube/<your-site>.yaml delete -k <your-overlay>`,
   then `kubectl --kubeconfig ~/.kube/<your-site>.yaml delete -f k8s/eks/traefik.yml`,
   which removes the load balancer.
2. RDS and ElastiCache, their subnet groups, and their security group.
3. The cluster: `eksctl --profile <your-profile> delete cluster -f k8s/eks/cluster.yml`.
   It removes the network, the nodes and the role `anahita-files`.
4. Last, what is kept until deleted by hand: the disks `storage-class.yml` made
   (`reclaimPolicy: Retain`), in EC2 › Volumes; the bucket, emptied first,
   all its versions with it; SES's identity; the DNS records.
