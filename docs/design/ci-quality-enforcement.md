# CI Quality Enforcement Design

**Status:** Implementation
**Date:** 2025-10-20

## Problem Statement

Current CI workflow (`.github/workflows/go.yml`) has critical issues:
1. **Wrong Go version**: CI uses 1.20, but `go.mod` declares 1.24.6
2. **No quality enforcement**: Doesn't run fmt/vet/lint (violates CLAUDE.md standards)
3. **No race detection**: Missing `-race` flag in tests
4. **No coverage**: No visibility into test coverage
5. **No caching**: Downloads dependencies on every run
6. **Inefficient**: Tests entire codebase even for small changes

## Goals

1. **Enforce CLAUDE.md standards** in CI (fmt, vet, lint mandatory)
2. **Optimize for speed** - only check/test changed code
3. **Correct Go version** - match go.mod (1.24.6)
4. **Modern actions** - use latest versions with built-in caching
5. **Fast feedback** - developers see issues in <2 minutes

## Changed-Files-Only Strategy

### Why?
- Small PR touching `analyzer/drift.go` shouldn't test entire `providers/aws/` tree
- Faster feedback loop (2min vs 10min for full codebase)
- Still safe: full build catches integration issues

### What Gets Checked?

**Changed files only:**
- `golangci-lint` - only lint changed `.go` files
- `go fmt` - only format-check changed files
- `go vet` - only vet changed packages

**Full checks (always):**
- `go build ./...` - catch integration issues
- `go test` - run tests for changed packages + dependent packages

### Implementation

```bash
# Get changed Go files
CHANGED_FILES=$(git diff --name-only origin/main...HEAD | grep '\.go$')

# Get changed packages
CHANGED_PKGS=$(go list -f '{{.Dir}}' ./... | xargs -I {} sh -c 'git diff --name-only origin/main...HEAD | grep -q {} && echo {}' | xargs)

# Run checks on changed files only
golangci-lint run --new-from-rev=origin/main
gofmt -l $CHANGED_FILES

# Run tests for changed packages
go test -race -v $CHANGED_PKGS
```

## CI Workflow Structure

```yaml
jobs:
  quality:
    # Format, vet, lint (changed files only)

  test:
    # Test changed packages + race detection

  build:
    # Full build (always - catches integration issues)
```

## Edge Cases

1. **First PR / No base branch**
   - Fallback to checking all files
   - `git diff origin/main...HEAD || find . -name "*.go"`

2. **Non-Go file changes (docs, yaml)**
   - Skip Go checks
   - Still run build (to catch broken imports)

3. **go.mod/go.sum changes**
   - Always run full test suite
   - Dependencies changed = potential wide impact

4. **main branch push**
   - Always run full checks
   - No shortcuts on main

## Performance Improvements

### Before (Current)
```yaml
- go build ./...        # ~30s
- go test ./...         # ~2min
# Total: ~2.5min (no caching, no parallelization)
```

### After (Optimized)
```yaml
- setup-go (with cache) # ~5s (cached)
- lint (changed files)  # ~10s
- test (changed pkgs)   # ~30s (for small changes)
- build (full)          # ~20s (cached deps)
# Total: ~1min for small PRs, ~3min for large changes
```

## Quality Gates

All must pass for PR merge:
- ✅ `go fmt` - no unformatted files
- ✅ `go vet` - no suspicious code
- ✅ `golangci-lint` - all linters pass
- ✅ `go test -race` - all tests pass, no races
- ✅ `go build` - successful build
- ✅ Coverage doesn't decrease (future: enforce 80%+)

## Success Metrics

- CI runtime < 2min for small PRs (1-5 files changed)
- CI runtime < 4min for large PRs (10+ files changed)
- Zero false positives (checks should be accurate)
- Catches all CLAUDE.md violations before merge
