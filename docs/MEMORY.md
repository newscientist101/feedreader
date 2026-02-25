# ES Modules Migration — Working Memory

## Run 1 — Phase 1 complete

**Completed:** All 7 Phase 1 tasks.

- Created `srv/static/modules/` directory
- Extracted `icons.js` (7 SVG constants), `utils.js` (5 functions + PREVIEW_TEXT_LIMIT), `api.js` (fetch wrapper)
- Added `utils.test.js` (15 tests) and `api.test.js` (5 tests) with direct ES module imports
- Added ESLint flat-config override for `modules/**/*.js` with `sourceType: "module"`
- `app.js` is **unchanged** — functions are duplicated in modules for now.

## Run 2 — Phase 2 started (settings, dropdown, timestamps)

**Completed:**
- Switched `base.html` to `<script type="module">` for `app.js` — moved up from Phase 4 since imports require module context. Verified deferred execution is equivalent (app uses `DOMContentLoaded`).
- Added comprehensive `window.X = X` transitional exports at bottom of `app.js` for all functions called from inline `onclick`/`onchange`/`onsubmit` handlers in templates and JS-built HTML strings (~40 functions).
- Updated `test-helper.js`: `loadApp()` is now **async**. It strips `import` lines from `app.js` (replacing them with `const { ... } = window;` destructuring) and pre-loads all modules via `preloadModules()` before eval.
- Updated `app.test.js`: `beforeEach` is now `async` to `await loadApp()`.
- Updated ESLint config: added `app.js` to the `sourceType: "module"` override alongside `modules/**/*.js`.
- Extracted `modules/settings.js`: `getSetting`, `saveSetting`, `applyHideReadArticles`, `applyHideEmptyFeeds`. Note: `applyUserPreferences` was **not** extracted — it calls `updateAllReadMessage()` which belongs to `articles.js` (not yet extracted). It stays in `app.js` for now.
- Extracted `modules/dropdown.js`: `toggleDropdown`, `initDropdownCloseListener`. The click-outside listener is now called explicitly via `initDropdownCloseListener()` at module top level in `app.js`.
- Extracted `modules/timestamps.js`: `initTimestampTooltips`. Imports `formatLocalDate` from `utils.js`.
- Removed `formatLocalDate` from `app.js` (already in `modules/utils.js`, only used by `initTimestampTooltips`).
- Removed `initTimestampTooltips` and `toggleDropdown` from `UNTESTED_FUNCTIONS` in `app.test.js` (no longer in `app.js`).
- Added missing `window.saveFeed` and `window.closeEditModal` exports (called from JS-built onclick strings).

**Key patterns established:**
1. Extract function to module → remove from `app.js` → add `import` at top of `app.js`
2. `window.X = X` at bottom of `app.js` for anything called from inline handlers
3. `test-helper.js` auto-loads all modules and strips imports — no manual module list needed
4. When removing a function from app.js, check if it's in `UNTESTED_FUNCTIONS` in `app.test.js` and remove it

**Next run:** Continue Phase 2 — extract `views.js`, `sidebar.js`, `articles.js`. Note that `applyUserPreferences` should move to `settings.js` once `articles.js` is extracted (it needs `updateAllReadMessage` from there). The duplicate utility functions (`formatTimeAgo`, `stripHtml`, `truncateText`, `getArticleSortTime`) are still in `app.js` — they can be removed and imported from `modules/utils.js` as part of extracting the modules that use them.

## Run 3 — Phase 2 continued (views, sidebar)

**Completed:**
- Extracted `modules/views.js`: `setView`, `getViewScope`, `initView`, `migrateLegacyViewDefaults`, `getDefaultViewForScope`, `applyDefaultViewForScope`. Imports `getSetting`/`saveSetting` from settings.js.
- Extracted `modules/sidebar.js`: `toggleSidebar`, `setSidebarActive`, `navigateFolder`, `toggleFolderCollapse`, `collapseFolder`. Uses late-binding pattern (`setSidebarLoadCategory`) to avoid circular dep with feeds module — `navigateFolder` needs `loadCategoryArticles` which is still in app.js.
- Removed corresponding entries from UNTESTED_FUNCTIONS in app.test.js.
- Only imported the functions actually used in app.js (not all exports) to keep eslint happy.

