package hooks

import "testing"

// runner_shebang_test.go unit-tests the pure shebang-parsing helper that drives the Windows
// hook-execution path (hookInvocation). parseShebang is OS-independent, so the Windows behavior is
// covered here on every platform — the runtime.GOOS guard in hookInvocation is the only piece not
// exercised, and it is a one-line early return.

func TestParseShebang(t *testing.T) {
	cases := []struct {
		name    string
		head    string
		interp  string
		optArgs []string
		ok      bool
	}{
		{"simple sh", "#!/bin/sh\necho hi\n", "/bin/sh", nil, true},
		{"bash with opt", "#!/usr/bin/env bash -e\necho hi\n", "bash", []string{"-e"}, true},
		{"env python", "#!/usr/bin/env python3\nprint(1)\n", "python3", nil, true},
		{"plain path interp", "#!/usr/local/bin/python\nprint(1)\n", "/usr/local/bin/python", nil, true},
		{"no shebang", "echo hi\n", "", nil, false},
		{"empty", "", "", nil, false},
		{"bare hashbang no interp", "#!\n", "", nil, false},
		{"shebang only spaces", "#!   \n", "", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interp, optArgs, ok := parseShebang([]byte(tc.head))
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if interp != tc.interp {
				t.Errorf("interp = %q, want %q", interp, tc.interp)
			}
			if !equalSlices(optArgs, tc.optArgs) {
				t.Errorf("optArgs = %v, want %v", optArgs, tc.optArgs)
			}
		})
	}
}

func TestIsShellShebang(t *testing.T) {
	for _, interp := range []string{"/bin/sh", "/bin/bash", "/usr/bin/dash", "/bin/zsh"} {
		if !isShellShebang(interp) {
			t.Errorf("isShellShebang(%q) = false, want true", interp)
		}
	}
	for _, interp := range []string{"/usr/bin/env python3", "/bin/awk", "/usr/bin/perl"} {
		if isShellShebang(interp) {
			t.Errorf("isShellShebang(%q) = true, want false", interp)
		}
	}
}

// equalSlices compares two string slices for order-sensitive equality (nil and empty are equal).
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
