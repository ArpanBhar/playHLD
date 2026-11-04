# Modeling semantics

playHLD simulates synthetic requests against the configured graph. The numbers are model estimates, not measurements of a deployed system.

## Functional workload contracts (engine 0.7.0)

Capacity-only mode retains the original probabilistic cache and request-path model. URL mapping and key-value modes instead generate typed creates/redirects or puts/gets over a bounded key space. Datastores persist actual values. A cache fills from a datastore after a miss, evicts least-recently-used entries at its configured capacity, respects optional TTL, and invalidates the matching key on a write. Its hit rate is measured, not assigned. API and worker boxes may have separate read and write routes. An output is checked against the datastore's current expected value; the first twelve completed requests appear with their paths and outputs in the result.

Asynchronous jobs model code submissions. A message queue acknowledges an accepted submission before workers finish. It buffers jobs while connected consumers are full. A worker processes a submission, and a database or object store persists its verdict. Client acknowledgment P95/P99, enqueued jobs, completed verdicts, failed jobs, and unfinished jobs are separate metrics. A synchronous worker path can persist a verdict, but does not satisfy a queue-specific job-completion goal.

These are bounded request simulations, not small-sample extrapolations. The editor does not yet offer a general programmable function language for arbitrary components. Functional scenarios expose specific contracts while leaving the topology and component counts to the user.

Dev also exposes a per-component failure probability. A failed processing attempt drops that client request, or fails an already acknowledged async job, and increments that component's fault counter. Failures use each node's seeded random stream. Problems lock failure probability to zero. There is no retry or failover after a fault.

## Diagram and capacity

Architecture schema version 3 uses one node per component instance. The Clients node is the workload generator, not a deployed server. Every other node has processing slots, a maximum in-flight request count, service time, and a queue limit. Processing slots represent provisioned parallel work capacity and consume the problem's slot budget. The in-flight limit counts both processing and waiting requests; raising it alone can reduce rejection or increase the queue, but cannot make the instance process requests faster. Raising processing slots or adding another connected instance can increase capacity. A load balancer routes across its connected targets by round robin or weights. A message queue dispatches to its connected consumers in round-robin order without a separate load balancer.

A cache or CDN serves read hits at that node unless a hit route is drawn; misses must follow a miss route. Database and object-storage nodes can use different read and write service times. Worker nodes represent one job processor each. In capacity-only mode, queues still use the older synchronous routing rule. Functional async jobs use separate acknowledgment and worker completion. The model does not yet simulate durable storage, retries, visibility timeouts, exactly-once processing, data replication, health checks, or correlated outages. Functional cache invalidation is idealized and global. Multiple database nodes have independent state, not automatically synchronized copies.

## Live preview

The toolbar's Run simulation control starts a live preview paced against wall-clock time. Snapshots arrive about once a second, and the preview continues until Pause or Stop. The pattern length in the Dev inspector controls a repeating bounded traffic window; it does not stop the live session. Problems use their published workload pattern. The worker keeps recent snapshots for a rolling chart. P95 and P99 latency are shown from the most recently completed window.

The engine accepts valid topology additions, new connections, and component setting changes in place. Requests already queued or in flight on existing components remain in the engine. A disconnected component cannot receive traffic; the previous valid graph continues running, and the UI shows a pending update warning until the graph becomes valid. A workload change or component removal starts a fresh bounded window. Functional caches and completed datastore writes carry across repeating windows. Requests and jobs still in flight at a window boundary are not transferred.

Each node's "Processing busy" percentage is occupied processing-slot time divided by provisioned slot time over the latest snapshot interval. It is a synthetic capacity measure, not measured CPU usage. Changing the in-flight limit does not change its denominator. Cache hit and miss rates use lookups accumulated in the current bounded window. Edge traversal counts record routes selected by requests. The animated dots are a capped visual sample of those traversals, not a one-dot-per-request trace at high rates.

## Fixed runs

The **Evaluate** button executes a bounded deterministic run with the current graph and workload. It does not animate request dots. Editing topology, component settings, or workload invalidates displayed results and goal badges. New functional problems score output correctness or job completion as well as client completion and tail latency. Component performance is fixed per problem; a shared budget caps instances and processing slots. The Solution tab contains one tested passing design, which users may load or ignore. The server applies the same constraints before it verifies a saved attempt. Exported workspace JSON includes the graph and Dev workload; architecture-only imports remain supported.
