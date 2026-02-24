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

**Next run:** Extract `articles.js`. This is complex because `renderArticles` depends on pagination state, `processEmbeds`, `applyUserPreferences`, `initAutoMarkRead`, and `queuedArticleIds`/`queuedIdsReady`. Consider extracting pagination.js first or simultaneously to make articles.js cleaner. The `showHiddenArticles` function also calls `api` and `renderArticles` (self-referential within the module). The duplicate utility functions (`formatTimeAgo`, `stripHtml`, `truncateText`, `getArticleSortTime`) in app.js can be removed once articles.js imports them from utils.js.
