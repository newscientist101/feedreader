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

## Run 9 — Phase 3 continued (index.html, queue.html)

**Completed:**
- Replaced all 18 inline handlers in `index.html` with `data-action` attributes and delegated event listeners.
- Confirmed `queue.html` already had 0 inline handlers (done in Run 7). Marked task complete.
- Added delegated listener functions to 5 modules:
  - `views.js`: `initViewListeners()` — delegates clicks on `.view-toggle [data-view]` buttons
  - `dropdown.js`: `initDropdownListeners()` — delegates clicks on `.dropdown-toggle` buttons
  - `article-actions.js`: `initArticleActionListeners()` — delegates `mark-as-read`, `open-external`, `mark-read-silent`, article-body clicks, content-preview clicks, feed-name stopPropagation
  - `feeds.js`: `initFeedActionListeners()` — delegates `edit-feed` and `refresh-feed` buttons
  - `articles.js`: `initArticleListListeners()` — delegates `show-hidden-articles` and `show-read-articles` buttons
- Also updated JS-built HTML in `articles.js` (`buildArticleCardHtml`, `renderArticles`, `updateAllReadMessage`) and `feeds.js` (`showFeedErrorBanner`) to use `data-action` attributes instead of inline onclick strings — these are covered by the same delegated listeners.
- Removed 8 `window.X` transitional exports that are no longer needed: `toggleDropdown`, `setView`, `markAsRead`, `showHiddenArticles`, `showReadArticles`, `openArticle`, `openArticleExternal`, `markReadSilent`.
- Removed corresponding unused imports from `app.js`.
- Added 16 new tests across 5 test files for the new delegated listener functions.
- Updated `feeds.test.js` to expect `data-action` attributes instead of `onclick` in banner HTML.

**Remaining JS-built onclick strings (for the separate TODO task):**
- `articles.js` `renderArticleActions()`: `markRead`/`markUnread`, `toggleStar`, `toggleQueue` still use `onclick` in HTML strings
- `articles.js` `updateReadButton()`: sets `onclick` attribute
- `feeds.js` `createEditFeedModal()`: `closeEditModal`, `saveFeed` onsubmit
- `article-actions.js` `toggleStar`/`toggleQueue`: use `querySelectorAll('[onclick="..."]')` to find buttons

**Window exports still needed** (for feeds.html, scrapers.html, settings.html inline handlers + JS-built onclick):
`api`, `getSetting`, `saveSetting`, `saveFeed`, `applyUserPreferences`, `applyHideReadArticles`, `applyHideEmptyFeeds`, `openCreateFolderModal`, `closeCreateFolderModal`, `closeEditModal`, `submitCreateFolder`, `deleteCategory`, `deleteFeed`, `editFeed`, `exportOPML`, `importOPML`, `filterFeeds`, `markRead`, `markUnread`, `refreshFeed`, `renameCategory`, `runCleanup`, `setFeedCategory`, `toggleStar`, `toggleQueue`, `unparentCategory`, `copyNewsletterAddress`, `generateNewsletterAddress`, `updateQueueCacheIfStandalone`

**Next run:** Continue Phase 3 — tackle `feeds.html` (15 handlers) or `settings.html` (16 handlers). These are smaller than `scrapers.html` (34). Consider also tackling the "Replace onclick strings built in JS" task since several have already been partially done.

**Total tests: 435 (111 in app.test.js, 324 across 20 module test files).**

## Run 10 — Phase 3 continued (feeds.html, settings.html)

**Completed:** Replaced 15 handlers in feeds.html and 16 in settings.html. See git log for details.

## Run 11 — Phase 3 completed (scrapers.html, JS-built onclick, window exports, ESLint)

