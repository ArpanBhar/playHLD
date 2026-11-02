# Modeling semantics

playHLD simulates synthetic requests against the configured graph. The numbers are model estimates, not measurements of a deployed system.

## Diagram and capacity

Architecture schema version 3 uses one node per component instance. The Clients node is the workload generator, not a deployed server. Every other node has processing slots, a maximum in-flight request count, service time, and a queue limit. Processing slots represent provisioned parallel work capacity and consume the problem's slot budget. The in-flight limit counts both processing and waiting requests; raising it alone can reduce rejection or increase the queue, but cannot make the instance process requests faster. Raising processing slots or adding another connected instance can increase capacity. A load balancer routes across its connected targets by round robin or weights. A message queue dispatches to its connected consumers in round-robin order without a separate load balancer.

A cache or CDN serves read hits at that node unless a hit route is drawn; misses must follow a miss route. Database and object-storage nodes can use different read and write service times. Worker nodes represent one job processor each. The queue is a simplified processing and buffering stage: end-to-end job latency includes downstream worker and storage time. The model does not yet simulate durable acknowledgements, retries, cache coherence, data replication, health checks, or failures. Multiple database nodes are separate capacity in the graph, not automatically synchronized copies.

## Live preview

The toolbar's Run simulation control starts a live preview paced against wall-clock time. Snapshots arrive about once a second, and the preview continues until Pause or Stop. The pattern length in the Dev inspector controls a repeating bounded traffic window; it does not stop the live session. Problems use their published workload pattern. The worker keeps recent snapshots for a rolling chart. P95 and P99 latency are shown from the most recently completed window.

The engine accepts valid topology additions, new connections, and component setting changes in place. Requests already queued or in flight on existing components remain in the engine. A disconnected component cannot receive traffic; the previous valid graph continues running, and the UI shows a pending update warning until the graph becomes valid. A workload change or component removal starts a fresh bounded window, which discards its outstanding requests. Windows also restart at their configured pattern boundary, so queue and in-flight state do not carry across that boundary.

Each node's "Processing busy" percentage is occupied processing-slot time divided by provisioned slot time over the latest snapshot interval. It is a synthetic capacity measure, not measured CPU usage. Changing the in-flight limit does not change its denominator. Cache hit and miss rates use lookups accumulated in the current bounded window. Edge traversal counts record routes selected by requests. The animated dots are a capped visual sample of those traversals, not a one-dot-per-request trace at high rates.

## Fixed runs

Dev's Capture fixed run and Problems' Check goals execute a bounded deterministic run with the current graph and workload. Problems score completion percentage, P95, and P99 against the published targets. Component performance is fixed per problem; a shared resource budget caps component instances, processing slots, per-kind counts, and processing slots per component. The Solution tab contains one tested passing design, which users may load or ignore. The server applies the same constraints before it verifies a saved attempt. These results are separate from the continuously running preview.
