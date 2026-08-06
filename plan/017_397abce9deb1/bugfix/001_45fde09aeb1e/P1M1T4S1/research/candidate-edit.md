# Candidate clarifying edit — go-install ~/go/bin auto-detection note

## Origin
The item contract (LOGIC clause) names exactly ONE example of a "materially helpful" clarifying note:
"e.g. noting that go install installs are auto-detected via ~/go/bin". This is the only edit the audit
found worth considering; BUG-001/BUG-003 yield no user-facing doc change (internal logic fixes, no
CLI/config surface change).

## Why this note (and why only this one)
- It serves the EXACT user population BUG-002 affected: someone who ran
  `go install github.com/.../stagecoach@latest` (binary lands at ~/go/bin, GOPATH unset — the modern-Go
  default). Pre-fix they were misdetected as `direct` and (with BUG-001) their upgrade failed entirely.
  Post-fix `stagecoach upgrade` detects ~/go/bin and delegates to `go install ...@latest`.
- It tells the user WHERE detection looks, confirming the default-GOPATH case works — which is precisely
  the gap BUG-002 closed. Without it, the Updating section lists "go install" as a delegated channel but
  a user burned by the pre-fix bug has no explicit reassurance the ~/go default is honored.
- `~/go/bin` is stable Go convention (`go env GOPATH` defaults to it; README L135 already says
  "usually ~/go/bin"), so the note is not a volatile implementation detail.

## Why NOT to touch BUG-001 / BUG-003 in the docs
- BUG-001 (sanity-run v-normalization): purely internal. No doc describes the sanity-run; the user-facing
  claims ("self-swaps only for a direct install"; exit 0 = upgraded) are now TRUE because of the fix.
  Adding sanity-run internals to user docs would be noise.
- BUG-003 (non-semver tag deprioritization): --prerelease's user-facing behavior (admit pre-release tags)
  is unchanged; the fix only changes WHICH tag is selected when non-semver moving tags exist. A note
  here is a maintainer concern, not user help.

## Exact proposed edit (PRIMARY recommendation)
FILE: README.md
SECTION: `### Updating` (currently one paragraph at L146)
PLACEMENT: append one short parenthetical at the END of the delegation/self-swap sentence (the sentence
that currently ends "...self-swaps only for a direct (curl|sh / manual) install.").

CURRENT (end of that sentence):
  ...and self-swaps only for a direct (curl\|sh / manual) install. It never overwrites a package-manager-owned file...

PROPOSED (insert one sentence immediately AFTER "...manual) install."):
  ...self-swaps only for a direct (curl\|sh / manual) install. A `go install` binary under `~/go/bin`
  is detected automatically even when `GOPATH` is unset, so it delegates to `go install …@latest`
  rather than self-swapping. It never overwrites a package-manager-owned file...

Rationale for this wording:
- "under `~/go/bin`" matches the README's own L135 phrasing ("usually ~/go/bin") — internal consistency.
- "even when `GOPATH` is unset" is the EXACT condition BUG-002 fixed and the one a reader can't infer
  from the bare "go install" list entry.
- "delegates to `go install …@latest`" mirrors FR-U3 / the Updating section's own verb.

## Decision rubric (edit vs no-edit) — APPLY unless BOTH hold
The contract explicitly permits a no-edit outcome ("If all docs are already accurate, record that
finding and make no edits"). Use this rubric:

1. If the go-install ~/go/bin note reads as REDUNDANT with the existing "go install" list entry to you
   (i.e. you judge a reader already infers auto-detection from the list), SKIP it and make NO edits.
2. Otherwise APPLY the single sentence above.

In EITHER case the deliverable includes a recorded finding (see PRP "Output"): which files were touched
or confirmed accurate. If zero edits: the commit message + task result state "verified accurate, no
drift; no edits" and enumerate the three docs reviewed.

## Hard constraints (do NOT violate)
- Edit ONLY README.md (and only if applying the note). docs/cli.md and docs/configuration.md are
  CONFIRMED ACCURATE — do not edit them for this sweep.
- Do NOT touch PRD.md, .gitignore, any source file (the Mode A source doc comments are already done in
  T1.S1/T2.S1/T3.S1), docs/how-it-works.md, FUTURE_SPEC.md, or any tasks.json/prd_snapshot.md.
- Do NOT add a --prerelease / BUG-003 note (flag behavior unchanged; maintainer noise).
- Do NOT describe the sanity-run / BUG-001 in user docs (internal detail).