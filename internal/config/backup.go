package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteTimestampedBackup copies the file at path to a timestamped sibling (path + ".bak.<timestamp>")
// and returns the backup path. If path does not exist, it returns "" and nil (nothing to back up —
// e.g. a first-time `config init` with no existing file). Used by every config-writing command to
// satisfy PRD §9.17 FR-B8's reversible-write guarantee ("any such write also leaves a timestamped
// backup of the prior file alongside it, mirroring FR-I3's no-mangle backup protocol").
//
// The timestamp format is RFC3339 compact (2006-01-02T150405Z) — second-granular, lexicographically
// sortable, filesystem-safe (no colons). A backup is created EVEN when the content is unchanged, so
// the audit trail is unconditional; callers that no-op (e.g. upgrade on an already-current file)
// skip this call entirely.
func WriteTimestampedBackup(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // nothing to back up
		}
		return "", fmt.Errorf("read for backup %s: %w", path, err)
	}
	stamp := time.Now().UTC().Format("2006-01-02T150405Z")
	backup := filepath.Join(filepath.Dir(path), filepath.Base(path)+".bak."+stamp)
	if err := os.WriteFile(backup, data, 0o644); err != nil {
		return "", fmt.Errorf("write backup %s: %w", backup, err)
	}
	return backup, nil
}
