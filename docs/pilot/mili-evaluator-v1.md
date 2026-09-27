# Mili pilot evaluator rubric v1

Status: frozen rubric for approved tasks MLI-01–MLI-05. This file defines
observable acceptance checks for the clean source commit
`49402ad341a7e8a01aee7968d1b10d91b44c0d1c`. It does not assert that any task
has been implemented or evaluated. Run every condition from a fresh copy of
that commit, with only that condition's approved instruction files overlaid.

## Common execution rules

- Working directory: `backend/core` in the isolated Mili copy.
- Use Java 25, matching `core/build.gradle`'s toolchain. Do not run from the
  original modified checkout.
- The evaluator test sources must be maintained separately from agent-authored
  changes and frozen by content hash before runs. The class names below are the
  required evaluator entry points.
- Primary evaluator tests must be deterministic and must not extend
  `com.mili.core.BaseIntegrationTest`, import `TestContainersConfiguration`,
  start Spring Boot with external infrastructure, or contact a database,
  broker, network service, or wall clock.
- In PowerShell, run a task with:

  ```powershell
  & '.\gradlew.bat' test --tests 'com.mili.core.webhook.pilot.Mli01AcceptanceTest' --no-daemon
  ```

  Substitute the task's class name below. A zero exit code is necessary but
  insufficient: the listed assertions must be present in the independently
  frozen evaluator test source and all must pass.
- `PASS`: all required primary assertions pass and no required behavior is
  unverified. `PARTIAL`: tested behavior passes but an explicitly identified
  part of the contract cannot be established by the permitted primary checks.
  `FAIL`: a required assertion fails, the command fails, a required test is
  absent, or the task introduces an exception/status contrary to the contract.
  A skipped check is not a pass.

## Baseline

Before applying a task condition, run this command in the clean pinned copy
from `backend/core`:

```powershell
& '.\gradlew.bat' test --tests '*WebhookMonitoringServiceTest' --tests '*EventTypePathTest' --tests '*WebhookInboundCacheTest' --no-daemon
```

The draft records 8 passing unit tests. This is a focused baseline, not a full
build or integration-test result. The existing controller integration tests
inherit `BaseIntegrationTest` and start PostgreSQL/pg_partman and RabbitMQ via
Testcontainers; they are not part of the no-Docker primary evaluator.

## MLI-01 — Monitoring uses the authenticated tenant

Command (run independently for this task):

```powershell
& '.\gradlew.bat' test --tests 'com.mili.core.webhook.pilot.Mli01AcceptanceTest' --no-daemon
```

Required deterministic setup: set a `SecurityContext` whose authenticated
principal is tenant A, create a distinct tenant B and a B-owned event in a
fake/mock repository, and invoke the registered monitoring controller through
in-process MockMvc or an equivalent controller harness. Do not use a database.

Required observations:

1. Listing under tenant A only queries/returns A's scope. Supplying `tenantId=B`
   in the query string must not select B (whether the implementation ignores or
   rejects this obsolete parameter is a documented API detail; B data must not
   be disclosed).
2. Detail for B's event returns 404 and has the same externally observable
   not-found response as an unknown ID.
3. Reprocess for B's event returns the same 404 as an unknown ID and never
   invokes a reset/mutation for that event.
4. A tenant-A event remains visible to A, to prove the endpoint is functional
   rather than globally denying access.

Status: `PASS` only if all four assertions pass and service/repository calls
carry the authenticated A scope for list, detail, and reprocess. Otherwise
`FAIL`.

## MLI-02 — Malformed pagination cursors are rejected

Command:

```powershell
& '.\gradlew.bat' test --tests 'com.mili.core.webhook.pilot.Mli02AcceptanceTest' --no-daemon
```

Required deterministic observations, using a mocked repository/service and an
in-process HTTP controller:

1. Missing and blank cursor values start page one and reach the repository with
   both boundary components null.
2. A valid fixed cursor, `1730000000123|evt-2`, is parsed as epoch millis
   `1730000000123` and event ID `evt-2`.
3. Each malformed cursor (`abc|evt-2`, `1730000000123`,
   `1730000000123|`, and overflowing epoch `9223372036854775808|evt-2`)
   produces HTTP 400 and causes no repository query. Malformed input must not
   silently restart at page one or become HTTP 500.
4. A deterministic 50-item first-page response emits exactly
   `<last.receivedAt.toEpochMilli()>|<last.id>` as its next cursor; feeding that
   cursor to a second controller/service request passes the same boundary to
   the repository. A pure in-memory keyset fixture must also demonstrate no
   duplicate IDs across its two pages.

