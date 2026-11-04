# playHLD black-box evaluation

Date: 2026-11-02 (Asia/Calcutta). Surface: local Chrome at `http://127.0.0.1:5173/`, 1440 × 900, with the local Go API on port 8080. Testing used browser UI interactions through Playwright/CDP. The integrated browser host was unavailable (`The browser access host is unavailable`). No application source was read or changed. Authentication and Contest were excluded as requested. The API used no database connection; Dev account projects were therefore outside scope.

## Executive findings

The canvas can build all five requested diagrams and the fixed-run engine produced coherent capacity, queue, cache, and latency measurements. Capacity calculations in the controlled cases below agreed with the observed throughput and backlogs to within run-boundary rounding. I found **one confirmed misleading-results defect** and several important **modeling gaps**. I found no confirmed mathematical defect.

| Priority | ID | Type | Finding | Severity | Confidence |
| --- | --- | --- | --- | --- | --- |
| 1 | BB-01 | Defect | Completed metrics remain displayed after the current design changes | High | Confirmed |
| 2 | BB-02 | Capability gap | API services and workers cannot branch to two downstream systems | High | Confirmed |
| 3 | BB-03 | Capability gap | Queue and worker path has no asynchronous acknowledgment model | High | Confirmed |
| 4 | BB-04 | Capability gap | No failure injection or propagation controls | Medium | Confirmed |
| 5 | BB-05 | Reproducibility gap | Architecture JSON import does not restore the workload used for its run | Medium | Confirmed |

## Issues

### BB-01 — Completed metrics remain displayed after design edits

- **Severity/type:** High; incorrect presentation of prior results as current, not a numerical engine error.
- **Affected feature:** Dev fixed-run metrics and design editing.
- **Architecture/configuration:** Two-worker data pipeline, even 200 req/s for 2 seconds; both workers initially 80 ms service time. One worker was then changed to 100 ms.
- **Reproduction:** (1) Capture a fixed run. (2) Observe `Completed`, throughput 201 req/s and P95/P99 106.3 ms. (3) Select `Live worker 2` and change `Service time (ms)` from 80 to 100 without running again. (4) Read the metrics panel. (5) Capture again.
- **Expected:** The prior run should be labeled as belonging to the previous configuration, or the metrics should be cleared/marked stale after the edit.
- **Actual:** The panel still said `Completed` and showed 201 req/s, P95/P99 106.3 ms after the edit. A new capture changed the result to 200 req/s and P95/P99 126.3 ms. Topology edits also retained previous metrics: [the judge graph screenshot](evidence/01-online-judge.png) shows metrics from the earlier default graph before the judge graph was captured.
- **Evidence:** [Browser observation log](evidence/09-stale-metrics.json), [judge graph screenshot](evidence/01-online-judge.png).
- **Confidence/reproduced:** Confirmed; reproduced with both parameter and topology edits.

### BB-02 — Ordinary service nodes cannot branch to two downstream systems

- **Severity/type:** High; missing diagramming and routing capability.
- **Affected feature:** Connection rules for API service and worker nodes.
- **Architecture/configuration:** Online judge: `Judge API → Submission cache` and attempted `Judge API → Job queue`; `Judge worker → Artifact store` and attempted `Judge worker → Submission DB`.
- **Reproduction:** (1) Add and connect the first outgoing route from an API or worker. (2) Drag its source port to a second downstream component.
- **Expected:** A system design canvas should allow an API to enqueue a submission and access a cache, and a worker to write both an artifact and job metadata. The simulator could then request a routing/fanout policy where needed.
- **Actual:** The second connection was refused with `This component already has an outgoing route.` The judge required a second API behind the load balancer to draw two separate paths. This changes the meaning: traffic is split between paths rather than one submission triggering both. Load balancers and message queues did accept multiple outgoing routes, so the restriction is node-specific.
- **Evidence:** [Judge screenshot with refusal notice](evidence/01-online-judge.png), [exported partial judge graph](evidence/01-online-judge.json).
- **Confidence/reproduced:** Confirmed; reproduced on API and worker. Classified as a capability gap rather than a simulation arithmetic bug.

### BB-03 — Queue path has no asynchronous acknowledgment model

