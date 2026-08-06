package upgrade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// preReleaseWithLinuxAsset is a custom /releases array whose winning non-draft release is a
// prerelease (v2.0.0-rc1) that ALSO carries a real goreleaser-named linux_amd64 asset. cannedReleases's
// pre asset ("pre.tar.gz") does NOT match the goreleaser name, so this fixture exists to prove the
// prerelease channel succeeds on BOTH the endpoint (the array, not /latest) AND the asset selection.
func preReleaseWithLinuxAsset() string {
	return `[
		{"tag_name": "v1.9.0", "prerelease": false, "draft": false,
		 "assets": [{"name": "stagecoach_1.9.0_linux_amd64.tar.gz", "browser_download_url": "u1", "size": 1}]},
		{"tag_name": "v2.0.0-rc1", "prerelease": true, "draft": false,
		 "assets": [
			{"name": "stagecoach_2.0.0-rc1_linux_amd64.tar.gz", "browser_download_url": "u2", "size": 2},
			{"name": "stagecoach_2.0.0-rc1_darwin_arm64.tar.gz", "browser_download_url": "u3", "size": 3}
		 ]}
	]`
}

// mustHasSuffix fails the test if path does not end with suffix. Used to assert which endpoint the
// Client hit, proving resolveRelease dispatched to the correct method.
func mustHasSuffix(t *testing.T, path, suffix string) {
	t.Helper()
	if !strings.HasSuffix(path, suffix) {
		t.Errorf("hit path %q, want suffix %q", path, suffix)
	}
}

// TestResolveTarget_LatestStable proves the default channel: zero Version + Prerelease==false ⇒
// c.LatestStable (GET /releases/latest), and the returned release + linux_amd64 asset are correct.
func TestResolveTarget_LatestStable(t *testing.T) {
	var hit string
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		hit = r.URL.Path
		mustHasSuffix(t, hit, "/releases/latest")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(cannedLatest()))
	})
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatalf("ResolveTarget: unexpected error: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3", rel.Tag)
	}
	if asset.Name != "stagecoach_1.2.3_linux_amd64.tar.gz" {
		t.Errorf("asset.Name = %q, want stagecoach_1.2.3_linux_amd64.tar.gz", asset.Name)
	}
	if asset.DownloadURL != "https://example.com/linux.tar.gz" {
		t.Errorf("asset.DownloadURL = %q, want https://example.com/linux.tar.gz", asset.DownloadURL)
	}
}

// TestResolveTarget_Prerelease proves the prerelease channel: Prerelease==true + empty Version ⇒
// c.LatestAdmittingPrereleases (GET /releases, the array endpoint — NOT /latest), the prerelease
// wins after draft exclusion, and a matching linux_amd64 asset is selected.
func TestResolveTarget_Prerelease(t *testing.T) {
	var hit string
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		hit = r.URL.Path
		mustHasSuffix(t, hit, "/releases")
		if strings.HasSuffix(hit, "/releases/latest") {
			t.Errorf("prerelease channel must NOT hit /releases/latest; hit %q", hit)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(preReleaseWithLinuxAsset()))
	})
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{Prerelease: true, GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatalf("ResolveTarget: unexpected error: %v", err)
	}
	if rel.Tag != "v2.0.0-rc1" {
		t.Errorf("Tag = %q, want v2.0.0-rc1 (prerelease outranks stable)", rel.Tag)
	}
	if asset.Name != "stagecoach_2.0.0-rc1_linux_amd64.tar.gz" {
		t.Errorf("asset.Name = %q, want stagecoach_2.0.0-rc1_linux_amd64.tar.gz", asset.Name)
	}
}

// TestResolveTarget_Version proves the pinned-version path: Version != "" ⇒ c.ReleaseByTag (GET
// /releases/tags/{tag}), and the returned release + asset are correct.
func TestResolveTarget_Version(t *testing.T) {
	var hit string
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		hit = r.URL.Path
		mustHasSuffix(t, hit, "/releases/tags/v1.2.3")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(cannedLatest()))
	})
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{Version: "v1.2.3", GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatalf("ResolveTarget: unexpected error: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3", rel.Tag)
	}
	if asset.Name != "stagecoach_1.2.3_linux_amd64.tar.gz" {
		t.Errorf("asset.Name = %q, want stagecoach_1.2.3_linux_amd64.tar.gz", asset.Name)
	}
}

