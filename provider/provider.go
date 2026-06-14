// Package provider implements the Pulumi Postmark provider: a native Go provider
// (built on github.com/pulumi/pulumi-go-provider's infer framework) that manages
// Postmark email infrastructure — Servers, Domains, Sender Signatures and
// Templates — via the github.com/mrz1836/postmark client.
package provider

import (
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// Version is initialized by the Go linker to contain the semver of this build.
var Version string

// Name controls how this provider is referenced in package names and elsewhere.
const Name string = "postmark"

// Provider creates a new instance of the Postmark provider.
func Provider() p.Provider {
	prov, err := infer.NewProviderBuilder().
		WithNamespace("hummingbird-me").
		WithDisplayName("Postmark").
		WithDescription("A Pulumi provider for managing Postmark email infrastructure: "+
			"servers, domains, sender signatures and templates.").
		WithHomepage("https://github.com/hummingbird-me/pulumi-postmark").
		WithLicense("Apache-2.0").
		WithRepository("https://github.com/hummingbird-me/pulumi-postmark").
		WithKeywords("postmark", "email", "transactional-email", "category/network").
		// Mirror the framework defaults, overriding only the npm package name
		// (published under the kitsu-io scope). WithGoImportPath below mutates the
		// "go" entry of this same map, so it must come after.
		WithLanguageMap(map[string]any{
			"nodejs": map[string]any{
				"respectSchemaVersion": true,
				"packageName":          "@kitsu-io/pulumi-postmark",
			},
			"go": map[string]any{
				"generateResourceContainerTypes": true,
				"respectSchemaVersion":           true,
			},
			"python": map[string]any{
				"respectSchemaVersion": true,
				"pyproject":            map[string]any{"enabled": true},
			},
			"csharp": map[string]any{
				"respectSchemaVersion": true,
			},
		}).
		WithGoImportPath("github.com/hummingbird-me/pulumi-postmark/sdk/go/pulumi-postmark").
		WithPluginDownloadURL("github://api.github.com/hummingbird-me/pulumi-postmark").
		WithConfig(infer.Config(Config{})).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		}).
		WithResources(
			infer.Resource(&Server{}),
			infer.Resource(&Domain{}),
			infer.Resource(&DomainVerification{}),
			infer.Resource(&SenderSignature{}),
			infer.Resource(&Template{}),
		).
		Build()
	if err != nil {
		panic(fmt.Errorf("unable to build provider: %w", err))
	}
	return prov
}