- **Severity/type:** High; missing simulation semantics for asynchronous work.
- **Affected feature:** Message queue and worker simulation.
- **Architecture/configuration:** `Client → Ingest LB → Ingest API → Event queue → Batch worker → Data lake`; 100 req/s, 2 seconds, even spacing, 10 worker slots, no queue backlog.
- **Reproduction:** (1) Set worker service time to 50 ms and capture: 100 req/s, P95/P99 76.3 ms. (2) Change only worker service time to 80 ms and capture: 100 req/s, P95/P99 106.3 ms.
- **Expected:** For an asynchronous enqueue-and-ack request, API response latency should end at enqueue; downstream worker time should affect job completion latency separately. A synchronous mode can also be useful if explicitly selected.
- **Actual:** End-to-end request latency increased exactly 30 ms with worker service time. The values equal the sum of service times through the worker and storage. There is no visible setting for enqueue acknowledgment, independent job completion, delivery delay, retry, or dead-letter behavior.
- **Evidence:** [One-worker pipeline export](evidence/04-data-pipeline-one-worker-experiment.json); exact observations and calculations in [SIMULATION_EXPERIMENTS.md](SIMULATION_EXPERIMENTS.md).
- **Confidence/reproduced:** Confirmed for current synchronous path behavior; reproduced. This is a missing mode, not proof that the current arithmetic is wrong.

### BB-04 — Failure propagation cannot be exercised

- **Severity/type:** Medium; missing simulation capability.
- **Affected feature:** All component settings and simulation input.
- **Architecture/configuration:** The eight available component types in Dev and the Practice problem inspector.
- **Reproduction:** Inspect each component’s settings and the workload controls. Look for failure rate, outage, timeout, retry, circuit breaker, or dependency failure controls; run a normal capture to inspect available outputs.
- **Expected:** A distributed-system evaluation can inject a component or dependency failure and observe rejected requests and downstream effects.
- **Actual:** No failure controls were exposed. Rejection from capacity/queue limits is modeled, but an outage and its propagation could not be configured or simulated.
- **Evidence:** [COMPONENT_INVENTORY.md](COMPONENT_INVENTORY.md) records every observed control; architecture screenshots in [evidence/](evidence/).
- **Confidence/reproduced:** Confirmed UI capability absence across the inspected controls; reproduced. It is not an incorrect existing feature.

### BB-05 — Exported architecture does not reproduce its workload

- **Severity/type:** Medium; reproducibility and usability gap.
- **Affected feature:** Dev `Export JSON` / `Import JSON` and workload state.
- **Architecture/configuration:** One-worker pipeline exported while the arrival rate was 300 req/s and pattern length 2 seconds. Later the Dev workload was set to an invalid `-1` req/s.
- **Reproduction:** (1) Export the pipeline JSON with 300 req/s. (2) Change the current arrival rate to `-1`. (3) Import the exported graph. (4) Inspect workload fields before running.
- **Expected:** A shareable experiment should restore, or clearly distinguish, the workload that produced the exported result.
- **Actual:** The graph was restored but the arrival field remained `-1`; attempting a run showed `Enter a valid rate, duration, read percentage, and decimal seed before running.` The architecture export itself may intentionally contain only the graph, but the UI offers no complete experiment export.
- **Evidence:** [One-worker architecture export](evidence/04-data-pipeline-one-worker-experiment.json) and the browser observations in [SIMULATION_EXPERIMENTS.md](SIMULATION_EXPERIMENTS.md).
- **Confidence/reproduced:** Confirmed behavior; reproduced. Classified as a reproducibility gap, not a data corruption bug.

## Architecture challenges

All graphs below were made by adding, naming, moving, and connecting nodes in the browser. Each was exported through the app’s `Export JSON` button. Screenshots show the diagrams; exported JSON preserves the actual canvas graph. The diagrams do not imply that the simulator supports every real-world behavior represented by their labels.

