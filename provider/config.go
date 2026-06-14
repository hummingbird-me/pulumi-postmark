package provider

import "github.com/pulumi/pulumi-go-provider/infer"

// Config is the provider-level configuration for the Postmark provider.
//
// Postmark uses two different API tokens with different scopes:
//
//   - The Account API token (X-Postmark-Account-Token) manages account-level
//     resources: Servers, Domains and Sender Signatures.
//   - A Server API token (X-Postmark-Server-Token) manages resources that live
//     inside a single server: Templates.
//
// AccountToken is therefore required for almost everything. ServerToken is an
// optional default used for Template operations when a Template resource does
// not supply its own token (see Template.ServerToken / Template.ServerID).
type Config struct {
	AccountToken string `pulumi:"accountToken,optional" provider:"secret"`
	ServerToken  string `pulumi:"serverToken,optional" provider:"secret"`
	BaseURL      string `pulumi:"baseUrl,optional"`
}

// Annotate attaches descriptions, defaults and environment-variable fallbacks
// to the provider configuration.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.AccountToken, "Postmark Account API token (X-Postmark-Account-Token). "+
		"Required to manage Servers, Domains and Sender Signatures. "+
		"May also be set via the POSTMARK_ACCOUNT_TOKEN environment variable.")
	a.SetDefault(&c.AccountToken, "", "POSTMARK_ACCOUNT_TOKEN")

	a.Describe(&c.ServerToken, "Default Postmark Server API token (X-Postmark-Server-Token) used for "+
		"Template operations when a Template does not set its own `serverToken`/`serverId`. "+
		"Only useful when all Templates live on a single server. "+
		"May also be set via the POSTMARK_SERVER_TOKEN environment variable.")
	a.SetDefault(&c.ServerToken, "", "POSTMARK_SERVER_TOKEN")

	a.Describe(&c.BaseURL, "Base URL of the Postmark API. Defaults to https://api.postmarkapp.com. "+
		"May also be set via the POSTMARK_BASE_URL environment variable (useful for testing).")
	a.SetDefault(&c.BaseURL, defaultBaseURL, "POSTMARK_BASE_URL")
}
