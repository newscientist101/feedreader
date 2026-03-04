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
