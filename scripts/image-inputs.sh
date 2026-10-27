#!/bin/sh
# Prints one hash of everything an image is made from: its Dockerfile, the
# image it starts FROM, and every file it COPYs, the compiled binaries
# among them. Two builds with the same hash make the same image, so a
# release rebuilds only the images whose hash has changed, and re-tags the
# rest (.github/workflows/images.yml; docs/releases.md).
#
# The binaries must be built with GOFLAGS="-buildvcs=false -trimpath", or
# every build stamps the commit into them and every hash changes.
#
#   scripts/image-inputs.sh dockerfiles/auth-service.dockerfile
#
# With BASE_DIGEST set (the digest of the FROM image, which the workflow
# looks up), a new base image changes the hash too.
set -eu

file=$1
sum() {
  if command -v sha256sum >/dev/null; then sha256sum; else shasum -a 256; fi | cut -c1-64
}

{
  echo "dockerfile $(sum <"$file")"
  grep '^FROM ' "$file"
  echo "base ${BASE_DIGEST:-}"
  # Every source of every COPY: all but its last word, and its options.
  awk '/^COPY / { for (i = 2; i < NF; i++) if ($i !~ /^--/) print $i }' "$file" |
    while read -r source; do
      find "$source" -type f | LC_ALL=C sort | while read -r path; do
        echo "$path $(sum <"$path")"
      done
    done
} | sum
