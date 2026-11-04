# Browser evidence index

Each numbered `.json` architecture was saved using the application's `Export JSON` control after constructing the graph through the browser. The matching `.png` files are browser screenshots. Imported-graph screenshots show diagram state, not fresh simulation results. `09-stale-metrics.json` records before-edit, after-edit, and rerun values obtained from the browser metrics panel.

| Files | What they show |
| --- | --- |
| `01-online-judge.*` | Nine-component judge diagram and the refusal of a second ordinary-service output. The screenshot's 501 req/s/25.8 ms metrics belong to a previous default-graph run and demonstrate stale output; the judge's later capture was 498 req/s/36.3 ms. |
| `02-url-shortener.*` | Six-component redirect path. |
| `03-chat.*` | Eight-component chat and presence paths. |
| `04-data-pipeline.json`, `04-data-pipeline-two-workers.*`, `04-data-pipeline-one-worker-experiment.json` | One- and two-worker processing graphs, including the configured one-worker experiment export. |
| `05-hybrid-analytics.*` | Ten-component telemetry/dashboard hybrid. |
| `06-url-cache-zero-hit.json`, `07-url-two-api.*` | Cache/DB bottleneck and explicit API scale-out configurations. |
| `08-practice-problems-drawer.png` | Problems side drawer and challenge editor. |
| `09-stale-metrics.json` | Confirmed prior-result retention after worker service-time change. |