**Completed:**
- **scrapers.html (34 inline handlers):** Created `modules/scraper-page.js` with all scraper page logic (AI status check, AI form, manual form, tab switching, schema panel, edit/delete/save scraper, insert-field helper). Replaced all 34 inline handlers with `data-action`/`data-*` attributes. Removed entire `<script>` block from template. Added `initScraperPage()` + `initScraperPageListeners()` called from app.js.
- **JS-built onclick strings:** Replaced `onclick` attributes in `renderArticleActions()` (articles.js) with `data-action="toggle-read"`/`data-action="toggle-star"`/`data-action="toggle-queue"` + `data-article-id` + `data-is-read`. Updated `updateReadButton()` to use `data-is-read` attribute instead of `onclick`. Replaced `onclick`/`onsubmit` in `createEditFeedModal()` (feeds.js) with `data-action="close-edit-modal"` and delegated form submit.
- **querySelectorAll lookups:** Replaced `querySelectorAll('[onclick="toggleStar(...)"]')` and `querySelectorAll('[onclick="toggleQueue(...)"]')` in article-actions.js with `data-action`+`data-article-id` selectors.
- **Delegated handlers added:** `toggle-read`/`toggle-star`/`toggle-queue` in `initArticleActionListeners()`, `close-edit-modal`/`edit-feed-form submit` in `initFeedActionListeners()`.
- **Window exports removed:** All 12 `window.X = X` transitional exports removed from app.js. No functions are global anymore.
- **category_settings.html:** Moved inline `<script>` logic into `initCategorySettingsPage()` in folders.js + extended `initFoldersPageListeners()` with delegated handlers for `unparent-category` and `delete-exclusion`. Added `data-category-id` attribute to pass category ID from template.
- **ESLint:** Removed `varsIgnorePattern` whitelist — no longer needed since no functions are called from inline handlers.
- **Module caching fix:** Updated server.go static file handler to use short cache (`max-age=60, must-revalidate`) for files in `modules/` directory, since ES module imports can't use cache-busting query params.
- Added `scraper-page.test.js` (30 tests) covering all scraper page functions and delegated listeners.
- Updated tests in articles.test.js, article-actions.test.js, app.test.js to match new `data-action` output.

**Phase 3 is now complete.** All inline event handlers eliminated across all templates and JS-built HTML. Zero `onclick`/`onchange`/`onsubmit`/`oninput` attributes remain.

**app.js is now 363 lines** (down from 2277 original). It's a pure entry point: imports, dependency wiring, DOMContentLoaded init, form handlers, scroll listener.

**Total tests: 485 (111 in app.test.js, 374 across 21 module test files).**

**Next run:** Begin Phase 4 — finalize. The `<script type="module">` change was already done in Run 2. Remaining: delete test-helper.js and rewrite tests as direct imports, verify coverage, update AGENTS.md and eslint config.

**Known issue:** Browser caches ES module files aggressively. The server now sends `max-age=60, must-revalidate` for module files, but browsers that cached modules before the fix may not see updates immediately. A proper solution would be import maps with version hashes. This can be addressed in Phase 4 or as a follow-up.

## Run 12 — Phase 4 started (test migration, eslint, script type)

**Completed:**
- Confirmed `<script type="module">` was already done in Run 2. Checked off the TODO item.
- Deleted `test-helper.js` and `app.test.js` (the eval-based test infrastructure).
- Migrated 16 unique test scenarios from `app.test.js` to their respective module test files:
  - `articles.test.js`: 4 `renderArticles` tests, 1 `showHiddenArticles` test, 1 `processEmbeds` twitter test
  - `article-actions.test.js`: 4 auto-mark-read integration tests, 2 `markReadSilent` timer/debounce tests, 1 `markAsRead` category test
  - `pagination.test.js`: 1 `loadMoreArticles` success path test, 1 `checkScrollForMore` near-bottom test
  - `feeds.test.js`: 1 `refreshFeed` polling test
- The remaining 95 tests in `app.test.js` were duplicates of existing module tests and were dropped.
- Updated `eslint.config.mjs`: removed `test-helper.js` from ignores, changed `sourceType` to `"module"` for all files (removed the separate script/module override), removed the now-unnecessary `sourceType: "module"` override block.
- Updated `vitest.config.mjs`: removed `test-helper.js` from coverage excludes.

**Test counts: 390 tests across 20 module test files.** Down from 501 (390 module + 111 app.test.js) but no coverage lost — the 111 dropped tests were either duplicates or the meta "test coverage check" which is no longer needed.

**Remaining Phase 4 tasks:**
- Implement import maps with version hashes (cache busting)
- Verify coverage reports meaningful numbers with `npx vitest run --coverage`
- Update `AGENTS.md` code layout section for new module structure
- Final `make check`, commit

**Next run:** Tackle import maps, coverage verification, and AGENTS.md update.

## Run 13 — Phase 4 completed (import maps, coverage, AGENTS.md)

**Completed:**
- **Import maps with version hashes:** Added `moduleImportMap` template function in `server.go` that generates a JSON import map mapping each module's absolute URL (`/static/modules/foo.js`) to its versioned URL (`/static/modules/foo.js?v=hash`). Added `<script type="importmap">` to `base.html` before the module script. Browser now resolves all `import` statements through the import map, getting proper cache busting with immutable caching (`max-age=31536000`). Removed the special short-cache case for `modules/` directory (no longer needed). Added `moduleImportMap` stub to template linter's FuncMap.
- **Coverage verification:** Ran `npx vitest run --coverage`. Results: 82.25% statement / 85.4% function coverage for modules, 74% overall (dragged down by `app.js` entry point at 0% and `script.js`). Coverage reports real, meaningful numbers with direct ES module imports.
- **AGENTS.md update:** Updated Code Layout section to document the `modules/` directory with all 20 modules. Updated Key Patterns to describe the ES module architecture (import maps, data-action delegation, late-bound deps, companion test files).
- Also fixed `$HOME/go/bin` not being in PATH (added to `.profile`).