// TestResolveTarget_VersionPrecedenceOverPrerelease proves Version wins over Prerelease: when BOTH
// are set, c.ReleaseByTag (/releases/tags/{tag}) is hit and the array endpoint is NEVER consulted.
// The handler FAILS the test if the array endpoint is requested.
func TestResolveTarget_VersionPrecedenceOverPrerelease(t *testing.T) {
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases") && !strings.Contains(r.URL.Path, "/tags/") {
			t.Errorf("Version must win over Prerelease; array endpoint hit: %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		mustHasSuffix(t, r.URL.Path, "/releases/tags/v1.2.3")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(cannedLatest()))
	})
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{Version: "v1.2.3", Prerelease: true, GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatalf("ResolveTarget: unexpected error: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3 (Version wins over Prerelease)", rel.Tag)
	}
	if asset.Name != "stagecoach_1.2.3_linux_amd64.tar.gz" {
		t.Errorf("asset.Name = %q, want stagecoach_1.2.3_linux_amd64.tar.gz", asset.Name)
	}
}

// TestResolveTarget_VersionWhitespaceFallsThrough proves a whitespace-only Version is treated as
// empty: it falls through to the channel path (LatestStable), NOT ReleaseByTag. This guards the
// "TrimSpace decides the branch" contract — checking raw Version != "" would wrongly send " " to
// ReleaseByTag.
func TestResolveTarget_VersionWhitespaceFallsThrough(t *testing.T) {
	var hit string
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		hit = r.URL.Path
		mustHasSuffix(t, hit, "/releases/latest")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(cannedLatest()))
	})
	rel, _, err := ResolveTarget(context.Background(), c, ResolveOptions{Version: "  ", GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatalf("ResolveTarget: unexpected error: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3 (whitespace Version falls through to LatestStable)", rel.Tag)
	}
}

// TestResolveTarget_AssetSelectionPerPlatform proves GOOS/GOARCH are honored off the test host:
// linux/darwin succeed against cannedLatest (which has both assets); windows has NO asset in
// cannedLatest ⇒ ErrNoMatchingAsset. cannedLatest is reused as the ErrNoMatchingAsset fixture.
func TestResolveTarget_AssetSelectionPerPlatform(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusOK, cannedLatest()))
	ctx := context.Background()

	t.Run("linux_amd64", func(t *testing.T) {
		_, asset, err := ResolveTarget(ctx, c, ResolveOptions{GOOS: "linux", GOARCH: "amd64"})
		if err != nil {
			t.Fatalf("ResolveTarget linux/amd64: unexpected error: %v", err)
		}
		if !strings.Contains(asset.Name, "linux_amd64") {
			t.Errorf("asset.Name = %q, want substring linux_amd64", asset.Name)
		}
	})

	t.Run("darwin_arm64", func(t *testing.T) {
		_, asset, err := ResolveTarget(ctx, c, ResolveOptions{GOOS: "darwin", GOARCH: "arm64"})
		if err != nil {
			t.Fatalf("ResolveTarget darwin/arm64: unexpected error: %v", err)
		}
		if !strings.Contains(asset.Name, "darwin_arm64") {
			t.Errorf("asset.Name = %q, want substring darwin_arm64", asset.Name)
		}
	})

	t.Run("windows_amd64_no_matching_asset", func(t *testing.T) {
		rel, asset, err := ResolveTarget(ctx, c, ResolveOptions{GOOS: "windows", GOARCH: "amd64"})
		if err == nil {
			t.Fatalf("ResolveTarget windows/amd64: expected ErrNoMatchingAsset, got nil (asset=%+v rel=%+v)", asset, rel)
		}
		if !errors.Is(err, ErrNoMatchingAsset) {
			t.Errorf("err = %v, want errors.Is ErrNoMatchingAsset", err)
		}
		// Zero-on-error contract: the windows case doubles as the zero-return assertion.
		// Release contains a slice, so it cannot be compared with !=; use reflect.DeepEqual.
		if !reflect.DeepEqual(rel, Release{}) {
			t.Errorf("rel = %+v, want zero Release on error", rel)
		}
		if (asset != Asset{}) {
			t.Errorf("asset = %+v, want zero Asset on error", asset)
		}
	})
}

