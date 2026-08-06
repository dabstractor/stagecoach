// stage.go implements the FR-U5 download→verify→extract→sanity-run step of the direct-binary
// self-swap (§9.29 FR-U5 steps 4-6 + FR-U11). It composes the GitHub Releases Client's download/verify
// primitives (download.go) using the pre-selected asset, extracts the single stagecoach binary from
// the archive (stdlib archive/tar+gzip / archive/zip), and sanity-runs it (--version reports the
// target tag) BEFORE any on-disk swap (P1.M3.T2). On any failure it leaves the staging tempDir for
// inspection and returns a typed error (FR-U11 abort-before-swap). It is walled off (FR-U12:
// stdlib-only, no internal/* imports). File comment only — releases.go owns the package doc.
package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sentinel errors for the staging step (download+verify+extract+sanity). Each is wrapped with %w
// at its use site so errors.Is reaches the sentinel AND the concrete message survives — mirroring
// download.go's convention. The download sentinels (ErrChecksumMissing/ErrChecksumMismatch/ErrHTTP/
// ErrNoChecksumsFile/ErrChecksumParse) are PROPAGATED AS-IS from download.go's primitives so the
// command layer's errors.Is(err, upgrade.ErrChecksumMismatch) keeps working; only the NEW staging
// sentinels below are wrapped here.
var (
	// ErrArchiveNoBinary is returned when the archive's single stagecoach entry is missing (FR-U5
	// step 5: a corrupt or foreign archive must never yield a staged binary path).
	ErrArchiveNoBinary = errors.New("upgrade: archive has no stagecoach binary entry")

	// ErrSanityVersionMismatch is returned when the staged binary's --version output does not
	// contain the target tag (FR-U5 step 6 + FR-U11: a binary that misreports its version is NEVER
	// swapped).
	ErrSanityVersionMismatch = errors.New("upgrade: staged binary --version does not report the target tag")

	// ErrSanityRunFailed is returned when the staged binary fails to run or exits non-zero
	// (FR-U5 step 6 + FR-U11: a binary that does not run cleanly is NEVER swapped).
	ErrSanityRunFailed = errors.New("upgrade: staged binary failed to run")
)

// execVersion is the injectable sanity-run seam. The default runs the staged binary with
// "--version" via os/exec (cmd.Env unset ⇒ the child inherits os.Environ(), so callers/tests that
// drive the binary through env keep working). Tests override this package-level var to drive the
// cmd/stubcli fake binary (happy / wrong-tag / non-zero-exit). PACKAGE-LEVEL ⇒ stage_test.go must
// NOT call t.Parallel() (two concurrent tests would race on it); the sibling upgrade tests never
// touch it and may stay parallel.
var execVersion = func(ctx context.Context, path string) ([]byte, error) {
	return exec.CommandContext(ctx, path, "--version").Output()
}

// sanityCheck runs the staged binary with "--version" through the execVersion seam and asserts the
// output contains wantTag (a plain substring check — NOT a semver compare; that is the command
// layer's job) and that it exits 0. An exec error or non-zero exit ⇒ ErrSanityRunFailed; a missing
// tag ⇒ ErrSanityVersionMismatch. FR-U5 step 6 + FR-U11 abort-before-swap.
func sanityCheck(ctx context.Context, path, wantTag string) error {
	out, err := execVersion(ctx, path)
	if err != nil {
		return fmt.Errorf("sanity-run %s: %w", path, ErrSanityRunFailed)
	}
	if !bytes.Contains(out, []byte(wantTag)) {
		return fmt.Errorf("sanity-run %s: output %q lacks tag %q: %w", path, out, wantTag, ErrSanityVersionMismatch)
	}
	return nil
}

