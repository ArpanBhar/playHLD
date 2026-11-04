# Controlled browser simulation experiments

All measurements below came from the app's browser UI. `Capture run` used the displayed workload and generated the reported metrics. Except where noted, the arrival pattern was even, read traffic 100%, and seed 42. `Processing slots` is the app's provisioned parallel capacity, not measured CPU. Capacity estimates use `slots × 1000 / service_ms` requests per second for a constant-time component; they are upper bounds before upstream/downstream effects. Small differences near the fixed-run boundary are not classified as defects.

## E-01 — Worker scale-out and queue backlog

Graph: client → LB → API → message queue → worker(s) → object storage. Arrival 300 req/s for 2 s. LB 10 slots × 0.3 ms; API 20 slots × 20 ms; queue 10 slots × 1 ms; each worker 10 slots × 50 ms; storage 10 slots × 5 ms. The only architectural change was one versus two identical, connected workers behind the queue.

| Worker count | Throughput | P95 / P99 | Max queue | Rejected |
| --- | ---: | ---: | ---: | ---: |
| 2 | 300 req/s | 76.3 / 76.3 ms | 0 | 0 |
| 1 | 200 req/s | 576.3 / 576.3 ms | 100 | 100 |

**Sanity check:** One worker can process `10 × 1000 / 50 = 200` req/s; two can process 400 req/s. The 300 req/s offer is below the two-worker cap and above the one-worker cap. Over 2 s, the one-worker case offers about 600 requests, can complete about 400, holds 100 in the queue, and rejects about 100. The observed 200 req/s, queue 100, and rejected 100 agree. The empty-path service sum is `0.3 + 20 + 1 + 50 + 5 = 76.3` ms. A full 100-request queue at 200 req/s adds up to 500 ms, matching the reported 576.3 ms tail. [One-worker export](evidence/04-data-pipeline-one-worker-experiment.json) · [two-worker export](evidence/04-data-pipeline-two-workers.json).

## E-02 — Cache hit-rate change at a slow database

Graph: client → CDN → LB → API → cache → DB. Arrival 500 req/s for 2 s. CDN read hit 0%; API 10 slots × 20 ms (500 req/s cap); URL cache 10 slots × 0.5 ms; DB 10 slots × 50 ms (200 req/s cap). Only the URL cache read-hit setting changed.

| URL cache hit | Throughput | P95 / P99 | Max queue | Rejected |
| --- | ---: | ---: | ---: | ---: |
| 80% | 484 req/s | 71.1 / 71.1 ms | 2 | 0 |
| 0% | 201 req/s | 571.1 / 571.1 ms | 100 | 500 |

**Sanity check:** At 80% hits, DB demand is roughly `500 × 0.2 = 100` req/s, below its 200 req/s cap. At 0% hits, DB demand is about 500 req/s, far above the cap. Across 2 s, roughly 1,000 requests arrive, about 400 can finish at the DB, 100 can queue, and the rest can be rejected. The reported 201 req/s, queue 100, and 500 rejected are consistent within run-boundary rounding. The DB-miss service sum is `0.3 CDN + 0.3 LB + 20 API + 0.5 cache + 50 DB = 71.1` ms; the tail rose by 500 ms when the DB queue filled. The cache inspector showed 80% hit / 20% miss and then 0% hit / 100% miss. [Zero-hit configuration](evidence/06-url-cache-zero-hit.json).

## E-03 — Database service-time change

The E-02 zero-hit graph and 500 req/s workload were held fixed. Only DB service time changed from 50 ms to 5 ms.

| DB service time | DB capacity estimate | Throughput | P95 / P99 | Max queue | Rejected |
| --- | ---: | ---: | ---: | ---: | ---: |
| 50 ms | 200 req/s | 201 req/s | 571.1 / 571.1 ms | 100 | 500 |
| 5 ms | 2,000 req/s | 501 req/s | 26.1 / 26.1 ms | 0 | 0 |

**Sanity check:** At 5 ms, DB capacity exceeds the 500 req/s offer and the empty-path service sum becomes `0.3 + 0.3 + 20 + 0.5 + 5 = 26.1` ms, exactly the displayed P95/P99. No mathematical defect was identified.

## E-04 — Load balancing across explicit API instances

Graph: client → CDN (0% hit) → LB → API instance(s) → URL cache (0% hit) → DB (5 ms). Arrival 800 req/s for 2 s. Each API has 10 slots × 20 ms = 500 req/s capacity. The architecture change was adding one equal API instance with LB → API and API → cache routes.

| API instances | Capacity estimate | Throughput | P95 / P99 | Max queue | Rejected |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1 | 500 req/s | 501 req/s | 226.1 / 226.1 ms | 100 | 500 |
| 2 | 1,000 req/s | 800 req/s | 26.1 / 26.1 ms | 0 | 0 |