// TestResolveTarget_EmptyGOOSDefaultsRuntime proves empty GOOS/GOARCH default to runtime.GOOS/GOARCH.
// A custom payload whose only asset is the goreleaser name for the RUNTIME platform is built via the
// in-package assetName helper; empty opts must select exactly that asset.
func TestResolveTarget_EmptyGOOSDefaultsRuntime(t *testing.T) {
	wantName := assetName("v1.2.3", runtime.GOOS, runtime.GOARCH)
	body := fmt.Sprintf(`{
		"tag_name": "v1.2.3",
		"prerelease": false,
		"draft": false,
		"assets": [
			{"name": %q, "browser_download_url": "https://example.com/runtime-archive", "size": 999}
		]
	}`, wantName)
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		mustHasSuffix(t, r.URL.Path, "/releases/latest")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{}) // GOOS/GOARCH zero ⇒ runtime
	if err != nil {
		t.Fatalf("ResolveTarget with empty GOOS/GOARCH: unexpected error: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3", rel.Tag)
	}
	if asset.Name != wantName {
		t.Errorf("asset.Name = %q, want runtime-default %q (GOOS=%s GOARCH=%s)", asset.Name, wantName, runtime.GOOS, runtime.GOARCH)
	}
}

// TestResolveTarget_ClientErrorPropagates proves Client failures propagate UNWRAPPED so errors.Is
// reaches the sentinels in the command layer. 404 ⇒ ErrNoReleases; 403 ⇒ ErrRateLimited.
func TestResolveTarget_ClientErrorPropagates(t *testing.T) {
	ctx := context.Background()

	t.Run("404_NoReleases", func(t *testing.T) {
		c := newFakeClient(t, statusServer(http.StatusNotFound, `{"message":"Not Found"}`))
		rel, asset, err := ResolveTarget(ctx, c, ResolveOptions{GOOS: "linux", GOARCH: "amd64"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNoReleases) {
			t.Errorf("err = %v, want errors.Is ErrNoReleases", err)
		}
		if !reflect.DeepEqual(rel, Release{}) || (asset != Asset{}) {
			t.Errorf("non-error returns must be zero on error: rel=%+v asset=%+v", rel, asset)
		}
	})

	t.Run("403_RateLimited", func(t *testing.T) {
		c := newFakeClient(t, statusServer(http.StatusForbidden, `{"message":"rate limited"}`))
		rel, asset, err := ResolveTarget(ctx, c, ResolveOptions{GOOS: "linux", GOARCH: "amd64"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrRateLimited) {
			t.Errorf("err = %v, want errors.Is ErrRateLimited", err)
		}
		if !reflect.DeepEqual(rel, Release{}) || (asset != Asset{}) {
			t.Errorf("non-error returns must be zero on error: rel=%+v asset=%+v", rel, asset)
		}
	})
}

// TestResolveTarget_ClientErrorPropagatesPrereleaseChannel proves the error-propagation contract
// also holds on the prerelease branch (the /releases array endpoint), not just LatestStable.
func TestResolveTarget_ClientErrorPropagatesPrereleaseChannel(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusNotFound, `{"message":"Not Found"}`))
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{Prerelease: true, GOOS: "linux", GOARCH: "amd64"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("err = %v, want errors.Is ErrNoReleases", err)
	}
	if !reflect.DeepEqual(rel, Release{}) || (asset != Asset{}) {
		t.Errorf("non-error returns must be zero on error: rel=%+v asset=%+v", rel, asset)
	}
}

// TestResolveTarget_VersionClientErrorPropagates proves the error-propagation contract also holds
// on the ReleaseByTag branch (the /releases/tags/{tag} endpoint).
func TestResolveTarget_VersionClientErrorPropagates(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusNotFound, `{"message":"Not Found"}`))
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{Version: "v9.9.9", GOOS: "linux", GOARCH: "amd64"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("err = %v, want errors.Is ErrNoReleases", err)
	}
	if !reflect.DeepEqual(rel, Release{}) || (asset != Asset{}) {
		t.Errorf("non-error returns must be zero on error: rel=%+v asset=%+v", rel, asset)
	}
}
