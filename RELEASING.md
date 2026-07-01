# Releasing

Releases are git-tag driven, in lockstep with the SDKs. The version lives in the
tag only; it is injected into the binary at build time via ldflags (no version
field in `go.mod`).

## One-time setup

1. Confirm the module/repo path `github.com/DemografixGenderize/demografix-cli` and make the repo
   public (required for the Go module proxy and `go install`).
2. Create the Homebrew tap repo `github.com/DemografixGenderize/homebrew-tap`.
3. Add a repository secret `HOMEBREW_TAP_GITHUB_TOKEN` — a PAT with write access
   to the tap repo (the default `GITHUB_TOKEN` cannot push to another repo).

## Cutting a release

1. Ensure `main` is green.
2. Validate the config locally:

   ```sh
   goreleaser check
   goreleaser release --snapshot --clean   # builds into ./dist, publishes nothing
   ```

3. Tag and push:

   ```sh
   git tag v0.1.0
   git push origin v0.1.0
   ```

The `Release` workflow cross-compiles every target, publishes a GitHub Release
with checksums, and updates the Homebrew cask in the tap.

## Install channels

- One-line: `curl -fsSL https://raw.githubusercontent.com/DemografixGenderize/demografix-cli/main/install.sh | sh`
- Homebrew: `brew install demografixgenderize/tap/demografix`
- Direct: download from the GitHub Release for your OS/arch.

winget and Scoop manifests are deferred; add a `scoops:` block and winget
manifests when needed — nothing in this setup blocks them.
