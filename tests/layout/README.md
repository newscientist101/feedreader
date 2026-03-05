# Layout Tests

Automated layout regression tests using [Playwright Test](https://playwright.dev/) with Chromium.

These tests run in a real headless browser to get accurate bounding-rect
measurements, detecting element overlap, crushed controls, and broken layouts
at various viewport widths.

## Prerequisites

1. **Running feedreader server** on port 8000:
   ```bash
   make build && DEV=1 ./feedreader
   ```

2. **mitmdump auth proxy** on port 3000:
   ```bash
   mitmdump -p 3000 --mode reverse:http://localhost:8000 \
     --set modify_headers='/~q/X-Exedev-Userid/dev-user-1' \
     --set modify_headers='/~q/X-Exedev-Email/test@example.com'
   ```

3. **Playwright browsers** installed:
   ```bash
   npx playwright install chromium
   ```

## Running

```bash
make layout-test
```

Or directly:

```bash
npx playwright test
```

## Helpers

`helpers.js` exports reusable assertion functions:

| Function | Description |
|----------|-------------|
| `overlaps(boxA, boxB)` | Returns true if two bounding-box objects intersect |
| `assertNoOverlap(page, selectorA, selectorB)` | Assert no bounding-rect overlap between two elements |
| `assertMinWidth(page, selector, minPx)` | Assert element width ≥ N pixels |
| `assertVisible(page, selector)` | Assert element has non-zero width and height |

All `assert*` functions skip silently if the target element is missing from
the DOM, so tests degrade gracefully when the page structure changes.

## Writing Tests

Test files go in this directory with a `.test.js` suffix. They are picked
up automatically by the Playwright config.

```js
import { test, expect } from '@playwright/test';
import { assertVisible, assertNoOverlap, assertMinWidth } from './helpers.js';

test('my layout test', async ({ page }) => {
  await page.goto('/');

  // Resize viewport
  await page.setViewportSize({ width: 860, height: 900 });

  await assertVisible(page, '.my-element');
  await assertNoOverlap(page, '.element-a', '.element-b');
  await assertMinWidth(page, '.search-box', 120);
});
```

**Note:** These tests are NOT part of `make check` because they require a
running server. Run them explicitly with `make layout-test`.
