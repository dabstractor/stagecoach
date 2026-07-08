// Package prompt — work-description mode (PRD §9.26 FR-W1–W8).
//
// Work-description mode inverts the default diff-first prompt (§9.5): instead of feeding the whole
// diff and letting the model reverse-engineer intent, it leads with the user-supplied work description
// + the numstat skeleton (the file menu) and lets the model pull specific file diffs on demand via a
// loose `READ <path>` text protocol (FR-W3). This file holds the prompt-construction half:
//
//   - BuildWorkDescSystemPrompt: the system prompt that states the READ protocol, the round budget N
//     (FR-W6), and the description-authoritative instruction (FR-W2 — describe the described work
//     faithfully in repo style; diffs only sharpen specifics).
//   - BuildWorkDescPayload: the description-first user payload (FR-W2) — work description + skeleton
//     (the file menu) + optional --context, with NO diff bodies.
//
// The read/answer LOOP (FR-W4/FR-W5) and the `READ <path>` line parser (FR-W3) live in
// internal/generate/workdesc.go (they need the git.Git + provider seams).
package prompt

import "fmt"

// workDescReadProtocolIntro opens the system prompt's protocol block. It states the single verb and
// the round budget so the model budgets its reads (FR-W6). It is the verbatim preamble the loop also
// relies on: a response with no valid READ line is the commit message (FR-W7).
const workDescReadProtocolIntro = `You are generating a git commit message for a repository. The user has provided a DESCRIPTION of the
work this commit covers. The description is content-authoritative: describe the described work
faithfully in THIS repository's commit style (governed by the recent-commit examples below). Do NOT
invent a framing that contradicts the description; the file diffs serve only to sharpen specifics you
choose to verify.

The changed files are listed below as a numstat skeleton (added/deleted/path). You may inspect any
staged file's diff on demand by writing, on its own line:
	READ <path>
Paths are matched against the staged set (the skeleton); case-insensitive, whitespace-forgiving, one
path per line or comma-separated, several per response. A path not in the staged changes is ignored
with a note. When you have enough to write the message, respond with NO READ line and put ONLY the
commit message in your response (the message parser takes your full output minus any READ lines).`

// workDescRoundBudgetFmt is the system-prompt line stating the exact read-round budget N (FR-W6).
// Stated up front so the model budgets its reads. N is the resolved cfg.WorkDescReadRounds.
const workDescRoundBudgetFmt = "Read budget: you may issue READ requests in at most %d responses. After that, no further reads are answered and you must output the commit message."

// skeletonIntro labels the numstat skeleton block in the description-first payload (FR-W2). The
// skeleton is the same block StagedDiff prepends (FR3g); it doubles as the menu of READ-able paths.
const skeletonIntro = "Changed files (numstat: added\tdeleted\tpath — request any with READ <path>):"

// workDescContextIntro labels the optional --context block in the description-first payload. It is
// distinct from the default-path contextIntro because the framing differs ("directing guidance" vs
// "additional context"); FR-W1 says --context is the _how_, --work-description is the _what_.
const workDescContextIntro = "Directing guidance from the user (how to phrase the message):"

// descriptionIntro labels the work-description block (the _what_) at the top of the payload.
const descriptionIntro = "Work description (what this commit does — describe this faithfully):"

// BuildWorkDescSystemPrompt builds the system prompt for work-description mode (PRD §9.26 FR-W2/FR-W6).
// It composes the base system prompt (style examples, format/locale — the SAME base the default path
// uses) with the READ-protocol preamble and the exact round budget N. baseSys is the already-built
// system prompt from BuildSystemPrompt/BuildFallbackPrompt (style learning + format + locale); this
// appends the protocol block so the model knows how to read files and how many rounds it has.
//
// N is the resolved cfg.WorkDescReadRounds (default 5). It is interpolated verbatim into the budget
// line so the model sees the exact cap (FR-W6).
func BuildWorkDescSystemPrompt(baseSys string, readRounds int) string {
	n := readRounds
	if n < 1 {
		n = 1 // defensive: a non-positive budget collapses to 1 round (no panic; FR-W6 guarantees termination)
	}
	return baseSys + "\n\n" + workDescReadProtocolIntro + "\n\n" + fmt.Sprintf(workDescRoundBudgetFmt, n)
}

// BuildWorkDescPayload builds the description-first user payload (PRD §9.26 FR-W2): the work
// description (the _what_), the numstat skeleton (the file menu / READ-able paths), and optional
// --context (the _how_) — with NO diff bodies (the model pulls those via READ). It is the turn-1
// payload of the read/answer loop (FR-W4).
//
// Assembly (FR-W2):
//
//	descriptionIntro + "\n" + workDescription + "\n\n"
//	[ + workDescContextIntro + "\n" + context + "\n\n"  if context != ""]
//	+ skeletonIntro + "\n" + skeleton
//
// The skeleton is passed in (already rendered by git.StagedNumstatSkeleton); it is appended VERBATIM
// (its trailing newline topology is the renderer's contract). workDescription and context are used
// VERBATIM — no trimming, no validation (FR-F7/FR-W1 parity). An empty skeleton (nothing staged) is
// the caller's responsibility to gate (FR-W8 auto-stages when nothing is staged, so this is non-empty
// in practice); an empty description yields a degenerate payload but is not a panic.
func BuildWorkDescPayload(workDescription, context, skeleton string) string {
	var b []byte
	b = append(b, descriptionIntro...)
	b = append(b, '\n')
	b = append(b, workDescription...)
	b = append(b, "\n\n"...)
	if context != "" {
		b = append(b, workDescContextIntro...)
		b = append(b, '\n')
		b = append(b, context...)
		b = append(b, "\n\n"...)
	}
	b = append(b, skeletonIntro...)
	b = append(b, '\n')
	b = append(b, skeleton...)
	return string(b)
}
