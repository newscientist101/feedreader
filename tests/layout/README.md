# Layout Tests

Automated layout regression tests using **Vitest browser mode** with
**Playwright** (Chromium, headless). Tests measure real bounding rects in a
real browser to detect element overlap, crushed controls, and broken layouts
at various viewport widths.

## How It Works

Vitest browser mode runs test code inside a browser iframe. Since layout tests
need to measure the actual app (not components rendered in the iframe), we use
**custom Vitest commands** that run server-side with access to the full
Playwright API:

- `fetchPageHTML(url)` — navigates a Playwright page to the app URL and
  returns the rendered HTML.
- `measureLayout(url, selectors, width, height)` — opens a Playwright page at
  the given viewport size, navigates to the URL, and returns bounding rects
  for each selector.

This gives us real browser layout measurements without cross-origin
restrictions.

## Prerequisites

1. **App server** running on port 8000:
   ```bash
   DEV=1 ./feedreader  # or: make build && sudo systemctl restart feedreader
   ```

2. **Auth proxy** on port 3000 (injects test user headers):
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
npx vitest run --config tests/config/vitest.browser.config.mjs
```

> **Note**: Layout tests are _not_ part of `make check` since they require a
> running server with auth proxy.

## Helper API

Import from `./helpers.js`:

| Function | Description |
|---|---|
| `overlaps(a, b)` | Returns `true` if two bounding boxes intersect |
| `measure(url, selectors, width?, height?)` | Get bounding rects via Playwright |
| `assertNoOverlap(rects, selA, selB)` | Assert two elements don't overlap |
| `assertMinWidth(rects, selector, minPx)` | Assert element width ≥ N px |
| `assertVisible(rects, selector)` | Assert element has non-zero dimensions |

### Example

```js
import { describe, test } from 'vitest';
import { assertNoOverlap, assertVisible, measure } from './helpers.js';

describe('header at 860px', () => {
  test('view toggles are visible and don\'t overlap title', async () => {
    const rects = await measure('/', [
      '.view-header h1',
      '.view-toggle',
      '.header-actions',
    ], 860, 720);

    assertVisible(rects, '.view-toggle');
    assertNoOverlap(rects, '.view-header h1', '.view-toggle');
    assertNoOverlap(rects, '.view-toggle', '.header-actions');
  });
});
```

## Existing Tests

| File | Description |
|---|---|
| `smoke.test.js` | Infrastructure smoke test — verifies `fetchPageHTML` and `measureLayout` commands work |
| `header-mobile.test.js` | Header layout at 375px and 430px (mobile breakpoint) with 7 feed name lengths |
| `header-860.test.js` | Header layout at 860px (Goldilocks zone) — catches feedreader-wei regression |
| `header-wide.test.js` | Header layout at 1920px (wide desktop) with full controls visible |

## Adding Tests

Create new `*.test.js` files in this directory. They'll be picked up
automatically by the `tests/config/vitest.browser.config.mjs` include pattern.

### Custom Commands

In addition to the helper functions, these Vitest custom commands are
available (via `commands` from `vitest/browser`):

| Command | Description |
|---|---|
| `fetchPageHTML(url)` | Navigate to URL and return rendered HTML |
| `measureLayout(url, selectors, width?, height?)` | Get bounding rects at a viewport size |
| `measureLayoutMultiWidth(url, selectors, widths, height?)` | Get rects at multiple widths (single page load) |
| `getFeedName(feedId)` | Get current feed name via API |
| `setFeedName(feedId, name)` | Rename a feed via API |
