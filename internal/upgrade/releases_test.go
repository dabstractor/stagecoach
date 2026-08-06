package upgrade

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// newFakeClient spins up an httptest.Server running handler and returns a Client pointed at it
// (Repo is a canned "owner/repo"; BaseURL is the fake server). The server is closed via t.Cleanup.
func newFakeClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &Client{BaseURL: ts.URL, Repo: "owner/repo"}
}

// cannedLatest returns a GitHub-shaped /releases/latest payload.
func cannedLatest() string {
	return `{
		"tag_name": "v1.2.3",
		"name": "Release 1.2.3",
		"prerelease": false,
		"draft": false,
		"assets": [
			{"name": "stagecoach_1.2.3_linux_amd64.tar.gz",
			 "browser_download_url": "https://example.com/linux.tar.gz",
			 "size": 12345},
			{"name": "stagecoach_1.2.3_darwin_arm64.tar.gz",
			 "browser_download_url": "https://example.com/darwin.tar.gz",
			 "size": 6789}
		]
	}`
}

// cannedReleases returns a GitHub-shaped /releases array (newest first, per the API default).
func cannedReleases() string {
	return `[
		{"tag_name": "v3.0.0", "prerelease": false, "draft": true,
		 "assets": []},
		{"tag_name": "v1.9.0", "prerelease": false, "draft": false,
		 "assets": [{"name": "stable.tar.gz", "browser_download_url": "u", "size": 1}]},
		{"tag_name": "v2.0.0-rc1", "prerelease": true, "draft": false,
		 "assets": [{"name": "pre.tar.gz", "browser_download_url": "u2", "size": 2}]}
	]`
}

// statusServer returns a handler that always replies with the given status + body.
func statusServer(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestClient_LatestStable_OK(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusOK, cannedLatest()))
	rel, err := c.LatestStable(context.Background())
	if err != nil {
		t.Fatalf("LatestStable: unexpected error: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3", rel.Tag)
	}
	if len(rel.Assets) != 2 {
		t.Fatalf("Assets len = %d, want 2", len(rel.Assets))
	}
	want := Asset{Name: "stagecoach_1.2.3_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux.tar.gz", Size: 12345}
	if rel.Assets[0] != want {
		t.Errorf("Assets[0] = %+v, want %+v", rel.Assets[0], want)
	}
	if rel.Assets[1].Size != 6789 {
		t.Errorf("Assets[1].Size = %d, want 6789", rel.Assets[1].Size)
	}
}

func TestClient_LatestStable_404_NoReleases(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusNotFound, `{"message":"Not Found"}`))
	_, err := c.LatestStable(context.Background())
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("err = %v, want errors.Is ErrNoReleases", err)
	}
}

func TestClient_LatestStable_403_RateLimited(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusForbidden, `{"message":"rate limited"}`))
	_, err := c.LatestStable(context.Background())
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want errors.Is ErrRateLimited", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "token") {
		t.Errorf("err message %q should mention a token hint", err.Error())
	}
}

func TestClient_LatestStable_429_RateLimited(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusTooManyRequests, `{"message":"too many requests"}`))
	_, err := c.LatestStable(context.Background())
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want errors.Is ErrRateLimited", err)
	}
}

func TestClient_LatestStable_500_HTTP(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusInternalServerError, `server boom`))
	_, err := c.LatestStable(context.Background())
	if !errors.Is(err, ErrHTTP) {
		t.Errorf("err = %v, want errors.Is ErrHTTP", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("err message %q should mention status 500", err.Error())
	}
}

func TestClient_Prerelease_PicksHighest(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusOK, cannedReleases()))
	rel, err := c.LatestAdmittingPrereleases(context.Background())
	if err != nil {
		t.Fatalf("LatestAdmittingPrereleases: unexpected error: %v", err)
	}
	// Draft v3.0.0 excluded; v2.0.0-rc1 (prerelease) outranks stable v1.9.0 by core precedence.
	if rel.Tag != "v2.0.0-rc1" {
		t.Errorf("Tag = %q, want v2.0.0-rc1", rel.Tag)
	}
}

func TestClient_Prerelease_Empty_NoReleases(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusOK, `[]`))
	_, err := c.LatestAdmittingPrereleases(context.Background())
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("err = %v, want errors.Is ErrNoReleases", err)
	}
}