**Key decisions:**
- `navigateFolder` calls `loadCategoryArticles` which lives in app.js (will be in feeds.js later). Used a `setSidebarLoadCategory(fn)` setter pattern to inject the dependency from app.js, avoiding circular imports.
- `applyDefaultViewForScope` is still needed in app.js (called from `loadCategoryArticles` and `loadFeedArticles`), so it's imported.
- `getViewScope`, `migrateLegacyViewDefaults`, `getDefaultViewForScope`, `collapseFolder` are NOT imported into app.js — they're only used within their own modules.

## Run 4 — Phase 2 continued (articles, article-actions, pagination)

**Completed:**
- Extracted `modules/articles.js`: `renderArticleActions`, `buildArticleCardHtml`, `renderArticles`, `updateReadButton`, `showArticlesLoading`, `updateAllReadMessage`, `showReadArticles`, `showHiddenArticles`, `processEmbeds`, `extractYouTubeId`, `applyUserPreferences`, `getIncludeReadUrl`, `showingHiddenArticles` state.
- Extracted `modules/article-actions.js`: `markRead`, `markUnread`, `toggleStar`, `toggleQueue`, `markAsRead`, `markReadSilent`, `openArticle`, `openArticleExternal`, `markCardAsRead`, `initAutoMarkRead`, `observeNewArticles`, `flushMarkReadQueue`, `findNextUnreadFolder`. Also owns `queuedArticleIds`, `queuedIdsReady`, auto-mark-read observer state.
- Extracted `modules/pagination.js`: `updateEndOfArticlesIndicator`, `updatePaginationCursor`, `getPaginationUrl`, `loadMoreArticles`, `checkScrollForMore`, `PAGE_SIZE`, pagination cursor state.
- Removed duplicate utility functions (`formatTimeAgo`, `stripHtml`, `truncateText`, `getArticleSortTime`, `api`, all SVG icon constants, `PREVIEW_TEXT_LIMIT`) from app.js — they're now only in their respective modules.
- Removed `markRead`, `markUnread`, `toggleStar`, `toggleQueue`, `showArticlesLoading`, `updateReadButton` from UNTESTED_FUNCTIONS in app.test.js (no longer in app.js).
- Added `articles.test.js` (29 tests), `article-actions.test.js` (19 tests), `pagination.test.js` (18 tests).
- Updated `test-helper.js`: `replaceImports()` now handles multi-line imports using a regex replacement. Added property accessors (defineProperty shims) for internal module state (`autoMarkReadObserver`, `_markReadQueue`, pagination state, `showingHiddenArticles`) so legacy app.test.js tests still work.
- Added `_resetXxxState()` functions to each module so test-helper can reset module state between loadApp() calls (modules persist across tests, unlike the old eval'd code).
- app.js DOMContentLoaded now uses `setQueuedArticleIds`/`setQueuedIdsReady` setters instead of directly assigning module variables.
- app.js pagination init uses `setPaginationState()` instead of direct variable assignment.

**Key patterns:**
- Circular dependencies between articles ↔ article-actions ↔ pagination resolved via:
  - `articles.js` imports from `article-actions.js` (queuedArticleIds, initAutoMarkRead)
  - `pagination.js` imports from both `articles.js` and `article-actions.js`
  - `article-actions.js` uses late-bound deps (`setArticleActionDeps`) for `updateReadButton` (from articles) and `updateCounts`/`updateQueueCacheIfStandalone` (still in app.js)
  - `articles.js` uses late-bound deps (`setArticlesDeps`) for pagination functions
- Module internal state that tests need: exposed via `_getXxx`/`_setXxx` accessors, bridged to `window.xxx` via `Object.defineProperty` in test-helper. Will be removed in Phase 4 when tests migrate to direct imports.
- `_resetXxxState()` pattern needed for each module with mutable state.

**app.js reduced from 2124 → 1518 lines.**

**Next run:** Continue Phase 2 — extract `feeds.js`, `folders.js`, `counts.js`. These are relatively straightforward API-calling functions. `feeds.js` will include `loadFeedArticles`, `loadCategoryArticles`, `refreshFeed`, `deleteFeed`, `editFeed`, `saveFeed`, `filterFeeds`, `setFeedCategory`, `showFeedErrorBanner`, `removeFeedErrorBanner`, `createEditFeedModal`, `closeEditModal`. Note that `loadFeedArticles`/`loadCategoryArticles` depend on `renderArticles`, `showArticlesLoading`, `applyDefaultViewForScope`, `setSidebarActive` — all already in modules.

## Run 5 — Phase 2 continued (feeds, folders, counts)

**Completed:**
- Extracted `modules/feeds.js`: `showFeedErrorBanner`, `removeFeedErrorBanner`, `loadCategoryArticles`, `loadFeedArticles`, `refreshFeed`, `deleteFeed`, `filterFeeds`, `createEditFeedModal`, `editFeed`, `closeEditModal`, `saveFeed`, `setFeedCategory`. Imports from `api`, `articles`, `sidebar`, `views`, `counts`.
- Extracted `modules/folders.js`: `openCreateFolderModal`, `closeCreateFolderModal`, `submitCreateFolder`, `renameCategory`, `unparentCategory`, `deleteCategory`. Only depends on `api`.
- Extracted `modules/counts.js`: `updateCounts`, `updateFeedStatusCell`, `updateFeedErrors`. Uses `setCountsDeps` late-binding for `showFeedErrorBanner`, `removeFeedErrorBanner` (from feeds.js) and `applyUserPreferences` (from articles.js) to avoid circular imports.
- Added `feeds.test.js` (26 tests), `folders.test.js` (15 tests), `counts.test.js` (14 tests).
- Removed 18 entries from UNTESTED_FUNCTIONS in app.test.js (functions no longer in app.js).
- Dependency graph: feeds.js → counts.js (direct), counts.js → feeds.js (late-bound via setCountsDeps).

**app.js reduced from 1518 → 926 lines.**

**Next run:** Continue Phase 2 — extract `drag-drop.js`, `opml.js`, `queue.js`. Then `settings-page.js` and `offline.js`. After those, Phase 2 will be complete except for the "Wire transitional window.X" and "Verify make check" tasks (which are ongoing and can be checked off). The remaining functions in app.js are: `exportOPML`, `importOPML`, drag-drop functions, settings-page functions, offline/service-worker functions, and the DOMContentLoaded/form-handler blocks.

## Run 6 — Phase 2 continued (drag-drop, opml, settings-page)

**Completed:**
- Extracted `modules/drag-drop.js`: `initFolderDragDrop`, `initDragDrop`, `syncFolderOrder`, `reorderElements`, `getDragAfterElementAmongSiblings`. Imports `api` from api.js.
- Extracted `modules/opml.js`: `exportOPML`, `importOPML`. No module dependencies (uses raw `fetch` for FormData upload).
- Extracted `modules/settings-page.js`: `initSettingsPage`, `runCleanup`, `loadNewsletterAddress`, `generateNewsletterAddress`, `showNewsletterAddress`, `copyNewsletterAddress`. Imports `api` and `getSetting`.
- Added `drag-drop.test.js` (16 tests), `opml.test.js` (5 tests), `settings-page.test.js` (17 tests).
- Emptied `UNTESTED_FUNCTIONS` in `app.test.js` — all offline functions already had tests from a previous run, so they didn't need skip entries.

**app.js reduced from 926 → 560 lines.**

**Remaining in app.js:**
- Offline/PWA functions: `initOfflineSupport`, `cacheQueueForOffline`, `handleOnlineStateChange`, `showOfflineBanner`, `disableNonQueueUI`, `enableAllUI`, `replayPendingActions`, `updateQueueCacheIfStandalone`, `_isStandalone`
- DOMContentLoaded init blocks (2 of them)
- Form handlers (add-feed form, search input)
- Dragstart prevention for chevrons
- Scroll listener for pagination
- Wire-up calls for late-bound deps
- Transitional `window.X = X` exports

**Next run:** Extract `modules/queue.js` and `modules/offline.js`. Note: the `queue.js` task in the TODO refers to `queuedArticleIds`, `queuedIdsReady`, `queueNext` — but `queuedArticleIds` and `queuedIdsReady` are already in `article-actions.js`, and `queueNext` is inline in `queue.html` template. The queue.js task may be best marked as N/A or adapted to just extract any remaining queue-specific code. The `offline.js` extraction is straightforward — all offline functions are still in app.js.

## Run 7 — Phase 2 completed (queue, offline)

**Completed:**
- Extracted `modules/queue.js`: `initQueuePage`, `queueNext`, `setQueueDeps`. Moved `queueNext` out of inline `<script>` in `queue.html` into the module. Queue article IDs are now passed via a `<script id="queue-data" type="application/json">` element instead of inline JS. Removed `onclick="queueNext()"` from the button; `initQueuePage()` wires it via `addEventListener`.
- Extracted `modules/offline.js`: `initOfflineSupport`, `cacheQueueForOffline`, `handleOnlineStateChange`, `showOfflineBanner`, `disableNonQueueUI`, `enableAllUI`, `replayPendingActions`, `updateQueueCacheIfStandalone`, `_isStandalone`, `setOfflineDeps`. Uses `setOfflineDeps` late-binding for `updateCounts`.
- Added `queue.test.js` (10 tests), `offline.test.js` (30 tests).
- Removed offline tests from `app.test.js` (they're now in `offline.test.js`).
- Marked all 4 remaining Phase 2 tasks complete (queue, offline, window exports, verify).

**app.js reduced from 560 → 386 lines.** Phase 2 is now complete.

**Remaining in app.js (entry point only):**
- Import statements (~40 lines)
- Module dependency wiring (~20 lines)
- Two DOMContentLoaded blocks: main init + form handlers (~180 lines)
- Dragstart prevention listener
- Scroll listener for pagination
- Transitional `window.X = X` exports (~40 lines)

**Next run:** Begin Phase 3 — eliminate inline event handlers. Start with templates that have the fewest handlers (`queue.html` already done during this run, `category_settings.html` has 2). Then tackle `base.html` (4), working up to the larger files.

**Total tests: 413 (111 in app.test.js, 302 across 19 module test files).**

## Run 8 — Phase 3 started (audit, base.html, category_settings.html)

**Completed:**
- Audited all inline handlers across 7 template files. Exact counts: base.html (4), index.html (18), feeds.html (15), scrapers.html (34), settings.html (16), category_settings.html (2), queue.html (0). Total: 89 in templates. Plus ~20 inline onclick strings built in JS modules (articles.js, feeds.js).
- Replaced 4 inline handlers in `base.html` with `data-action` attributes: `toggle-sidebar` (menu button + overlay), `toggle-folder` (chevron), `navigate-folder` (folder link). Added `initSidebarListeners()` to sidebar.js using delegated event listeners on document. Chevron uses capture phase so `stopPropagation()` works correctly.
- Replaced 2 inline handlers in `category_settings.html`: `unparentCategory` button uses `data-action="unparent-category"`, `deleteExclusion` button uses `data-action="delete-exclusion"`. Both wired via a delegated click handler in the page's `<script>` block. Removed the standalone `deleteExclusion()` function.
- Removed `window.toggleSidebar`, `window.toggleFolderCollapse`, `window.navigateFolder` from app.js transitional exports (no longer needed since handlers are delegated).
- Added 6 new tests to sidebar.test.js for `initSidebarListeners` (toggle-sidebar open/close, toggle-folder expand/stopPropagation, navigate-folder SPA/non-SPA).

**Key patterns established for Phase 3:**
- Template inline handlers → `data-action="action-name"` + `data-*` attributes for parameters
- Delegated listeners via `document.addEventListener('click', ...)` matching `[data-action]`
- For handlers that need `stopPropagation`, use capture phase (`addEventListener(..., true)`)
- Tests for delegated listeners: call `initXxxListeners()` once (not per-test, since document listeners accumulate), set up DOM per test
- Page-specific `<script>` blocks (like category_settings.html) can wire their own data-action handlers locally

**Next run:** Continue Phase 3 — tackle `index.html` (18 handlers) and `queue.html` (0 — already done). Index.html handlers include view switchers, mark-as-read dropdown, feed action buttons, and article body/link clicks. Consider grouping: view buttons → views.js `initViewListeners`, mark-read buttons → article-actions.js, feed buttons → feeds.js, article clicks → articles.js.

**Total tests: 419 (111 app.test.js + 308 across module test files).**
