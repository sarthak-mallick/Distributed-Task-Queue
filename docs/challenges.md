# Engineering Challenges & Fixes

This document records non-trivial correctness, reliability, and maintainability
issues found during a full-repository code review, the fixes applied, and the
reasoning behind them. It is intended as a durable record of *why* the code looks
the way it does in these areas.

## Correctness & Reliability

### 1. Weather fallback fabricated results for invalid input

**Where:** `worker/main.go` — `fallbackWeatherClient.Fetch`

**Problem:** When the mock fallback was enabled, the fallback client called the
mock on *any* primary-provider error. A non-retryable error such as "no
coordinates found for city=…" (an unknown city) was masked by mock weather, so a
job for a nonexistent city completed as `completed` with fabricated data
(`Partly Cloudy / 21°C`). This also rendered the retry classifier in
`fetchWeatherWithRetry` dead whenever fallback was enabled, since `Fetch` never
surfaced an error.

**Fix:** The fallback now only masks **transient** (retryable) failures. A
non-retryable error from the primary provider propagates unchanged, so invalid
input fails the job instead of returning fabricated weather.

### 2. Data race / double-close on the RabbitMQ progress channel

**Where:** `worker/main.go` — `reopenProgressChannel`, `run`, `close`

**Problem:** The progress-responder goroutine reassigned `w.rabbitChan` on every
reconnect with no synchronization, while shutdown's deferred `close()` read and
closed the same field from another goroutine. `run()` did not wait for the
responder goroutine to exit before returning, so `close()` could race the
reassignment (a data race under `-race`) and double-close a channel.

**Fix:** `run()` now tracks the responder goroutine via a `progressDone` channel
and waits on it (bounded by the shutdown drain timeout, via
`awaitProgressResponder`) after cancelling services and before returning. This
guarantees the responder has stopped before the deferred `close()` touches
`w.rabbitChan`, eliminating the race without a lock.

### 3. Progress-request requeue storm on transient dependency failures

**Where:** `worker/main.go` — progress responder consume loop

**Problem:** When building a progress reply failed (e.g. a transient Redis error),
the delivery was `Nack`'d with `requeue=true` and immediately redelivered by the
broker. With no backoff this became a tight redelivery loop that hammered Redis
and saturated CPU until the dependency recovered.

**Fix:** Added a configurable backoff (`RABBITMQ_PROGRESS_REQUEUE_BACKOFF`,
default 500ms) before a requeued message can be redelivered. The wait is
context-aware so it does not delay shutdown.

### 4. Non-atomic Redis status write could leak TTL-less keys

**Where:** `api/main.go` — `writeQueuedStatus`, `writeFailedEnqueueStatus`

**Problem:** Status was written with a separate `HSET` then `EXPIRE`. If `EXPIRE`
failed after `HSET` succeeded (transient error or an elapsed context deadline
between the two round trips), the status hash was persisted permanently with no
TTL. Repeated partial failures would accumulate un-expiring keys and grow Redis
memory unbounded.

**Fix:** Both writes now use a `TxPipeline` (MULTI/EXEC) so the `HSET` and
`EXPIRE` either both apply or neither does.

### 5. GraphQL subscription panicked when the worker gRPC client was absent

**Where:** `api/graphql.go` — `runProgressSubscription`

**Problem:** The query path (`getJobStatusViaGRPC`) guarded against a nil
`workerGRPC` client and returned a clean error, but the subscription path called
`workerGRPC.SubscribeJobProgress` with no such guard, panicking the streaming
goroutine when the client was not configured.

**Fix:** The subscription path now performs the same nil check and emits a
GraphQL error + `complete` frame instead of panicking.

## UI

### 6. Apollo client rebuilt on every keystroke in the API URL field

**Where:** `ui/src/App.jsx`

**Problem:** The Apollo client and its WebSocket were derived directly from the
API base URL input. Each keystroke rebuilt the client and disposed the previous
one, tearing down any in-flight subscription and thrashing WebSocket
reconnects.

