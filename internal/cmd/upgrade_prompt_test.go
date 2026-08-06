package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// setTTY flips the confirmUpgradeIsTTY seam for a test and restores it via t.Cleanup so it never leaks
// into a sibling test (mirrors the package-var override idiom used for interactiveStdinIsTTY).
func setTTY(t *testing.T, isTTY bool) {
	t.Helper()
	orig := confirmUpgradeIsTTY
	confirmUpgradeIsTTY = func() bool { return isTTY }
	t.Cleanup(func() { confirmUpgradeIsTTY = orig })
}

func TestConfirmUpgrade_AssumeYes(t *testing.T) {
	var out bytes.Buffer
	// assumeYes short-circuits before the TTY gate, so the seam value is irrelevant here.
	ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap the direct-binary install.", true, strings.NewReader(""), &out)
	if !ok || err != nil {
		t.Errorf("assumeYes: want (true,nil), got (%v,%v)", ok, err)
	}
	if strings.Contains(out.String(), "Proceed?") {
		t.Errorf("assumeYes must not prompt; got %q", out.String())
	}
}

func TestConfirmUpgrade_NonTTYRefuses(t *testing.T) {
	setTTY(t, false)
	var out bytes.Buffer
	// Even with "y" on stdin, non-TTY + no --yes must refuse (a piped/scripted run must not silently swap).
	ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap.", false, strings.NewReader("y\n"), &out)
	if ok {
		t.Errorf("non-TTY: want ok=false, got true")
	}
	if err == nil {
		t.Errorf("non-TTY: want an error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "re-run with --yes") {
		t.Errorf("non-TTY err missing --yes hint: %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "stagecoach:") {
		t.Errorf("non-TTY err must NOT have 'stagecoach:' prefix (main adds it): %v", err)
	}
	if strings.Contains(out.String(), "Proceed?") {
		t.Errorf("non-TTY must not print the prompt; got %q", out.String())
	}
}

func TestConfirmUpgrade_TTYYes(t *testing.T) {
	setTTY(t, true)
	for _, input := range []string{"y\n", "Y\n", "yes\n", "yeah\n"} {
		var out bytes.Buffer
		ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap.", false, strings.NewReader(input), &out)
		if !ok || err != nil {
			t.Errorf("input %q: want (true,nil), got (%v,%v)", input, ok, err)
		}
		if !strings.Contains(out.String(), "1.2.3 → 1.3.0") || !strings.Contains(out.String(), "Proceed? [y/N]") {
			t.Errorf("input %q: prompt missing current→target/Proceed; got %q", input, out.String())
		}
	}
}

func TestConfirmUpgrade_TTYDeclines(t *testing.T) {
	setTTY(t, true)
	for _, input := range []string{"n\n", "N\n", "\n", "no\n", "asdf\n"} {
		var out bytes.Buffer
		ok, err := confirmUpgrade("1.2.3", "1.3.0", "Self-swap.", false, strings.NewReader(input), &out)
		if ok || err != nil {
			t.Errorf("input %q: want (false,nil), got (%v,%v)", input, ok, err)
		}
		if !strings.Contains(out.String(), "aborted") {
			t.Errorf("input %q: want 'aborted' notice; got %q", input, out.String())
		}
	}
}

func TestPrintDelegatedUpdate(t *testing.T) {
	var out bytes.Buffer
	printDelegatedUpdate(&out, "aur", "sudo pacman -Syu stagecoach-bin")
	s := out.String()
	if !strings.Contains(s, "aur") {
		t.Errorf("missing channel: %q", s)
	}
	if !strings.Contains(s, "sudo pacman -Syu stagecoach-bin") {
		t.Errorf("command not echoed verbatim: %q", s)
	}
	if !strings.Contains(s, "does not run") {
		t.Errorf("missing FR-U4 'does not run' framing: %q", s)
	}
}

func TestPrintPrivilegeCommand(t *testing.T) {
	var out bytes.Buffer
	printPrivilegeCommand(&out, `sudo "/usr/local/bin/stagecoach" upgrade`)
	s := out.String()
	if !strings.Contains(s, `sudo "/usr/local/bin/stagecoach" upgrade`) {
		t.Errorf("command not echoed verbatim: %q", s)
	}
	if !strings.Contains(s, "not writable") {
		t.Errorf("missing 'not writable' framing: %q", s)
	}
	if !strings.Contains(s, "never auto-elevates") {
		t.Errorf("missing FR-U4 'never auto-elevates' framing: %q", s)
	}
}
