// resolve.go implements the FR-U5 target-resolution + asset-selection step of the direct-binary
// self-swap (§9.29 FR-U5 steps 2-3). It composes the GitHub Releases Client (releases.go) per the
// channel/version flags and delegates platform-asset matching to SelectAsset (download.go). It is a
// THIN composition: no new network or match logic, no up-to-date determination (that is the command
// layer's job, P1.M4.T2), and it is walled off (FR-U12: stdlib-only, no internal/* imports). File
// comment only — releases.go owns the package doc.
package upgrade

import (
	"context"
	"runtime"
	"strings"
)

// ResolveOptions selects the target channel (Version/Prerelease) and the platform (GOOS/GOARCH) for
// ResolveTarget. Every field is an opt-in override: the zero value resolves LatestStable for the
// runtime GOOS/GOARCH.
type ResolveOptions struct {
	// Version, when non-empty after TrimSpace (--version <v>), resolves the exact tag via
	// ReleaseByTag. It takes PRECEDENCE over Prerelease — a pinned version is an explicit user
	// instruction that overrides the channel, even when Prerelease is also true.
	Version string

	// Prerelease selects the prerelease channel (--prerelease) via LatestAdmittingPrereleases. It is
	// consulted only when Version is empty (after TrimSpace); a non-empty Version always wins.
	Prerelease bool

	// GOOS selects the target OS. Empty ("") defaults to runtime.GOOS at resolve time. It is
	// injectable so per-platform asset selection is testable off the test host (e.g. asserting the
	// darwin asset from a linux CI runner).
	GOOS string

	// GOARCH selects the target architecture. Empty ("") defaults to runtime.GOARCH at resolve time.
	// Injectable for the same off-host per-platform testing reason as GOOS.
	GOARCH string
}

// ResolveTarget resolves the target release for the channel/version flags and selects the platform
// archive asset for the requested (or runtime) GOOS/GOARCH — steps 2-3 of FR-U5 (the direct-binary
// self-swap's RESOLVE step). It is a THIN composition:
//
//   - channel/version dispatch (Version > Prerelease > LatestStable) is delegated to the Client
//     methods in releases.go (ReleaseByTag / LatestAdmittingPrereleases / LatestStable); and
//   - platform-asset matching is delegated to SelectAsset in download.go.
//
// GOOS/GOARCH default to runtime.GOOS/GOARCH when empty and may be overridden via ResolveOptions so
// per-platform selection is testable off the test host. ResolveTarget does NOT determine whether the
// target is newer than the running version (no Compare / CurrentSemver): that up-to-date / --check
// determination is the command layer's job (P1.M4.T2 runUpgrade), which calls ResolveTarget and then
// compares the returned tag itself. Errors from the Client (ErrNoReleases / ErrRateLimited / ErrHTTP)
// and from SelectAsset (ErrNoMatchingAsset) are propagated UNWRAPPED so errors.Is reaches the
// sentinels in the command layer; on any error the non-error return values are zero-valued.
//
// c must be non-nil (the package does not nil-guard the Client, matching releases.go's convention).
func ResolveTarget(ctx context.Context, c *Client, opts ResolveOptions) (Release, Asset, error) {
	release, err := resolveRelease(ctx, c, opts)
	if err != nil {
		return Release{}, Asset{}, err
	}
	goos, goarch := opts.GOOS, opts.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	asset, err := SelectAsset(release, goos, goarch)
	if err != nil {
		return Release{}, Asset{}, err
	}
	return release, asset, nil
}

// resolveRelease is the 3-way channel/version dispatcher. It picks the Client method for the flags:
// a non-empty (after TrimSpace) Version ⇒ ReleaseByTag (precedence, even when Prerelease is also
// true); else Prerelease ⇒ LatestAdmittingPrereleases; else the LatestStable default. The trimmed
// version is passed to ReleaseByTag so a whitespace-only Version correctly falls through to the
// channel path rather than being sent to ReleaseByTag (which rejects empty tags with ErrHTTP).
func resolveRelease(ctx context.Context, c *Client, opts ResolveOptions) (Release, error) {
	if v := strings.TrimSpace(opts.Version); v != "" {
		return c.ReleaseByTag(ctx, v) // pinned --version wins, even with --prerelease
	}
	if opts.Prerelease {
		return c.LatestAdmittingPrereleases(ctx)
	}
	return c.LatestStable(ctx) // default channel
}
