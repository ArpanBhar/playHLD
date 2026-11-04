# Browser-visible component inventory

Observed in Dev at 1440 × 900 on 2026-11-02. All components were added from the left palette and selected in the browser. The values below are **new-instance defaults**, not implementation constants. Practice problems can override or lock them.

## Palette and per-instance controls

Every non-source instance had: Name; Processing slots; Max in-flight requests; Service time (ms); Service time model (`Constant`, `Exponential`); Queue limit; Delete component. New instances used 10 processing slots, 110 max in-flight requests, a 100-request queue, and a constant service-time model. The inspector describes processing busy as occupied slots, not CPU use, and says max in-flight affects admission rather than processing capacity. There is no replica-count field; adding another connected box creates another instance.

| Component | Default service time | Extra observed controls | Observed health/behavior |
| --- | ---: | --- | --- |
| Load balancer | 0.3 ms | Routing policy: `Round robin`, `Weighted` | Processing busy, queue, rejected. Accepted multiple outgoing routes to API instances. Per-route weight editing was not verified. |
| API service | 20 ms | None | Processing busy, queue, rejected. A second outgoing route was refused with `This component already has an outgoing route.` |
| Worker | 10 ms | None | Processing busy, queue, rejected. A second outgoing route was refused. Worker service time contributed to end-to-end request latency. |
| Cache | 0.5 ms | Read hit rate (%), default 80 | Processing busy, queue, rejected, hit rate, miss rate. The output route was labeled `cache miss`. Changing 80% → 0% hit changed DB load as expected. |
| CDN | 0.3 ms | Read hit rate (%), default 80 | Same hit/miss display and `cache miss` output pattern as cache. |
| Database | 5 ms | Read time override (ms), Write time override (ms), both default 0 | Processing busy, queue, rejected. Two separate upstream branches connected to one DB. Slowing DB caused a queue and rejection bottleneck. |
| Object storage | 5 ms | Read time override (ms), Write time override (ms), both default 0 | Processing busy, queue, rejected. Used as a terminal sink in tested diagrams. |
| Message queue | 1 ms | None beyond shared processing/queue settings | Processing busy, queue, rejected. Accepted outgoing routes to two workers; backlog and worker scaling were observable. Queue-to-worker remains on the same request-latency path. |

**Clients/source:** Required source node with an outgoing port. Dev workload fields: Traffic profile (`Steady`, `Ramp up`, `Middle burst`), Arrival pattern (`Even spacing`, `Poisson (seeded)`), Arrival rate (req/s), Pattern length (seconds), Read traffic (%), and Seed. The source had exactly one required `always` route; a disconnected Practice source produced `MISSING_ROUTE at architecture.edges: source requires exactly one always edge`. The default Dev workload displayed 500 req/s, 30 seconds, 100% reads, seed 42. Live traffic repeats the configured pattern until stopped; `Capture run` uses the pattern length.

## Connection and canvas semantics observed

| Operation | Browser result |
| --- | --- |
| Add component | Palette button adds one box and selects it; the inspector exposes its settings. |
| Move node | Dragging moves its box and connected edges. `Fit View` recenters the graph. |
| Connect ports | Drag source handle to target handle; route count and edge appear. |
| Duplicate route | Refused with `These components are already connected.` Route count stayed at 5. |
| Multiple LB outputs | Allowed; one LB connected to two APIs. At 800 req/s, two 500 req/s API instances served all traffic. |
| Multiple queue outputs | Allowed; one queue connected to two workers, each feeding object storage. |
| Multiple API/worker outputs | Refused after the first outgoing route. This blocked a single API from both cache access and enqueue, and a worker from DB plus artifact writes. |
| Multiple incoming routes | Allowed in tested graphs: two API paths fed one cache; two worker paths fed one storage node; two chat branches fed one DB. |
| Cache/CDN miss route | Output edge is labeled `cache miss`. The cache hit-rate setting determined the downstream DB demand in controlled runs. This supports the inference that read hits finish at the cache/CDN. |
| Delete edge | Select an edge, then use `Delete connection`; Undo/Redo restored and removed it again. |
| Delete node | Inspector `Delete component` removed optional nodes and their connected routes. Required Practice client/API did not show deletion. |
| Disconnected node | Fixed run rejected an unreachable worker with `UNREACHABLE_NODE at architecture.nodes: all nodes must be reachable from entry`. |
| Cycle | UI allowed a worker → upstream API route after the worker's previous output was removed; fixed run rejected `CYCLE at architecture.edges: cycles are not supported`. |
| Import/export | `Export JSON` downloaded an architecture file. `Import JSON` restored nodes/routes. Workload fields persisted independently; the imported file did not restore the previous run's traffic input. |

## Simulation outputs and UI capability

- Fixed run: Throughput, P95, P99, max queue depth, rejected requests, a chart of offered versus completed requests, and component health. A continuous run supports pause, resume, stop, and live configuration/topology edits.
- Cache/CDN inspector: hit and miss rates, plus processing busy, queue, rejected. Processing nodes: processing busy, queue, rejected. Canvas color indicates high busy or cache miss; specific visual thresholds were not independently calibrated in this evaluation.
- Distinct component boxes and processing slots permit explicit capacity modeling. Queue fanout to workers and load balancing across APIs were actually simulated.
- There were no observed controls for async acknowledgment, job-completion latency separate from request latency, key distributions, cache invalidation, retries, timeouts, fault injection, outage schedules, partitioning, replication, WebSocket sessions, pub/sub delivery, or data consistency. These are absent simulation capabilities, not broken configured behaviors.

## Practice-mode constraints observed

The first problem started with Clients and a required API server, 0 routes, an 800 req/s fixed workload for 10 seconds, 100% reads, seed 42, and a resource budget of 6 component instances, 9 total processing slots, at most 4 slots per component. The API server started with 1 slot and fixed 3 ms service time. The inspector disabled service time, service model, and queue limit; it allowed processing slots and max in-flight requests. Entering 5 slots was refused and reverted to 3 with a `Limit: 4 processing slots per component.` notice. The Resources tab listed per-component allowances; the Solution tab gave one load-balancer/two-API design with a `Load example` action. `Reset design` restored the original client and API with no route.
