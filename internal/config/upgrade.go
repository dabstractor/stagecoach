package config

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

// LoadUpgradeConfig reads ONLY the global config file (the XDG discovery path — the same path
// globalConfigPath() / GlobalConfigPath() resolve) and returns the [upgrade] table with the built-in
// Defaults applied (PRD §9.29 FR-U10). It is the DEDICATED reader the `stagecoach upgrade` command
// (P1.M4.T1) calls, deliberately walled off from config.Load because:
//   - config.Load's Layer 2 bootstraps (writes) a config on a missing global file (FR-B3); upgrade
//     runs outside a git repo with a no-op PersistentPreRunE and must NOT write.
//   - config.Load's Layer 3 reads the per-repo .stagecoach.toml; FR-U10 says a per-repo [upgrade] is
//     IGNORED. (The --verbose "ignored" note is the upgrade command's job, P1.M4.T1.S1.)
//
// Semantics: a MISSING global file ⇒ Defaults().Upgrade with a nil error (NO write, NO error). A
// present file is decoded into fileConfig; non-empty [upgrade] fields override the defaults
// (mirrors materialize()'s non-zero overlay). It never reads repo/git-config/env/flags, never writes,
// and emits NO advisory (it never touches noticeOut). A parse error is wrapped with the path.
func LoadUpgradeConfig() (UpgradeConfig, error) {
	uc := Defaults().Upgrade // seed: Channel="stable", SourceRepo="dabstractor/stagecoach"
	path := globalConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return uc, nil // missing global file ⇒ defaults; NO bootstrap write, NO error (FR-B3 boundary)
		}
		return uc, fmt.Errorf("read upgrade config %s: %w", path, err)
	}

	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return uc, fmt.Errorf("parse upgrade config %s: %w", path, err)
	}

	// Non-empty wins (mirrors materialize's non-zero overlay; a "" value must not clobber the default).
	if fc.Upgrade.Channel != "" {
		uc.Channel = fc.Upgrade.Channel
	}
	if fc.Upgrade.SourceRepo != "" {
		uc.SourceRepo = fc.Upgrade.SourceRepo
	}
	return uc, nil
}
