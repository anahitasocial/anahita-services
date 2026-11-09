#!/bin/sh
# Makes a Birth release (docs/releases.md) of anahita-services and
# anahita-web together, with the same version, so that the latest tagged
# anahita-web always works with the latest tagged anahita-services.
#
#   ANAHITA_WEB=~/projects/anahita-web make release v1.2.0
#
# ANAHITA_WEB is where anahita-web is checked out; it is best set once, in
# one's shell profile. Without it nothing is done.
#
# Both are looked at first, and nothing changes in either unless both are
# ready: on main, clean, level with GitHub, the version after their newest
# tag, and their tests passing. Then in each: the version raised
# (softwareVersion here, package.json there) and the changelog's Embryo
# section named after the version and the day, in one commit; main pushed;
# that commit tagged, and the tag pushed. Always in that order, so a tag
# cannot land on a commit saying the version before it, which the image
# workflow refuses.
#
# Nothing here moves a tag. Once images exist under a version, moving its
# tag would make that one name mean two things.
set -eu

version=${1:-}
number=${version#v}
day=$(date +%Y-%m-%d)
services=$(pwd)
web=${ANAHITA_WEB:-}
node_info=auth-service/cmd/api/handlers/node_info.go

stop() {
  echo "$1" >&2
  exit 1
}

echo "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
  stop "Say which version: make release v1.2.0"
[ -n "$web" ] ||
  stop "Say where anahita-web is: ANAHITA_WEB=~/projects/anahita-web make release $version"
grep -q '"name": "anahita-web"' "$web/package.json" 2>/dev/null ||
  stop "$web is not a checkout of anahita-web."

# ready <name> <dir>: on main, clean, level with GitHub, the version new.
ready() {
  name=$1
  cd "$2"
  [ "$(git branch --show-current)" = main ] || stop "$name: a release is made from main."
  # go.work.sum gains lines by itself as Go runs, and no build needs them.
  [ -z "$(git status --porcelain -- . ':!go.work.sum')" ] ||
    stop "$name: there are uncommitted changes: commit them or put them away first."
  git fetch -q --tags origin
  [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] ||
    stop "$name: main is not level with GitHub: push or pull first."
  ! git rev-parse -q --verify "refs/tags/$version" >/dev/null ||
    stop "$name: $version is tagged already."
  newest=$(git tag --list 'v*' --sort=-v:refname | head -n 1)
  [ -z "$newest" ] ||
    [ "$(printf '%s\n%s\n' "$newest" "$version" | sort -V | tail -n 1)" = "$version" ] ||
    stop "$name: $version is not after $newest."
}

# changelog <file> <if-empty>: its Embryo section named after the version
# and the day. One named so already is left. With no Embryo section,
# anahita-services stops (a release with nothing in it), and anahita-web
# says it is tagged with the services.
changelog() {
  file=$1
  heading="## $version Birth, $day"
  if grep -q "^## $version Birth" "$file"; then
    return
  fi
  if grep -q '^## Embryo$' "$file"; then
    awk -v h="$heading" '!done && /^## Embryo$/ { print h; done = 1; next } { print }' "$file" > "$file.new"
    mv "$file.new" "$file"
    grep -q "^$heading\$" "$file" || stop "$file: the Embryo section was not named $version."
    return
  fi
  [ "$2" = stop ] && stop "$file has no Embryo section: nothing to release."
  awk -v h="$heading" '!done && /^## / { print h; print ""; print "Nothing changed here; tagged with Anahita Services."; print ""; done = 1 } { print }' "$file" > "$file.new"
  mv "$file.new" "$file"
}

ready anahita-services "$services"
ready anahita-web "$web"

cd "$services" && make test
cd "$web" && yarn test

# anahita-services: the version NodeInfo says, and the changelog.
cd "$services"
if [ "$(sed -n 's/^var softwareVersion = "\(.*\)"$/\1/p' "$node_info")" != "$number" ]; then
  sed -i.bak "s/^var softwareVersion = \".*\"$/var softwareVersion = \"$number\"/" "$node_info"
  rm -f "$node_info.bak"
fi
[ "$(sed -n 's/^var softwareVersion = "\(.*\)"$/\1/p' "$node_info")" = "$number" ] ||
  stop "softwareVersion was not raised in $node_info."
changelog CHANGELOG.md stop
if [ -n "$(git status --porcelain -- "$node_info" CHANGELOG.md)" ]; then
  git commit -q -m "release - $version" \
    -m "- changed: softwareVersion is $number; the changelog's Embryo is $version Birth, $day" \
    -- "$node_info" CHANGELOG.md
fi

# anahita-web: package.json, and the changelog.
cd "$web"
sed -i.bak "s/^  \"version\": \".*\",$/  \"version\": \"$number\",/" package.json
rm -f package.json.bak
grep -q "^  \"version\": \"$number\",$" package.json || stop "the version was not raised in anahita-web's package.json."
changelog CHANGELOG.md say
if [ -n "$(git status --porcelain -- package.json CHANGELOG.md)" ]; then
  git commit -q -m "release - $version" \
    -m "- changed: package.json version is $number; the changelog's Embryo is $version Birth, $day" \
    -- package.json CHANGELOG.md
fi

# Both pushed, then both tagged.
cd "$services" && git push origin main
cd "$web" && git push origin main
cd "$services" && git tag "$version" && git push origin "$version"
cd "$web" && git tag "$version" && git push origin "$version"

echo "$version is tagged in anahita-services, whose images are building on GitHub, and in anahita-web."
echo "Next: the release notes on GitHub, then anahita-recipes once the images are built (docs/releases.md)."
