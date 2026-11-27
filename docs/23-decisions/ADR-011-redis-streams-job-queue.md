# ADR-011: Use Redis Streams as the Async Job Queue (Not Kafka, RabbitMQ, or SQS)

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Product Owner, Architect (MODE A)
- **Refs:** `docs/05-architecture/LOW_LEVEL_DESIGN.md`, `docs/05-architecture/SEQUENCE_DIAGRAMS.md` (`SD-3`), `ADR-003`, `ADR-013`

## Context

When Admin uploads an evidence PDF or scanned image (`POST /api/v1/admin/cases/{id}/evidence`), `backend-go` must hand off parsing, SHA-256 verification, and OCR to the isolated Rust worker (`processor-rust`) asynchronously so HTTP requests do not time out and untrusted documents cannot crash the API server. Redis 7 (`7.0.15`) is already provisioned in the local sandbox (`Makefile`) and planned for rate limiting, idempotency keys, and Admin JWT revocation.

## Problem

We need durable-enough at-least-once job delivery, consumer-group acknowledgment, crash recovery (`XAUTOCLAIM`), and dead-letter queue (DLQ) semantics between Go and Rust without adding another heavy broker daemon, JVM dependency, or cloud-vendor lock-in inside a `$300–$500` solo-founder budget.

## Alternatives

1. **Rejected — Apache Kafka (or Redpanda):**
   - *Reasoning:* Massive operational and RAM overhead (multi-GB footprint, broker/topic/partition management) designed for millions of events per second; completely disproportionate for dozens to hundreds of case evidence uploads per month.
2. **Rejected — RabbitMQ (AMQP):**
   - *Reasoning:* Requires running and monitoring an Erlang VM daemon in addition to Redis (which is already required for rate limiting and JWT revocation), increasing VPS RAM usage and local non-Docker sandbox setup complexity (`ADR-003`).
3. **Rejected — AWS SQS:**
   - *Reasoning:* Creates immediate cloud-provider lock-in (violating `ADR-013` provider deferral), adds local sandbox emulation friction (requiring LocalStack/Docker, blocked by `ADR-003`), and incurs external network latency/billing setup.
4. **Selected — Redis Streams (`evidence:jobs`, `evidence:results`, `evidence:dlq`) on the existing Redis 7 instance:**
   - *Reasoning:* Reuses an already-verified lightweight C binary (`Redis 7.0.15`), provides native consumer groups (`XREADGROUP`), explicit acknowledgments (`XACK`), crash recovery (`XAUTOCLAIM`), bounded memory (`MAXLEN ~`), and zero vendor lock-in.

## Selected

1. Use three Redis Streams keys: `evidence:jobs` (Go → Rust), `evidence:results` (Rust → Go), and `evidence:dlq` (poison/failed jobs after 3 exponential retries).
2. Enable Redis AOF (`appendonly yes`, `appendfsync everysec`) in staging and production for crash durability.
3. Enforce idempotency by `job_id` stored on `evidence_items.last_processed_job_id` so at-least-once redeliveries are strictly safe.
4. Failure mode: If Redis is unavailable, new evidence uploads fail closed (`HTTP 503`) rather than losing extraction jobs; pending jobs survive restart via AOF and are reclaimed via `XAUTOCLAIM`.

## Rationale

At our launch volume (10 cases × ~10–30 documents = ~100–300 extraction jobs total, scaling to ~1,500 at 50 cases), Redis Streams delivers rock-solid at-least-once semantics using `< 50 MB` RAM and zero extra infrastructure components.

## Trade-offs

- Redis is primarily in-memory; an ungraceful kernel crash with `appendfsync everysec` could lose up to 1 second of unacknowledged stream writes (mitigated because PostgreSQL holds `evidence_items.processing_status = 'PENDING'`, allowing Go/Admin to detect and re-enqueue any orphaned `PENDING` item older than 5 minutes).
- No built-in delayed-message scheduling (handled simply in `processor-rust` via bounded exponential backoff timers before re-enqueueing attempts 2 and 3).

## Risks

- Unbounded stream growth if `MAXLEN` trimming is omitted.
- **Mitigation:** Every `XADD` passes `MAXLEN ~ 10000` (and `5000` for `evidence:dlq`), and Go enforces a backpressure admission cap (`XLEN evidence:jobs <= 500`).

## Consequences

- `backend-go/internal/queue` and `processor-rust/queue-consumer` depend solely on standard RESP Redis client libraries.
- Zero additional broker services needed in `Makefile` or GitHub Actions CI (existing Redis 7 service container suffices).

## Reversal Strategy

Because both Go (`internal/queue`) and Rust (`queue-consumer`) wrap stream operations behind narrow producer/consumer interfaces, swapping Redis Streams for PostgreSQL `SKIP LOCKED`, NATS JetStream, or SQS later requires replacing only those adapter modules without touching domain models or parsers.
