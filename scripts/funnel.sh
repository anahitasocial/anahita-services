#!/bin/sh
# Opens a development stack to the internet through Tailscale Funnel, or closes
# it: `make up` opens it and `make down` closes it, for whoever asks for that
# with DEV_FUNNEL=on in .env (docs/getting-started.md, "Reachable from outside").
# Nobody else's stack is opened. The address is the machine's own, kept by
# Tailscale and in .env, and named nowhere here.
set -eu

action=${1:?on or off}
setting=$(sed -n 's/^DEV_FUNNEL=//p' .env 2>/dev/null | tr -d '"' | tail -1)
[ "$setting" = on ] || exit 0

if ! command -v tailscale >/dev/null 2>&1; then
    echo "funnel: DEV_FUNNEL=on, but tailscale is not installed." >&2
    exit 0
fi

case "$action" in
    on) tailscale funnel --bg 80 || echo "funnel: it did not open." >&2 ;;
    off) tailscale funnel --https=443 off || echo "funnel: it did not close." >&2 ;;
    *) echo "funnel: on or off, not $action." >&2; exit 1 ;;
esac
