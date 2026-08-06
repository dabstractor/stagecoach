# Research notes — P1.M1.T3.S2 (asset + checksums.txt download + SHA256 verify)

## 1. What the sibling (P1.M1.T3.S1) delivers (treat as CONTRACT)

`internal/upgrade/releases.go` (NEW in sibling, `package upgrade`) exports:

```go
var (                                   // typed errors, errors.Is-reachable, wrapped with %w
    ErrNoReleases  = errors.New("upgrade: no releases found")
    ErrRateLimited = errors.New("upgrade: GitHub rate limited")
    ErrHTTP        = errors.New("upgrade: HTTP request failed")   // REUSED by this subtask for download transport/status failures
)
type Asset struct { Name string; DownloadURL string; Size int64 }  // DownloadURL = browser_download_url (absolute)
type Release struct { Tag string; Assets []Asset }                // Tag has leading "v" (e.g. v1.0.0)
type Client struct { HTTP *http.Client; BaseURL string; Repo string; Token string } // nil HTTP → DefaultClient; BaseURL for metadata only
```

This subtask is **additive** to the SAME package. It MUST NOT edit `releases.go` (parallel item).
Reference `Client`/`Release`/`Asset`/`ErrHTTP` by bare name (same package). `version.go` (P1.M1.T1.S2)
owns the package doc + `Compare`; P1.M1.T3.S1 enriches that doc — do not touch it again. `download.go`
gets a file-level comment only (blank-line-separated from `package upgrade`) — NO competing package doc.

## 2. goreleaser artifact naming (the linchpin) — confirmed THREE ways

**Source A — `.goreleaser.yaml` (the actual release config in this repo):**
```yaml
archives:
  - name_template: '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}'   # stagecoach_1.0.0_linux_amd64.tar.gz
    format_overrides:
      - goos: windows
        formats: [zip]      # windows → .zip; everything else → .tar.gz
checksum:
  name_template: '{{ .ProjectName }}_{{ .Version }}_checksums.txt'             # stagecoach_1.0.0_checksums.txt
  algorithm: sha256
ldflags: - -X main.version={{.Version}}   # comment confirms: "{{.Version}} = tag without leading 'v' (e.g. 1.0.0)"
```

**Source B — `plan/017_397abce9deb1/architecture/external_deps.md` §1 + §2 (in-repo authoritative):**
> "Asset download: ... via the browser_download_url (a objects.githubusercontent.com redirect). Go's
> net/http follows redirects by default; stream to a temp file (archives can be multi-MB)."
> "checksums.txt is one line per artifact: `<64-hex-sha256>  <artifact-filename>` (two spaces)."

**Source C — https://goreleaser.com/customization/package/checksum/** (external corroboration):
> "GoReleaser generates a project_1.0.0_checksums.txt file and uploads it with the release."
(doublecmd issue 349: "it creates checksum with two spaces.")

### CRITICAL naming derivation
- `{{ .Version }}` = the git **tag with the leading `v` stripped** (tag `v1.0.0` → Version `1.0.0`).
- **Release.Tag from the GitHub API retains the `v`** (`v1.0.0`). So to build the expected filename we
  must `ver := strings.TrimPrefix(release.Tag, "v")`. Forgetting this → no asset match → false failure.
- For our 6 goreleaser targets (linux/darwin/windows × amd64/arm64) the goreleaser `.Os`/`.Arch` EQUAL
  Go's `runtime.GOOS`/`runtime.GOARCH` (darwin, windows, linux, amd64, arm64) — **identity mapping**, no
  translation table needed. (Verify at impl per FR-D5; this identity holds for these targets.)
- Extension: `.tar.gz` for all, **`.zip` for `goos=="windows"`**.

## 3. Checksums.txt parse contract

- Lines: `<64-lowercase-hex>  <filename>` (goreleaser emits lowercase; two-space separator).
- Parse with `strings.Fields(line)` (robust to the 2-space run; stagecoach filenames contain no
  spaces so 2-fields split is exact). Validate `len==2` and `isHex64(fields[0])`. Store `map[name]sum`.
- Skip blank/whitespace-only lines. Malformed line → typed error (do not silently drop — FR-U11 aborts).

## 4. SHA256 verification (stdlib, constant-time)

- `sha256.New()` → `hash.Hash`; stream `io.Copy(h, file)` (never `os.ReadFile` on multi-MB archives —
  memory). `hex.EncodeToString(h.Sum(nil))` → 64-char lowercase hex.
