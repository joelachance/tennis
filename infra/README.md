# tennis on Cloudflare

Two Cloudflare properties, each doing one job:

| Host                    | Product        | Serves                                     |
| ----------------------- | -------------- | ------------------------------------------ |
| `use.tennis`            | Cloudflare Pages | the marketing site, and the docs at `/docs` |
| `download.use.tennis`   | R2               | `/install.sh` and the release archives      |

Nothing here is deployed by this repo. The Pages deploy and the R2 publish are
both driven from `.github/workflows/`, so this directory is just the one-time
account setup and the map of what lives where.

This replaced an AWS CloudFront + S3 stack that used to live here as a CDK app.
The layout it published is unchanged, so the installer did not have to change;
only the host moved.

## One-time setup

### 1. The zone

Add `tennis` as a Cloudflare zone and point the registrar's nameservers at
Cloudflare. Wait for delegation before going further — a certificate or
custom-domain connection against a zone the internet cannot see yet fails:

```bash
dig +short NS tennis
```

### 2. R2, and the bucket behind download.use.tennis

R2 asks for a payment method on first use, even inside the free tier. Without
one the service stops rather than degrades when the allowance runs out, so set
a billing alert. At this project's volumes (four ~5MB archives per release) the
10 GB free allowance is not a near-term concern, and egress is free regardless
of how much a release gets downloaded.

```bash
npx wrangler login
npx wrangler r2 bucket create tennis-releases
```

Then connect the custom domain. The zone ID is on the zone's Overview page:

```bash
npx wrangler r2 bucket domain add tennis-releases \
  --domain download.use.tennis --zone-id <ZONE_ID>
```

Equally: R2 → bucket → Settings → Public access → Custom Domains → Connect
Domain. Cloudflare writes the CNAME itself.

Use a custom domain rather than the `r2.dev` URL. `r2.dev` is rate-limited and
explicitly not for production use, and it is the only endpoint that does not sit
behind Cloudflare's cache.

A custom domain makes every object in the bucket public to anyone holding the
URL. That is correct here — release binaries are public — but it is a real
property of the bucket, not something the deploy configures.

### 3. The API token the release workflow uses

My Profile → API Tokens → Create Account API Token → **R2 Storage Token**,
permission `Account` → `R2 Storage` → `Edit`, scoped to the `tennis-releases`
bucket rather than the whole account. That token and the account ID are the
only two credentials CI holds.

### 4. Pages, for use.tennis

Create a Pages project named `tennis`. `pages.yml` deploys it, so nothing here
needs a build command or output directory configured — set the production
branch to `main` and attach the `use.tennis` custom domain under Pages →
Custom domains.

The site was a GitHub project page before, so its Next apps carried a
`/tennis` basePath that a real domain does not need. That prefix is gone from
`www/next.config.ts`, `docs/next.config.mjs`, and the two places that hardcoded
`/tennis/docs`.

## What ends up on download.use.tennis

```text
/install.sh                                 the installer            5 min
/VERSION                                    "0.1.0"                  5 min
/manifest.json                              version + per-platform   5 min
/404.txt                                    not-found body           5 min
/latest/tennis_<os>_<arch>.tar.gz           unversioned alias        5 min
/latest/tennis_<os>_<arch>.tar.gz.sha256                             5 min
/v0.1.0/tennis_0.1.0_<os>_<arch>.tar.gz     the real payload         1 year, immutable
/v0.1.0/checksums.txt                       goreleaser's checksums   1 year, immutable
```

`install.sh` reads `/VERSION`, then downloads the versioned archive and checks
it against that release's `checksums.txt`. The payload is therefore always an
immutable key. R2 stores `Cache-Control` on the object itself and Cloudflare's
edge honours it, so those two rows need no invalidation — which is the one
piece of machinery the CloudFront version needed and this one does not. Only
the small pointer files are short-lived, and a new release becomes visible as
soon as their TTL lapses.

`/latest/` exists for Dockerfiles that would rather not do the version lookup.
Nothing in the install path depends on it.

## Repository wiring

```bash
gh variable set TENNIS_DOWNLOAD_HOST   --body download.use.tennis
gh variable set TENNIS_DOWNLOAD_BUCKET --body tennis-releases
gh secret   set CLOUDFLARE_ACCOUNT_ID  --body <account id>
gh secret   set CLOUDFLARE_API_TOKEN   --body <r2 token>
```

Until all four are set the release workflow skips the publish step and cuts a
GitHub release only, so a tag never fails on missing infrastructure. That
fallback is also what the installer uses before the first R2 publish:
`scripts/install.sh` carries a `TENNIS_DOWNLOAD_HOST` placeholder that the
release workflow substitutes on upload, and an unsubstituted copy falls back to
reading the archive straight from the GitHub release.

## Publishing a release

```bash
git tag v0.3.0 && git push origin v0.3.0
```

The workflow tests, runs goreleaser, uploads to R2, then re-downloads `/VERSION`
and `/install.sh` from the edge to confirm the release is actually reachable
before it goes green.

## Pointing the docs at the new host

The checked-in docs quote the raw.githubusercontent URL for `install.sh`, which
works before the bucket exists. Once `download.use.tennis/VERSION` resolves,
switch `README.md`, `docs/content/docs/getting-started.mdx`,
`www/components/Hero.tsx`, and `www/PLAN.md` to:

```bash
curl -fsSL https://download.use.tennis/install.sh | sh
```

and re-run `npm run sync-llms` in `docs/` to regenerate `llms-full.txt`.

Do not edit the placeholder in `scripts/install.sh`. It has to stay
unsubstituted in git; the workflow replaces it only in the copy it uploads.

## Notes

- Users can pin a version (`TENNIS_VERSION=0.1.0`), redirect the install
  directory (`TENNIS_INSTALL_DIR`), or point at another host
  (`TENNIS_INSTALL_BASE_URL`) — that last one is how the installer is tested
  against a local stand-in in CI.
- Changing the host means updating `TENNIS_DOWNLOAD_HOST` and cutting a
  release; the installer picks it up on the next release with no code change.