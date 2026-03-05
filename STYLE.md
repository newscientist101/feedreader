# Style Guide

Coding conventions for this project. Agents and contributors must follow these
rules. Linters catch some of them automatically; the rest require discipline.

## Go

### Constants over magic strings

- Extract repeated string literals into named constants.
- If two packages use the same value (e.g., a User-Agent string), put the
  constant in a shared location and import it. Don't duplicate.

### Background goroutines

- Always use `s.bgCtx` (the server's background context) for goroutines that
  outlive a request. Never use `context.Background()` inside the server.
- Goroutines started in handlers must be cancellable via context so tests
  don't leak goroutines.

### Error handling

- Don't ignore errors from `Close()`, `Write()`, etc. in production code.
  Use `_ = foo.Close()` only when the error is genuinely unactionable, and
  add a comment explaining why.
- In tests, use `require.NoError(t, err)` for setup steps that must succeed.

### Database

- All queries go through sqlc. Edit `db/queries/*.sql`, then run
  `go generate ./db/...`. Never hand-edit `db/dbgen/`.
- Migrations are numbered sequentially (`001`, `002`, …). Never renumber.

## JavaScript

### Module structure

- All module-level `let` variables must be declared **at the top of the file**,
  before any function that references them. This prevents ESLint
  `no-use-before-define` warnings and keeps state easy to find.

  ```js
  // ✅ Good — state at top
  let _listenerAC = null;
  let currentPage = 0;

  export function _resetState() {
      if (_listenerAC) { _listenerAC.abort(); _listenerAC = null; }
      currentPage = 0;
  }
  ```

  ```js
  // ❌ Bad — variable declared after function that uses it
  export function _resetState() {
      if (_listenerAC) _listenerAC.abort();  // ESLint warning
  }

  let _listenerAC = null;  // declared too late
  ```

### AbortController pattern

All `init*Listeners` functions must use AbortController for cleanup:

```js
let _listenerAC = null;  // at top of file with other state

export function initSomethingListeners() {
    if (_listenerAC) _listenerAC.abort();
    _listenerAC = new AbortController();
    const signal = _listenerAC.signal;

    document.addEventListener('click', handler, { signal });
    // For capture listeners:
    document.addEventListener('focus', handler, { capture: true, signal });
}
```

- The AC variable must also be aborted in any `_reset*State()` function.
- Never pass `true` as the third argument to `addEventListener` — always use
  the options object form `{ capture: true, signal }` so the signal works.

### Event handlers

- No inline `onclick` attributes in HTML. Use `data-action` attributes with
  delegated `addEventListener`.
- All event wiring happens in `init*` functions, not at module top level.

## JavaScript Tests

### `vi.waitFor()` must always specify `{ interval: 1 }`

The default polling interval is 50ms, which wastes wall-clock time when mocked
promises resolve instantly. **Every** `vi.waitFor` call must include
`{ interval: 1 }`:

```js
// ✅ Good
await vi.waitFor(() => expect(fetch).toHaveBeenCalled(), { interval: 1 });

// ❌ Bad — wastes 50ms per call
await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
```

### No real-time sleeps in tests

Do not use `setTimeout` with a non-zero delay to wait for async work:

```js
// ❌ Bad — wastes 10ms of real time
await new Promise(r => setTimeout(r, 10));

// ✅ Good — flush microtask queue with zero delay
await new Promise(r => setTimeout(r, 0));

// ✅ Also good — explicit microtask flush
await new Promise(queueMicrotask);
```

If you need to wait for fire-and-forget async handlers triggered by DOM events,
a zero-delay setTimeout or `await vi.runAllTimersAsync()` (with fake timers)
is sufficient when all I/O is mocked.

### Scope test setup to where it's needed

Don't put expensive setup in a global `beforeEach` if only some `describe`
blocks need it. Pure function tests (e.g., `extractYouTubeId`,
`buildArticleCardHtml`) shouldn't pay for DOM setup and state resets.

### Keep DOM rendering proportional to what you're testing

If a test only checks a side effect (e.g., pagination state), don't render 50
full article cards. Mock the rendering function or use a minimal stub.

### Test file naming

- Unit tests: `module-name.test.js` (co-located in `srv/static/modules/`)
- Browser-mode tests: `*.browser.test.js`
- Layout tests: `tests/layout/*.test.js`

## CSS

- All styles in `srv/static/style.css` (single file, no preprocessor).
- Use `stylelint` conventions (see `.stylelintrc.json`).

## HTML Templates

- Server-rendered Go templates in `srv/templates/`.
- `base.html` is the shared layout — all pages extend it.
- Import map in `base.html` must appear **before** any `<link rel="modulepreload">`
  tags (per HTML spec).

## Feed Sources

- New feed types use the source registry in `srv/sources/`. Implement the
  `Source` interface and register in `defaults.go`.
- Client-side URL construction goes in `feeds.js` (like Reddit, YouTube).
- Server-side auto-naming goes in the source's `ResolveName` method.
- If a new feed type is plain RSS/Atom under the hood, set `FeedType()` to
  return `"rss"` — don't create a new fetcher path unnecessarily.

## General

- Run `make check` before committing. It catches most issues.
- Run `make fmt` if `make fmt-check` fails.
- Don't duplicate logic — extract constants, helpers, and shared functions.
- When fixing a pattern in one file, grep for the same pattern in other files
  and fix them all in the same change.
