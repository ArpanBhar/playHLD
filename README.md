# playHLD

playHLD is a web prototype for sketching a service architecture and observing synthetic load in a deterministic Go simulator. The same engine runs as native Go in the backend and as WebAssembly in a dedicated browser worker. See the [implementation plan](PLAYHLD_IMPLEMENTATION_PLAN.md) and [verified status](docs/implementation-status.md).

The current application has a shared graph editor, anonymous local simulation, nine Problems, signed-in server-verified attempts, private saved designs, and Dev projects with editable drafts, immutable versions, steady/ramp/burst traffic, seeded Poisson arrivals, exponential service times, saved native runs and comparisons. Four problems use functional contracts for URL mapping, key-value access, or asynchronous code judging. Collaboration and Contest remain under development. Simulated metrics are model estimates, not production capacity claims.

The dark workspace has separate URLs for the home page (`/`), problem list (`/problems`), individual problem workspaces (`/problems/:id`), Dev (`/dev`), and Contest (`/contest`). Old `/practice` links redirect to Problems. The logo returns home; the Problems button beside it opens the problem list drawer in the editor. Run, pause, stop, and fixed goal checks sit in the compact header above the editor. The problem statement is beside the canvas on desktop, where its width is resizable, and collapsible on mobile. Page navigation appears on browsing pages and stays out of the editor. Select a connection on the canvas to delete it with the on-screen control or the Delete key.

Drag the separators to resize the description, component list, inspector, and metrics. On narrow screens, drag each panel's bottom grip. Each panel has a collapse button; the component list scrolls independently, and pane sizes are remembered in this browser. Each diagram node is one component instance, except the Clients node, which generates the published workload. To scale out, add another API or worker node and connect it to a load balancer or message queue. Processing slots are provisioned parallel capacity for a single instance; the separate max in-flight setting only limits admission to processing and the queue. There is no hidden replica multiplier. The palette also includes CDN and object storage, with distinct cache and storage behavior.

The Problems page lists scenarios; picking one opens the editor with only a client and API server. Add, connect, and remove components, then use **Evaluate** for a fixed run. Evaluate does not animate request dots, and editing the graph or workload clears stale results. The description pane shows goals, resource limits, and an optional Solution tab with a tested example design that can be loaded onto the canvas. Service time and queue size are fixed for each problem; topology and per-instance processing slots can vary within the component and slot budget. Older capacity problems use a fixed cache hit probability. Functional problems measure cache hits from real keys, capacity, TTL, and writes, and check output correctness or completed verdicts. The max in-flight limit can change how many requests wait, but not processing throughput. Server verification enforces these rules independently of the browser. Run simulation starts a wall-clock paced live preview; the toolbar changes to Pause/Resume and Stop. Connected graph and setting edits update the live engine, and moving dots sample traversals on active routes. Nodes show processing-slot occupancy or cache hit/miss rate with green-to-red health coloring. Dev can choose a functional request behavior, inject independent component faults, and use Evaluate for a bounded result. See [modeling semantics](docs/modeling-semantics.md) for the model's behavior and limits.

The native API accepts architecture schema version 3. Browser drafts and imported version 1 or 2 diagrams are converted when opened; old capacity values become processing slots, and old version 1 replicas become visible component instances and routing nodes. The playHLD browser storage keys read existing drafts and editor layouts saved under the previous project name.

## Local development

Requires Go 1.26 and Node 24/npm. Accounts, sessions, designs, projects, and practice progress use embedded SQLite. The API creates `playhld.db` and applies its migrations on startup. Set `DATABASE_PATH` to store the database elsewhere. Existing PostgreSQL data is not imported automatically.

In one terminal from the repository root:

```powershell
$env:DATABASE_PATH = 'data\playhld.db' # optional; omit to use playhld.db
go run ./backend/cmd/server
```

In another terminal:

```powershell
cd frontend
npm ci
npm run dev
```

Open `http://127.0.0.1:5173/`. Vite proxies `/api` to port 8080. `predev` builds `frontend/public/wasm/simulator.wasm` with the installed Go toolchain and copies its matching `wasm_exec.js`. The backend enables account and project APIs with its SQLite database by default.

## Checks

```powershell
go test ./...
cd frontend
npm test
npm run typecheck
npm run build
```

Go integration tests create isolated temporary SQLite databases automatically. The frontend build runs the WASM build first.
