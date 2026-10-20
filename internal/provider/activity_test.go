package provider

import (
	"sync"
	"testing"
	"time"
)

// TestCountingWriter_AccumulatesAndStamps: sequential Writes accumulate in order, each returns
// (len(p), nil), and the last-write stamp is recent (non-zero, within 5s — monotonic-accurate via
// time.Since since both time.Now() calls carry the monotonic reading).
func TestCountingWriter_AccumulatesAndStamps(t *testing.T) {
	w := &countingWriter{}
	for i, chunk := range []string{"a", "bb", "ccc"} {
		n, err := w.Write([]byte(chunk))
		if err != nil {
			t.Fatalf("Write(%q): %v", chunk, err)
		}
		if n != len(chunk) {
			t.Errorf("Write(%q) = (%d, nil), want (%d, nil)", chunk, n, len(chunk))
		}
		if i == 0 {
			if w.LastWrite().IsZero() {
				t.Error("LastWrite() zero after first Write, want non-zero")
			}
		}
	}
	if got := string(w.Bytes()); got != "abbccc" {
		t.Errorf("Bytes() = %q, want %q", got, "abbccc")
	}
	lw := w.LastWrite()
	if lw.IsZero() {
		t.Fatal("LastWrite() zero after writes, want non-zero")
	}
	if since := time.Since(lw); since >= 5*time.Second {
		t.Errorf("time.Since(LastWrite()) = %v, want < 5s (the stamp must be recent)", since)
	}
}

// TestCountingWriter_ZeroBeforeFirstWrite: a fresh writer reports the zero time ("idle since
// forever" — the watcher baselines at arm time) and an empty buffer.
func TestCountingWriter_ZeroBeforeFirstWrite(t *testing.T) {
	w := &countingWriter{}
	if !w.LastWrite().IsZero() {
		t.Errorf("LastWrite() = %v on a fresh writer, want the zero time", w.LastWrite())
	}
	if got := w.Bytes(); len(got) != 0 {
		t.Errorf("Bytes() = %q on a fresh writer, want empty", got)
	}
}

// TestCountingWriter_SnapshotIsIndependent: Bytes() returns a COPY — a snapshot taken before a
// later Write is never mutated by it (callers keep a stable view).
func TestCountingWriter_SnapshotIsIndependent(t *testing.T) {
	w := &countingWriter{}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	snap := w.Bytes()
	if _, err := w.Write([]byte("y")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if string(snap) != "x" {
		t.Errorf("snapshot = %q after a later Write, want %q (Bytes() must return a copy)", snap, "x")
	}
	if got := string(w.Bytes()); got != "xy" {
		t.Errorf("Bytes() = %q, want %q", got, "xy")
	}
}

// TestCountingWriter_ConcurrentWrites is THE race gate (go test -race is the project standard):
// 8 goroutines × 200 Writes of 4 bytes each — the mutex must keep every byte and the stamp must be
// sane. A data race here fails the -race run loudly.
func TestCountingWriter_ConcurrentWrites(t *testing.T) {
	w := &countingWriter{}
	const goroutines = 8
	const writes = 200
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				if _, err := w.Write([]byte("abcd")); err != nil {
					t.Errorf("concurrent Write: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	want := goroutines * writes * 4
	if got := len(w.Bytes()); got != want {
		t.Errorf("len(Bytes()) = %d, want %d (%d goroutines × %d writes × 4 bytes)", got, want, goroutines, writes)
	}
	lw := w.LastWrite()
	if lw.IsZero() {
		t.Fatal("LastWrite() zero after concurrent writes, want non-zero")
	}
	if since := time.Since(lw); since >= time.Minute {
		t.Errorf("time.Since(LastWrite()) = %v, want < 1m", since)
	}
}
