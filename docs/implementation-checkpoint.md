# playHLD implementation checkpoint

Updated: 2026-11-02. This is a handoff of implementation state, decisions, and verification for another AI or engineer.

## Product and repository

- Product name: **playHLD**. The page title, accessible logo name, sign-in dialog, landing footer, API title, schema IDs and titles, backend log, exported graph filename, README, and implementation plan use the new name.
- Go module path: `playhld`. Frontend package: `playhld-frontend`. The Go WebAssembly bridge now registers `playHLD*` functions, and the browser worker calls those names.
- The implementation plan is `PLAYHLD_IMPLEMENTATION_PLAN.md`. The original specification was supplied separately as a document. The workspace directory retains its existing path; the project name inside the repository is playHLD.
- New browser storage uses `playhld:` keys. The app reads older draft and layout keys, converts the data, and saves it under the new keys. Old practice drafts for the same published problem version are also read. Old keys are left in place as fallback data.

## Architecture and simulation

- Architecture schema version 3 gives every diagram node one visible component instance. It separates `processingSlots` (provisioned parallel processing capacity) from `maxInFlight` (admitted processing plus waiting requests). The latter cannot increase processing throughput by itself. `processingUtilization` is occupied processing-slot time divided by provisioned slot time; the UI calls it “Processing busy” and says it is a synthetic measure, not measured CPU usage.
- Problem budgets charge component instances and processing slots. All five published problem solutions are simulated in Go tests and meet their success, P95, and P99 goals. Problem starters are Clients plus one API service with no route. Dev's starter includes two visible API services behind a load balancer.
- Component kinds: Clients/source, load balancer, API service, worker, cache, CDN, database, object storage, and message queue. Balancers route to explicitly connected targets; queues dispatch among explicit consumers. The Solution tab offers one optional, loadable worked design per problem.
- Browser imports and old local drafts with architecture versions 1 and 2 migrate to version 3. Old hidden replicas become visible instances and routing nodes. Native API simulation expects version 3. Historical v1/v2 schemas remain as references.
- Live simulation continues until paused or stopped and accepts valid graph changes. Dots sample request routes. See `docs/modeling-semantics.md` for the exact behavior and model limits.

## Verification

- `go test ./...` passed after the playHLD module rename. Engine tests include an overloaded case proving that increasing only `maxInFlight` does not add processing capacity. All five curated solutions pass their goals.
- `npm test` passed four frontend migration tests. `npm run build` passed, including Go WASM rebuild, TypeScript compilation, and Vite production output.
- Headless Chrome on the local app confirmed title `playHLD`, landing logo alt text, sign-in heading, Run/Pause/Stop controls, and a worked problem with three passing goals and no JavaScript errors.
- A browser check loaded an old local Dev draft and workspace layout, then saved the new keys with the same graph and rail width. Another check loaded an old problem draft on a 390 px viewport, preserved its route, and found no page overflow.
- Before the product rename, live browser QA of the capacity correction showed an API at 50% busy before and after raising its in-flight limit from 20 to 220. Provisioning 40 rather than 20 processing slots changed busy to 25%. The capacity problem's worked solution passed all three goals.

## Local services

- Frontend: `http://127.0.0.1:5173/`.
- Go API: `http://127.0.0.1:8080/`. It was started with an empty `DATABASE_URL`, so account persistence requires a configured PostgreSQL database. Problem browsing and browser simulation work without one.
- The workspace currently has no `.git` directory, so `git status` is unavailable.

## Model boundaries

This is a synthetic request-path simulator. Its numbers are model estimates, not measurements of deployed systems. The queue omits durable acknowledgements and retries; cache coherence, data replication, health checks, and failures are not simulated. Separate database nodes do not silently share data. Animated dots sample traffic at high rates.

## References used for component behavior

- AWS load balancer target groups: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-target-groups.html
- AWS SQS queue types and consumers: https://docs.aws.amazon.com/en_en/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-queue-types.html
- Cloudflare CDN caching: https://developers.cloudflare.com/cache/get-started/
