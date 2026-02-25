# ES Modules Refactor Plan

Refactor `srv/static/app.js` (2277 lines, plain `<script>`) into ES modules.

## Goals

1. **Coverage** — V8 coverage works with real ES module imports
2. **Testability** — direct imports replace the `new Function()` eval hack
3. **Maintainability** — split monolith into focused, navigable modules
4. **Tooling** — static analysis, go-to-definition, dead code detection

## Current State (after Phase 4)

- `app.js` is a 363-line entry point loaded as `<script type="module">`
- 20 ES modules in `srv/static/modules/`, each with a companion `.test.js`
- 390 tests across 20 test files, all using direct ES module imports
- Zero inline event handlers — all use `data-action` + `addEventListener`
- Zero `window.X` global exports
- Import maps provide cache busting for all module files
- 82% statement / 85% function coverage on modules
- 6 `setXxxDeps()` late-binding calls remain (Phase 5 target)

## Proposed Module Structure

```
srv/static/
  app.js              → thin entry point (imports modules, runs init)
  modules/
    api.js            → api() fetch wrapper
    settings.js       → getSetting, saveSetting, applyUserPreferences,
                        applyHideReadArticles, applyHideEmptyFeeds
    utils.js          → formatTimeAgo, formatLocalDate, stripHtml,
                        truncateText, getArticleSortTime
    icons.js          → SVG constant strings
    articles.js       → renderArticleActions, buildArticleCardHtml,
                        renderArticles, updateReadButton,
                        showArticlesLoading, updateAllReadMessage,
                        showReadArticles, showHiddenArticles,
                        processEmbeds, extractYouTubeId
    article-actions.js→ markRead, markUnread, toggleStar, toggleQueue,
                        markReadSilent, openArticle, openArticleExternal,
                        markAsRead, auto-mark-read observer
    pagination.js     → cursor state, loadMoreArticles, checkScrollForMore,
                        updatePaginationCursor, updateEndOfArticlesIndicator
    sidebar.js        → toggleSidebar, setSidebarActive, navigateFolder,
                        toggleFolderCollapse, collapseFolder
    feeds.js          → loadFeedArticles, loadCategoryArticles,
                        refreshFeed, deleteFeed, editFeed, saveFeed,
                        filterFeeds, setFeedCategory, feed error banner,
                        createEditFeedModal, closeEditModal
    folders.js        → openCreateFolderModal, closeCreateFolderModal,
                        submitCreateFolder, renameCategory,
                        unparentCategory, deleteCategory
    counts.js         → updateCounts, updateFeedStatusCell,
                        updateFeedErrors
    views.js          → setView, getViewScope, initView,
                        migrateLegacyViewDefaults, getDefaultViewForScope,
                        applyDefaultViewForScope
    drag-drop.js      → initDragDrop, initFolderDragDrop,
                        syncFolderOrder, reorderElements,
                        getDragAfterElementAmongSiblings
    opml.js           → exportOPML, importOPML
    queue.js          → queuedArticleIds, queuedIdsReady, queueNext
    settings-page.js  → initSettingsPage, runCleanup, newsletter functions
    offline.js        → service worker, online/offline handling,
                        cacheQueueForOffline, replayPendingActions
    dropdown.js       → toggleDropdown, click-outside listener
    timestamps.js     → initTimestampTooltips
    scraper-page.js   → scraper tab switching, schema panel, config modal
                        (currently inline in scrapers.html <script>)
```

## Migration Strategy

### Phase 1: Infrastructure (non-breaking)

1. Create `modules/` directory
2. Extract pure utility modules first (`utils.js`, `icons.js`, `api.js`)
   — these have zero DOM dependencies and are easiest to test
3. Update `vitest.config.mjs` to include `modules/` in coverage
4. Write module-style tests that directly import from the new files
5. Verify old tests still pass (both test styles coexist temporarily)

### Phase 2: Extract stateful modules

6. Extract `settings.js`, `views.js`, `dropdown.js`, `timestamps.js`
7. Extract `articles.js`, `article-actions.js`, `pagination.js`
8. Extract `sidebar.js`, `feeds.js`, `folders.js`, `counts.js`
9. Extract `drag-drop.js`, `opml.js`, `queue.js`
10. Extract `settings-page.js`, `offline.js`

During this phase, `app.js` gradually shrinks as code moves out. Each
extracted module exports its public functions. The entry point imports
and either calls them or registers them.

### Phase 3: Eliminate inline handlers

11. Replace `onclick="fn()"` in templates with `data-action` attributes
    or stable selectors, and wire them up via `addEventListener` in the
    entry point or per-page init functions
12. Replace `onclick` strings built in JS (article cards, modals) with
    DOM creation + `addEventListener`