**Fix:** The URL that drives the Apollo client is now debounced (400ms), so the
client is only rebuilt after the user stops typing.

### 7. Stale subscription frame could overwrite the tracked job's status

**Where:** `ui/src/App.jsx`

**Problem:** The subscription effect wrote any incoming `jobProgress` into the
displayed status without checking it belonged to the currently tracked job. After
switching the tracked job id, a late/buffered frame for the previous job could
overwrite the new job's status (cross-job state bleed).

**Fix:** The effect now applies a frame only when `progress.jobId === liveJobId`.

## Maintainability

### 8. Dead RabbitMQ request-reply status path in the API

**Where:** `api/main.go`

**Problem:** `handleSubmitJob`, `handleJobStatus`, `requestJobStatus`, and
`parseJobStatusPath` (plus their sentinel errors, the `progressCheckRequest`
type, and `submitJobResponse`) were never registered in `routes()` — the live
path uses GraphQL + gRPC. ~180 lines of unreachable code, including a full
RabbitMQ request-reply client, had to be kept compiling and in sync with no
caller.

**Fix:** Removed the unreachable handlers and their now-orphaned types/errors and
the corresponding test. Shared helpers still used by the GraphQL path
(`enqueueJob`, `validateSubmitJobRequest`, `writeQueuedStatus`, `decodeJSON`,
`progressCheckReply`) were retained.

### 9. Duplicated retry/backoff loop in the worker

**Where:** `worker/main.go` — `fetchWeatherWithRetry`

**Problem:** `fetchWeatherWithRetry` hand-rolled the same bounded
retry/backoff/classification loop that `retryOperation` already implemented for
status and result writes, so retry semantics lived in two divergent places.

**Fix:** `retryOperation` now accepts an `isRetryable func(error) bool`
classifier; `fetchWeatherWithRetry` delegates to it with
`isRetryableWeatherError`, and the existing status/result callers pass
`isRetryableWorkerStoreError`. One retry implementation, two classifiers.

### 10. Background service goroutines logged after shutdown (test data race)

**Where:** `worker/main.go` — `run`, `startGRPCServer`, `startMetricsServer`

**Problem:** `run()` started the gRPC and metrics servers in goroutines tied to
`serviceCtx`, but returned as soon as the Kafka loop drained without waiting for
those goroutines to finish stopping. Their shutdown/serve goroutines kept logging
through `w.logger` after `run()` returned. In `TestRunDrainsInFlightJobOnShutdown`
the logger is backed by `t.Log`, so a log after the test returned tripped
`go test -race` (a real but test-only symptom of an unsynchronized shutdown).

**Fix:** `startGRPCServer` and `startMetricsServer` now return a `done` channel
that closes once both their goroutines have exited. `run()` waits for the
progress responder, gRPC, and metrics services (`awaitServiceShutdown`, bounded by
the drain timeout) after cancelling them and before returning, so no background
goroutine logs or touches shared state after `run()` completes. `go test -race`
is now clean across repeated runs.

## Investigated and intentionally left unchanged

### In-flight job context on shutdown is decoupled from fetch cancellation

**Where:** `worker/main.go` — `processFetchedMessage`

A review candidate flagged that the per-job `processCtx` is derived from
`context.Background()` rather than the loop context. This is **intentional**: on
shutdown `cancelFetch()` stops fetching new messages, but the in-flight job must
be allowed to finish within `waitForKafkaLoopDrain` so its result is committed
(graceful drain). Deriving `processCtx` from the loop context would cancel the
in-flight job immediately and defeat the drain — a behavior asserted by
`TestRunDrainsInFlightJobOnShutdown`. The drain timeout already bounds how long
shutdown waits. A clarifying comment was added at the call site.

## Verification

All Go modules build, `go vet` cleanly, and pass `go test` and `go test -race`
(the worker race suite was run repeatedly). The UI (`npm run build`) builds
successfully.
