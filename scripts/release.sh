#!/bin/sh
# Makes a Birth release (docs/releases.md): raises the version NodeInfo says,
# commits it, pushes main, then tags that commit and pushes the tag. Always
# in that order, so a tag cannot land on a commit saying the version before
# it, which the image workflow refuses.
#
#   make release v1.0.14
#
# Nothing here moves a tag. Once images exist under a version, moving its
# tag would make that one name mean two things.
set -eu

version=${1:-}
file=auth-service/cmd/api/handlers/node_info.go

stop() {
  echo "$1" >&2
  exit 1
}

said() {
  sed -n 's/^var softwareVersion = "\(.*\)"$/\1/p' "$file"
}

echo "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
  stop "Say which version: make release v1.0.14"

[ "$(git branch --show-current)" = main ] || stop "A release is made from main."

# go.work.sum changes by itself and is never committed.
[ -z "$(git status --porcelain -- . ':!go.work.sum')" ] ||
  stop "There are uncommitted changes: commit them or put them away first."

git fetch -q --tags origin
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] ||
  stop "main is not level with GitHub: push or pull first."

! git rev-parse -q --verify "refs/tags/$version" >/dev/null ||
  stop "$version is tagged already."

newest=$(git tag --list 'v*' --sort=-v:refname | head -n 1)
[ -z "$newest" ] ||
  [ "$(printf '%s\n%s\n' "$newest" "$version" | sort -V | tail -n 1)" = "$version" ] ||
  stop "$version is not after $newest."

make test

if [ "$(said)" != "${version#v}" ]; then
  sed -i.bak "s/^var softwareVersion = \".*\"$/var softwareVersion = \"${version#v}\"/" "$file"
  rm -f "$file.bak"
  [ "$(said)" = "${version#v}" ] || stop "softwareVersion was not raised in $file."
  git commit -q -m "release - $version" -m "- changed: softwareVersion is ${version#v}" -- "$file"
fi

git push origin main
git tag "$version"
git push origin "$version"

echo "$version is tagged, and its images are building on GitHub."
