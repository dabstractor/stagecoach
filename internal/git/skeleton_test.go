package git

import (
	"testing"
)

// TestRenderNumstatSkeleton is the pure table test for renderNumstatSkeleton: empty→"", a normal
// numeric row, a binary row (mirrors `git diff --numstat`'s `-\t-\t<path>`), the sorted passthrough
// (S1 pre-sorts; render does not re-sort), and the header+trailing-blank-line shape. Deterministic
// exact-string equality. (PRD §9.1 FR3g — the render rules: §3 format, §4 empty→"", §5 binary `-\t-`.)
func TestRenderNumstatSkeleton(t *testing.T) {
	header := "Change summary (numstat: added\tdeleted\tpath):\n"
	cases := []struct {
		name string
		rows []numstatRow
		want string
	}{
		{
			name: "nil rows → \"\"",
			rows: nil,
			want: "",
		},
		{
			name: "empty slice → \"\"",
			rows: []numstatRow{},
			want: "",
		},
		{
			name: "one normal numeric row",
			rows: []numstatRow{{Added: 3, Deleted: 1, Path: "a.go"}},
			want: header + "3\t1\ta.go\n\n",
		},
		{
			name: "binary row renders literal hyphens (mirrors git numstat), NOT 0/0",
			rows: []numstatRow{{IsBinary: true, Added: 0, Deleted: 0, Path: "logo.png"}},
			want: header + "-\t-\tlogo.png\n\n",
		},
		{
			name: "sorted passthrough (rows are pre-sorted by S1; render does not re-sort)",
			rows: []numstatRow{
				{Added: 1, Deleted: 0, Path: "a.go"},
				{IsBinary: true, Path: "z.png"},
			},
			want: header + "1\t0\ta.go\n" + "-\t-\tz.png\n\n",
		},
		{
			name: "zero-change text row still renders 0/0 (not binary)",
			rows: []numstatRow{{Added: 0, Deleted: 0, Path: "mode-only.txt"}},
			want: header + "0\t0\tmode-only.txt\n\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderNumstatSkeleton(tc.rows)
			if got != tc.want {
				t.Errorf("renderNumstatSkeleton mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, tc.want)
			}
		})
	}
}
