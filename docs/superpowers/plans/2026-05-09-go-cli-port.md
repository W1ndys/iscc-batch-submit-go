# Go CLI Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a modular Go CLI that logs in to ISCC, discovers unsolved challenges, and concurrently submits candidate flags.

**Architecture:** The CLI entrypoint stays thin and delegates to `internal/runner`. HTTP/session behavior lives in `internal/iscc`, including cookie cache, nonce parsing, challenge parsing, result parsing, and submission.

**Tech Stack:** Go standard library, `golang.org/x/net/html` for HTML parsing, `spf13/cobra` for CLI flags, `go-task` Taskfile for operations.

---

### Task 1: Core Types And Parsers

**Files:**
- Create: `go.mod`
- Create: `internal/iscc/types.go`
- Create: `internal/iscc/parse.go`
- Create: `internal/iscc/parse_test.go`

- [x] Define challenge, attempt, result, cookie cache, and result map types.
- [x] Add HTML parsing for nonce, challenges, team path, and solved IDs helpers.
- [x] Add result parsing and solved/result message helpers.
- [x] Cover parsers with unit tests.

### Task 2: HTTP Client And Cookie Cache

**Files:**
- Create: `internal/iscc/client.go`
- Create: `internal/iscc/cookie.go`
- Create: `internal/iscc/client_test.go`

- [x] Implement per-client HTTP transport with proxy/trust-env controls and browser-like headers.
- [x] Implement login, nonce fetch, challenge fetch, solves fetch, and flag submission.
- [x] Implement JSON cookie cache load/save and header-cookie restoration.
- [x] Cover client behavior with `httptest`.

### Task 3: Runner And CLI

**Files:**
- Create: `internal/runner/config.go`
- Create: `internal/runner/runner.go`
- Create: `cmd/iscc-submit/main.go`

- [x] Parse CLI flags equivalent to the Python script.
- [x] Build logged-in client from explicit cookie, cookie file, cache, or username/password/env.
- [x] Load flags, filter challenges, run concurrent retry rounds, and print round summaries.
- [x] Preserve interrupt behavior.

### Task 4: Operations Tooling And Docs

**Files:**
- Create: `Taskfile.yml`
- Modify: `README.md`

- [x] Add tasks for format, test, build, run, and clean.
- [x] Document install/build/run examples and environment variables.
- [x] Run `gofmt`, `go test ./...`, and `go build ./cmd/iscc-submit`.
