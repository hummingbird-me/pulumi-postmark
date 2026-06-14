# Releasing & publishing `pulumi-postmark`

This guide explains how the provider is distributed, what's automated, and the
one-time setup needed to publish.

## How a Pulumi provider is distributed

Unlike a single npm package, a Pulumi provider ships as **several artifacts**:

| Artifact | Where it lives | How consumers get it | Account needed? |
|---|---|---|---|
| **Plugin binaries** (`pulumi-resource-postmark`) | GitHub Release assets | Auto-installed by `pulumi up` (the schema bakes in the GitHub download URL), or `pulumi plugin install` | ❌ none (GitHub) |
| **Go SDK** | the git repo (`/sdk`, tagged `sdk/vX.Y.Z`) | `go get github.com/hummingbird-me/pulumi-postmark/sdk` | ❌ none (git) |
| **Node/TS SDK** | npm (`@kitsu-io/pulumi-postmark`) | `npm install @kitsu-io/pulumi-postmark` | ✅ npm (`@kitsu-io` scope) |
| Python / .NET / Java SDKs | _not currently published_ | build from `sdk/<lang>` if needed | — |

The first two rows require **no external accounts** and make the provider fully
usable on their own. npm is the only registry currently wired up; PyPI, NuGet and
Maven Central are intentionally parked (see the end of this doc).

## What's automated

Three workflows under `.github/workflows/`:

- **`ci.yml`** — every push/PR: build, `go vet`, `gofmt`, tests, verifies
  `schema.json` is up to date, and builds the Go SDK + example. No secrets.
- **`release.yml`** — on a `vX.Y.Z` tag: builds the plugin binaries for
  linux/darwin/windows × amd64/arm64, publishes a **GitHub Release** with the
  tarballs + checksums, and pushes the **`sdk/vX.Y.Z`** tag for the Go SDK. Uses
  only the built-in `GITHUB_TOKEN`. **Works with zero setup.**
- **`publish-sdks.yml`** — on a `vX.Y.Z` tag: builds the npm SDK and publishes it
  **only if `NPM_TOKEN` is set** (otherwise it logs "skipping" and passes).

## Cutting a release

```bash
# 1. Regenerate the SDKs + schema at the new version and commit them
#    (the SDK packages carry the version in their manifests).
make schema build_sdks VERSION=0.2.0
git commit -am "chore: regenerate SDKs and schema for v0.2.0"

# 2. Tag and push. The tag triggers release.yml + publish-sdks.yml.
git tag v0.2.0
git push origin main v0.2.0
```

`release.yml` produces the GitHub Release + binaries + Go SDK tag immediately;
`publish-sdks.yml` publishes npm if `NPM_TOKEN` is configured.

> Keep the tag version and `make … VERSION=` identical: the plugin binary's
> version comes from the tag; the npm package version comes from the regenerated
> `sdk/nodejs/package.json`.

## One-time setup

### Go — nothing to do ✅
Pushing the `sdk/vX.Y.Z` tag (automated) is the whole "publish". The Go module
proxy serves it on first `go get`.

### npm — `@kitsu-io/pulumi-postmark`
1. Make sure the **`kitsu-io`** npm org/scope exists and you can publish to it.
2. Create an **automation** access token (npmjs.com → Access Tokens → Granular/Automation,
   with publish rights to the scope).
3. Add it as repo secret **`NPM_TOKEN`** (Settings → Secrets and variables → Actions).

That's it — the next `vX.Y.Z` tag publishes `@kitsu-io/pulumi-postmark` (scoped &
public, via `npm publish --access public`).

## Suggested rollout

1. **Tag `v0.1.0` now** → GitHub Release + Go SDK. The provider is immediately usable:
   ```
   pulumi plugin install resource postmark v0.1.0 --server github://api.github.com/hummingbird-me/pulumi-postmark
   ```
   (and Go programs `go get …/sdk`). No registry accounts required.
2. Add **`NPM_TOKEN`** whenever you want `npm install` to work.

## Adding PyPI / NuGet / Maven later

The SDK sources for Python, .NET and Java are still generated into `sdk/` and build
fine — they're just not published. To add one:

- **PyPI** (`hummingbird_me_postmark`, or rename via `language.python.packageName`
  in `provider/provider.go`): add a `pypi` job that runs `python -m build` in
  `sdk/python` and `twine upload` with a `PYPI_TOKEN` secret (or OIDC trusted publishing).
- **NuGet** (`HummingbirdMe.Postmark`): add a `nuget` job that runs `dotnet pack` in
  `sdk/dotnet` and `dotnet nuget push` with a `NUGET_API_KEY` secret. Note the generated
  `.csproj` lists `Authors = Pulumi Corp.` — set a publisher in `provider/provider.go`
  and regenerate, or edit the `.csproj` first.
- **Maven Central** (the hard one): needs a Sonatype Central account, a verified
  namespace (e.g. `io.github.hummingbird-me` — regenerate the Java SDK with that base),
  and GPG signing. The generated `sdk/java/build.gradle` already includes the
  `gradle-nexus.publish-plugin`; add `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` /
  `OSSRH_USERNAME` / `OSSRH_PASSWORD` secrets and a `maven` job.

### Pulumi Registry listing — optional
The provider works without it (the schema's `pluginDownloadURL` points consumers at
your GitHub Releases). Listing on [registry.pulumi.com](https://www.pulumi.com/registry/)
only adds a docs page; follow Pulumi's publishing docs when you want it.