**All Phase 4 tasks are now complete. The ES modules migration is finished.**

**Final state:**
- 20 ES modules in `srv/static/modules/`, each with a `.test.js` companion
- 390 tests across 20 test files
- `app.js` is a 363-line entry point (down from 2277 original)
- Zero inline event handlers — all use `data-action` + `addEventListener`
- Zero `window.X` global exports
- Import maps provide cache busting for all module files
- Coverage: 82% statements, 85% functions (modules only)

## Run 14 — Phase 5 started (non-circular late-bound deps)

**Completed:**
- Replaced 5 non-circular `setXxxDeps` late-bound dependencies with direct ES module imports:
  - `article-actions.js`: now imports `updateCounts` from `counts.js` and `updateQueueCacheIfStandalone` from `offline.js` directly
  - `counts.js`: now imports `applyUserPreferences` from `articles.js` directly
  - `offline.js`: now imports `updateCounts` from `counts.js` directly
  - `queue.js`: now imports `updateQueueCacheIfStandalone` from `offline.js` directly
- Removed `setOfflineDeps` and `setQueueDeps` entirely (no longer exported or called)
- Simplified `setArticleActionDeps` to only accept `updateReadButton` (the real cycle)
- Simplified `setCountsDeps` to only accept `showFeedErrorBanner`/`removeFeedErrorBanner` (the real cycle)
- Removed corresponding wiring from `app.js`
- Updated 4 test files (article-actions, counts, offline, queue) to use `vi.mock()` for the now-directly-imported modules instead of `setXxxDeps`
- All 387 tests pass, `make check` clean

**Remaining `setXxxDeps` calls in app.js (3):**
1. `setArticleActionDeps({ updateReadButton })` — real cycle: article-actions ↔ articles
2. `setArticlesDeps({ updatePaginationCursor, updateEndOfArticlesIndicator, setPaginationState })` — real cycle: articles ↔ pagination
3. `setCountsDeps({ showFeedErrorBanner, removeFeedErrorBanner })` — real cycle: counts ↔ feeds
4. `setSidebarLoadCategory(...)` — legitimate top-down wiring (not a cycle hack)

**Next run:** Tackle the 3 real cycle eliminations (Phase 5 remaining tasks). Start with the `counts ↔ feeds` cycle since it's simplest — move `showFeedErrorBanner`/`removeFeedErrorBanner` into `counts.js` or a new `feed-errors.js`.

## Run 15 — Phase 5 completed (all 3 real cycles eliminated)

**Completed:**
- **`counts ↔ feeds` cycle:** Extracted `showFeedErrorBanner`/`removeFeedErrorBanner` from `feeds.js` into new `feed-errors.js` leaf module. Both `counts.js` and `feeds.js` now import from `feed-errors.js`. Removed `setCountsDeps` entirely.
- **`article-actions ↔ articles` cycle:** Extracted `updateReadButton` from `articles.js` into new `read-button.js` leaf module. `article-actions.js` imports directly from `read-button.js`. `articles.js` re-exports for backward compat. Removed `setArticleActionDeps` entirely.
- **`articles ↔ pagination` cycle:** Made `articles.js` directly import `updatePaginationCursor`, `updateEndOfArticlesIndicator`, `setPaginationState` from `pagination.js`. The circular import (pagination also imports from articles) works fine because all exports are hoisted `function` declarations. Removed `setArticlesDeps` entirely.
- Created `feed-errors.test.js` (6 tests) and `read-button.test.js` (5 tests).
- Updated `counts.test.js`, `article-actions.test.js`, `articles.test.js`, `feeds.test.js` to use `vi.mock()` instead of `setXxxDeps`.
- Removed unused `updatePaginationCursor` import from `app.js`.

**Phase 5 is now complete.** Only `setSidebarLoadCategory` remains in `app.js` — legitimate top-down wiring, not a cycle hack.

**Total: 394 tests across 22 test files. All `make check` passes clean.**

**app.js is 355 lines** (down from 363).

**Next run:** Begin Phase 6 — clean up app.js entry point. Start by moving the add feed form handler (72 lines) to `feeds.js` as `initAddFeedForm()`, and the search handler (36 lines) to a new `search.js` module.

## Run 16 — Phase 6 started (add feed form, search, feed item clicks)

