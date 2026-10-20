# External procfs notes — pgrp rchar poller (2 Hz, Linux)

Scope: API semantics for a poller that sums /proc/<pid>/io `rchar` across a process
group every ~0.5 s to detect agent activity. External research, fetched 2026-10-20:
man-pages 6.19 (man7.org), docs.kernel.org, torvalds/linux master. "LOCAL" = measured
on the dev box 2026-10-20 (single run, indicative, not a benchmark suite).

## 1. /proc/[pid]/stat layout & parsing
- Fields 1-6 (proc_pid_stat(5)): (1) pid, (2) comm in parentheses (TASK_COMM_LEN=16
  incl. NUL, silently truncated), (3) state char, (4) ppid, (5) **pgrp**, (6) session.
- After the LAST `)`, whitespace-split: token 0=state, 1=ppid, **2=pgrp**, 3=session.
- Pitfall: comm can contain spaces AND parentheses (`foo)bar`, `my agent -x`), so
  splitting the whole line or scanning for the first `)` misaligns every field.
  Robust parse: last `)` (Go: `bytes.LastIndexByte(data, ')')`), parse right side;
  optional sanity check token[0] is a valid state char. Source: proc(5) maintainer
  thread oss-sec/2022/q4/225 ("locate closing parenthesis with strrchr(')') from
  the end"). Note: man-pages 6.19 proc_pid_stat(5) itself does NOT document this.
- stat inode is S_IRUGO — world-readable; only man-page fields marked [PT] are
  zeroed when the PTRACE_MODE_READ_FSCREDS|PTRACE_MODE_NOAUDIT check denies
  (proc_pid_stat(5); base.c `ONE("stat", S_IRUGO, ...)`). pgrp is not [PT].
- Lifetime/races: zombies keep stat (state 'Z'). After the process is reaped, the
  /proc/<pid> dir disappears → open() = ENOENT. LOCAL: zombie stat+io readable;
  after waitpid(), stat open fails errno 2 (ENOENT). Also: an already-open FD never
  sees a recycled pid — later ops fail ESRCH (docs.kernel.org/filesystems/proc.html
  "Note that an open file descriptor..."). Poller rules: treat ENOENT as "member
  gone" (not error), re-identify members every tick, don't cache FDs across ticks.
- hidepid≥1 / subset=pid mounts (proc(5)) can hide other-uid pids entirely; same-uid
  children stay visible (hidepid=2 corner cases: unverified).

## 2. rchar semantics (/proc/[pid]/io)
- rchar = "The number of bytes returned by successful read(2) and similar system
  calls" (proc_pid_io(5)). Kernel doc: "simply the sum of bytes which this process
  passed to read() and pread(). It includes things like tty IO and it is unaffected
  by whether or not actual physical disk IO was required (the read might have been
  satisfied from pagecache)" (docs.kernel.org/filesystems/proc.html).
  → pipes/tty/socket read(2) count; pagecache hits count; short reads count only
  bytes actually returned; failed reads (-1) count nothing. `read_bytes` is the
  block-layer counter — different thing, don't use for activity.
- Dynamic loader: only read()-family syscalls count; mmap-mapped ELF segments add
  nothing to rchar (inference from kernel-doc wording). Exact ld.so read-vs-mmap
  split: unverified — irrelevant for activity detection (exec implies reads).
- Per-thread → per-process: each exiting thread's counters fold into the thread
  group's shared ioac (kernel/exit.c __exit_signal: `task_io_accounting_add(&sig->
  ioac, &tsk->ioac)`); /proc/<pid>/io shows the whole thread group. (Per-thread view
  exists at /proc/<pid>/task/<tid>/io, base.c.)
- **Double-counting hazard**: when a process is reaped, its ioac folds into the
  PARENT's (exit.c wait path: `task_io_accounting_add(&psig->ioac, &p->ioac)` +
  `&sig->ioac`); proc_pid_io(5): file shows stats "for the process and its
  waited-for children". So a group-member shell that waited on children carries
  their bytes. For a delta/activity signal this can inflate but never false-negative;
  exact summed totals may overcount. LOCAL: parent rchar jumped after waitpid() of a
  child that read ~100 KB — consistent with source.
- Atomicity: counters "are not atomic: ... a counter could be updated simultaneously
  with a read, yielding an incorrect intermediate value" (proc_pid_io(5) CAVEATS;
  kernel doc: "racy on 32-bit machines"). x86-64 aligned 64-bit loads: fine in
  practice (inference) — tolerate rare odd deltas, clamp negatives to 0.

## 3. Permissions (unprivileged, same-uid, no root)
- io: inode S_IRUSR (base.c `ONE("io", S_IRUSR, proc_tgid_io_accounting)`) AND open
  gated by ptrace(2): "Permission to access this file is governed by ptrace(2)
  access mode PTRACE_MODE_READ_FSCREDS" (proc_pid_io(5)).
- ptrace(2) access algorithm: same thread group → always allowed; allowed if
  target's real+effective+saved-set uid (and gid) all match the caller's, or caller
  has CAP_SYS_PTRACE in target's userns; denied if target dumpable != 1 (setuid/
  setgid, PR_SET_DUMPABLE=0 e.g. ssh-agent) without capability. ⇒ ANY same-uid
  process can read our own children's io — no root, and no parent-child relation
  required.
- Non-dumpable targets: io open fails EACCES, /proc/<pid>/* chowned root:root
  (proc_pid(5)); stat itself stays readable (S_IRUGO). Plain agent child processes
  are dumpable → unaffected.
- Yama interaction: kernel.yama.ptrace_scope restricts only attach-mode checks —
  yama_lsm.c: `if (mode & PTRACE_MODE_ATTACH)`; PTRACE_MODE_READ (io, stat [PT]) is
  unaffected at every scope value (Yama.rst + source). Distro default scope=1 does
  not break the poller.
- docs.kernel.org phrasing "reading other processes' /proc/PID/* requires
  CAP_SYS_PTRACE... or CAP_PERFMON" describes cross-uid access; same-uid dumpable
  targets pass the ptrace(2) uid-match rule above (reconciled from both sources).

## 4. Cost & gotchas at 2 Hz
- LOCAL: full glob `/proc/[0-9]*/stat` + open+read+parse each ≈ 13 ms/sweep over
  ~1091 pids (~431 kernel threads) in Python → ≈2.6% of a 500 ms cycle; Go lower.
  Dominant cost is enumerating ALL pids; the <20-member io reads are noise.
- Kernel threads appear in the glob. Their io is root-owned → unprivileged open =
  EACCES (inference from §3 rules: uid mismatch). Order of ops: parse stat FIRST
  (world-readable), filter by pgrp, only then open io for matching members.
- pid reuse between readdir and open → ENOENT (tolerate). pid NUMBER reuse across
  ticks aliasing to the same pgrp: theoretically possible; cheap hardener is
  /proc/<pid>/stat starttime (field 22) match across ticks.

## 5. Go implementations to learn from (names only — not reviewed this session)
- github.com/shirou/gopsutil/v4 — process.Process.IOCounters() wraps /proc/<pid>/io.
- github.com/prometheus/procfs — stat parsing + proc iteration (used by node_exporter).
- github.com/c9s/goprocinfo — older /proc parser.
- kubelet/pkg/probe and node_exporter process collector for glob/enumeration idioms.
  (All four: UNVERIFIED this session — consult before adopting their parsing.)

## Sources (all fetched 2026-10-20)
- proc_pid_stat(5) 6.19 — man7.org/linux/man-pages/man5/proc_pid_stat.5.html
- proc_pid_io(5) 6.19 — man7.org/linux/man-pages/man5/proc_pid_io.5.html
- proc(5)/proc_pid(5) — man7.org/linux/man-pages/man5/proc_pid.5.html
- ptrace(2) — man7.org/linux/man-pages/man2/ptrace.2.html
- https://docs.kernel.org/filesystems/proc.html
- torvalds/linux: fs/proc/base.c, kernel/exit.c, security/yama/yama_lsm.c,
  Documentation/admin-guide/LSM/Yama.rst
- https://seclists.org/oss-sec/2022/q4/225 (stat parse-after-last-`)` guidance)
