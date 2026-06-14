# pulumi-postmark

A [Pulumi](https://www.pulumi.com) provider for [Postmark](https://postmarkapp.com),
for managing transactional-email infrastructure declaratively: **Servers**,
**Domains**, **Sender Signatures**, and **Templates**.

Built as a native Go provider on
[`pulumi-go-provider`](https://github.com/pulumi/pulumi-go-provider) and wrapping the
[`mrz1836/postmark`](https://github.com/mrz1836/postmark) client.

## Resources

| Resource | What it manages | Token used |
|---|---|---|
| `postmark:index:Server` | A sending server: inbound-email config, tracking, and its own (secret) API token | Account |
| `postmark:index:Domain` | A sending domain; exposes DKIM / Return-Path DNS records as outputs | Account |
| `postmark:index:DomainVerification` | Triggers DKIM / Return-Path verification after your DNS is published | Account |
| `postmark:index:SenderSignature` | A single verified From address | Account |
| `postmark:index:Template` | A transactional template or layout (lives inside a Server) | Server |

## Authentication

Postmark uses two tokens with different scopes:

- **Account API token** (`X-Postmark-Account-Token`) — manages Servers, Domains and
  Sender Signatures. Required.
- **Server API token** (`X-Postmark-Server-Token`) — manages Templates inside one
  server.

Configure the provider:

```bash
pulumi config set --secret postmark:accountToken <ACCOUNT_TOKEN>
# optional default server token for Templates (single-server convenience):
pulumi config set --secret postmark:serverToken <SERVER_TOKEN>
```

Or via environment variables: `POSTMARK_ACCOUNT_TOKEN`, `POSTMARK_SERVER_TOKEN`,
`POSTMARK_BASE_URL`.

### Templates and server tokens

A `Template` needs the API token of *its parent server*. The idiomatic way is to wire
it from the Server resource's secret `apiTokens` output:

```go
srv, _ := postmark.NewServer(ctx, "app", &postmark.ServerArgs{Name: pulumi.String("app")})
postmark.NewTemplate(ctx, "welcome", &postmark.TemplateArgs{
    Name:        pulumi.String("Welcome"),
    Subject:     pulumi.String("Hi"),
    HtmlBody:    pulumi.String("<h1>Hi {{name}}</h1>"),
    ServerToken: srv.ApiTokens.Index(pulumi.Int(0)), // <- wired from the server
})
```

Token resolution precedence for a Template: `serverToken` input → `serverId` lookup
(the provider fetches the token using the account token) → provider `serverToken` config.

## DNS verification flow (Domains)

Verification is deliberately split from provisioning so `pulumi up` never blocks on DNS:

1. `Domain.Create` returns the records to publish as outputs: `dkimPendingHost` /
   `dkimPendingTextValue` (a TXT record) and `returnPathDomainCnameValue` (a CNAME).
2. Wire those outputs into your DNS provider (Route53, Cloudflare, …).
3. Add a `DomainVerification` resource (depending on your DNS records) to call
   Postmark's verify endpoints. Set `pollTimeoutSeconds` to wait for propagation up to
   a bounded timeout; the default (0) makes a single, non-blocking attempt.

DKIM rotation (`RotateDKIM`) is **not** automated — the active/pending/revoked records
are surfaced as outputs for you to manage deliberately.

## Sender Signature confirmation

A `SenderSignature` is created **unconfirmed**: Postmark emails the address a link that
a human must click. The provider never blocks on this — `confirmed` is a read-only
output that flips to `true` (observable after `pulumi refresh`) once the link is
clicked. Change the `triggerResend` input to resend the confirmation email. Prefer a
verified **Domain** where possible, since it needs no human step.

## Importing existing resources

```bash
pulumi import postmark:index:Server          app          12345
pulumi import postmark:index:Domain          example-com  36736
pulumi import postmark:index:SenderSignature ceo          9876
pulumi import postmark:index:Template        welcome      12345/678   # {serverId}/{templateId}
```

Importing a Template requires a usable server token (`postmark:serverToken` config or a
resolvable `serverId`), since reading a template is a server-scoped operation.

## Development

```bash
make provider       # build bin/pulumi-resource-postmark
make install        # build + copy the plugin into $GOPATH/bin (on the local plugin path)
make codegen        # regenerate schema.json + all SDK sources
make build_sdks     # compile all language SDKs (go, nodejs, python, dotnet, java)
make test_provider  # run provider unit tests
make lint           # golangci-lint
```

This repo follows the [pulumi-provider-boilerplate](https://github.com/pulumi/pulumi-provider-boilerplate)
layout and tooling (root Go module, `pulumictl`-based versioning, GoReleaser).
Tool versions are pinned in `.mise.toml` / `.pulumi.version`.

To try the example against a real (sandbox) Postmark account:

```bash
make install
cd examples/simple
pulumi config set --secret postmark:accountToken <ACCOUNT_TOKEN>
pulumi up
```

For local provider debugging without installing, use a binary `path` under
`plugins.providers` in `Pulumi.yaml`, or `PULUMI_DEBUG_PROVIDERS`.

## Releasing

CI runs on every push/PR; pushing a `vX.Y.Z` tag builds the plugin binaries,
publishes a GitHub Release, tags the Go SDK, and publishes the npm SDK
(`@kitsu-io/pulumi-postmark`). See [docs/RELEASING.md](docs/RELEASING.md) for the
release flow and one-time setup.

## License

Apache-2.0