**Completed:**
- Moved add feed form handler (72 lines) from app.js → `feeds.js` as `initAddFeedForm()`. Handles Reddit URL building, HuggingFace config construction, category assignment.
- Moved feed item click handler (12 lines) from app.js → `feeds.js` as `initFeedItemClickListeners()`. SPA feed navigation on article pages.
- Extracted search handler (36 lines) from app.js → new `search.js` module with `initSearch()`. Self-contained: debounced input, AbortController, original HTML restore, context-scoped search URLs. Added `_resetSearchState()` for test cleanup.
- Created `search.test.js` (9 tests): no-op, listener attachment, restore on clear, short query skip, debounce+fetch, feed/category scoping, abort on new input, error handling.
- Added to `feeds.test.js`: 7 tests for `initAddFeedForm` (no-op, basic RSS, Reddit URL, empty subreddit rejection, HuggingFace daily_papers, category assignment, API failure alert) and 3 tests for `initFeedItemClickListeners` (SPA navigation, normal fallthrough, feed name from data attribute).
- Removed unused `renderArticles` import from app.js.

**app.js reduced from 355 → 172 lines.** Down from 2277 original.

**Total: 413 tests across 23 test files.**

**Next run:** Continue Phase 6 — remaining tasks:
- Sidebar mobile close (8 lines) → `sidebar.js` as part of `initSidebarListeners()`
- Pagination bootstrap + scroll listener (11 lines) → `pagination.js` as `initPagination()`
- Queue hydration (16 lines) → `article-actions.js` as `initQueueState()`
- Drag prevention (4 lines) → `drag-drop.js` as part of init
- Modulepreload hints in base.html
- Final cleanup: remove empty DOMContentLoaded blocks, verify ~120 lines

## Run 17 — Phase 6 continued (sidebar mobile close, pagination init, queue hydration)

**Completed:**
- Moved sidebar mobile close (8 lines) from app.js → `sidebar.js` as `initSidebarMobileClose()`. Attaches click listeners to sidebar links that close sidebar on mobile (≤768px).
- Moved pagination bootstrap + scroll listener (11 lines) from app.js → `pagination.js` as `initPagination()`. Reads initial article cards, sets cursor state, registers scroll listener.
- Moved queue hydration (16 lines) from app.js → `article-actions.js` as `initQueueState(renderArticleActions)`. Fetches queue IDs, populates `queuedArticleIds`, hydrates action-button placeholders. Takes `renderArticleActions` as a parameter to avoid importing from articles.js (which would create a circular import).
- Removed unused `api` import from app.js, removed `toggleSidebar` import (no longer needed directly), removed `checkScrollForMore`/`setPaginationState`/`PAGE_SIZE`/`updateEndOfArticlesIndicator` imports, removed `setQueuedArticleIds`/`setQueuedIdsReady`/`queuedArticleIds` imports.
- Removed `window.addEventListener('scroll', checkScrollForMore)` from bottom of app.js (now inside `initPagination`).
- Added 3 tests to `sidebar.test.js` for `initSidebarMobileClose` (no-op, mobile click, desktop no-op).
- Added 4 tests to `pagination.test.js` for `initPagination` (cursor from last card, PAGE_SIZE boundary, empty list, scroll listener registration).
- Added 3 tests to `article-actions.test.js` for `initQueueState` (fetch+populate, hydrate placeholders, API failure). Used real timers for these tests since they depend on real promise resolution.

**app.js is now 117 lines** (down from 172). Almost entirely imports, init calls, and DOMContentLoaded sequencing.

**Total: 423 tests across 23 test files.**

**Remaining Phase 6 tasks:**
- Drag prevention (4 lines) → `drag-drop.js` as part of init
- Modulepreload hints in base.html
- Remove empty DOMContentLoaded blocks from app.js
- Verify app.js is ~120 lines (currently 117 — ✓)
- Final `make check`

## Run 18 — Phase 6 completed (drag prevention, modulepreload, finalize)

**Completed:**
- Moved drag prevention (4 lines) from app.js → `drag-drop.js` as `initDragPrevention()`. Prevents dragstart on `.folder-chevron` elements via capture-phase document listener.
- Added `modulePreloadTags` template function in `server.go` that generates `<link rel="modulepreload">` tags for all 23 module files with version hashes. Tags are sorted for deterministic output. Added to `base.html` before the import map.
- Cleaned up app.js: removed trailing blank lines and double blank lines.
- Added 2 tests to `drag-drop.test.js` for `initDragPrevention` (chevron prevented, non-chevron allowed).
- Added `modulePreloadTags` stub to template linter's FuncMap.

**app.js is now 126 lines** (down from 2277 original). Pure entry point: imports, listener init calls, DOMContentLoaded sequencing.

**Phase 6 is complete. All phases (1–6) are done.**

**Total: 425 tests across 23 test files. All `make check` passes clean.**

**Only remaining items in TODO are Backlog (outside ES modules scope).**
