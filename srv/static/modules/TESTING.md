# JS Test Conventions

## Fetch Mocking

Use **`vi.spyOn(globalThis, 'fetch')`** to mock HTTP calls. This auto-restores
via `vi.restoreAllMocks()` in afterEach.

```js
// ✅ Correct: spy-based, auto-restores
vi.spyOn(globalThis, 'fetch').mockResolvedValue({
    ok: true,
    json: () => Promise.resolve({ data: [] }),
});

// ✅ Also correct for complex implementations
vi.spyOn(globalThis, 'fetch').mockImplementation(async (url) => ({
    ok: true,
    json: async () => ({ data: url.includes('feeds') ? feeds : [] }),
}));

// ❌ Don't use: doesn't auto-restore, leaks between tests
window.fetch = vi.fn().mockResolvedValue(...);

// ❌ Don't use: unnecessary when vi.spyOn works
vi.stubGlobal('fetch', vi.fn().mockResolvedValue(...));
```

### Which strategy to use

- **Modules that import `api()`**: mock `./api.js` via `vi.mock()` — tests
  the module's logic, not HTTP plumbing.
- **`api.test.js` itself**: spy on `globalThis.fetch` — tests the real HTTP layer.
- **Modules that call `fetch()` directly**: spy on `globalThis.fetch`.

## Environment

Tests run in **happy-dom** (not jsdom). happy-dom normalizes self-closing
HTML/SVG tags, so use `toContain` with distinctive substrings rather than
exact innerHTML matching.

## Auto-Mocks (`__mocks__/`)

Common modules have auto-mock files in `__mocks__/`. To use them, call
`vi.mock('./module.js')` without a factory — Vitest picks up the
corresponding `__mocks__/module.js` file automatically.

Available auto-mocks:
- `api.js` — `api()` as `vi.fn()`
- `articles.js` — all exports as `vi.fn()` with sensible defaults
- `counts.js` — `updateCounts()` as `vi.fn()`
- `feed-errors.js` — `showFeedErrorBanner()`, `removeFeedErrorBanner()`
- `modal.js` — `openModal()`, `closeModal()`
- `offline.js` — all exports as `vi.fn()`
- `pagination.js` — `updatePaginationCursor()`, etc.
- `read-button.js` — `updateReadButton()`
- `settings.js` — `getSetting(key, def)` returns `def`, `saveSetting()` no-op
- `sidebar.js` — all exports as `vi.fn()`
- `toast.js` — `showToast()`
- `views.js` — all exports as `vi.fn()`

If your test needs a real implementation alongside stubs (e.g. real
`getSetting` with mocked `saveSetting`), use `importOriginal`:

```js
vi.mock('./settings.js', async (importOriginal) => {
    const real = await importOriginal();
    return { ...real, saveSetting: vi.fn() };
});
```

## Shared Test Helpers (`test-helpers.js`)

- `MockIntersectionObserver` — Drop-in IntersectionObserver for happy-dom.
  Supports `observe()`, `disconnect()`, and `_fire(entries)` for manual
  trigger.

### Fixture Factories

- **`makeFetchResponse(data, opts)`** — Build a fake Response-like object
  with `ok`, `status`, `json()`, and `text()` methods. Eliminates
  `{ ok: true, json: () => Promise.resolve(...) }` boilerplate.

  ```js
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse({ articles: [] }));
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(makeFetchResponse('err', { ok: false, status: 500 }));
  ```

- **`makeCountsResponse(overrides)`** — Build a counts API response with
  all required fields defaulting to zero/empty. Compose with
  `makeFetchResponse()` for a full mock:

  ```js
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      makeFetchResponse(makeCountsResponse({ unread: 5, feeds: { '10': 3 } })),
  );
  ```

- **`makeArticle(overrides)`** — Build an article object with sensible
  defaults (`id: 1`, `title: 'Test Article'`, `is_read: false`, etc.).
  Use for `buildArticleCardHtml()`, `renderArticles()`, and anywhere
  article shapes are needed:

  ```js
  buildArticleCardHtml(makeArticle({ title: 'Custom', is_read: true }));
  ```
