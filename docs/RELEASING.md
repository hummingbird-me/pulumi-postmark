# Releasing & publishing `pulumi-postmark`

This repo follows the [pulumi-provider-boilerplate](https://github.com/pulumi/pulumi-provider-boilerplate)
layout and tooling (root Go module, `pulumictl`-based versioning, GoReleaser).
This guide explains how it's distributed and the one-time setup to publish.

## How a Pulumi provider is distributed

Unlike a single npm package, a Pulumi provider ships as **several artifacts**:

| Artifact | Where it lives | How consumers get it | Account needed? |
|---|---|---|---|
| **Plugin binaries** (`pulumi-resource-postmark`) | GitHub Release assets | Auto-installed by `pulumi up` (the schema bakes in the GitHub download URL), or `pulumi plugin install` | ❌ none (GitHub) |
| **Go SDK** | the git repo (`sdk/go/pulumi-postmark`, tagged `sdk/go/pulumi-postmark/vX.Y.Z`) | `go get github.com/hummingbird-me/pulumi-postmark/sdk/go/pulumi-postmark` | ❌ none (git) |
| **Node/TS SDK** | npm (`@kitsu-io/pulumi-postmark`) | `npm install @kitsu-io/pulumi-postmark` | ✅ npm (`@kitsu-io` scope) |
| Python / .NET / Java SDKs | _not currently published_ | build from `sdk/<lang>` if needed | — |

The first two rows require **no external accounts** and make the provider fully
usable on their own. npm is the only registry currently wired up; PyPI, NuGet and
Maven Central are parked (see the end of this doc).

## What's automated

Three workflows under `.github/workflows/`:

- **`ci.yml`** — every push/PR: `golangci-lint`, provider tests, a `schema.json`
  drift check, and an example build. No secrets.
- **`release.yml`** — on a `vX.Y.Z` tag: **GoReleaser** builds the plugin binaries
  for darwin/linux/windows × amd64/arm64 and publishes a **GitHub Release** with the
  tarballs + checksums; a second job pushes the **`sdk/go/pulumi-postmark/vX.Y.Z`**
  Go-module tag. Uses only the built-in `GITHUB_TOKEN`. **Works with zero setup.**
- **`publish-sdks.yml`** — on a `vX.Y.Z` tag: builds the npm SDK and publishes it
  **only if `NPM_TOKEN` is set** (otherwise it logs "skipping" and passes).

## Cutting a release

```bash
# 1. Regenerate the schema + SDK sources at the new version and commit them
#    (the SDK packages carry the version in their manifests). pulumictl normalises
#    the version string.
make codegen PROVIDER_VERSION=0.2.0
git commit -am "chore: regenerate schema and SDKs for v0.2.0"

# 2. Tag and push. The tag triggers release.yml + publish-sdks.yml.
git tag v0.2.0
git push origin main v0.2.0
```

`release.yml` produces the GitHub Release + binaries + Go SDK tag immediately;
`publish-sdks.yml` publishes npm if `NPM_TOKEN` is configured.

> The plugin binary's version comes from the git tag (GoReleaser `{{ .Version }}`);
> the npm package version comes from the regenerated `sdk/nodejs/package.json`. Keep
> the tag and `PROVIDER_VERSION=` in sync.

## One-time setup

### Go — nothing to do ✅
Pushing the `sdk/go/pulumi-postmark/vX.Y.Z` tag (automated) is the whole "publish".
The Go module proxy serves it on first `go get`.

### npm — `@kitsu-io/pulumi-postmark`
1. Make sure the **`kitsu-io`** npm org/scope exists and you can publish to it.
2. Create an **automation** access token (npmjs.com → Access Tokens) with publish
   rights to the scope.
3. Add it as repo secret **`NPM_TOKEN`** (Settings → Secrets and variables → Actions).

The next `vX.Y.Z` tag then publishes `@kitsu-io/pulumi-postmark` (scoped & public).

## Suggested rollout

1. **Tag `v0.1.0` now** → GitHub Release + Go SDK. The provider is immediately usable:
   ```
   pulumi plugin install resource postmark v0.1.0 --server github://api.github.com/hummingbird-me/pulumi-postmark
   ```
   (and Go programs `go get …/sdk/go/pulumi-postmark`). No registry accounts required.
2. Add **`NPM_TOKEN`** whenever you want `npm install` to work.

## Adding PyPI / NuGet / Maven later

The SDK sources for Python, .NET and Java are still generated into `sdk/` by
`make codegen` and build fine — they're just not published. To add one, mirror the
boilerplate's per-language steps (`make python_sdk` / `dotnet_sdk` / `java_sdk` build
them) in a new `publish-sdks.yml` job guarded by its registry secret:

- **PyPI** (`hummingbird_me_postmark`): `python -m build` in `sdk/python` + `twine upload`
  with a `PYPI_TOKEN` secret (or OIDC trusted publishing).
- **NuGet** (`HummingbirdMe.Postmark`): `dotnet pack` + `dotnet nuget push` with a
  `NUGET_API_KEY`. The generated `.csproj` lists `Authors = Pulumi Corp.` — set a
  publisher in `provider/provider.go` (`.WithPublisher(...)`) and regenerate, or edit it.
- **Maven Central** (the hard one): needs a Sonatype Central account, a verified
  namespace (e.g. `io.github.hummingbird-me`), and GPG signing.

### Pulumi Registry listing — optional
The provider works without it (the schema's `pluginDownloadURL` points consumers at
your GitHub Releases). Listing on [registry.pulumi.com](https://www.pulumi.com/registry/)
only adds a docs page; follow Pulumi's publishing docs when you want it.
