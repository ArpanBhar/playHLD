# Black-box test coverage

Status meanings: **Pass** = observed behavior matched the tested expectation; **Fail** = confirmed defect; **Blocked** = no relevant product control or environment dependency; **Not executed** = outside the completed priority set or explicitly excluded. Browser automation failures are identified separately from app failures.

## Environment and method

- Local frontend: `http://127.0.0.1:5173/`; Go API: port 8080; Chrome headless through Playwright/CDP; desktop viewport 1440 × 900.
- The integrated browser host returned `The browser access host is unavailable`. Regular Playwright/CDP was used as the browser-automation fallback. The initial Problems catalog showed `Problems could not be loaded` while the API was stopped; it loaded normally after the API started. Neither is filed as an application defect.
- No source code was inspected or edited. All graphs, captures, imports, exports, and settings changes were performed through the running browser application. `evaluation/` contains the only written artifacts.

## Stage 1 — Platform discovery

| Test | Status | Observation |
| --- | --- | --- |
| Landing → Problems → challenge editor | Pass | Five problem statements were visible after API startup; first and second challenge navigation worked. |
| Dev entry and default graph | Pass | Initial Dev graph had 6 components and 6 routes. |
| Component palette and settings inventory | Pass | Added and inspected all eight palette types; see [COMPONENT_INVENTORY.md](COMPONENT_INVENTORY.md). |
| Workload controls | Pass | Steady/ramp/burst, even/seeded Poisson, rate, duration, read share, and seed were visible. |
| Add, move, connect, zoom/fit, select | Pass | Used these controls to build five architectures. |
| Delete nodes and connections | Pass | Optional nodes and selected routes were deleted; required Practice nodes remained. |
| Collapse and resize panels | Partially tested | Controls and separators were present; detailed drag sizing and persistence were not rerun because simulation correctness had priority. |
| Weighted route editing | Not executed | Weighted policy was selectable on LB, but the target edge could not be selected reliably in the automation session; no claim about weight editing is made. |
| Authentication and Contest | Not executed | Explicitly excluded as unfinished. |

## Stage 2 — Browser-built architectures

| Architecture | Status | Evidence | Simulation |
| --- | --- | --- | --- |
| Online coding judge | Built with modeling gaps | [Graph JSON](evidence/01-online-judge.json), [screenshot](evidence/01-online-judge.png) | Fixed run: 498 req/s, P95/P99 36.3 ms at 500 req/s offer. |
| URL shortener | Built with modeling gaps | [Graph JSON](evidence/02-url-shortener.json), [screenshot](evidence/02-url-shortener.png) | Cache/DB and API-scale experiments executed. |
| Real-time chat | Built with modeling gaps | [Graph JSON](evidence/03-chat.json), [screenshot](evidence/03-chat.png) | Fixed run: 299 req/s, P95/P99 36.3 ms at 300 req/s offer. |
| Distributed data processing | Built with modeling gaps | [Two-worker JSON](evidence/04-data-pipeline-two-workers.json), [screenshot](evidence/04-data-pipeline-two-workers.png) | One/two-worker, queue, latency-semantics, seeded-arrival, and live-topology experiments executed. |
| Original telemetry/dashboard hybrid | Built with modeling gaps | [Graph JSON](evidence/05-hybrid-analytics.json), [screenshot](evidence/05-hybrid-analytics.png) | Fixed run: 291 req/s, P95/P99 36.6 ms at 300 req/s offer; live rate change tested. |

The modeling gaps are documented in [BLACK_BOX_REPORT.md](BLACK_BOX_REPORT.md) and are separate from whether a graph could be drawn.

## Stage 3 — Controlled simulations

