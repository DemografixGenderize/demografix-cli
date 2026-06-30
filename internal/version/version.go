// Package version holds the CLI version string.
package version

// Version is the CLI version, reported by `demografix version` and sent in the
// User-Agent on every request. It defaults to "dev" and is overridden at build
// time with:
//
//	-ldflags "-X github.com/DemografixGenderize/demografix-cli/internal/version.Version=<tag>"
var Version = "dev"

// UserAgent is the value sent in the User-Agent header on every request, so CLI
// traffic is distinguishable from the SDKs and direct API calls in the logs.
func UserAgent() string {
	return "demografix-cli/" + Version
}
