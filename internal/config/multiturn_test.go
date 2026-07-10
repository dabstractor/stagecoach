// Package config — multi-turn config-knob tests (PRD §9.24 FR-T1c/FR-T3).
//
// TestMaterializeOverlay_MultiTurn proves the two multi-turn [generation] keys thread through the
// file-decode layer (fileGeneration → materialize → overlay) into the resolved Config the generate core
// (P1.M1.T3.S2 reads cfg.MultiTurnChunkTokens; P1.M1.T3.S3 reads cfg.MultiTurnFallback) consumes.
// Mirrors the gold-standard TestMaterializeOverlay_DiffContext_TokenLimit.
//
// Contract cases (P1.M1.T2.S3) — *bool semantics (P1.M1.T1.S1 fixed the only-true-propagates bug):
//
//	(b) multi_turn_chunk_tokens = 16000 in a file → resolved 16000 (int override honored)
//	(c) multi_turn_fallback = true in a file → resolved true (bool override honored)
//	(d) multi_turn_fallback = false in a file → resolved FALSE (FIXED: *bool overlay now propagates *false;
//	    previously silently ignored under the only-true-propagates bool guard)
//	(e) overlay precedence: repo-file chunk-tokens value overrides global-file value
//
// Case (a) — Defaults() returns true/32000 — is already pinned by TestDefaults (S1, LANDED); not duplicated.
package config

import "testing"

