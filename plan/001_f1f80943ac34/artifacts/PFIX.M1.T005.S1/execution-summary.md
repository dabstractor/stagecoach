# Execution Summary

**Status**: Success
**Fix Attempts**: 0


## Validation Results


### Level 1: Level 1 gate

- Status: PASSED
- Command: go build ./cmd/stagehand/
- Skipped: No

      

### Level 1: Level 1 gate

- Status: PASSED
- Command: test -z "$(gofmt -l cmd/stagehand/stage.go cmd/stagehand/stage_test.go)"
- Skipped: No

      

### Level 1: Level 1 gate

- Status: PASSED
- Command: go vet ./cmd/stagehand/
- Skipped: No

      

### Level 2: Level 2 gate

- Status: PASSED
- Command: go test ./cmd/stagehand/ -run TestMaybeAutoStage -v
- Skipped: No

      

### Level 2: Level 2 gate

- Status: PASSED
- Command: go test ./cmd/stagehand/
- Skipped: No

      

### Level 3: Level 3 gate

- Status: PASSED
- Command: go test ./...
- Skipped: No

      

## Artifacts

No artifacts recorded.
