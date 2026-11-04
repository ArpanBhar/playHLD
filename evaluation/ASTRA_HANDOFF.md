# Astra handoff — playHLD black-box evaluation

**Scope completed:** Browser-built and simulated all five requested architectures; inventoried all eight palette components; tested Practice and Dev, capacity, queue, cache, DB, explicit API/worker scale-out, seeded arrivals, live edits, graph validation, and reset/history. Auth and Contest were excluded. No application source was inspected or modified. See [BLACK_BOX_REPORT.md](BLACK_BOX_REPORT.md), [SIMULATION_EXPERIMENTS.md](SIMULATION_EXPERIMENTS.md), and the browser-generated [evidence/](evidence/) artifacts.

## Highest-priority findings

1. **BB-01, confirmed defect:** Fixed-run metrics stay labeled `Completed` after graph or component changes. Example: 200 req/s, two 80 ms workers yielded P95 106.3 ms. Changing one worker to 100 ms left `Completed / 106.3 ms` on screen until rerun, when P95 became 126.3 ms. [Exact browser observation](evidence/09-stale-metrics.json). This can mislead design decisions.
2. **BB-02, confirmed capability gap:** An API or worker can have only one outgoing route. An online judge API could not both access cache and enqueue, and its worker could not write artifact storage plus DB metadata. A second API behind an LB merely splits requests; it does not model one submission triggering both effects. LB and queue multi-output routes did work.
3. **BB-03, confirmed capability gap:** Queue-to-worker-to-storage remains one synchronous request path. At 100 req/s with no backlog, worker time 50 → 80 ms caused P95 76.3 → 106.3 ms. Async enqueue acknowledgment and separate job completion are unavailable. The current arithmetic is internally consistent for a synchronous path.
4. **BB-04/05, gaps:** No fault injection/retry/outage controls. Architecture JSON restores graph but not workload; a 300 req/s exported graph imported while the current workload was `-1` still showed `-1` and could not run.

## Mathematics: what checked out, what remains open

- **No confirmed mathematical defect.** One 10-slot/50 ms worker has a 200 req/s ceiling; two have 400 req/s. At 300 req/s offered for 2 s, the browser showed one worker at 200 req/s with queue 100/rejected 100 versus two workers at 300 req/s with no queue/rejection.
- At 800 req/s, one 10-slot/20 ms API saturated around 500 req/s with queue 100/rejected 500; two instances delivered 800 req/s with no queue. A second LB was not involved in the gain.
- DB 10 slots × 50 ms gives 200 req/s; reducing service to 5 ms gives 2,000 req/s. Under 500 req/s with cache misses at 100%, browser throughput rose 201 → 501 req/s and P95 fell 571.1 → 26.1 ms. The zero-queue P95 equaled the service-time sum.
- **Unresolved metric-definition question, not a filed defect:** In the launch-spike Practice problem, 800 req/s for 10 s, 1 API slot at 3 ms, UI showed throughput 334 req/s, queue 100, rejected 4,567, and `Measured 42.9%` completion. `(8000 − 4567)/8000 = 42.91%`, suggesting the displayed completion rate counts admitted requests. About 3,340 requests would complete at 334 req/s within the 10 s arrival window, closer to 41.8%. The engine may drain accepted work after arrivals end or define throughput over a different window. Investigate exact offered/accepted/completed totals and reporting windows before claiming a defect.
- Seeded Poisson was repeatable: seed 42 produced identical 113 req/s, P95 154.0, P99 162.8, queue 8 in two captures. Seed 43 produced 97 req/s, P95 133.8, P99 140.4, queue 5.

## Deeper investigation suggested

1. Determine the intended simulation contract: HTTP request completion versus async job completion, and whether branch/fanout and multiple side effects are in scope. Test expected routes before changing the engine.
2. Reconcile Practice completion percentage with exact event counters, including queued work after the published workload ends.
3. Verify weighted LB route-weight editing through manual browser interaction. The policy selector existed; automation could not reliably select its edge, so no defect was asserted.
4. Add browser-visible total offered, admitted, completed, rejected, and remaining-in-queue counters for mathematical auditability.
5. If exporting reproducible experiments is a goal, include workload settings and model/version metadata alongside the graph, or label the current export as architecture-only.

**Evidence note:** [01-online-judge.png](evidence/01-online-judge.png) shows the constructed judge graph and an outgoing-route refusal notice. Its metrics are from a prior default-graph capture, illustrating BB-01; the judge's own later capture was 498 req/s, P95/P99 36.3 ms. The other graph screenshots were taken from fresh browser tabs after importing the browser-exported JSON. Automation timeouts and the initially stopped local API are recorded in [TEST_COVERAGE.md](TEST_COVERAGE.md) and are not filed as app defects.
