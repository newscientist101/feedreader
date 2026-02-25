# Page Load Benchmark — Baseline

Captured **2026-02-24** before ES modules refactor.
Single `<script>` tag loading `app.js` (87 KB raw, 26 KB gzipped).

## Navigation Timing

| Metric              | Time   |
|---------------------|--------|
| TTFB                | 124ms  |
| DOM Interactive      | 218ms  |
| DOMContentLoaded     | 226ms  |
| Load Complete        | 231ms  |
| First Paint          | 236ms  |
| First Contentful Paint | 236ms |

## JS/CSS Assets

| Asset     | Raw Size | Transfer (gzip) | Duration |
|-----------|----------|-----------------|----------|
| app.js    | 87 KB    | 26 KB           | 18ms     |
| style.css | 53 KB    | 13 KB           | 11ms     |
| script.js | 547 B    | —               | —        |

## Full Page Summary

| Metric          | Value      |
|-----------------|------------|
| Total requests  | 35         |
| Total transfer  | 185 KB     |
| JS requests     | 1 (app.js) |
| Favicon requests | 33        |

## Notes

- Measured via browser Performance API through mitmdump reverse proxy
  (localhost:3000 → localhost:8000)
- 33 of 35 requests are sidebar favicon images (~1 KB each, 82–224ms each
  due to DB lookups / proxying)
- One fetch request to `/api/queue` (120 KB, 92ms) fires after DOMContentLoaded
- JS/CSS load times are negligible (<20ms) since it's all localhost
- After the refactor, expect ~15 JS module requests replacing the single
  app.js request. Each module will be 1–10 KB. Impact on load time should
  be minimal given localhost latency.

## How to Re-measure

```bash
# Start auth proxy if not running
mitmdump --mode reverse:http://localhost:8000 --listen-port 3000 \
  --set modify_headers='/~q/X-Exedev-Email/test@example.com' \
  --set modify_headers='/~q/X-Exedev-Userid/testuser123'
```

Then in the browser console on `http://localhost:3000/`:

```js
const nav = performance.getEntriesByType('navigation')[0];
const paint = performance.getEntriesByType('paint');
const resources = performance.getEntriesByType('resource');
console.table({
  ttfb: Math.round(nav.responseStart),
  domInteractive: Math.round(nav.domInteractive),
  domContentLoaded: Math.round(nav.domContentLoadedEventEnd),
  loadComplete: Math.round(nav.loadEventEnd),
});
console.log('Paint:', paint.map(p => `${p.name}: ${Math.round(p.startTime)}ms`));
console.log(`${resources.length} requests, ${Math.round(resources.reduce((s,r) => s + (r.transferSize||0), 0) / 1024)} KB`);
```

---

# Page Load Benchmark — After ES Modules Refactor

Captured **2026-02-25** after completing Phases 1–4 of the ES modules
refactor. `app.js` is now a 363-line entry point loading 20 ES modules
via `<script type="module">` with import maps for cache busting.

## Navigation Timing (cold load)

| Metric               | Time   | Δ from baseline |
|----------------------|--------|----------------|
| TTFB                 | 257ms  | +133ms         |
| DOM Interactive      | 378ms  | +160ms         |
| DOMContentLoaded     | 554ms  | +328ms         |
| Load Complete        | 570ms  | +339ms         |
| First Paint          | 404ms  | +168ms         |
| First Contentful Paint | 404ms | +168ms         |

## Navigation Timing (warm/cached load)

| Metric               | Time       |
|----------------------|------------|
| TTFB                 | 223–264ms  |
| DOMContentLoaded     | 287–331ms  |
| Load Complete        | 293–336ms  |
| First Paint          | 292–336ms  |
| Transfer             | **0 KB**   |

## JS/CSS Assets

| Asset                  | Raw Size | Transfer (gzip) |
|------------------------|----------|----------------|
| app.js                 | 13 KB    | 4 KB           |
| modules/feeds.js       | 17 KB    | 4 KB           |
| modules/articles.js    | 13 KB    | 4 KB           |
| modules/article-actions.js | 12 KB | 3 KB          |
| modules/scraper-page.js | 11 KB   | 3 KB           |
| modules/drag-drop.js   | 9 KB     | 2 KB           |
| modules/offline.js     | 8 KB     | 3 KB           |
| 14 smaller modules     | 36 KB    | 12 KB          |
| **Total JS**           | **119 KB** | **35 KB**    |
| style.css              | 52 KB    | 13 KB          |

## Full Page Summary

| Metric           | Value          | Δ from baseline |
|------------------|----------------|----------------|
| Total requests   | 58             | +23            |
| JS requests      | 21             | +20            |
| JS raw size      | 119 KB         | +32 KB (+37%)  |
| JS gzip size     | 35 KB          | +9 KB (+35%)   |
| Favicon requests | 35             | +2             |

## Analysis

**Cold load is slower** due to the ES module import waterfall. The
browser discovers imports sequentially as it parses each module,
creating a dependency chain. With 20 modules and a 3-level import
depth, this adds multiple sequential round trips even on localhost.

**Warm load is fast** — import maps with content hashes enable
immutable caching (`max-age=31536000`), so subsequent visits transfer
0 bytes and load in ~300ms, comparable to the pre-refactor cold load.

**Raw JS size grew 37%** — import/export statements and late-binding
boilerplate (`setXxxDeps` patterns) add overhead. Gzip is also less
effective across many small files vs. one large file.

## Potential Optimizations

- **`<link rel="modulepreload">`** for each module in `base.html`
  would eliminate the waterfall by fetching all modules in parallel
- **Phase 5/6 cleanup** will reduce module count and remove
  late-binding boilerplate
- **Bundling** for production remains an option but was explicitly
  deferred (see ES_MODULES_PLAN.md)
