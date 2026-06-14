# Releasing & publishing `pulumi-postmark`

This guide explains how the provider is distributed, what's automated, and the
one-time setup needed for each package registry.

## How a Pulumi provider is distributed

Unlike a single npm package, a Pulumi provider ships as **several artifacts**:

| Artifact | Where it lives | How consumers get it | Registry account needed? |
|---|---|---|---|
| **Plugin binaries** (`pulumi-resource-postmark`) | GitHub Release assets | Auto-installed by `pulumi up` (the schema bakes in the GitHub download URL), or `pulumi plugin install` | ❌ None (just GitHub) |
| **Go SDK** | the git repo itself (`/sdk`, tagged `sdk/vX.Y.Z`) | `go get github.com/hummingbird-me/pulumi-postmark/sdk` | ❌ None (just git) |
| **Python SDK** | PyPI (`hummingbird-me-postmark`) | `pip install` | ✅ PyPI |
| **Node/TS SDK** | npm (`@hummingbird-me/postmark`) | `npm install` | ✅ npm (the `@hummingbird-me` scope) |
| **.NET SDK** | NuGet (`HummingbirdMe.Postmark`) | `dotnet add package` | ✅ NuGet |
| **Java SDK** | Maven Central (`com.hummingbirdme:postmark`) | Gradle/Maven dep | ✅ Sonatype (hard — see below) |

**The important takeaway:** the first two rows require **no external accounts** and
make the provider fully usable on their own. Everything else is optional and can
be turned on one registry at a time.

## What's automated

Three workflows under `.github/workflows/`:

- **`ci.yml`** — every push/PR: builds, `go vet`, `gofmt`, tests, verifies
  `schema.json` is up to date, builds the Go SDK and the example. No secrets.
- **`release.yml`** — on a `vX.Y.Z` tag: builds the plugin binaries for
  linux/darwin/windows × amd64/arm64, publishes a **GitHub Release** with the
  tarballs + checksums, and pushes the **`sdk/vX.Y.Z`** tag for the Go SDK. Uses
  only the built-in `GITHUB_TOKEN`. **Works with zero setup.**
- **`publish-sdks.yml`** — on a `vX.Y.Z` tag: builds each language SDK, and
  **publishes only if the matching secret is present** (otherwise it logs "skipping"
  and passes). So it's safe to have enabled before you've set up any registry.

## Cutting a release

```bash
# 1. Regenerate the SDKs + schema at the new version and commit them
#    (the SDK packages carry the version in their manifests).
make schema build_sdks VERSION=0.2.0
git commit -am "chore: regenerate SDKs and schema for v0.2.0"

# 2. Tag and push. The tag is what triggers release.yml + publish-sdks.yml.
git tag v0.2.0
git push origin main v0.2.0
```

That's it. `release.yml` produces the GitHub Release + binaries + Go SDK tag
immediately; `publish-sdks.yml` publishes whichever language registries you've
configured.

> Tip: keep `VERSION` in the tag and in `make … VERSION=` identical. The plugin
> binary's version comes from the tag; the SDK packages' versions come from the
> regenerated manifests.

## One-time registry setup

Add each token under **Settings → Secrets and variables → Actions → New repository secret**.

### Go — nothing to do ✅
Pushing the `sdk/vX.Y.Z` tag (automated) is the whole "publish". The Go module
proxy serves it on first `go get`.

### npm — `@hummingbird-me/postmark` (you've done this before)
1. Create/own the **`hummingbird-me`** npm org/scope (npmjs.com → add org).
2. Create an **automation** access token (Account → Access Tokens → Granular/Automation).
3. Add it as secret **`NPM_TOKEN`**.
   - Because the package is scoped and public, the workflow publishes with
     `--access public`.

### PyPI — `hummingbird-me-postmark` (similar to rubygems)
1. Make a [PyPI](https://pypi.org) account.
2. Account settings → **API tokens** → create one (scope it to the project after the
   first upload, or account-wide for the first publish).
3. Add it as secret **`PYPI_TOKEN`** (the workflow uses it as the `__token__` password).
   - *Optional, nicer:* PyPI [Trusted Publishing](https://docs.pypi.org/trusted-publishers/)
     lets the workflow authenticate via GitHub OIDC with **no token**. To switch,
     configure a trusted publisher on PyPI for this repo/workflow and replace the
     publish step with `pypa/gh-action-pypi-publish`.

### NuGet — `HummingbirdMe.Postmark`
1. Make a [nuget.org](https://www.nuget.org) account (sign in with Microsoft/GitHub).
2. **API Keys** → create a key (scope: Push, glob `HummingbirdMe.*`).
3. Add it as secret **`NUGET_API_KEY`**.
   - Cosmetic: the generated `.csproj` lists `Authors = Pulumi Corp.`. To fix it for
     all SDKs, set a publisher in `provider/provider.go` (`.WithPublisher("hummingbird-me")`)
     and regenerate, or edit `sdk/dotnet/*.csproj` before packing.

### Java / Maven Central — deferred (the hard one)
Maven Central is materially more work than the others and is **not wired up**:
- You need a **Sonatype Central** account and a **verified namespace**. The generated
  coordinates use `com.hummingbirdme`, which you can't claim; the realistic choice is
  the GitHub-verified namespace **`io.github.hummingbird-me`** (regenerate the Java SDK
  with that package base), or your own domain.
- Maven Central requires **GPG-signed** artifacts (a published key) and a portal token.

Recommendation: skip Java until there's demand. If/when you want it, the cleanest path
is the Sonatype Central Portal + the `gradle-nexus.publish-plugin` already present in
the generated `sdk/java/build.gradle`, plus `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` /
`OSSRH_USERNAME` / `OSSRH_PASSWORD` secrets and a `maven` job mirroring the others.

### Pulumi Registry listing — optional discoverability
The provider already works without being listed (the schema's `pluginDownloadURL`
points consumers at your GitHub Releases). Listing it on
[registry.pulumi.com](https://www.pulumi.com/registry/) only adds a docs page and
discoverability; follow Pulumi's
[publishing docs](https://www.pulumi.com/docs/iac/guides/pulumi-packages/) when you want it.

## Suggested rollout order

1. **Tag `v0.1.0` now** → GitHub Release + Go SDK. The provider is usable by anyone:
   ```
   pulumi plugin install resource postmark v0.1.0 --server github://api.github.com/hummingbird-me/pulumi-postmark
   ```
   (and Go programs `go get …/sdk`). Zero registry accounts required.
2. Add **`NPM_TOKEN`** when you want `npm install` to work.
3. Add **`PYPI_TOKEN`** and **`NUGET_API_KEY`** as needed.
4. Defer **Maven Central** and the **Pulumi Registry** listing until there's demand.
