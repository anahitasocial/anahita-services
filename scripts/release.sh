#!/bin/sh
# Makes a Birth release of anahita-services (docs/releases.md):
#
#   make release v1.2.4
#
# The services lead; Anahita Web and the other clients have their own
# versions, each saying which services releases it works with, and are
# released on their own (the user, 2026-11-09).
#
# This repository is looked at first, and nothing changes unless it is
# ready: on main, clean, level with GitHub, the version after its newest
# tag, and its tests passing. Then: the version raised (softwareVersion)
# and the changelog's Embryo section named after the version and the day,
# in one commit; main pushed; that commit tagged, and the tag pushed.
# Always in that order, so a tag cannot land on a commit saying the version
# before it, which the image workflow refuses.
#
# Nothing here moves a tag. Once images exist under a version, moving its
# tag would make that one name mean two things.
set -eu

version=${1:-}
number=${version#v}
day=$(date +%Y-%m-%d)
node_info=auth-service/cmd/api/handlers/node_info.go

stop() {
  echo "$1" >&2
  exit 1
}

echo "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
  stop "Say which version: make release v1.2.4"

# On main, clean, level with GitHub, the version new.
[ "$(git branch --show-current)" = main ] || stop "A release is made from main."
# go.work.sum gains lines by itself as Go runs, and no build needs them.
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
grep -q '^## Embryo$' CHANGELOG.md || stop "CHANGELOG.md has no Embryo section: nothing to release."

make test

# The version NodeInfo says, and the changelog.
if [ "$(sed -n 's/^var softwareVersion = "\(.*\)"$/\1/p' "$node_info")" != "$number" ]; then
  sed -i.bak "s/^var softwareVersion = \".*\"$/var softwareVersion = \"$number\"/" "$node_info"
  rm -f "$node_info.bak"
fi
[ "$(sed -n 's/^var softwareVersion = "\(.*\)"$/\1/p' "$node_info")" = "$number" ] ||
  stop "softwareVersion was not raised in $node_info."
awk -v h="## $version Birth, $day" '!done && /^## Embryo$/ { print h; done = 1; next } { print }' CHANGELOG.md > CHANGELOG.md.new
mv CHANGELOG.md.new CHANGELOG.md
grep -q "^## $version Birth, $day\$" CHANGELOG.md || stop "The Embryo section was not named $version."

git commit -q -m "release - $version" \
  -m "- changed: softwareVersion is $number; the changelog's Embryo is $version Birth, $day" \
  -- "$node_info" CHANGELOG.md

# Pushed, then tagged.
git push origin main
git tag "$version" && git push origin "$version"

echo "$version is tagged, and its images are building on GitHub."
echo "Next: the release notes on GitHub, then anahita-recipes once the images are built (docs/releases.md)."
