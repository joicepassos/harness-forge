# Candidate pilot task set (draft v1)

Status: **draft for review; not frozen; no agent runs**. The task text and
acceptance criteria below were derived from pinned clean commits, not from
uncommitted files in the original checkouts. Each run must start from a fresh
copy of the exact commit and use the same pre-registered task in every
instruction condition.

## Candidate repositories and baseline checks

| Repository | Commit | Baseline command in isolated clean copy | Result |
| --- | --- | --- | --- |
| Mili | `49402ad341a7e8a01aee7968d1b10d91b44c0d1c` | `gradlew.bat test --tests '*WebhookMonitoringServiceTest' --tests '*EventTypePathTest' --tests '*WebhookInboundCacheTest' --no-daemon` | Passed, 8 unit tests; full integration tests were not run because Docker is unavailable. |
| PromptForge | `e33bee6a45ce40aba2d6413fd0c283e8e62eeefc` | `go test ./...` from `backend/` | Passed compilation; packages currently contain no test files. |
| Toca | `3000adc7c5766e1a4cd88493d864843b4647abdd` | `npm ci && npm run build` from repository root | Passed; builds the server and client workspaces. |

The original checkouts are not baselines: Mili has modified and untracked user
files, PromptForge has an untracked Claude worktree directory, and Toca has
untracked local files. The pinned commits above were checked out separately.

## Proposed tasks

These 12 tasks are candidates for a 12–20 task pilot. They require user review
before freezing. “Acceptance” describes observable behavior and required tests;
the specific evaluator commands and assertions must be frozen alongside each
task before any agent run.

### Mili — Java service

1. **MLI-01 — Scope inbound-event monitoring to the authenticated tenant.**
   Replace client-selected tenant identity with the authenticated API-key tenant
   for list, detail, and reprocess. Add tests proving tenant A cannot list,
   inspect, or reprocess tenant B events; cross-tenant detail/reprocess must be
   indistinguishable from not found. Avoid database-backed tests in the primary
   acceptance command; use isolated controller/service tests.

2. **MLI-02 — Reject malformed event pagination cursors.**
   Missing or blank cursors start the first page. A cursor with malformed
   timestamp, missing separator, or empty event ID returns HTTP 400. A cursor
   produced by the API continues the next page without duplicates. Add service
   and controller tests.

3. **MLI-03 — Invalidate webhook authentication cache on pause/delete.**
   After pausing or deleting a webhook, the next lookup of its ingest token must
   reflect the new state immediately rather than serving the prior cached
   configuration. Preserve cache hits while the webhook is unchanged. Add
   deterministic cache/service tests and a non-database request test.

4. **MLI-04 — Return a not-found response for reprocessing an absent event.**
   Reprocessing an unknown ID returns 404; a FAILED event still returns 204 and
   moves to PENDING; a non-FAILED event returns 409. Add service and MVC tests
   that do not require Testcontainers.

5. **MLI-05 — Validate JSONPath syntax when creating a webhook.**
   Reject malformed event-type expressions before persistence with HTTP 400;
   accept a valid expression such as `$.type`. Add domain and controller/service
   tests. Keep persistence tests outside the no-Docker acceptance path.

### PromptForge — Go service and static frontend

6. **PF-01 — Add executable tests for the health endpoint contract.**
   Tests must verify GET `/api/health` returns 200, JSON content type, and the
   exact `status` and `service` fields. Exercise the registered router using an
   in-process HTTP test server. `go test ./...` must pass.

7. **PF-02 — Do not report the API as online for failed or invalid responses.**
   The frontend reports `online` only for a successful HTTP response with valid
   JSON containing `status: "ok"`. Non-2xx responses, malformed JSON, other
   payloads, and network failures must not be shown as online. Add tests using
   Node's built-in test runner; `node --test` and `go test ./...` must pass.

8. **PF-03 — Make server startup and shutdown testable and bounded.**
   Extract server construction so tests can start it on an ephemeral port,
   verify the health route, and request shutdown. Production startup handles
   interrupt/termination with a bounded graceful shutdown; `go test ./...`
   passes without Docker or external services.

### Toca — independent server and client workspaces

9. **TOCA-01 — Remove a user's private playlist when leaving a room.**
   When a user leaves or disconnects, remove that user's playlist and queue
   entry, and ensure the next DJ selection never replays their stale tracks.
   Add deterministic server tests; server and client workspace builds pass.

10. **TOCA-02 — Persist user data atomically and recover from invalid JSON.**
    A save must not leave a partial users file after interruption. Loading a
    missing file creates a usable store; malformed or structurally invalid JSON
    produces an explicit safe outcome without crashing the server. Add tests
    against an injectable temporary data directory; server build passes.

11. **TOCA-03 — Share one Socket.IO event contract between client and server.**
    Remove the mirrored hand-maintained event/type definitions and have both
    workspaces compile against one versioned shared contract. Add a contract
    check and keep both workspace builds passing.

12. **TOCA-04 — Validate untrusted Socket.IO event payloads at the server edge.**
    Reject malformed chat, track, and duration payloads without throwing,
    mutating room state, or broadcasting success. Valid payloads continue to
    work. Add unit tests for boundary values and build both workspaces.

## Freeze requirements

Before execution, freeze the accepted task texts, independent evaluator checks,
repository commits, baseline/team/Forge condition contents, exact agent product
and build, model, sandbox/tool permissions, run order, repetition count, and
review rubric. Current local inventory found Codex CLI only; Cursor, Claude Code,
and OpenCode command-line clients were not found. The recorded Codex CLI version
is `0.158.0-alpha.2.1`, but the model and run configuration are not frozen.
Do not describe these proposals or baseline build checks as pilot results.
