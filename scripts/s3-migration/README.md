# Move a bucket's pictures to the v1 layout

Avatars, covers and photos are kept under `assets/v1/`:

~~~
assets/v1/identity-image-service/avatars/<kind>/<id>/<name>_<size>.<ext>
assets/v1/identity-image-service/covers/<kind>/<id>/<name>_<size>.<ext>
assets/v1/photo-service/photo/<owner id>/<name>_<size>.<ext>
~~~

This script moves a bucket there from either of the two layouts before it:

| `-from` | Where the files are | Whose layout |
| --- | --- | --- |
| `v0` | `assets/public/n<node id>/com_<component>/…` | The PHP application's. This is the file half of an upgrade from it; the database half is `migration-job/legacy/upgrade.sql` |
| `v2` | `assets/v2/<service>/…`, with avatars under `avatar-service` and covers under `cover-service` | The one this platform used while it was being built, before its first release |

Videos, exports and link previews are not under `assets/` and are not
touched.

## Three steps

Each step can be run again. The store copies the files itself: nothing is
downloaded.

~~~sh
# 1. What would be copied. Changes nothing.
go run . -from v2

# 2. Copy. The old files stay where they are.
go run . -from v2 -apply

# 3. Once the site shows its pictures from the new places, remove the old.
go run . -from v2 -apply -delete-old
~~~

Between steps 2 and 3, deploy the services that read the v1 layout and
look at the site. Until step 3 both layouts are in the bucket, so going
back is a matter of deploying the services from before.

The bucket is read once at the start, and files are copied two dozen at
a time, so tens of thousands take minutes. Each step prints how many files
went where; `-v` names every one.

An old file is deleted only when its copy is in place and the same size.
A file that is not a picture this knows is left where it is and listed.

## Settings

It reads the same settings as file-grpc-service
([Configuration › Storage](../../docs/configuration.md#storage)):

~~~sh
STORAGE_BUCKET=my-bucket STORAGE_REGION=ca-central-1 go run . -from v2
~~~

For a store other than AWS S3, also set `STORAGE_ENDPOINT`,
`STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY` and, usually,
`STORAGE_PATH_STYLE=true`. On AWS the credentials come from the usual AWS
environment or profile when those two are unset: `AWS_PROFILE=name` picks
a profile from `~/.aws/credentials`. The earlier names
`AWS_FILES_S3_BUCKET` and `AWS_REGION` are read as well.

To move a bucket from one store to another without changing its layout, use
[rclone](https://rclone.org/) instead.