13. Remove the ESLint `varsIgnorePattern` whitelist
14. Remove all `window.X = X` assignments (nothing needs to be global)

### Phase 4: Finalize

15. Change `<script src="/static/app.js">` to
    `<script type="module" src="/static/app.js">` in `base.html`
16. Delete `test-helper.js` and the eval-based test machinery
17. Rewrite tests as direct ES module imports
18. Verify coverage reports meaningful numbers
19. Update `AGENTS.md` code layout docs

### Phase 5: Eliminate late-bound dependencies

The `setXxxDeps()` pattern was used during Phase 2 to break circular
import dependencies. Of the 6 uses, only 3 are actual cycles — the
other 5 are unnecessary and can be replaced with direct imports. The
3 real cycles can be eliminated through small restructurings.

**Non-circular (replace with direct imports):**

- `article-actions` → `counts.updateCounts` — no cycle
- `article-actions` → `offline.updateQueueCacheIfStandalone` — no cycle
- `counts` → `articles.applyUserPreferences` — no cycle
- `offline` → `counts.updateCounts` — no cycle
- `queue` → `offline.updateQueueCacheIfStandalone` — no cycle

**Real cycles (restructure to eliminate):**

20. `article-actions ↔ articles`: `article-actions` needs
    `updateReadButton` from `articles`, and `articles` imports
    `queuedArticleIds`, `queuedIdsReady`, `initAutoMarkRead` from
    `article-actions`. Fix: extract `updateReadButton` into a small
    shared module (e.g. `read-button.js`) — it's a pure DOM
    manipulation function that updates a button's icon and data
    attributes. Both modules import from it.

21. `articles ↔ pagination`: `pagination` imports
    `buildArticleCardHtml`, `processEmbeds`, `applyUserPreferences`,
    `getShowingHiddenArticles` from `articles`, while `articles`
    needs `updatePaginationCursor`, `updateEndOfArticlesIndicator`,
    `setPaginationState` from `pagination`. Fix: the pagination state
    functions that `articles` needs are really part of "rendering
    articles" — move them into `articles.js`, making `pagination.js`
    a pure "load more" module that calls into `articles`.
    Alternatively, extract the shared rendering functions
    (`buildArticleCardHtml`, `processEmbeds`) into a lower-level
    `article-rendering.js`.

22. `counts ↔ feeds` and `sidebar ↔ feeds`: `feeds` imports
    `updateCounts` from `counts` and `setSidebarActive` from
    `sidebar`. `counts` needs `showFeedErrorBanner` /
    `removeFeedErrorBanner` from `feeds`. `sidebar` needs
    `loadCategoryArticles` from `feeds`. Fix: move
    `showFeedErrorBanner`/`removeFeedErrorBanner` into `counts.js`
    (or a new `feed-errors.js`) since they're only called from
    `updateFeedErrors` in counts. For sidebar → feeds,
    `loadCategoryArticles` is a high-level orchestration function —
    keep the callback from `app.js` (this is legitimate top-down
    wiring, not a circular dependency hack).

After this phase, the only remaining `setXxxDeps` call should be
`setSidebarLoadCategory` in `app.js`, which is genuine top-down
wiring from the entry point.

## Key Decisions

### No bundler

Modern browsers handle ES module imports natively. This is a single-user
app behind a proxy — the extra HTTP requests for ~15 module files are
irrelevant. A bundler adds build complexity for no real benefit here.

### Transitional `window` exports

During Phases 2–3, functions still called from `onclick` attributes must
remain on `window`. The entry point can do:
```js
import { toggleStar } from './modules/article-actions.js';
window.toggleStar = toggleStar;
```
This is removed in Phase 3 when inline handlers are eliminated.

### Shared state

Mutable state (`paginationCursorTime`, `queuedArticleIds`, etc.) lives
in the module that owns it, exposed via getter/setter functions rather
than direct variable access. This makes dependencies explicit.

### Test migration

Tests can be migrated incrementally. As each module is extracted, its
tests switch from `window.fn()` to `import { fn }`. The eval-based
test helper remains functional for not-yet-extracted code.

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Circular dependencies between modules | 3 real cycles identified (see Phase 5); resolved via small restructurings — extract shared leaf modules or consolidate tightly-coupled state |
| Breaking inline handlers mid-migration | Transitional `window` exports keep everything working |
| `<script type="module">` is deferred (runs later than classic scripts) | This app already uses `DOMContentLoaded`; deferred execution is equivalent |
| `type="module"` doesn't run in very old browsers | Not a concern — single-user app on modern browser |
| Large diff is hard to review | Phase-by-phase commits, each independently passing `make check` |