// extractBinary extracts the single stagecoach binary from the archive at archivePath into destDir
// and returns its path. The archive FORMAT is derived from the asset-name suffix (".zip" ⇒
// archive/zip; else ".tar.gz" ⇒ archive/tar+compress/gzip) so extraction is platform-agnostic and
// testable off-host — NO host-OS introspection import. The binary ENTRY base name is "stagecoach"(+".exe" for a
// .zip / windows asset); the DEST is "new-stagecoach"(+".exe") under destDir (the rename avoids
// colliding with the running "stagecoach"; P1.M3.T2 renames new-stagecoach→stagecoach at swap time).
//
// ONLY the single matching entry is extracted — every other entry (README, LICENSE, checksums) is
// skipped. The entry is matched by filepath.Base so a "./" tar prefix is tolerated. After writing,
// a best-effort os.Chmod(dest, 0o755) ensures executability on unix (archive/tar preserves header
// mode; archive/zip does not — covering both uniformly). A missing entry ⇒ ErrArchiveNoBinary.
func extractBinary(archivePath, destDir, assetName string) (string, error) {
	exe := strings.HasSuffix(assetName, ".zip") // .zip ⇒ windows ⇒ .exe suffix; .tar.gz ⇒ none.
	entryBase := "stagecoach"
	if exe {
		entryBase += ".exe"
	}
	destName := "new-stagecoach"
	if exe {
		destName += ".exe"
	}
	dest := filepath.Join(destDir, destName)

	found := false
	if exe {
		// archive/zip path: open the whole reader, iterate File entries, copy the matching one.
		zr, err := zip.OpenReader(archivePath)
		if err != nil {
			return "", fmt.Errorf("zip open %s: %w", archivePath, err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			if filepath.Base(f.Name) != entryBase {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("zip open entry %s in %s: %w", f.Name, archivePath, err)
			}
			out, err := os.Create(dest)
			if err != nil {
				rc.Close()
				return "", fmt.Errorf("create %s: %w", dest, err)
			}
			if _, err := io.Copy(out, rc); err != nil {
				rc.Close()
				out.Close()
				return "", fmt.Errorf("extract %s!%s: %w", archivePath, f.Name, err)
			}
			rc.Close()
			if err := out.Close(); err != nil {
				return "", fmt.Errorf("close %s: %w", dest, err)
			}
			found = true
			break
		}
	} else {
		// archive/tar + compress/gzip path: stream the tarball, copy the matching entry.
		f, err := os.Open(archivePath)
		if err != nil {
			return "", fmt.Errorf("tar open %s: %w", archivePath, err)
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("gzip open %s: %w", archivePath, err)
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", fmt.Errorf("tar read %s: %w", archivePath, err)
			}
			if filepath.Base(hdr.Name) != entryBase {
				continue
			}
			out, err := os.Create(dest)
			if err != nil {
				return "", fmt.Errorf("create %s: %w", dest, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", fmt.Errorf("extract %s!%s: %w", archivePath, hdr.Name, err)
			}
			if err := out.Close(); err != nil {
				return "", fmt.Errorf("close %s: %w", dest, err)
			}
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("extract %s: want %s: %w", archivePath, entryBase, ErrArchiveNoBinary)
	}
	// Best-effort chmod 0o755: ensures executability on unix (archive/zip drops mode bits;
	// archive/tar preserves them from the header). Near-no-op on windows. Never fails the extraction.
	_ = os.Chmod(dest, 0o755)
	return dest, nil
}

// StageNewBinary performs the download→verify→extract→sanity-run step of the FR-U5 direct-binary
// self-swap (§9.29 FR-U5 steps 4-6 + FR-U11) using the PRE-SELECTED asset (ResolveTarget's output,
// P1.M3.T1.S1) and the staging tempDir. It is the MISSING MIDDLE of the swap pipeline:
//
//   - ResolveTarget (P1.M3.T1.S1) returns (Release, Asset);
//   - StageNewBinary turns that into a verified, sanity-checked new-binary path in tempDir; and
//   - the backup + atomic swap (P1.M3.T2) renames new-stagecoach→stagecoach after a backup.
//
// It COMPOSES the SAME download/verify primitives the goos/goarch-based archive helper in
// download.go composes — FetchChecksums, DownloadFile, VerifySHA256 — but DIRECTLY via the
// pre-selected asset (NOT that helper: it takes goos/goarch, re-selects the asset, and removes the
// partial archive on failure, none of which fit the (release, asset) input nor this step's "leave
// tempDir for inspection" contract).
//
// Extraction is FORMAT-FROM-ASSET-SUFFIX (.zip ⇒ archive/zip; else .tar.gz ⇒ archive/tar+gzip) and
// pulls ONLY the single stagecoach entry to tempDir/new-stagecoach(.exe). The sanity-run execs the
// staged binary with "--version" through the package-level execVersion seam and asserts exit 0 AND
// the output contains release.Tag (a substring check, not a semver compare — that is the command
// layer's job).
//
// On ANY failure (download/verify/extract/sanity) StageNewBinary returns a typed error — the
// download.go sentinels (ErrNoChecksumsFile/ErrChecksumParse/ErrHTTP/ErrChecksumMissing/
// ErrChecksumMismatch) propagate AS-IS so errors.Is reaches them, and the NEW stage sentinels
// (ErrArchiveNoBinary/ErrSanityRunFailed/ErrSanityVersionMismatch) are wrapped at their use site —
// and LEAVES tempDir for inspection (no staging cleanup anywhere). On success it returns
// the newBinPath and does NOT clean tempDir either: P1.M3.T2 owns the success-path cleanup after the
// atomic swap.
//
// StageNewBinary NEVER touches the running binary — it writes ONLY inside tempDir (the archive +
// new-stagecoach) and performs no rename/move/chmod outside it. This is the FR-U11 invariant: a
// failure here leaves the running binary byte-for-byte unchanged. ZERO production callers after this
// subtask (the command layer, P1.M4.T2 runUpgrade, is the consumer).
//
// c must be non-nil (the package does not nil-guard the Client, matching releases.go's convention).
func StageNewBinary(ctx context.Context, c *Client, release Release, asset Asset, tempDir string) (string, error) {
	// (1) Download + verify via the pre-selected asset (FR-U5 step 4 + FR-U11). Compose the SAME
	// primitives the goos/goarch archive helper composes, but via the asset — no SelectAsset, no
	// remove-on-failure. FetchChecksums/DownloadFile/VerifySHA256 already wrap their sentinels
	// with %w; return them AS-IS so the command layer's errors.Is keeps working.
	sums, err := c.FetchChecksums(ctx, release)
	if err != nil {
		return "", err // propagates ErrNoChecksumsFile / ErrChecksumParse / ErrHTTP.
	}
	want, ok := sums[asset.Name]
	if !ok {
		return "", fmt.Errorf("asset %q not in checksums.txt: %w", asset.Name, ErrChecksumMissing)
	}
	archivePath := filepath.Join(tempDir, asset.Name)
	if err := c.DownloadFile(ctx, asset.DownloadURL, archivePath); err != nil {
		return "", err // propagates ErrHTTP (partial archive removed by DownloadFile itself).
	}
	if err := VerifySHA256(archivePath, want); err != nil {
		return "", err // propagates ErrChecksumMismatch (or a wrapped open/read I/O error).
	}

	// (2) Extract the single binary (FR-U5 step 5). Format + .exe suffix derived from the asset name.
	newBinPath, err := extractBinary(archivePath, tempDir, asset.Name)
	if err != nil {
		return "", err // ErrArchiveNoBinary / I/O.
	}

	// (3) Sanity-run (FR-U5 step 6 + FR-U11 abort-before-swap). A binary that fails to run or
	// misreports its version is NEVER swapped.
	if err := sanityCheck(ctx, newBinPath, release.Tag); err != nil {
		return "", err // ErrSanityRunFailed / ErrSanityVersionMismatch.
	}

	// (4) Success: return the staged path. Do NOT clean tempDir (P1.M3.T2 owns the success-path
	// cleanup after the atomic swap). On failure the caller likewise inspects tempDir as-is.
	return newBinPath, nil
}
