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
- [ ] Extract `modules/drag-drop.js` — `initDragDrop`, `initFolderDragDrop`, `syncFolderOrder`, `reorderElements`, `getDragAfterElementAmongSiblings`
- [ ] Extract `modules/opml.js` — `exportOPML`, `importOPML`
- [ ] Extract `modules/queue.js` — `queuedArticleIds`, `queuedIdsReady`, `queueNext`
- [ ] Extract `modules/settings-page.js` — `initSettingsPage`, `runCleanup`, `loadNewsletterAddress`, `generateNewsletterAddress`, `showNewsletterAddress`, `copyNewsletterAddress`
- [ ] Extract `modules/offline.js` — `initOfflineSupport`, `cacheQueueForOffline`, `handleOnlineStateChange`, `showOfflineBanner`, `disableNonQueueUI`, `enableAllUI`, `replayPendingActions`, `updateQueueCacheIfStandalone`
- [ ] Wire transitional `window.X = X` exports in `app.js` for all functions still called from `onclick`
- [ ] Verify `make check` passes after each extraction, commit per module or small batch

## Phase 3: Eliminate inline event handlers

- [ ] Audit all `onclick`/`onchange`/`oninput`/`onsubmit` in templates (~90 occurrences across 7 files)
- [ ] Replace inline handlers in `base.html` (4) with `addEventListener`
- [ ] Replace inline handlers in `index.html` (18) with `addEventListener` or `data-action` delegation
- [ ] Replace inline handlers in `feeds.html` (15) with `addEventListener`
- [ ] Replace inline handlers in `scrapers.html` (34) with `addEventListener`
- [ ] Replace inline handlers in `settings.html` (16) with `addEventListener`
- [ ] Replace inline handlers in `category_settings.html` (2) with `addEventListener`
- [ ] Replace inline handlers in `queue.html` (1) with `addEventListener`
- [ ] Replace `onclick` strings built in JS (`renderArticleActions`, `buildArticleCardHtml`, `createEditFeedModal`, `showFeedErrorBanner`, etc.) with DOM element creation + `addEventListener`
- [ ] Replace `document.querySelectorAll('[onclick="toggleStar(...)"]')` lookups with `data-*` attribute selectors
- [ ] Remove all `window.X = X` transitional exports from `app.js`
- [ ] Remove ESLint `varsIgnorePattern` whitelist
- [ ] Verify `make check` passes, commit

## Phase 4: Finalize

- [ ] Change `<script src="/static/app.js">` to `<script type="module" src="/static/app.js">` in `base.html`
- [ ] Delete `test-helper.js` and the `new Function()` eval machinery
- [ ] Rewrite all tests as direct ES module imports
- [ ] Verify coverage reports meaningful numbers with `npx vitest run --coverage`
- [ ] Update `AGENTS.md` code layout section for new module structure
- [ ] Update `eslint.config.mjs` (remove globals/ignore patterns that no longer apply)
- [ ] Final `make check`, commit