Status: `PASS` requires assertions 1–4 and a separate proof of the actual
PostgreSQL keyset query semantics. If only parser/controller/mock or an
in-memory comparator checks run, report `PARTIAL`, even if those checks pass.
Report `FAIL` for any failed primary assertion.

SQL limitation: the pinned repository uses PostgreSQL-specific row comparison
`(received_at, id) < (:beforeReceivedAt::timestamptz, :beforeId)`, ordered by
`received_at DESC, id DESC`, with `LIMIT 50`. A mocked repository or a copied
in-memory comparator proves cursor plumbing, not that this SQL produces a
complete duplicate-free page sequence. Existing end-to-end pagination tests
use `BaseIntegrationTest` and therefore Testcontainers. To claim `PASS`, run a
separate database-backed check against PostgreSQL (Docker is not required if a
controlled PostgreSQL instance is supplied); if unavailable, record `PARTIAL`
and leave SQL behavior unverified. Do not silently omit this distinction.

## MLI-03 — Pause/delete invalidate ingest authentication cache

Command:

```powershell
& '.\gradlew.bat' test --tests 'com.mili.core.webhook.pilot.Mli03AcceptanceTest' --no-daemon
```

Required deterministic setup: use the production cache implementation with a
mocked configuration repository and the real management service with mocked
repositories. Exercise the non-database ingest request through an in-process
controller harness with mocked authenticator/publisher dependencies.

Required observations:

1. Two lookups of an unchanged ACTIVE token return ACTIVE while querying the
   backing repository once (a cache hit is preserved).
2. After the real management service successfully pauses that webhook, the
   very next lookup/request reflects PAUSED; it must not serve the earlier
   ACTIVE cache entry. An ingest POST for the token returns 404 and does not
   publish.
3. After the real management service successfully deletes the webhook, the
   very next lookup/request reflects absence; an ingest POST returns 404 and
   does not publish.
4. Assertions occur after the service call has returned, so invalidation is
   effective after a successful update/delete. The path must not rely on
   sleeping for the current 60-second TTL.

Status: `PASS` if all four pass for both pause and delete; otherwise `FAIL`.
The request harness and cache/service tests must not use Testcontainers.

## MLI-04 — Reprocessing an absent event returns 404

Command:

```powershell
& '.\gradlew.bat' test --tests 'com.mili.core.webhook.pilot.Mli04AcceptanceTest' --no-daemon
```

Required deterministic observations: service tests with a mocked repository,
plus an in-process MVC/controller test using the real exception advice.

1. Missing event ID on reprocess maps to HTTP 404.
2. FAILED event returns HTTP 204 and calls `resetToPending(id)` exactly once.
3. A non-FAILED event returns HTTP 409 and never calls `resetToPending`.
4. Missing ID response is the expected not-found response, not a leaked
   `IllegalArgumentException` or HTTP 500.

Status: `PASS` if all observations pass. These checks must not inherit
`BaseIntegrationTest` and must not require Testcontainers.

## MLI-05 — JSONPath is validated before webhook persistence

Command:

```powershell
& '.\gradlew.bat' test --tests 'com.mili.core.webhook.pilot.Mli05AcceptanceTest' --no-daemon
```

Required deterministic observations:

1. Domain validation accepts `$.type` and rejects a syntactically malformed,
   pre-frozen expression. Use a stable exception type for invalid input.
2. Creating with the malformed expression returns HTTP 400.
3. The service rejects malformed syntax before either webhook configuration
   repository saves anything. Mockito verification must establish zero `save`
   calls (tenant/name prechecks may run).
4. Creating with `$.type` passes expression validation. Do not require actual
   database persistence in this primary evaluator.

Before freezing the evaluator source, verify the selected malformed literal is
rejected by the Jayway JsonPath version resolved from the pinned commit; do not
depend on incidental parser message text.

Status: `PASS` if all four observations pass; otherwise `FAIL`. All primary
checks must be unit/controller tests without Testcontainers. Persistence tests
may run separately and must be reported separately.

## Reporting template

Record one row per task and condition, with command exit code and evidence:

| Task | Status | Command exit | Assertions | Docker used? | Unverified limitation |
| --- | --- | ---: | --- | --- | --- |
| MLI-01 |  |  |  | No |  |
| MLI-02 |  |  |  |  | PostgreSQL keyset SQL must be checked separately; otherwise `PARTIAL`. |
| MLI-03 |  |  |  | No |  |
| MLI-04 |  |  |  | No |  |
| MLI-05 |  |  |  | No |  |
