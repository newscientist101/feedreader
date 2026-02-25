# ES Modules Refactor — TODO

See `ES_MODULES_PLAN.md` for context and rationale.

## Phase 1: Infrastructure (non-breaking)

- [x] Create `srv/static/modules/` directory
- [x] Extract `modules/icons.js` — SVG constant strings
- [x] Extract `modules/utils.js` — `formatTimeAgo`, `formatLocalDate`, `stripHtml`, `truncateText`, `getArticleSortTime`
- [x] Extract `modules/api.js` — `api()` fetch wrapper
- [x] Add direct-import tests for `utils.js` and `api.js`
- [x] Update `vitest.config.mjs` coverage include to cover `modules/`
- [x] Verify `make check` passes, commit

## Phase 2: Extract stateful modules

- [x] Extract `modules/settings.js` — `getSetting`, `saveSetting`, ~~`applyUserPreferences`~~, `applyHideReadArticles`, `applyHideEmptyFeeds`
- [x] Extract `modules/dropdown.js` — `toggleDropdown`, `initDropdownCloseListener`
- [x] Extract `modules/timestamps.js` — `initTimestampTooltips`
- [x] Extract `modules/views.js` — `setView`, `getViewScope`, `initView`, `migrateLegacyViewDefaults`, `getDefaultViewForScope`, `applyDefaultViewForScope`
- [x] Extract `modules/sidebar.js` — `toggleSidebar`, `setSidebarActive`, `navigateFolder`, `toggleFolderCollapse`, `collapseFolder`
- [x] Extract `modules/articles.js` — `renderArticleActions`, `buildArticleCardHtml`, `renderArticles`, `updateReadButton`, `showArticlesLoading`, `updateAllReadMessage`, `showReadArticles`, `showHiddenArticles`, `processEmbeds`, `extractYouTubeId`
- [x] Extract `modules/article-actions.js` — `markRead`, `markUnread`, `toggleStar`, `toggleQueue`, `markReadSilent`, `openArticle`, `openArticleExternal`, `markAsRead`, auto-mark-read observer (`initAutoMarkRead`, `observeNewArticles`, `flushMarkReadQueue`, `markCardAsRead`)
- [x] Extract `modules/pagination.js` — cursor state, `loadMoreArticles`, `checkScrollForMore`, `updatePaginationCursor`, `updateEndOfArticlesIndicator`, `getPaginationUrl`
- [x] Extract `modules/feeds.js` — `loadFeedArticles`, `loadCategoryArticles`, `refreshFeed`, `deleteFeed`, `editFeed`, `saveFeed`, `filterFeeds`, `setFeedCategory`, `showFeedErrorBanner`, `removeFeedErrorBanner`, `createEditFeedModal`, `closeEditModal`
- [x] Extract `modules/folders.js` — `openCreateFolderModal`, `closeCreateFolderModal`, `submitCreateFolder`, `renameCategory`, `unparentCategory`, `deleteCategory`
- [x] Extract `modules/counts.js` — `updateCounts`, `updateFeedStatusCell`, `updateFeedErrors`
- [x] Extract `modules/drag-drop.js` — `initDragDrop`, `initFolderDragDrop`, `syncFolderOrder`, `reorderElements`, `getDragAfterElementAmongSiblings`
- [x] Extract `modules/opml.js` — `exportOPML`, `importOPML`
- [x] Extract `modules/queue.js` — `initQueuePage`, `queueNext`, `setQueueDeps`
- [x] Extract `modules/settings-page.js` — `initSettingsPage`, `runCleanup`, `loadNewsletterAddress`, `generateNewsletterAddress`, `showNewsletterAddress`, `copyNewsletterAddress`
- [x] Extract `modules/offline.js` — `initOfflineSupport`, `cacheQueueForOffline`, `handleOnlineStateChange`, `showOfflineBanner`, `disableNonQueueUI`, `enableAllUI`, `replayPendingActions`, `updateQueueCacheIfStandalone`
- [x] Wire transitional `window.X = X` exports in `app.js` for all functions still called from `onclick`
- [x] Verify `make check` passes after each extraction, commit per module or small batch

## Phase 3: Eliminate inline event handlers