func TestClient_Prerelease_AllDrafts_NoReleases(t *testing.T) {
	body := `[
		{"tag_name": "v9.9.9", "prerelease": false, "draft": true, "assets": []},
		{"tag_name": "v8.8.8", "prerelease": true, "draft": true, "assets": []}
	]`
	c := newFakeClient(t, statusServer(http.StatusOK, body))
	_, err := c.LatestAdmittingPrereleases(context.Background())
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("err = %v, want errors.Is ErrNoReleases", err)
	}
}

func TestClient_ReleaseByTag_OK(t *testing.T) {
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/tags/v1.2.3") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(cannedLatest()))
	})
	rel, err := c.ReleaseByTag(context.Background(), "v1.2.3")
	if err != nil {
		t.Fatalf("ReleaseByTag: unexpected error: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3", rel.Tag)
	}
}

func TestClient_ReleaseByTag_404_NoReleases(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusNotFound, `{"message":"Not Found"}`))
	_, err := c.ReleaseByTag(context.Background(), "v9.9.9")
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("err = %v, want errors.Is ErrNoReleases", err)
	}
}

func TestClient_ReleaseByTag_EmptyTag(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusOK, `[]`))
	_, err := c.ReleaseByTag(context.Background(), "")
	if !errors.Is(err, ErrHTTP) {
		t.Errorf("err = %v, want errors.Is ErrHTTP (empty tag rejected)", err)
	}
}

// recordingHandler records the Authorization and User-Agent headers per request (thread-safe) so the
// AuthHeader / UserAgent tests can assert against what was actually sent.
func recordingHandler(t *testing.T, status int, body string, gotUA, gotAuth *string, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		*gotUA = r.Header.Get("User-Agent")
		*gotAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestClient_AuthHeader(t *testing.T) {
	// With a token: Authorization: Bearer <token> present.
	{
		var gotUA, gotAuth string
		var mu sync.Mutex
		c := newFakeClient(t, recordingHandler(t, http.StatusOK, cannedLatest(), &gotUA, &gotAuth, &mu))
		c.Token = "xyz"
		if _, err := c.LatestStable(context.Background()); err != nil {
			t.Fatalf("LatestStable: %v", err)
		}
		mu.Lock()
		auth := gotAuth
		mu.Unlock()
		if auth != "Bearer xyz" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer xyz")
		}
	}
	// Without a token: header absent.
	{
		var gotUA, gotAuth string
		var mu sync.Mutex
		c := newFakeClient(t, recordingHandler(t, http.StatusOK, cannedLatest(), &gotUA, &gotAuth, &mu))
		c.Token = ""
		if _, err := c.LatestStable(context.Background()); err != nil {
			t.Fatalf("LatestStable: %v", err)
		}
		mu.Lock()
		auth := gotAuth
		mu.Unlock()
		if auth != "" {
			t.Errorf("Authorization = %q, want empty (no token set)", auth)
		}
	}
}

func TestClient_UserAgent(t *testing.T) {
	var gotUA, gotAuth string
	var mu sync.Mutex
	c := newFakeClient(t, recordingHandler(t, http.StatusOK, cannedLatest(), &gotUA, &gotAuth, &mu))
	if _, err := c.LatestStable(context.Background()); err != nil {
		t.Fatalf("LatestStable: %v", err)
	}
	mu.Lock()
	ua := gotUA
	mu.Unlock()
	if !strings.HasPrefix(ua, "stagecoach/") {
		t.Errorf("User-Agent = %q, want prefix %q", ua, "stagecoach/")
	}
	if strings.HasPrefix(ua, "Go-http-client") {
		t.Errorf("User-Agent = %q, must NOT be the Go default", ua)
	}
}

func TestClient_TransportFailure_HTTP(t *testing.T) {
	// Close the server first, then call — the request fails at the transport layer.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := &Client{BaseURL: ts.URL, Repo: "owner/repo"}
	ts.Close()

	_, err := c.LatestStable(context.Background())
	if !errors.Is(err, ErrHTTP) {
		t.Errorf("err = %v, want errors.Is ErrHTTP (transport failure)", err)
	}
}

func TestClient_ContextCanceled(t *testing.T) {
	c := newFakeClient(t, statusServer(http.StatusOK, cannedLatest()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call so the request is born dead
	_, err := c.LatestStable(ctx)
	if err == nil {
		t.Fatal("LatestStable with canceled ctx: expected error, got nil")
	}
	// A canceled context surfaces as a transport-layer failure → ErrHTTP (the three-sentinel contract).
	if !errors.Is(err, ErrHTTP) {
		t.Errorf("err = %v, want errors.Is ErrHTTP", err)
	}
}