- Compare digest to `want` with `crypto/subtle.ConstantTimeCompare([]byte(got), []byte(want))`; ==1 ⇒
  match. (Both are 64 hex chars → length constant; no length leak.) Normalize `want` to lower+trim first.
- Mismatch → `ErrChecksumMismatch` (distinct sentinel — caller branches on tampered-vs-missing-line).
- I/O failures (open/read) → plain wrapped error, NOT ErrChecksumMismatch (different failure class).

## 5. Download streaming + redirect-following (stdlib)

- `browser_download_url` is `https://github.com/.../releases/download/<tag>/<file>` → 302 to
  `objects.githubusercontent.com`. **net/http follows redirects by default** (CheckRedirect default).
- Stream `resp.Body` → `os.Create(dest)` via `io.Copy` (chunks; no whole-file buffer). `defer` drain
  (`io.Copy(io.Discard, resp.Body)`) + `Close` for connection reuse (same gotcha as metadata layer).
- Reuse the Client's `*http.Client`, `User-Agent: stagecoach/<ver>`, and `Bearer <Token>` headers (build
  the asset GET with the same header logic as `releases.go`'s `newReq`, but against the ABSOLUTE url —
  `BaseURL` is NOT used for asset downloads, only the asset's `DownloadURL`).
- Non-2xx / transport failure → reuse `ErrHTTP` (same network surface as the metadata layer).

## 6. Design decision — method vs package-func placement

| function | placement | why |
|---|---|---|
| `SelectAsset(release, goos, goarch)` | **package func** (pure) | no network; trivially unit-testable without a Client |
| `VerifySHA256(path, want)` | **package func** (pure-ish; opens+reads one file) | no network; no Client state needed |
| `(c *Client) DownloadFile(ctx, url, dest)` | **method** | reuses c.HTTP + UA + Bearer token |
| `(c *Client) FetchChecksums(ctx, release)` | **method** | downloads via the Client |
| `(c *Client) DownloadAndVerifyArchive(ctx, release, goos, goarch, destDir)` | **method** | composes the above; returns verified archive path |

All live in ONE new file `internal/upgrade/download.go`; tests in `internal/upgrade/download_test.go`.

## 7. New sentinels for this subtask (declared in download.go)

```go
var (
    ErrNoMatchingAsset  = errors.New("upgrade: no release asset matches GOOS/GOARCH")
    ErrNoChecksumsFile  = errors.New("upgrade: release has no checksums.txt asset")
    ErrChecksumMissing  = errors.New("upgrade: selected asset absent from checksums.txt")
    ErrChecksumParse    = errors.New("upgrade: checksums.txt has a malformed line")
    ErrChecksumMismatch = errors.New("upgrade: downloaded archive SHA256 ≠ checksums.txt")
)
// ErrHTTP (transport/status) is REUSED from releases.go — same network surface.
```

## 8. FR-U11 (abort-before-write) — scope in THIS subtask

The "real" FR-U11 gate is the binary swap in P1.M3.T2 (never overwrite the running binary until the
new one is downloaded+verified+sanity-run). In this subtask the analog is: `DownloadAndVerifyArchive`
writes ONLY into the caller-provided `destDir` (a staging/temp dir, not the install path); on ANY
failure it `os.Remove`s its own partial `dest` file so no corrupt bytes linger. The caller (P1.M3.T1)
owns the staging dir and the eventual extract+swap. Nothing here touches the real binary location.

## 9. Test approach (httptest — never the real API in CI)

Mirror `releases_test.go`'s `newFakeClient(t, handler)` + `httptest.NewServer` + `t.Cleanup(Close)`.
- The fake server routes by `r.URL.Path`: serves canned archive bytes + canned checksums.txt text.
- A Release is hand-built with `Assets` whose `DownloadURL = ts.URL + "/dl/<name>"`.
- Tampered test = server serves bytes whose sha ≠ the checksums.txt line → ErrChecksumMismatch + dest removed.
- Missing-line test = checksums.txt omits the archive name → ErrChecksumMissing.
- No-asset test = (goos,goarch) with no matching archive → ErrNoMatchingAsset.
- Pure tests (SelectAsset, VerifySHA256) need no server.

## 10. Authoritative URLs for the PRP references

- goreleaser checksums: https://goreleaser.com/customization/package/checksum/
- goreleaser archives (name_template): https://goreleaser.com/customization/archives/
- crypto/sha256: https://pkg.go.dev/crypto/sha256
- crypto/subtle.ConstantTimeCompare: https://pkg.go.dev/crypto/subtle#ConstantTimeCompare
- net/http client (redirects): https://pkg.go.dev/net/http#Client (CheckRedirect default follows up to 10)