- [x] Audit all `onclick`/`onchange`/`oninput`/`onsubmit` in templates (~90 occurrences across 7 files)
- [x] Replace inline handlers in `base.html` (4) with `addEventListener`
- [x] Replace inline handlers in `index.html` (18) with `addEventListener` or `data-action` delegation
- [x] Replace inline handlers in `feeds.html` (15) with `addEventListener`
- [x] Replace inline handlers in `scrapers.html` (34) with `addEventListener`
- [x] Replace inline handlers in `settings.html` (16) with `addEventListener`
- [x] Replace inline handlers in `category_settings.html` (2) with `addEventListener`
- [x] Replace inline handlers in `queue.html` (1) with `addEventListener`
- [x] Replace `onclick` strings built in JS (`renderArticleActions`, `buildArticleCardHtml`, `createEditFeedModal`, `showFeedErrorBanner`, etc.) with DOM element creation + `addEventListener`
- [x] Replace `document.querySelectorAll('[onclick="toggleStar(...)"]')` lookups with `data-*` attribute selectors
- [x] Remove all `window.X = X` transitional exports from `app.js`
- [x] Remove ESLint `varsIgnorePattern` whitelist
- [x] Verify `make check` passes, commit

## Phase 4: Finalize

- [x] Change `<script src="/static/app.js">` to `<script type="module" src="/static/app.js">` in `base.html`
- [x] Delete `test-helper.js` and the `new Function()` eval machinery
- [x] Implement import maps with version hashes
- [x] Rewrite all tests as direct ES module imports
- [x] Verify coverage reports meaningful numbers with `npx vitest run --coverage`
- [x] Update `AGENTS.md` code layout section for new module structure
- [x] Update `eslint.config.mjs` (remove globals/ignore patterns that no longer apply)
- [x] Final `make check`, commit

## Phase 5: Eliminate late-bound dependencies

Replace `setXxxDeps()` late-binding with direct imports or small
restructurings. See `ES_MODULES_PLAN.md` Phase 5 for rationale.

### Non-circular (replace with direct imports)

- [x] `article-actions.js`: import `updateCounts` directly from `counts.js` (no cycle exists)
- [x] `article-actions.js`: import `updateQueueCacheIfStandalone` directly from `offline.js` (no cycle exists)
- [x] `counts.js`: import `applyUserPreferences` directly from `articles.js` (no cycle exists)
- [x] `offline.js`: import `updateCounts` directly from `counts.js` (no cycle exists)
- [x] `queue.js`: import `updateQueueCacheIfStandalone` directly from `offline.js` (no cycle exists)
- [x] Remove corresponding `setXxxDeps` parameters and wiring from `app.js`

### Real cycles (restructure)

- [x] `article-actions ↔ articles`: extract `updateReadButton` into a shared module (`read-button.js`) so both can import it without a cycle
- [x] `articles ↔ pagination`: articles.js now directly imports from pagination.js — circular ES module imports work fine since all exports are hoisted function declarations
- [x] `counts ↔ feeds`: extracted `showFeedErrorBanner`/`removeFeedErrorBanner` into `feed-errors.js` — both `counts.js` and `feeds.js` import from the shared leaf module
- [x] Keep `setSidebarLoadCategory` in `app.js` — this is legitimate top-down entry-point wiring, not a circular dependency hack
- [x] Remove all remaining `setXxxDeps` functions and late-bound `let _dep = null` variables
- [x] Verify `make check` passes, commit

## Phase 6: Clean up app.js entry point

Move page-specific logic out of `app.js` so it's a pure entry point
(imports, dependency wiring, init calls). See `ES_MODULES_PLAN.md`
Phase 6 for rationale.

### Move to existing modules

- [x] Add feed form handler (72 lines) → `feeds.js` as `initAddFeedForm()`
- [x] Feed item click handler (12 lines) → `feeds.js` as `initFeedItemClickListeners()`
- [ ] Sidebar mobile close (8 lines) → `sidebar.js` as part of `initSidebarListeners()`
- [ ] Pagination bootstrap + scroll listener (11 lines) → `pagination.js` as `initPagination()`
- [ ] Queue hydration (16 lines) → `article-actions.js` as `initQueueState()`
- [ ] Drag prevention (4 lines) → `drag-drop.js` as part of init

### New module

- [x] Search handler (36 lines) → new `search.js` module with `initSearch()`

### Performance

- [ ] Add `<link rel="modulepreload">` hints in `base.html` for all 20 modules — eliminates the import waterfall that added +328ms to cold DOMContentLoaded (browser discovers imports sequentially across 3 depth levels; modulepreload fetches all in parallel)

### Finalize

- [ ] Remove empty DOMContentLoaded blocks from `app.js`
- [ ] Verify `app.js` is ~120 lines (imports, init calls, DOMContentLoaded sequencing)
- [ ] Verify `make check` passes, commit

## Backlog (outside ES modules scope)

- [ ] Consistent user-facing error handling — currently `folders.js` uses `alert()` on failure while `feeds.js`, `article-actions.js`, `counts.js`, `drag-drop.js` and others silently `console.error()`. The user sees nothing when a star toggle, feed load, or drag-drop reorder fails. Add a toast/notification system or at minimum surface errors visibly across all modules.
