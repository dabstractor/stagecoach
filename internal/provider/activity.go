// activity.go provides the FR-A3 (§9.30) OUTPUT-SIDE activity channel: countingWriter, an
// io.Writer that both accumulates agent output and records WHEN it was last written. P1.M2.T1.S2
// (the Execute idle watcher) assigns countingWriters to cmd.Stdout/cmd.Stderr — every
// agent-emitted chunk bumps lastWrite (the idle clock resets) and accumulates in the buffer (S2
// stringifies Bytes() for Execute's return contract, replacing executor.go's plain bytes.Buffer).
// The INPUT-SIDE channel (the agent silently reading without emitting) is groupReadBytes in
// activity_linux.go / activity_other.go — the other half of FR-A3's activity definition.
//
// Zero-value contract: &countingWriter{} is ready to use — no constructor, no init. A never-written
// writer reports the zero time.Time from LastWrite(), which time.Since treats as "idle since
// forever"; the watcher baselines its idle clock at arm time (max(LastWrite, armTime) — S2's
// concern), so a quiet-from-birth agent is not instantly killed.
package provider

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// countingWriter is an io.Writer that wraps a bytes.Buffer behind a mutex and records the time of
// the last successful Write atomically. It is the stdout/stderr ACTIVITY channel for the FR-A3 idle
// watcher (P1.M2.T1.S2): assigned to cmd.Stdout/cmd.Stderr, every agent-emitted chunk both
// accumulates (S2 returns the buffer contents as Execute's stdout/stderr strings) and bumps
// lastWrite (the idle clock). Zero-value ready.
type countingWriter struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	lastWrite atomic.Pointer[time.Time] // whole time.Now() — monotonic reading preserved; nil ⇒ never written
}

var _ io.Writer = (*countingWriter)(nil) // compile-time contract: usable as cmd.Stdout/cmd.Stderr

// Write appends p to the buffer under the mutex and, when any bytes landed (n>0), stores the whole
// time.Now() (Go's clock embeds the monotonic reading) as the last-write stamp. A zero-length Write
// does not bump the clock — no bytes, no activity.
func (w *countingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.buf.Write(p)
	w.mu.Unlock()
	if n > 0 {
		now := time.Now() // monotonic-embedded (Go clock); stored whole via Pointer
		w.lastWrite.Store(&now)
	}
	return n, err
}

// LastWrite returns the time of the last successful (n>0) Write, or the zero time.Time if none.
// Compare with time.Since(w.LastWrite()) — monotonic-accurate when both times carry the reading,
// immune to NTP wall-clock jumps (unlike a UnixNano int64).
func (w *countingWriter) LastWrite() time.Time {
	if tp := w.lastWrite.Load(); tp != nil {
		return *tp
	}
	return time.Time{}
}

// Bytes returns a SNAPSHOT (copy) of the accumulated buffer — safe to call concurrently with
// writes; the returned slice is never mutated afterward. S2 stringifies this for the return contract.
func (w *countingWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.buf.Bytes()...) // COPY — callers keep a stable snapshot
}
