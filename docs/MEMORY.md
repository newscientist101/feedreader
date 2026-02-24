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
