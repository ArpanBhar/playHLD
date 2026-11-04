# playHLD implementation checkpoint

Updated: 2026-11-04. Workspace: `D:/playHLD`. This is a handoff of implementation state, decisions, and verification for another AI or engineer.

## 2026-11-03 functional simulation checkpoint

- Engine version `0.7.0` keeps the original capacity mode and adds URL mapping, key-value, and asynchronous job contracts in native Go and browser WASM. URL/key-value requests carry keys and checked outputs. Datastores hold values; caches use LRU capacity and optional TTL, with measured hit rates. API/worker boxes can have read/write routes.
- Async submissions are acknowledged at a message queue, buffered when consumers are full, processed by explicit workers, and persisted as verdicts. Client acknowledgment metrics and job completion counts are separate.
- New problems: **Design a URL Shortener**, **Design Pastebin**, **Design a Key-Value Store**, and **Design an Online Coding Judge**. Their solutions pass published goals in Go tests. All still start with client and API boxes only.
- Fixed-run control is **Evaluate**. It does not animate dots. Editing graph/config/workload clears stale fixed results and goal badges. Workspace JSON exports Dev workload with graph; old architecture-only imports still work. Live functional cache and completed datastore state transfer between bounded windows.
- Go tests cover the new problems, failed API-only URL design, measured cache-size effect, warm-cache transfer, and queue backlog under slow workers. Full `go test ./...`, `npm test`, and `npm run build` passed after the async changes. Headless Chrome verified 8,000 correct URL requests, zero dots during Evaluate, stale-result invalidation after a slot edit, live dots, Stop, and continued playback across a window boundary. The online judge browser run showed 3,000 acknowledged and completed jobs with five passing goal badges, no JavaScript errors, and no horizontal overflow at 390 px.
- Dev now supports seeded per-component failure probability and a visible fault count. Go and Chrome checks confirmed that database failures fail URL requests and worker failures leave client acknowledgments intact while failing jobs. Remaining: a general editable operation language; retries, correlated outages, and replication; preservation of in-flight jobs across live window boundaries. See `docs/modeling-semantics.md`.

## Product and repository

- Product name: **playHLD**. The page title, accessible logo name, sign-in dialog, landing footer, API title, schema IDs and titles, backend log, exported graph filename, README, and implementation plan use the new name.
- Go module path: `playhld`. Frontend package: `playhld-frontend`. The Go WebAssembly bridge now registers `playHLD*` functions, and the browser worker calls those names.
- The implementation plan is `PLAYHLD_IMPLEMENTATION_PLAN.md`. The original specification was supplied separately as a document. The current workspace is `D:/playHLD`.
- New browser storage uses `playhld:` keys. The app reads older draft and layout keys, converts the data, and saves it under the new keys. Old practice drafts for the same published problem version are also read. Old keys are left in place as fallback data.

## Architecture and simulation

- Architecture schema version 3 gives every diagram node one visible component instance. It separates `processingSlots` (provisioned parallel processing capacity) from `maxInFlight` (admitted processing plus waiting requests). The latter cannot increase processing throughput by itself. `processingUtilization` is occupied processing-slot time divided by provisioned slot time; the UI calls it “Processing busy” and says it is a synthetic measure, not measured CPU usage.
- Problem budgets charge component instances and processing slots. All nine published problem solutions are simulated in Go tests and meet their published goals. Problem starters are Clients plus one API service with no route. Dev's starter includes two visible API services behind a load balancer.
- Component kinds: Clients/source, load balancer, API service, worker, cache, CDN, database, object storage, and message queue. Balancers route to explicitly connected targets; queues dispatch among explicit consumers. The Solution tab offers one optional, loadable worked design per problem.
- Browser imports and old local drafts with architecture versions 1 and 2 migrate to version 3. Old hidden replicas become visible instances and routing nodes. Native API simulation expects version 3. Historical v1/v2 schemas remain as references.
- Live simulation continues until paused or stopped and accepts valid graph changes. Dots sample request routes. See `docs/modeling-semantics.md` for the exact behavior and model limits.

## Verification

- `go test ./...` passed after the latest simulation changes. Engine tests include an overloaded case proving that increasing only `maxInFlight` does not add processing capacity. All nine curated solutions pass their goals.
- `npm test` passed four frontend migration tests. `npm run build` passed, including Go WASM rebuild, TypeScript compilation, and Vite production output.
- Headless Chrome on the local app confirmed title `playHLD`, landing logo alt text, sign-in heading, Run/Pause/Stop controls, and a worked problem with three passing goals and no JavaScript errors.
- A browser check loaded an old local Dev draft and workspace layout, then saved the new keys with the same graph and rail width. Another check loaded an old problem draft on a 390 px viewport, preserved its route, and found no page overflow.
- Before the product rename, live browser QA of the capacity correction showed an API at 50% busy before and after raising its in-flight limit from 20 to 220. Provisioning 40 rather than 20 processing slots changed busy to 25%. The capacity problem's worked solution passed all three goals.

## Local services

- Frontend: `http://127.0.0.1:5173/`.
- Go API: `http://127.0.0.1:8080/`. Account and project persistence now uses embedded SQLite at `playhld.db` by default; `DATABASE_PATH` overrides that location. Existing PostgreSQL data is not migrated automatically.
- The workspace has a Git repository. The pre-existing `evaluation/` reports are untracked and have not been edited by this implementation task.

## Model boundaries

This is a synthetic simulator. Its numbers are model estimates, not measurements of deployed systems. Functional scenarios now check values or job verdicts, and Dev can inject independent component faults. The queue still omits durable storage and retries; data replication, health checks, and correlated outages are not simulated. Separate database nodes do not silently share data. Animated dots sample traffic at high rates.

## References used for component behavior

- AWS load balancer target groups: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-target-groups.html
- AWS SQS queue types and consumers: https://docs.aws.amazon.com/en_en/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-queue-types.html
- Cloudflare CDN caching: https://developers.cloudflare.com/cache/get-started/