| Architecture | Built in browser | Fixed-run observation | What the model could not express |
| --- | --- | --- | --- |
| Online coding judge | 9 components, 8 routes: client → LB → two API paths; one cache → DB path, one queue → worker → artifact path. [Screenshot](evidence/01-online-judge.png) · [JSON](evidence/01-online-judge.json) | 500 req/s, 30 s: 498 req/s throughput, P95/P99 36.3 ms, queue 0, rejected 0. | One API cannot both use cache and enqueue; worker cannot write DB and artifact store; no code execution/sandbox or async job completion. The two API paths split requests rather than modeling one submission lifecycle. |
| URL shortener | 6 components, 5 routes: client → CDN → LB → API → cache → mapping DB. Later added a second API behind the LB. [Screenshot](evidence/02-url-shortener.png) · [JSON](evidence/02-url-shortener.json) | 500 req/s, 2 s with CDN 0% hit, URL cache 80% hit, DB 50 ms: 484 req/s, P95/P99 71.1 ms, rejected 0. | No short-code keyspace, collisions, persistent mappings, write/read routing, expiration, or invalidation. Cache hit rate is configured, not generated from a key distribution. |
| Real-time chat | 8 components, 8 routes: LB splits message API → queue → delivery worker → DB and presence API → cache → DB. [Screenshot](evidence/03-chat.png) · [JSON](evidence/03-chat.json) | 300 req/s, 2 s: 299 req/s, P95/P99 36.3 ms, queue 0, rejected 0. | No bidirectional connections, rooms, per-recipient fanout, pub/sub, presence expiry, or delivery acknowledgments. The two branches receive split traffic, not a single message producing both effects. |
| Distributed data processing | Client → LB → API → queue → one/two workers → object storage; queue fanout to two workers did work. [Screenshot](evidence/04-data-pipeline-two-workers.png) · [two-worker JSON](evidence/04-data-pipeline-two-workers.json) | At 300 req/s and 50 ms workers: two workers delivered 300 req/s, P95/P99 76.3 ms, no backlog; one worker delivered 200 req/s, P95/P99 576.3 ms, queue 100, rejected 100. | Worker time is counted in request response latency; no batches, partitions, checkpoints, retry, or independent job-completion metric. |
| Original hybrid: telemetry ingest and dashboard | 10 components, 9 routes: client → CDN → LB, then ingest API → queue → aggregator → analytics DB or dashboard API → cache → report archive. [Screenshot](evidence/05-hybrid-analytics.png) · [JSON](evidence/05-hybrid-analytics.json) | 300 req/s, 2 s: 291 req/s, P95/P99 36.6 ms, queue 0, rejected 0. | No event-to-dashboard materialization, periodic aggregation, or relationship between data written on one branch and read on the other. |

## Other browser observations

- **Practice/Problems:** The catalog loaded after the API was started. The first challenge began with only a client and API server. A disconnected graph produced `MISSING_ROUTE`. With 1 slot at 3 ms and 800 req/s, `Check goals` showed 334 req/s, P95/P99 303 ms, queue 100, 4,567 rejected and 42.9% measured completion; all goals failed. Raising only processing slots to 3 yielded 801 req/s, P95/P99 3 ms, queue 0, rejected 0, and all goals passed. Resource limits, worked solution, loading the example, reset, and the side problem list worked. [Drawer screenshot](evidence/08-practice-problems-drawer.png).
- **Live editing:** A continuous run changed from about 299 to 595 req/s after arrival input changed 300 → 600. In another run, adding and connecting a second worker while running raised throughput from about 127 to 200 req/s, cleared current queue depth, and left the earlier cumulative rejection count visible. Pause, Resume, and Stop controls changed state correctly.
- **Canvas reliability:** Duplicate routes were refused without increasing route count. Route deletion, Undo, and Redo changed route count 5 → 4 → 5 → 4. A cycle was drawable but the fixed run rejected it with `CYCLE at architecture.edges: cycles are not supported`. An unreachable extra worker yielded `UNREACHABLE_NODE`. A Practice slot value above its per-component cap was refused with `Limit: 4 processing slots per component.`
- **Usability:** The browser-visible validation errors are precise but use engine terms (`MISSING_ROUTE`, `UNREACHABLE_NODE`, `CYCLE`). The weighted LB policy was visible; per-route weight editing was not verified because the targeted edge could not be selected reliably in the automation session. No defect is asserted for weights.

## Limits of this evaluation

Failure propagation, asynchronous jobs, real-time connection behavior, cache invalidation, and DB replication could not be tested because no corresponding browser controls were found. Authentication, account-backed projects, and Contest were intentionally excluded. The initial Problems-page load failure occurred while the local API was stopped; it disappeared after starting the API and is an environment setup result, not a reported application defect. No destructive load test or source inspection was performed. The exact Practice completion-percentage denominator remains an open mathematical question; see [ASTRA_HANDOFF.md](ASTRA_HANDOFF.md).
