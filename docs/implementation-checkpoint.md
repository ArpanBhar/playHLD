# SystemForge implementation checkpoint

Updated: 2026-11-01. This is a handoff of decisions, implementation state, and verification for another AI or engineer. It is not a transcript of private reasoning.

## Follow-up in progress: separate processing capacity from admission limit

The user observed that increasing "Concurrent requests" lowered the displayed load. In v2, that field actually sized parallel processing slots and the load denominator, so its label implied the wrong operation. The correction is to introduce architecture v3 with `processingSlots` (provisioned parallel capacity, budgeted) and `maxInFlight` (admission cap, including processing and waiting). The engine's processing utilization must divide busy processing time by `processingSlots`; raising `maxInFlight` alone must not change capacity. Migrate saved v1/v2 browser graphs; update schemas, published challenge data and version, frontend controls, UI labels, tests, docs, and WASM. At this checkpoint the v3 correction has not yet been applied.

## User request

Each diagram node should represent one actual component instance. Remove hidden replica counts. Users scale out by placing and connecting individual components, including load balancers and API services. Apply this consistently across the editor and problems, add useful component types, provide an optional solution for each problem, and preserve continuous live simulation, resource limits, and the compact editor.

## Implemented

- Architecture schema version 2 replaces `replicas` and `concurrencyPerReplica` with one `concurrency` value per visible node. The workload schema remains version 1. The Go engine calculates capacity from that node alone. V2 graphs containing hidden replica fields are rejected.
- Nine component kinds are available: Clients/source, load balancer, API service, worker, cache, CDN, database, object storage, and message queue. The fixed Clients node is not in the add menu. Load balancers distribute to explicitly connected targets. Queues can dispatch across multiple explicitly connected workers. CDN and cache have hit/miss routing; database and object storage have read/write service times.
- Dev's starter architecture visibly draws two API services behind a load balancer. Problem starters remain Clients plus one API service, with no route; the user builds the design.
- All five published problems use v2 budgets based on visible component count, total processing slots (sum of per-node concurrency), per-kind limits, and per-node concurrency. Each has one curated solution with an explanation. The Solution tab offers a persistent `Load example` action, and loading the graph is undoable.
- The frontend editor, inspector, health displays, component palette, resource UI, and problem view use the new model. The inspector describes concurrency as local to one instance. Old v1 browser drafts and imported graphs are converted into visible nodes and routing; old v1 schemas and fixture remain archived for reference.
- API catalog and OpenAPI simulation schema point to v2. README, modeling semantics, decisions, and implementation-status docs are updated. The WASM simulator was rebuilt.

## Model boundaries

This is a synthetic request-path simulator, not a deployable infrastructure description or a measurement of a production system. The queue models buffering and dispatch, but not durable acknowledgements or retries. Cache coherence, data replication, health checks, and failures are not simulated. Separate database nodes do not silently share data. Animated dots sample traversals at high traffic rates. See `docs/modeling-semantics.md` for the full behavior, including live update and restart rules.

## Verification

- `go test ./...` passed after the queue fan-out change. The challenge test simulates all five published solutions and checks their goals; a separate queue test confirms that both worker branches receive jobs.
- `npm test` passed two frontend migration tests: v1 replicas become visible instances, and v2 hidden replica fields are rejected.
- `npm run build` passed again after the final Solution footer UI adjustment, including TypeScript compilation, the Go WASM rebuild, and the Vite production bundle.
- Browser QA on the local Vite app verified: starter is two nodes with no edges; loading the capacity solution gives four nodes and three edges; all three goals pass through the browser WASM; live Run and Stop work; new component kinds are present; 390px viewport has no horizontal overflow; an old v1 local draft migrates; no JavaScript errors. Queue solution QA verified six visible nodes, two worker nodes, six routes, three passing goals, no mobile overflow or JS errors. The Solution action stays visible at mobile pane height.

## Local services

- Frontend Vite server: `http://127.0.0.1:5173/` (running at handoff).
- Go backend: `http://127.0.0.1:8080/` (running with `DATABASE_URL` empty, so account persistence is unavailable). Problems, catalog, and local simulation do not need a database. If either process has stopped, see `README.md` for start commands.
- This workspace does not currently have a `.git` directory; `git status` is unavailable.

## References used for modeling decisions

- AWS Application Load Balancer target groups: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-target-groups.html
- AWS SQS queue types and worker consumption: https://docs.aws.amazon.com/en_en/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-queue-types.html
- Cloudflare CDN cache hit/miss and origin behavior: https://developers.cloudflare.com/cache/get-started/

## Remaining work / next action

No required work remains for this request. Later extensions could add richer distributed-system semantics (durability, retry, replication, failure, consistency), but those are not implemented or promised by the present simulator.