| Test | Status | Observation |
| --- | --- | --- |
| Throughput/capacity from processing slots | Pass | 1 vs 3 Practice API slots matched 333 vs 1,000 req/s capacity estimates; fixed-run metrics changed accordingly. |
| Queue backlog under worker bottleneck | Pass | One 200 req/s worker at 300 req/s offer produced 100 queued and 100 rejected over 2 s. |
| Worker scale-out | Pass | Two workers handled 300 req/s without backlog; one handled 200 req/s with backlog. |
| Load balancing across API instances | Pass | One 500 req/s API at 800 req/s offer saturated; two APIs handled 800 req/s without rejection. |
| Cache hit/miss behavior | Pass | 80% → 0% URL-cache hits increased DB demand and cut throughput from 484 to 201 req/s with slow DB. |
| DB bottleneck | Pass | DB service 50 → 5 ms removed queue/rejections and raised throughput 201 → 501 req/s. |
| End-to-end latency | Pass for current synchronous model | Service-time sums matched zero-queue latencies; worker time changed original request latency. Async job latency is unavailable. |
| Seeded Poisson reproducibility | Pass | Two seed-42 captures were identical; seed 43 differed. |
| Live traffic edit | Pass | 300 → 600 configured req/s while running changed observed throughput about 299 → 595 req/s. |
| Live topology edit | Pass | Added/connected a second worker while running; throughput recovered to 200 req/s without stop/restart. |
| Failure propagation | Blocked | No failure-injection or retry/timeout controls observed. |
| Exact Practice completion denominator | Not resolved | See [ASTRA_HANDOFF.md](ASTRA_HANDOFF.md); no exact total-completed counter was available in UI. |

Full configurations, numerical results, and calculations are in [SIMULATION_EXPERIMENTS.md](SIMULATION_EXPERIMENTS.md).

## Stage 4 — Reliability and usability

| Test | Status | Observation |
| --- | --- | --- |
| Fixed-run result after a design edit | **Fail — BB-01** | Metrics stayed `Completed` with old values until rerun, even after worker service changed 80 → 100 ms. |
| Duplicate route | Pass | Refused with a notice; route count stayed unchanged. |
| API/worker second outgoing route | Capability gap — BB-02 | Refused; prevented realistic judge fanout. |
| Disconnected extra node | Pass | Fixed run produced `UNREACHABLE_NODE`, no fabricated metrics. |
| Disconnected Practice source | Pass | `MISSING_ROUTE` validation shown. |
| Cycle | Pass | Cycle could be drawn, then fixed run rejected it with `CYCLE`. |
| Delete connection + Undo + Redo | Pass | Route count 5 → 4 → 5 → 4; Undo restored the graph afterward. |
| Practice per-component slot cap | Pass | Attempted 5 slots; value reverted and cap notice shown. |
| Negative Dev arrival input | Pass with usability note | `-1` remained in field, but a capture was refused with an explanatory notice. Prior metrics stayed visible, as in BB-01. |
| Practice solution, load example, reset | Pass | Solution listed four nodes/three routes; `Load example` loaded them; reset returned to client + API, 0 routes. |
| Problems side drawer and challenge switch | Pass | Side list opened and switched from launch spike to bursting jobs. |
| Continuous Pause / Resume / Stop | Pass | Button labels and simulation status changed correctly. |
| Import/export graph | Pass for graph; gap BB-05 for workload | Export/import restored nodes and routes; workload was independent. |
| Malformed JSON import | Not executed | Avoided redundant destructive-input probing after valid import, graph validation, and input validation were covered. |
| Mobile viewport / cross-browser | Not executed | Desktop architecture/simulation correctness took priority. |
| Very large graphs / destructive stress | Not executed | Intentionally avoided destructive stress testing. Largest built graph had 10 components and 9 routes. |

## Automation-only failures

- A broad `Pause simulation` locator matched two legitimate controls; scoping to the header resolved the test. This was not an app failure.
- Several edge clicks and Dev screenshots timed out while the CDP page viewport had reverted to 762 × 484 after a browser automation kernel reset. Restoring 1440 × 900 and using fresh browser tabs resolved graph interaction and screenshot capture. No defect was inferred from those automation failures.
- Screenshots of imported graphs are diagram evidence. The judge screenshot also contains prior fixed-run metrics, which is itself evidence for BB-01; those numbers must not be read as the judge run's metrics.
