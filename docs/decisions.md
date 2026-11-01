# SystemForge decisions

The source specification is `SystemForge_Product_and_Technical_Specification_v2.docx`; the implementation sequence is in `SYSTEMFORGE_IMPLEMENTATION_PLAN.md`. Later sections 18–25 of the specification govern revised scope.

1. Architecture graph and workload are separate documents. Browser and backend simulation both consume a `SimulationConfig` containing both.
2. Incomplete but structurally valid drafts may be saved. Execution requires a reachable source and complete directed routes.
3. The first engine slice supports fixed arrivals, constant service time, source and API nodes, directed `always` edges, one request path, and a bounded drain interval. Additional components and stochastic behavior follow in M03.
4. Times are integer microseconds. The seed is a decimal string to preserve 64-bit values across JSON/JavaScript.
5. Simulation metrics describe a synthetic model, not production capacity.
6. The visual direction follows `docs/editor-concept.png`: a light engineering workspace with a component rail, graph canvas, inspector, and bottom metrics area. The image is a design reference; controls and text are code-native.
7. Snapshot windows are half-open: an event at a window boundary belongs to the next window. Sample events at the boundary execute before regular events at the same simulated timestamp. Finalization emits the last partial window.
8. The editor's live preview runs paced bounded engine windows continuously until paused or stopped. Valid topology additions and component setting edits update the current engine in place, preserving existing requests. Workload changes, removals, and window boundaries create a fresh engine. Fixed deterministic runs remain separate for Practice goals and Dev persistence; see `docs/modeling-semantics.md`.
9. Architecture schema version 2 makes each component node one instance. The capacity field is `concurrency`, with no hidden replica multiplier. Load balancers split to explicit targets; queues dispatch to explicit consumers. The Clients source remains a workload generator. Version 1 browser drafts and imports are converted into visible nodes and balancers; native APIs accept version 2. See `docs/implementation-checkpoint.md` for the implementation handoff.
10. Architecture schema version 3 separates `processingSlots` from `maxInFlight`. The former is provisioned parallel capacity and determines processing utilization; the latter is an admission limit and cannot increase throughput by itself. Published problem budgets charge processing slots. Browser imports migrate version 1 and 2 designs; native APIs accept version 3.

