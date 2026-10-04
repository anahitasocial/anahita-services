#!/bin/sh
# Prepares the local Garage store that docker-compose runs: gives its one node
# a role, and makes the development bucket and access key.
#
# Safe to run every time. Each step checks whether it has already been done.
# `make up` and `make up_build` run it; it can also be run by hand.
#
# The key below is published in this repository and is for local development
# only. It matches the STORAGE_* lines in docs/getting-started.md.

set -eu

BUCKET="${STORAGE_DEV_BUCKET:-anahita}"
KEY_ID="GKa0b1c2d3e4f5a6b7c8d9e0f1"
KEY_SECRET="a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1"
KEY_NAME="anahita-dev"

garage() {
    docker-compose exec -T -e RUST_LOG=warn garage /garage "$@"
}

if ! docker-compose ps --status running --services 2>/dev/null | grep -qx garage; then
    echo "storage-init: the garage container is not running, nothing to prepare."
    exit 0
fi

# The node answers a few seconds after the container starts.
tries=0
until garage status >/dev/null 2>&1; do
    tries=$((tries + 1))
    if [ "$tries" -gt 30 ]; then
        echo "storage-init: garage did not become ready." >&2
        exit 1
    fi
    sleep 1
done

# 1. A role for the node. Without one Garage stores nothing.
if garage status 2>/dev/null | grep -q "NO ROLE ASSIGNED"; then
    node=$(garage node id -q 2>/dev/null | cut -d@ -f1)
    garage layout assign -z dev -c 1G "$node" >/dev/null
    garage layout apply --version 1 >/dev/null
    echo "storage-init: assigned the node its role."
fi

# 2. The access key, with a fixed id and secret so every checkout agrees.
if ! garage key info "$KEY_ID" >/dev/null 2>&1; then
    garage key import --yes -n "$KEY_NAME" "$KEY_ID" "$KEY_SECRET" >/dev/null
    echo "storage-init: imported the development access key."
fi

# 3. The bucket, readable and writable by that key and nobody else.
if ! garage bucket info "$BUCKET" >/dev/null 2>&1; then
    garage bucket create "$BUCKET" >/dev/null
    echo "storage-init: created the bucket \"$BUCKET\"."
fi
garage bucket allow --read --write --owner "$BUCKET" --key "$KEY_ID" >/dev/null

echo "storage-init: local storage is ready (bucket \"$BUCKET\")."
