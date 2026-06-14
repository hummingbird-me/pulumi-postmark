// Package version holds the provider version, injected at build time via ldflags:
//
//	go build -ldflags "-X github.com/hummingbird-me/pulumi-postmark/provider/version.Version=1.2.3"
package version

// Version is the semver of this provider build. The default is only used for
// local `go run` / `go test`; release builds overwrite it with the git tag.
var Version = "0.0.1-dev"