**Sanity check:** One API is below the 800 req/s offer; in 2 s, about 1,600 are offered, 1,000 can complete, 100 can queue, and roughly 500 are rejected. Two APIs raise capacity above the offer. The observed metrics follow this prediction. A second load balancer was not added; the extra serving capacity came from the extra API instance. [Two-API export](evidence/07-url-two-api.json).

## E-05 — Worker time reveals queue response semantics

One-worker pipeline, 100 req/s for 2 s, even spacing, 10 worker slots, queue depth 0 in both captures. Only worker service time changed.

| Worker service | Worker capacity estimate | Throughput | P95 / P99 | Max queue |
| --- | ---: | ---: | ---: | ---: |
| 50 ms | 200 req/s | 100 req/s | 76.3 / 76.3 ms | 0 |
| 80 ms | 125 req/s | 100 req/s | 106.3 / 106.3 ms | 0 |

**Sanity check:** Both capacities exceed offered load, so no queue is needed. The P95/P99 increase of exactly 30 ms equals the worker service-time increase. The simulator is adding downstream worker time to the original request's latency. This is internally coherent for a synchronous path, and it demonstrates the absence of an asynchronous enqueue-and-ack mode.

## E-06 — Seeded arrival variation and repeatability

One-worker pipeline, 10 slots × 80 ms worker service, configured 100 req/s for 2 s. Only the arrival pattern/seed changed from the even case, and then the seed changed from 42 to 43. The two seed-42 captures had identical values.

| Arrival | Throughput | P95 | P99 | Max queue | Rejected |
| --- | ---: | ---: | ---: | ---: | ---: |
| Even | 100 req/s | 106.3 ms | 106.3 ms | 0 | 0 |
| Poisson, seed 42, first | 113 req/s | 154.0 ms | 162.8 ms | 8 | 0 |
| Poisson, seed 42, repeat | 113 req/s | 154.0 ms | 162.8 ms | 8 | 0 |
| Poisson, seed 43 | 97 req/s | 133.8 ms | 140.4 ms | 5 | 0 |

**Sanity check:** The configured rate is an expected rate for the Poisson arrival process, so a two-second realization need not equal exactly 100 req/s. Identical seed-42 captures support deterministic replay; changing the seed changed observed arrivals and tails. P99 exceeded P95 under bursty arrivals. No defect identified.

## E-07 — Practice goal capacity check

Problem `The launch traffic spike`: 800 req/s for 10 s, fixed API service 3 ms, queue limit 100, no extra components. The client was connected directly to the required API. Only API processing slots changed.

| API slots | Capacity estimate | Throughput | P95 / P99 | Max queue | Rejected | Goal result |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| 1 | 333.3 req/s | 334 req/s | 303 / 303 ms | 100 | 4,567 | 42.9% measured completion; 3 failed goals |
| 3 | 1,000 req/s | 801 req/s | 3 / 3 ms | 0 | 0 | 100% measured completion; 3 passed goals |

**Sanity check:** The throughput and latency response to slots agrees with `slots / 3 ms`. The `42.9%` completion figure is close to `(8000 − 4567) / 8000 = 42.91%`, i.e. the non-rejected fraction. The displayed throughput of 334 req/s implies about 3,340 completions in 10 s, a lower fraction. The simulator may drain accepted queued work after the arrival window, or the throughput may be an interval rate. Without a browser-visible total-completed count or precise reporting-window definition, this is an **unverified metric-definition question**, not a declared mathematical defect.

## E-08 — Continuous simulation reacts to edits

- On the hybrid graph, a running live simulation at 300 req/s displayed about 299 req/s. Changing only the arrival field to 600 while running yielded about 595 req/s; `Pause simulation`, `Resume simulation`, and `Stop simulation` changed the toolbar and status as expected.
- On the pipeline, a live 200 req/s offer with one 80 ms/10-slot worker showed about 127 req/s, queue 100, and accumulating rejections. Adding, configuring, and connecting a second 80 ms worker without stopping the run later showed 200 req/s, P95/P99 106.3 ms, current queue 0. The rejected counter retained 552 earlier rejections, consistent with a cumulative run total.

These are live observations, separate from the fixed-run comparisons above. The temporary partially connected state was not classified as a defect.

## Experiment limits

Fixed-run output is rounded; the UI does not show exact total offered/completed counts for all runs. Failure and retry experiments were unavailable because no browser controls were found. Cache keys, invalidation, DB replication, bidirectional sessions, and asynchronous job completion were not modeled. No conclusions about those unavailable mechanisms are drawn from the numeric runs.
