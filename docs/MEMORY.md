# ES Modules Migration — Working Memory

## Run 1 — Phase 1 complete

**Completed:** All 7 Phase 1 tasks.

- Created `srv/static/modules/` directory
- Extracted `icons.js` (7 SVG constants), `utils.js` (5 functions + PREVIEW_TEXT_LIMIT), `api.js` (fetch wrapper)
- Added `utils.test.js` (15 tests) and `api.test.js` (5 tests) with direct ES module imports
- Added ESLint flat-config override for `modules/**/*.js` with `sourceType: "module"` (must come AFTER the general `srv/static/**/*.js` config so it wins the merge)
- `vitest.config.mjs` globs already covered `modules/` — no changes needed
- `app.js` is **unchanged** — functions are duplicated in modules for now. Phase 2 will start removing from `app.js` and importing from modules.

**Next run:** Start Phase 2. First task is `modules/settings.js`. Note: to actually use imports in `app.js`, it must become `type="module"` in base.html. The plan says this is Phase 4, but practically it needs to happen at the start of Phase 2 (or else `app.js` can't import from modules). Consider switching `<script>` to `<script type="module">` as the first Phase 2 step — the app already uses `DOMContentLoaded`, so deferred execution should be equivalent.
