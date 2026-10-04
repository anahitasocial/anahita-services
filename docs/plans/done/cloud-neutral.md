# Digital sovereignty: any cloud, any S3-compatible storage

*Roadmap item 9, planned 2026-09-28. See the [roadmap](https://github.com/anahitasocial/anahita-web/blob/main/docs/plans/roadmap.md) for order and dependencies.*

## Context
We want an instance to run on AWS or on any provider, with storage from, say, a Canadian or European S3-compatible provider (OVHcloud, Scaleway, Hetzner and others) or self-hosted.

**Finding: there's almost no AWS lock-in.**
- No Lambda, SES, SQS/SNS, Secrets Manager, CloudFront, Cognito, RDS or ElastiCache features, IRSA, ECR, Terraform or CloudFormation, and no CI workflows.
- Mail is SMTP only, image resizing runs in the service itself, MySQL, Redis and RabbitMQ are plain, the k8s manifests use nginx ingress and hostPath, and images are on Docker Hub.

**The AWS-specific pieces:**

| Piece | Where | State |
| --- | --- | --- |
| **S3 in file-grpc-service** (the only real blocker) | `file-grpc-service/data/services/aws_s3.go` | `LoadDefaultConfig` with no options. There's an interface (`AwsS3ServiceInterface`, :24-32), but it has one implementation and no selector. |
| KMS for encryption at rest | `anahita-libs/crypt/kms.go` | Optional (`CRYPT_BACKEND`, default `local`), but the region is **hard-coded** to `ca-central-1` (`anahita-libs/constants/crypt.go:4`) |
| KMS for JWT signing | `auth-service/internal/signing/signer_kms.go` | Optional (`OAUTH_SIGNER_BACKEND`, default `local`). In local mode the private key PEM is stored **unencrypted** (`oauth_seed.go:88-92`). |
| Dead code | `anahita-libs/storage/*` | aws-sdk-go v1 with hard-coded `*.s3.amazonaws.com` hosts; nothing imports it |
| One-off script | `scripts/s3-migration` | Hard-coded bucket |
| Env var names | compose, k8s, docs | `AWS_*` everywhere; static keys, or a `~/.aws` credentials secret in `k8s/02-file-grpc.yml:27-59` |
| Web hosting | `anahita-web/amplify.yml` | Amplify, but the app is a plain static build (`docs/deploying.md` already has an nginx example) |

**What's stopping other S3-compatible providers today:**
- No path-style addressing option. MinIO, Garage and SeaweedFS usually need it, and the dotted bucket name `staging.anahita.io` forces it over HTTPS.
- No custom endpoint in config. The SDK may already honour `AWS_ENDPOINT_URL_S3`, but nothing sets or documents it.
- No separate *public* endpoint for presigned URLs. Browsers load media straight from presigned links, so presigning against an internal `http://minio:9000` gives links browsers can't reach.
- Every put sends `ACL: private` + `ServerSideEncryption: AES256`, which some providers reject. Garage has no SSE-S3.
- `Exists` treats any error as "not found".
- Local dev talks to **real AWS S3**. There's no MinIO or LocalStack.

**Also found, security (do now, not as part of this item):** `k8s/00-secret.yml:12-13` has a real-looking `AWS_ACCESS_KEY_ID` / secret on disk. It's gitignored and was never committed, but it should be **rotated** if it's live.

**Other sovereignty leaks** (third parties, not AWS):
- Google Fonts in `auth-service/web/templates/base.html:8`.
- Google Analytics, optional, in `anahita-web/src/routes/index.js:101-109`. It calls the react-ga4 API on the `react-ga` v3 package, which looks broken.
- OpenStreetMap tiles hard-coded in `anahita-web/src/components/Map.jsx:28`.
- Public Photon and Nominatim geocoders with hard-coded URLs (`anahita-libs/geocoding/*_provider.go`).
- The README logo is served from AWS S3.

**Decisions:**
- **S3-compatible storage only, no local-filesystem backend.** The reason: a store that never executes files is a hard security boundary. The legacy PHP video component was exploited through executable uploads. Small single-VM instances run Garage or MinIO in a container instead.
- **Keys stay local by default.** Fix the KMS region and encrypt the stored signing key. OpenBao/Vault Transit is a later follow-up.

## Implementation
**A. Storage that works with any S3-compatible provider** (file-grpc-service)
1. **Neutral config**, with the old `AWS_*` names still read as fallbacks so existing installs keep working:

   | Setting | Meaning |
   | --- | --- |
   | `STORAGE_ENDPOINT` | Empty means AWS |
   | `STORAGE_PUBLIC_ENDPOINT` | Used only for presigning, for example `https://media.DOMAIN` or the provider's public URL |
   | `STORAGE_REGION` | |
   | `STORAGE_BUCKET` | |
   | `STORAGE_PREFIX` | Replaces `AWS_FILES_S3_ASSETS_DIR`, which is required but unused today |
   | `STORAGE_ACCESS_KEY` / `STORAGE_SECRET_KEY` | Empty means the SDK's default chain, so AWS roles still work |
   | `STORAGE_PATH_STYLE` | true/false |
   | `STORAGE_SSE` | `none` / `AES256` |
   | `STORAGE_SEND_ACL` | true/false |

2. **Client construction:**
   - Rename `AwsS3Service` to `S3Service`. Build it with `config.LoadDefaultConfig(ctx, WithRegion, WithCredentialsProvider…)` and `s3.NewFromConfig(cfg, func(o){ o.BaseEndpoint = …; o.UsePathStyle = … })`.
   - Keep a **second client configured with the public endpoint** for `Presign` and `BatchPresign` (`aws_s3.go:180-226`). Uploads go to the internal endpoint and browsers get public links.
3. **Error handling:** `Exists` separates "not found" from other errors (`aws_s3.go:165-174`). `Delete`'s 5-minute wait for the object to vanish becomes a short timeout, since not every provider supports the waiter well.
4. **Upload hardening,** so no store ever serves active content:
   - A content-type allowlist per service (images for avatar, cover and photo).
   - Images are already re-encoded by `anahita-libs/utils/images.go`, which strips embedded payloads. Keep that as a rule for every future upload type.
   - Set `Content-Type` explicitly, plus `Content-Disposition: attachment` for anything that isn't an image.
   - Never accept SVG or HTML.
   - Recommend serving media from its **own host** (`media.DOMAIN` as `STORAGE_PUBLIC_ENDPOINT`), so even a mistake can't run script on the app's origin. This ties in with item 8's stable media proxy (phase 0.4): the proxy URL hides which backend is behind it.
5. **Tests:** integration tests that run the storage service against **MinIO and Garage** containers (upload, read, presign against the public endpoint, delete, exists), with a flag to run them against a real provider bucket.

**B. Local dev without AWS** (docker-compose)
- Add **Garage** (lightweight, open source, from a European co-op) or **MinIO** to `docker-compose.yml`, with a bucket-init step.
- Point file-grpc at it by default with `STORAGE_PATH_STYLE=true` and `STORAGE_PUBLIC_ENDPOINT=http://localhost:3900`.
- Update `docs/getting-started.md`, which never mentions storage today even though file-grpc won't start without it.
- Moving existing data between providers: document `rclone sync`, and retire `scripts/s3-migration`.

**C. Keys and cleanup**
- Make the KMS region configurable: `CRYPT_KMS_REGION`, falling back to `STORAGE_REGION` / `AWS_REGION`. Build the KMS client once instead of on every call.
- **Encrypt the local JWT signing key** with `anahita-libs/crypt` (local AES-GCM) in `oauth_signing_keys.private_key`, with a migration that encrypts existing rows.
- Delete `anahita-libs/storage` and the aws-sdk-go v1 dependency.
- Add a TLS option to Redis (`REDIS_TLS=true`, in `anahita-libs/sessions/redis.go` and `key_value_store.go`), since many managed Redis/Valkey offerings require it.
- Follow-up, recorded but not built: OpenBao/Vault Transit backends for `crypt` and the JWT signer.

**D. Removing third-party dependencies**
- Self-host the Roboto font in `auth-service/web/static`.
- Map tiles: a `REACT_APP_MAP_TILE_URL` variable (default OSM), documenting self-hosted Protomaps/PMTiles or OpenMapTiles.
- Geocoder base URLs as config (`GEOCODER_PHOTON_URL`, `GEOCODER_NOMINATIM_URL`, and a configurable User-Agent contact), for self-hosted Photon or Nominatim.
- Analytics: replace the broken `react-ga` call with a small provider switch, `REACT_APP_ANALYTICS=none|matomo|plausible|umami|ga`, defaulting to `none`.
- Put the README logo in the repo.

**E. Deployment anywhere**
- **k8s:** restructure `k8s/` as a Kustomize **base** plus overlays (`overlays/generic`, `overlays/aws`), with:
  - a storage secret instead of the `~/.aws` credentials mount;
  - cert-manager + Let's Encrypt TLS on the nginx ingress;
  - a StorageClass that the overlay sets (instead of `manual` hostPath);
  - the stale ingress fixed.
  
  This overlaps item 8's phase 0.8; do it once.
- **Single-VM option:** a compose `production` profile (nginx with TLS via Caddy or certbot, Garage, MySQL, Redis, RabbitMQ) for small instances on any VPS.
- **`docs/deploying.md`:** a provider matrix (AWS S3, OVHcloud, Scaleway, Hetzner, Garage, MinIO) with the exact `STORAGE_*` values for each, verified by the item A tests. Plus Amplify-free web hosting (already possible).
- Update `docs/configuration.md` (Storage section) and `docs/architecture.md` (file-grpc is "S3-compatible", not "S3").

## Estimate

| Part | Time |
| --- | --- |
| A. Storage: neutral config with fallbacks, endpoints, path style, SSE/ACL toggles, `Exists` fix, upload hardening, MinIO + Garage tests | 7–9 h |
| B. Garage or MinIO in compose, getting-started, rclone doc | 1–2 h |
| C. KMS region, encrypted signing key + migration, delete dead lib, Redis TLS | 3–4 h |
| D. Fonts, tiles, geocoder URLs, analytics switch, logo | 2–3 h |
| E. Kustomize base and overlays + cert-manager, compose production profile, `docs/deploying.md` provider matrix | 5–7 h |
| **Total** | **about 18–25 h, 5–6 sessions** |

## Verification
- **With AWS config:** an existing install with only the old `AWS_*` variables still uploads and presigns.
- **Garage and MinIO:** the storage integration tests pass. Local dev runs with no AWS credentials at all: upload an avatar, a cover and a photo, and see them load in the browser through the public endpoint.
- **A real non-AWS provider:** run the same tests against, for example, a Scaleway or OVH bucket. Record the working settings in the provider matrix.
- **Upload hardening:** uploading an `.html` or `.svg` disguised as an image is rejected, and a polyglot image is re-encoded.
- **Keys:** after the migration, the `oauth_signing_keys.private_key` column holds ciphertext, tokens still verify, and KMS works in a region other than `ca-central-1`.
- **Web:** the map uses the tile URL from the env var, and with analytics off no request goes to Google.
- **k8s:** `kustomize build overlays/generic` deploys to minikube with TLS from a local issuer.

## Decisions (item 9)
- S3-compatible storage only, never the local filesystem (security). Small instances run Garage or MinIO.
- Keys stay local by default; the KMS region fix and the encrypted signing key are done now, OpenBao/Vault later.

## Dependencies
- **Do the storage part (A) before item 8's phase 0.4** (media proxy); both touch media URLs.
- **E overlaps item 8's phase 0.8** (ingress and TLS); do it once, here.
- **Rotating the AWS key is urgent** and independent of this item.

## Documentation

A phase is finished only when these pages match the code. anahita-services pages are under its `docs/`, anahita-web pages under its `docs/`. When a page is added, add it to that repo's `docs/README.md` contents too.

- **services `configuration.md`**, *Storage*: rewrite around `STORAGE_*`, noting the old `AWS_*` fallbacks; *OAuth signing and encryption*: `CRYPT_KMS_REGION` and the encrypted signing key; Redis TLS; geocoder URLs.
- **services `getting-started.md`**: local storage with Garage or MinIO and no AWS account; moving buckets with `rclone`.
- **New services page `deploying.md`**: the provider matrix (AWS, OVHcloud, Scaleway, Hetzner, Garage, MinIO) with tested settings, Kustomize overlays, cert-manager, and the single-VM compose profile.
- **services `architecture.md`**: file-grpc is S3-compatible; the dead `anahita-libs/storage` package is gone; media served from its own host.
- **web `configuration.md`**: `REACT_APP_MAP_TILE_URL`, `REACT_APP_ANALYTICS`.
- **web `deploying.md`**: hosting described provider-neutrally, with Amplify as one option.
- **web `README.md`** (repo root): the logo no longer loads from S3.

## What was built (2026-10-03)

**A. Storage.** file-grpc-service reads `STORAGE_*` (endpoint, public endpoint, region, bucket, keys, path style, SSE, ACL, allowed types), with the old `AWS_*` names as fallbacks. Links are signed by a second client on the public endpoint. Every upload is judged by its bytes and stored only if its real type is on the allowed list. `Exists` tells "not found" from other errors, and `Delete` no longer waits five minutes. Tests: unit tests, and an integration test (`storage_integration_test.go`) that passes on Garage 2.1.

**B. Local storage.** docker-compose runs Garage, prepared by `scripts/storage-init.sh` (`make storage_init`). No AWS account is needed for development.

**C. Keys.** The locally stored token signing key is sealed with `CRYPT_BACKEND` (`sealed:v1:` prefix), and existing keys are sealed at start-up. The KMS region comes from `CRYPT_KMS_REGION` or the SDK's default, and the client is built once. `anahita-libs/storage` and aws-sdk-go v1 are gone. `REDIS_TLS` was added.

**D. Third parties.** auth-service serves Roboto itself. `IP_LOCATION_URL` (with `off`), `GEOCODER_PHOTON_URL`, `GEOCODER_NOMINATIM_URL` and `GEOCODER_CONTACT` are settings. In the web app, analytics is off by default and can be Plausible, Umami or Matomo, map tiles are configurable, and the logo is in the repository.

**E. Kubernetes.** `k8s/` is a Kustomize base with components and three overlays, described in `docs/deploying.md`:

- `base/`: one manifest per service, each given the same variables as in `docker-compose.yml`, from a generated ConfigMap (`anahita-config`) and Secret (`anahita-secrets`). Services are named as in compose, so the gateway is the same nginx image with the same `nginx.conf`, and the Ingress sends everything to it.
- `components/`: MySQL, Redis and RabbitMQ as StatefulSets on the default StorageClass, for sites without managed ones.
- `overlays/generic` (any cluster, any S3-compatible store, cert-manager and Let's Encrypt), `overlays/aws` (EKS: S3 through a pod role, RDS, ElastiCache) and `overlays/local` (minikube).
- The gateway's allowed origin was hard-coded to `http://localhost:3000`, which no live site could use. It is now `WEB_ORIGIN`, filled in by the nginx image's template step.
- feed-service and graph-grpc-service defaulted to `/defaults`, where their images hold nothing. They now default to `/app/defaults`.
- Makefile: the `k8s-*` targets were rewritten and always name the minikube context. The sweeper image is now built and pushed. `make nginx` did nothing, because a directory has that name. `LOCATION_VERSION` is `latest` like the others.

**How E was checked.** The `local` overlay was applied to a minikube cluster: all 23 pods became ready, the migration job created the schema with a non-root database account, a manual sweeper run completed, auth-service created and sealed its signing key and seeded the OAuth clients from the ConfigMap, the public routes answered through the gateway and through the ingress, and the cluster reached the development Garage. With cert-manager and a self-signed issuer in place of Let's Encrypt, the `generic` ingress got its certificate and served both hostnames over HTTPS. The `generic` and `aws` overlays passed a server-side dry run.

**Not done, or done differently:**

- **No production docker-compose profile.** Live sites run on Kubernetes (decided 2026-10-03). docker-compose stays the development setup.
- **Only AWS S3 and Garage are tested.** The MinIO image could not be pulled, and no account was at hand for OVHcloud, Scaleway or Hetzner. `docs/deploying.md` lists their settings from the providers' documentation, marked untested, with the one-minute test to run.
- **The `aws` overlay has not run on EKS.** No upload was made from inside the test cluster, and no real Let's Encrypt certificate was issued.
- **`scripts/s3-migration` was kept.** It converts the PHP file layout, which `rclone` does not do. It reads `STORAGE_*` now.
- **Google Analytics was dropped,** not kept as an option.
- **`STORAGE_PREFIX` was not added.** The variable it would have replaced was never used.
- **A namespaced `Issuer`, not a `ClusterIssuer`,** so an overlay stays inside its namespace. The StorageClass is the cluster's default, with a patch in `docs/deploying.md` to name another.
- **OpenBao/Vault Transit** for sealing and signing is still a follow-up.
- **Not found until this item:** auth-service decides which host is "the auth host" from `ENV` and `DOMAIN_NAME` alone (`IsAuthHost`), ignoring `AUTH_URL`. A live site therefore needs `auth.DOMAIN_NAME` to exist. Item 10 revisits hosting.

## Status

| Item | State |
| --- | --- |
| Plan | written 2026-09-28 |
| Parts A–D | done 2026-10-03, on branch `m2-foundations` |
| Part E (Kubernetes) | done 2026-10-03, on branch `m2-foundations` |
| Rotate the AWS key in `k8s/00-secret.yml` | open, for the site owner. The file is ignored by git and was left in place |
