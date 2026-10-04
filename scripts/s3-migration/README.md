# Migrate file paths from the PHP layout to the current one

The PHP app stored uploads under `assets/public/n<node id>/com_<component>/…`.
The Go services read them from `assets/v2/<service>/<type>/<node id>/…`. This
script copies every avatar, cover and photo in a bucket from the old layout to
the new one. It is the file half of an upgrade from the PHP app; the database
half is `migration-job/legacy/upgrade.sql`.

It only copies. The old objects are left in place, so it can be run again, and
removed by hand once the new paths are confirmed.

## Running

It reads the same settings as file-grpc-service
([Configuration › Storage](../../docs/configuration.md#storage)):

~~~sh
STORAGE_BUCKET=my-bucket STORAGE_REGION=ca-central-1 go run .
~~~

For a store other than AWS S3, also set `STORAGE_ENDPOINT`,
`STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY` and, usually, `STORAGE_PATH_STYLE=true`.
On AWS the credentials come from the usual AWS environment or profile when
those two are unset.

To move a bucket from one store to another without changing its layout, use
[rclone](https://rclone.org/) instead.