// TestMaterializeOverlay_MultiTurn is the load-bearing proof for the multi-turn config knobs across the
// materialize (file→Config) and overlay (Defaults → file) layers.
func TestMaterializeOverlay_MultiTurn(t *testing.T) {
	// ---- materialize-only: file → Config (the file→Config copy BEFORE any Defaults overlay) ----
	// materialize does NOT seed Defaults; an OMITTED key yields nil (NOT the default true). *bool semantics:
	// omitted ⇒ nil; explicit true ⇒ *true; explicit false ⇒ *false (all distinguishable now).
	materializeCases := []struct {
		name         string
		fileFallback *bool // nil = key omitted in the file
		fileChunk    int
		wantFallback *bool // nil ⇒ expect c.MultiTurnFallback == nil; non-nil ⇒ expect the pointed-to value
		wantChunk    int   // omitted ⇒ 0
	}{
		{"both_omitted_nil", nil, 0, nil, 0},
		{"chunk_set_16000", nil, 16000, nil, 16000},                  // (b) int honored at the file→Config copy
		{"fallback_set_true", boolPtr(true), 0, boolPtr(true), 0},    // (c) bool=true honored as *true
		{"fallback_set_false", boolPtr(false), 0, boolPtr(false), 0}, // explicit false now distinct from omitted (was indistinguishable under plain bool)
	}
	for _, tc := range materializeCases {
		tc := tc
		t.Run("materialize/"+tc.name, func(t *testing.T) {
			fc := &fileConfig{Generation: fileGeneration{
				MultiTurnFallback:    tc.fileFallback,
				MultiTurnChunkTokens: tc.fileChunk,
			}}
			c, err := materialize(fc, 0, 0)
			if err != nil {
				t.Fatalf("materialize: %v", err)
			}
			if tc.wantFallback == nil {
				if c.MultiTurnFallback != nil {
					t.Errorf("MultiTurnFallback = %v, want nil (materialize: omitted ⇒ nil; it does NOT seed the default)", c.MultiTurnFallback)
				}
			} else {
				if c.MultiTurnFallback == nil {
					t.Fatalf("MultiTurnFallback = nil, want non-nil *%v", *tc.wantFallback)
				}
				if *c.MultiTurnFallback != *tc.wantFallback {
					t.Errorf("*MultiTurnFallback = %v, want *%v (materialize copies the pointer through)", *c.MultiTurnFallback, *tc.wantFallback)
				}
			}
			if c.MultiTurnChunkTokens != tc.wantChunk {
				t.Errorf("MultiTurnChunkTokens = %d, want %d (materialize: omitted ⇒ 0; set ⇒ propagated)", c.MultiTurnChunkTokens, tc.wantChunk)
			}
		})
	}

	// ---- overlay chain: Defaults() → overlay(file) — the RESOLVED value the generate core reads ----
	// *bool overlay: nil ⇒ inherit the default true; non-nil (incl. *false) ⇒ explicit override. So an
	// omitted key resolves to true (default), while an explicit false resolves to false (the FIX).
	overlayCases := []struct {
		name         string
		fileFallback *bool
		fileChunk    int
		wantFallback bool // the RESOLVED value (via accessor): omitted ⇒ default true; explicit false ⇒ false
		wantChunk    int
	}{
		{"omitted_keeps_defaults", nil, 0, true, 32000},                 // both omitted ⇒ Defaults win
		{"chunk_override_16000", nil, 16000, true, 16000},               // (b) int override honored end-to-end
		{"chunk_override_48000", nil, 48000, true, 48000},               // a larger value
		{"fallback_true_reasserts_true", boolPtr(true), 0, true, 32000}, // (c) bool=true honored (redundant w/ default)
		{"fallback_false_disables", boolPtr(false), 0, false, 32000},    // (d) FIXED: false now DISABLES multi-turn end-to-end
	}
	for _, tc := range overlayCases {
		tc := tc
		t.Run("overlay/"+tc.name, func(t *testing.T) {
			cfg := Defaults() // MultiTurnFallback=boolPtr(true), MultiTurnChunkTokens=32000
			g, err := materialize(&fileConfig{Generation: fileGeneration{
				MultiTurnFallback:    tc.fileFallback,
				MultiTurnChunkTokens: tc.fileChunk,
			}}, 0, 0)
			if err != nil {
				t.Fatalf("materialize: %v", err)
			}
			overlay(&cfg, g)
			if cfg.MultiTurnFallbackValue() != tc.wantFallback {
				t.Errorf("MultiTurnFallbackValue() = %v, want %v (resolved value the generate core reads; *false now propagates)", cfg.MultiTurnFallbackValue(), tc.wantFallback)
			}
			if cfg.MultiTurnChunkTokens != tc.wantChunk {
				t.Errorf("MultiTurnChunkTokens = %d, want %d (resolved value the generate core reads)", cfg.MultiTurnChunkTokens, tc.wantChunk)
			}
		})
	}

	// ---- (e) overlay precedence: repo-file overrides global-file for chunk tokens ----
	t.Run("overlay/repo_overrides_global_chunk_tokens", func(t *testing.T) {
		cfg := Defaults() // 32000
		global, err := materialize(&fileConfig{Generation: fileGeneration{MultiTurnChunkTokens: 48000}}, 0, 0)
		if err != nil {
			t.Fatalf("materialize: %v", err)
		}
		overlay(&cfg, global)
		if cfg.MultiTurnChunkTokens != 48000 {
			t.Fatalf("after global overlay: MultiTurnChunkTokens = %d, want 48000", cfg.MultiTurnChunkTokens)
		}
		repo, err := materialize(&fileConfig{Generation: fileGeneration{MultiTurnChunkTokens: 16000}}, 0, 0)
		if err != nil {
			t.Fatalf("materialize: %v", err)
		}
		overlay(&cfg, repo) // higher layer (repo) wins
		if cfg.MultiTurnChunkTokens != 16000 {
			t.Errorf("after repo overlay: MultiTurnChunkTokens = %d, want 16000 (repo overrides global; higher layer wins)", cfg.MultiTurnChunkTokens)
		}
	})

	// ---- end-to-end via loadTOML (proves the TOML decode → resolved value path) ----
	t.Run("loadTOML/chunk_override_end_to_end", func(t *testing.T) {
		body := `
[generation]
multi_turn_chunk_tokens = 16000
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		if cfg.MultiTurnChunkTokens != 16000 {
			t.Errorf("loadTOML MultiTurnChunkTokens = %d, want 16000 (materialized file value)", cfg.MultiTurnChunkTokens)
		}
		dst := Defaults()
		overlay(&dst, cfg)
		if dst.MultiTurnChunkTokens != 16000 {
			t.Errorf("after overlay: MultiTurnChunkTokens = %d, want 16000 (resolved)", dst.MultiTurnChunkTokens)
		}
		if !dst.MultiTurnFallbackValue() {
			t.Errorf("after overlay: MultiTurnFallbackValue() = false, want true (key omitted ⇒ default wins)")
		}
	})

	// ---- end-to-end: multi_turn_fallback = false now DISABLES multi-turn (the *bool FIX) ----
	t.Run("loadTOML/fallback_false_disables_end_to_end", func(t *testing.T) {
		body := `
[generation]
multi_turn_fallback = false
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		// At materialize time the false decodes into a non-nil *false (DISTINGUISHABLE from omitted=nil now):
		if cfg.MultiTurnFallback == nil {
			t.Fatalf("loadTOML MultiTurnFallback = nil, want non-nil *false")
		}
		if *cfg.MultiTurnFallback != false {
			t.Errorf("loadTOML *MultiTurnFallback = %v, want false (materialized explicit false)", *cfg.MultiTurnFallback)
		}
		// And after overlay on Defaults, the *false propagates — multi-turn is DISABLED end-to-end:
		dst := Defaults()
		overlay(&dst, cfg)
		if dst.MultiTurnFallbackValue() {
			t.Errorf("after overlay: MultiTurnFallbackValue() = true, want false — *bool overlay now propagates an explicit false (the *bool FIX; was silently ignored under only-true-propagates)")
		}
	})

	// ---- §9.26 FR-W6: work_desc_read_rounds end-to-end (file → resolved value) ----
	t.Run("loadTOML/work_desc_read_rounds_end_to_end", func(t *testing.T) {
		body := `
[generation]
work_desc_read_rounds = 8
`
		path := writeTempTOML(t, body)
		cfg, err := loadTOML(path)
		if err != nil || cfg == nil {
			t.Fatalf("loadTOML: cfg=%v err=%v", cfg, err)
		}
		if cfg.WorkDescReadRounds != 8 {
			t.Errorf("loadTOML WorkDescReadRounds = %d, want 8 (materialized file value)", cfg.WorkDescReadRounds)
		}
		dst := Defaults() // 5
		overlay(&dst, cfg)
		if dst.WorkDescReadRounds != 8 {
			t.Errorf("after overlay: WorkDescReadRounds = %d, want 8 (resolved)", dst.WorkDescReadRounds)
		}
	})
}
