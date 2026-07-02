# Execution Summary

**Status**: Success
**Fix Attempts**: 0


## Validation Results


### Level 1: Level 1 gate

- Status: PASSED
- Command: make build
- Skipped: No

      

### Level 1: Level 1 gate

- Status: PASSED
- Command: go vet ./...
- Skipped: No

      

### Level 1: Level 1 gate

- Status: PASSED
- Command: test -z "$(gofmt -l .)"
- Skipped: No

      

### Level 1: Level 1 gate

- Status: PASSED
- Command: make -n test coverage vet fmt clean lint
- Skipped: No

      

### Level 2: Level 2 gate

- Status: PASSED
- Command: go test ./...
- Skipped: No

      

### Level 3: Level 3 gate

- Status: PASSED
- Command: ./bin/stagehand --version
- Skipped: No

      

### Level 3: Level 3 gate

- Status: PASSED
- Command: test "$(./bin/stagehand --version)" != "stagehand version dev"
- Skipped: No

      

## Artifacts

No artifacts recorded.
