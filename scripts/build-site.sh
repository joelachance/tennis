#!/bin/sh
# Build the site: the marketing app at the root, the docs app merged into
# /docs. They are two Next apps because they are two apps, but they ship as
# one static directory, because a domain serves one site and the docs are not
# a separate deployment with its own origin.
#
# This is the build command for the Cloudflare Pages project, and it is also
# runnable by hand, which is the point: the same four lines that live in a
# dashboard text field are the four lines you can execute locally.
#
#   scripts/build-site.sh && npx serve www/out
set -eu

root="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
out="$root/www/out"

# npm ci rather than npm install, so the lockfiles decide what ships. It
# clears node_modules first, which discards the Pages build cache — if the
# builds get slow enough to notice, switch to npm install and let the lockfile
# be a backstop rather than the mechanism.
for app in www docs; do
  printf '==> %s\n' "$app"
  (cd "$root/$app" && npm ci --no-audit --no-fund && npm run build)
done

# Into a fresh directory: moving onto an existing one nests rather than
# merges, and a stale www/out/docs left by an interrupted build would
# otherwise end up serving at /docs/docs.
rm -rf "$out/docs"
mv "$root/docs/out" "$out/docs"

printf 'site is in %s\n' "$out"