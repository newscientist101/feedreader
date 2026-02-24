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
