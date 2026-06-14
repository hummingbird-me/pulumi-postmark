// Example Pulumi program for the Postmark provider.
//
// It provisions a sending server (with inbound email wired to your app and its
// API token exported as a secret), a template on that server, and a sending
// domain whose DKIM/Return-Path DNS records are exported for you to publish and
// then verify.
package main

import (
	postmark "github.com/hummingbird-me/pulumi-postmark/sdk/go/pulumi-postmark"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// 1. A sending server. Inbound email is delivered to your app via
		//    inboundHookUrl; set up an MX record for inboundDomain pointing at
		//    inbound.postmarkapp.com to receive mail.
		srv, err := postmark.NewServer(ctx, "app", &postmark.ServerArgs{
			Name:           pulumi.String("app-production"),
			Color:          pulumi.String("Green"),
			DeliveryType:   pulumi.String("Live"),
			TrackOpens:     pulumi.Bool(true),
			InboundHookUrl: pulumi.String("https://api.example.com/postmark/inbound"),
			InboundDomain:  pulumi.String("inbound.example.com"),
		})
		if err != nil {
			return err
		}

		// The server's own API token (secret) — feed this into your app's
		// deployment (e.g. as an env var / Kubernetes secret).
		serverToken := srv.ApiTokens.Index(pulumi.Int(0))
		ctx.Export("serverApiToken", serverToken)
		ctx.Export("inboundAddress", srv.InboundAddress)

		// 2. A transactional template on that server. The server token is wired
		//    directly from the server resource, giving a correct dependency graph.
		_, err = postmark.NewTemplate(ctx, "welcome", &postmark.TemplateArgs{
			Name:        pulumi.String("Welcome"),
			Alias:       pulumi.String("welcome"),
			Subject:     pulumi.String("Welcome to Example!"),
			HtmlBody:    pulumi.String("<h1>Hello {{name}}</h1>"),
			TextBody:    pulumi.String("Hello {{name}}"),
			ServerToken: serverToken,
		})
		if err != nil {
			return err
		}

		// 3. A sending domain. Create returns the DNS records to publish.
		dom, err := postmark.NewDomain(ctx, "example-com", &postmark.DomainArgs{
			Name:             pulumi.String("example.com"),
			ReturnPathDomain: pulumi.String("pm-bounces.example.com"),
		})
		if err != nil {
			return err
		}
		ctx.Export("dkimRecordHost", dom.DkimPendingHost)
		ctx.Export("dkimRecordValue", dom.DkimPendingTextValue)
		ctx.Export("returnPathCnameValue", dom.ReturnPathDomainCnameValue)

		// ... wire the records above into your DNS provider (Route53, Cloudflare,
		//     etc.) as a TXT (DKIM) and CNAME (Return-Path) record here ...

		// 4. Trigger verification once DNS is published. Poll up to 5 minutes for
		//    propagation. Depend on your DNS resources so this runs after them.
		_, err = postmark.NewDomainVerification(ctx, "example-com-verify", &postmark.DomainVerificationArgs{
			DomainId:           dom.DomainId,
			PollTimeoutSeconds: pulumi.Int(300),
		})
		return err
	})
}
