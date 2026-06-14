// Package provider implements the Pulumi Postmark provider: a native Go provider
// (built on github.com/pulumi/pulumi-go-provider's infer framework) that manages
// Postmark email infrastructure — Servers, Domains, Sender Signatures and
// Templates — via the github.com/mrz1836/postmark client.
package provider

import (
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Name is the Pulumi package name and the resource-token prefix
// (e.g. postmark:index:Server). It must match the plugin binary name
// pulumi-resource-postmark.
const Name = "postmark"

// Provider builds the inferred Postmark provider.
func Provider() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithNamespace("hummingbird-me").
		WithDisplayName("Postmark").
		WithDescription("A Pulumi provider for managing Postmark email infrastructure: "+
			"servers, domains, sender signatures and templates.").
		WithHomepage("https://github.com/hummingbird-me/pulumi-postmark").
		WithLicense("Apache-2.0").
		WithRepository("https://github.com/hummingbird-me/pulumi-postmark").
		WithKeywords("postmark", "email", "transactional-email", "category/network").
		WithGoImportPath("github.com/hummingbird-me/pulumi-postmark/sdk/go/postmark").
		WithPluginDownloadURL("github://api.github.com/hummingbird-me/pulumi-postmark").
		WithConfig(infer.Config(Config{})).
		WithResources(
			infer.Resource(&Server{}),
			infer.Resource(&Domain{}),
			infer.Resource(&DomainVerification{}),
			infer.Resource(&SenderSignature{}),
			infer.Resource(&Template{}),
		).
		Build()
}
