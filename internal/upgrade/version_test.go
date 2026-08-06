package upgrade

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"1.0.0 < 1.0.1", "1.0.0", "1.0.1", -1},
		{"1.0.0 == v1.0.0 (missing v tolerated)", "1.0.0", "v1.0.0", 0},
		{"1.0.0 > 1.0.0-rc1 (release > prerelease)", "1.0.0", "1.0.0-rc1", 1},
		{"2.0.0 > 1.9.9 (numeric, not lexical)", "2.0.0", "1.9.9", 1},
		{"1.10.0 > 1.9.0 (numeric minor)", "1.10.0", "1.9.0", 1},
		{"v1.0.0-rc1 < v1.0.0-rc2", "v1.0.0-rc1", "v1.0.0-rc2", -1},
		{"1.0.0-alpha < 1.0.0-beta (lexical)", "1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-alpha.1 < 1.0.0-alpha.2", "1.0.0-alpha.1", "1.0.0-alpha.2", -1},
		{"1.0.0-alpha < 1.0.0-alpha.1 (shorter pre is lower)", "1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-1 < 1.0.0-2 (numeric pre)", "1.0.0-1", "1.0.0-2", -1},
		{"1.0.0-1 < 1.0.0-alpha (numeric id < alpha id)", "1.0.0-1", "1.0.0-alpha", -1},
		{"1.0.0+build == v1.0.0 (build metadata ignored)", "1.0.0+build", "v1.0.0", 0},
		{"unparseable a → 0 (no false update)", "dev", "1.0.0", 0},
		{"unparseable b → 0", "1.0.0", "dev", 0},
		{"1.0.0 == 1.0.0", "1.0.0", "1.0.0", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestParseAndClean(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantOk bool
	}{
		{"bare release", "1.0.0", "v1.0.0", true},
		{"v-prefixed release", "v1.0.0", "v1.0.0", true},
		{"prerelease", "1.0.0-rc1", "v1.0.0-rc1", true},
		{"build metadata stripped", "1.0.0+exp.sha.5", "v1.0.0", true},
		{"dev → not ok", "dev", "", false},
		{"dev with vcs suffix → not ok", "dev (19f4df7-dirty)", "", false},
		{"empty → not ok", "", "", false},
		{"two-part → not ok", "1.0", "", false},
		{"leading zero patch → not ok", "1.0.01", "", false},
		{"non-numeric → not ok", "x.y.z", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseAndClean(tc.in)
			if got != tc.want || ok != tc.wantOk {
				t.Errorf("ParseAndClean(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOk)
			}
		})
	}
}

func TestCurrentSemver(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantOk bool
	}{
		{"release bare", "0.1.0", "v0.1.0", true},
		{"release v-prefixed", "v0.1.0", "v0.1.0", true},
		{"dev build", "dev", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			saved := currentVersion
			SetCurrentVersion(tc.in)
			t.Cleanup(func() { currentVersion = saved })

			got, ok := CurrentSemver()
			if got != tc.want || ok != tc.wantOk {
				t.Errorf("CurrentSemver() = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOk)
			}
		})
	}
}
