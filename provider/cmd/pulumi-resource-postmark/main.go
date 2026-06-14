// Command pulumi-resource-postmark is the Postmark Pulumi provider plugin.
// The Pulumi engine launches this binary (its name must be
// pulumi-resource-<package>) and speaks to it over gRPC.
package main

import (
	"context"
	"fmt"
	"os"

	postmark "github.com/hummingbird-me/pulumi-postmark/provider"
)

func main() {
	if err := postmark.Provider().Run(context.Background(), postmark.Name, postmark.Version); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
