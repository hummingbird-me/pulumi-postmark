// Command pulumi-resource-postmark is the Postmark Pulumi provider plugin.
// The Pulumi engine launches this binary (its name must be
// pulumi-resource-<package>) and speaks to it over gRPC.
package main

import (
	"context"
	"fmt"
	"os"

	provider "github.com/hummingbird-me/pulumi-postmark/provider"
	"github.com/hummingbird-me/pulumi-postmark/provider/version"
)

func main() {
	prov, err := provider.Provider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to construct postmark provider: %v\n", err)
		os.Exit(1)
	}
	if err := prov.Run(context.Background(), provider.Name, version.Version); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
