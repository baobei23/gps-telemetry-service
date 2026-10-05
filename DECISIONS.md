# Architecture Decision Records (ADR)

This document records the foundational architectural decisions made for the Real-Time GPS Telemetry Ingestion Service.

---

## ADR-001: Keyset Cursor Pagination for High-Throughput Telemetry History

### Status
Accepted

### Context
GPS devices transmit telemetry continuous updates (at least 100 reports/second). Users query position history using time ranges (`from`, `to`) and page limits (`limit`).

Traditional pagination (`OFFSET` and `LIMIT`):
1. **Performance Degradation**: At scale (millions of rows), `OFFSET N` requires PostgreSQL to scan and discard `N` rows from disk before returning the target page ($O(N)$ execution time).
2. **Data Drift & Duplication**: When new GPS coordinates are ingested continuously at the head of the table, an `OFFSET` window shifts downward. Clients traversing pages encounter duplicate records or skip positions entirely.

### Decision
Implement **Keyset Cursor Pagination** ordered deterministically by `device_time DESC`:
- The client receives an opaque base64 cursor token encoding the last seen `device_time` in nanoseconds.
- For the next page, the server decodes the timestamp and queries `WHERE device_id = $1 AND device_time < $cursor_time ORDER BY device_time DESC LIMIT limit + 1`.
- The extra row (`limit + 1`) is peeked to determine if a subsequent page exists without issuing an expensive `COUNT(*)` query.

### Consequences
- **Positive**:
  - $O(1)$ query complexity: Directly seeks the B-Tree index using `Index Scan Backward` on `uq_positions_device_time`.
  - Zero in-memory sorting: Query latency on 8.64 million rows drops from `11.7 ms` to `0.1 ms`.
  - Stability: Zero data drift, zero missed records, and zero duplicates even under heavy concurrent write load.
- **Negative**:
  - Clients cannot jump directly to an arbitrary page number (e.g., "Go to page 40"); traversal is strictly sequential forward.

---

## ADR-002: Atomic Single Round-Trip CTE for Ingestion and Idempotency

### Status
Accepted

### Context
Each OsmAnd HTTP GET report updates the `devices` table (first seen, last seen) and inserts a new row into `positions`.
- Executing multiple sequential SQL statements across network roundtrips exhausts the database connection pool under concurrent traffic and risks partial writes.
- Network retries from GPS devices can re-transmit identical timestamps.
- Out-of-order packet delivery can cause older timestamps to overwrite a device's current `last_seen_at`.

### Decision
Execute both operations in a **single Common Table Expression (CTE)** query:
```sql
WITH ins_device AS (
    INSERT INTO devices (id, first_seen_at, last_seen_at)
    VALUES ($1, $2, $2)
    ON CONFLICT (id) DO UPDATE SET
        last_seen_at = GREATEST(devices.last_seen_at, EXCLUDED.last_seen_at),
        first_seen_at = LEAST(devices.first_seen_at, EXCLUDED.first_seen_at),
        updated_at = NOW()
)
INSERT INTO positions (
    device_id, device_time, server_time, lat, lon, valid,
    speed, bearing, altitude, accuracy, hdop, batt, charge
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (device_id, device_time) DO NOTHING;
```

### Consequences
- **Positive**:
  - Exactly one network roundtrip per ingestion report, maximizing database pool efficiency.
  - Strict idempotency: Duplicate `(device_id, device_time)` pairs are safely discarded by the PostgreSQL storage engine.
  - Safe out-of-order handling: `GREATEST` and `LEAST` functions guarantee monotonic progression of device visibility timestamps regardless of packet delivery sequence.
- **Negative**:
  - High concurrency updates to the same `devices` row may encounter row-level lock contention if a single device sends dozens of bursts per second (mitigated by real-world 1s intervals).

---

## ADR-003: Synchronous Durability Boundary vs Asynchronous Message Queues

### Status
Accepted

### Context
The service must reliably handle at least 100 requests/second from 100 active devices on developer hardware. We needed to choose between:
1. Direct synchronous writes to PostgreSQL with immediate `200 OK`.
2. Asynchronous write buffering using a message broker (e.g., Kafka / RabbitMQ) or in-memory worker pools.

### Decision
Adopt **Direct Synchronous Persistence** as the durability boundary:
- A `200 OK` response is sent to the GPS device only after the CTE commit completes in PostgreSQL.
- If the database is unreachable or query fails, an HTTP `503 Service Unavailable` or `500 Internal Server Error` is returned immediately.
- Adhere to YAGNI: Avoid adding external queue dependencies for workloads of ~100 req/s, which PostgreSQL natively handles with negligible CPU/disk overhead.

### Consequences
- **Positive**:
  - Guaranteed durability: No acknowledged position can be lost in volatile memory buffers.
  - Immediate Read-After-Write Consistency: Newly ingested coordinates appear instantly in `/api/devices/{id}/latest`.
  - Minimal operational footprint: Only requires Go runtime and PostgreSQL (no Kafka, ZooKeeper, or Redis overhead).
- **Negative**:
  - Ingestion throughput is bounded by PostgreSQL write latency and connection pool size (configured to 64 connections). If scale increases to 10,000+ devices (10,000 req/s), an asynchronous streaming tier (Kafka + TimescaleDB hypertable bulk copy) will be required.

### Dead-Letter Handling Considerations
- **Why full pubsub DLQ is omitted**:
  In a synchronous ingestion design, validation failures are immediately rejected with descriptive `400 Bad Request` responses, providing direct feedback to the sender. During database outages, returning `503 Service Unavailable` signals tracking devices to leverage their built-in local flash storage and network retry logic. Introducing an external pubsub broker (e.g., Kafka / RabbitMQ DLQ) solely for dead-letter queuing adds unwarranted operational complexity and failure points for a 100 req/s baseline.
- **Pragmatic evolutionary path**:
  If offline persistence during prolonged database downtime is desired, an **append-only disk spillover log (`deadletter.log`)** is preferred over distributed pubsub. A local sequential file write requires zero network dependencies, survives service restarts, and can be replayed cleanly via a lightweight background worker once PostgreSQL connectivity is restored.
