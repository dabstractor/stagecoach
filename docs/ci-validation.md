# CI Validation Procedure

> For the validation agent (and any coding agent) to confirm Stagecoach's CI is green against
> the latest commit. This is a belt-and-suspenders reference — the `gh` commands below are
> standard, but writing them down guarantees the step is deterministic and self-contained.
>
> **"Validation passes"** = a CI run against the latest commit on the branch _exists_,
> _completes_, and _all jobs succeed_. Anything else is a validation failure: surface it and
> stop. Do not mark the change complete until CI is green.

## Prerequisites

- `gh` (GitHub CLI) installed and authenticated. Check with `gh auth status`.
- Push access to the repo. `workflow_dispatch` is enabled on `.github/workflows/ci.yml`.

## 1. Make sure the latest commit is pushed

```bash
git push                  # push the current branch
git rev-parse HEAD        # the commit CI must validate
```

If the push is rejected (remote moved), rebase or merge first. Do **not** force-push unless
the PRP explicitly permits a history rewrite.

## 2. Trigger a CI run against this branch

`ci.yml` fires automatically on `push: main` and on `pull_request`. On any **other** branch a
plain `push` does **not** trigger CI — trigger it manually:

```bash
BRANCH=$(git rev-parse --abbrev-ref HEAD)

gh workflow run ci.yml --ref "$BRANCH"
```

`gh workflow run` returns immediately (it dispatches, then exits); it does **not** print the
run id. If the branch's `ci.yml` differs from `main`'s, GitHub uses the **branch's** version
of the workflow file.

> If the dispatch is rejected with *"workflow does not have `workflow_dispatch` trigger"*,
> the branch's `ci.yml` predates the trigger — either land the trigger change on `main` first,
> or open a PR (`pull_request` fires CI automatically; see step 6).

## 3. Find the run

```bash
gh run list --workflow=ci.yml --branch="$BRANCH" --limit=5
```

The just-dispatched run is the top row with event `workflow_dispatch`. Copy its run id (first
column).

## 4. Watch it to completion

```bash
gh run watch <run-id> --exit-status
```

`--exit-status` makes `gh` exit non-zero if any job fails, so you can branch on the result. It
**blocks** until the run finishes (the os × Go matrix can take several minutes). To poll
instead of block:

```bash
gh run view <run-id> --json status,conclusion -q '.status'    # "in_progress" → keep polling
```

## 5. If you are on `main`

A push to `main` triggers CI automatically — no dispatch needed. Watch the latest run:

```bash
gh run list --workflow=ci.yml --branch=main --limit=1     # find the run id
gh run watch <run-id> --exit-status
```

## 6. If a job failed — read the failure

```bash
gh run view <run-id>                  # summary: which jobs failed
gh run view <run-id> --log-failed     # logs of the failed steps only
```

For one job's full log:

```bash
gh run view --job=<job-id> --log
```

The CI jobs (every one must pass):

| Job                | What it checks                                          |
| ------------------ | ------------------------------------------------------- |
| `build-test`       | build + race-enabled tests across the os × Go matrix    |
| `lint`             | `golangci-lint`                                         |
| `vulncheck`        | `govulncheck`                                           |
| `coverage`         | test coverage gate (PRD §20.3)                          |
| `npm-smoke`        | npm wrapper smoke test across os                        |
| `nix-flake-check`  | `nix flake check`                                       |
| `asdf-plugin-smoke`| asdf plugin install smoke                               |

## 7. Alternative: open a PR instead of dispatching

`pull_request` triggers CI with no `workflow_dispatch` needed, and re-runs on every push to
the PR branch (`synchronize`). If a change is destined for review anyway:

```bash
gh pr create --draft --fill        # draft PR; CI fires on pull_request
gh pr checks --watch               # watch the PR's check runs to completion
```

The `concurrency` block in `ci.yml` auto-cancels superseded runs on the same ref for PRs, so
rapid pushes don't pile up. Dispatched runs (`workflow_dispatch`) are **not** auto-cancelled
and always run to completion.

## Outcome

- **All green** → validation passes. Report the run URL
  (`gh run view <run-id> --json url -q .url`).
- **Any red** → read the failure (step 6), surface it, and stop. The change is not complete.