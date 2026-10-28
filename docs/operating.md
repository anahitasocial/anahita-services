# Operating

[Documentation](README.md) › Operating

The pages for running a live site from day to day.

1. **Upgrading** to a new release: [Upgrading](deploying.md#upgrading). The
   migration job brings the database up to date.
2. **The database, Redis and RabbitMQ.** What runs in the cluster, and using a
   managed service instead:
   [Database, Redis and RabbitMQ](deploying.md#database-redis-and-rabbitmq).
   Nothing backs them up for you.
3. **Storage.** What is in the bucket, and moving an old one to the current
   layout: [Object storage](deploying.md#object-storage).
4. **Running more than one copy** of a service:
   [Running more than one copy of a service](deploying.md#running-more-than-one-copy-of-a-service).
5. **Accounts.** Deleted accounts, the 30 days to restore them, purging, the
   accounts list and the sweeper that clears out unused ones, with the
   hashtags and locations nothing uses any more:
   [Account lifecycle](account-lifecycle.md).
6. **Reports.** What administrators do with what people report:
   [What administrators do](abuse-reports.md#what-administrators-do).
7. **Federation.** Keeping the fediverse and Bluesky working, and what to
   check when they do not:
   [Keeping it running](federation.md#keeping-it-running) and
   [When something does not work](federation.md#when-something-does-not-work).
8. **Apps that sign in.** The web app and other clients, and changing what
   they may do: [OAuth scopes and clients](oauth.md).

Not written yet:

1. **Backing up and restoring:** what to back up (MySQL, the bucket, the keys
   in `secrets.env`), how often, and how to restore a site from it.
2. **Upgrading from one release to the next,** step by step: what to read
   first, and what to do when a migration fails.
