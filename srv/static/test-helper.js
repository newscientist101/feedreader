/**
 * Test helper: loads app.js into the jsdom global scope.
 *
 * app.js is an ES module, but we eval it inside an IIFE for testing.
 * Import statements are stripped and replaced with window destructuring
 * so the eval'd code can access imported symbols. The modules themselves
 * are loaded separately and their exports placed on window first.
 *
 * This file auto-scans app.js for top-level declarations (function,
 * const, let, var) and generates window assignments so every symbol
 * is accessible in tests — no manual list to maintain.
 */

import { readFileSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

/**
 * Minimal IntersectionObserver mock for jsdom.
 * Tracks observe/unobserve/disconnect calls so tests can assert on them.
 */
class MockIntersectionObserver {
  constructor(callback, options) {
    this._callback = callback;
    this._options = options;
    this._entries = [];
  }
  observe(el) { this._entries.push(el); }
  unobserve(el) { this._entries = this._entries.filter(e => e !== el); }
  disconnect() { this._entries = []; }

  /** Test helper: simulate entries firing. */
  _fire(entries) { this._callback(entries, this); }
}

/**
 * Scan source for top-level declarations and return:
 *   { functions: string[], consts: string[], lets: string[] }
 */
function extractTopLevelNames(src) {
  const functions = [];
  const consts = [];
  const lets = [];

  for (const line of src.split('\n')) {
    let m;
    if ((m = line.match(/^(?:async )?function\s+(\w+)/))) {
      functions.push(m[1]);
    } else if ((m = line.match(/^const\s+(\w+)/))) {
      consts.push(m[1]);
    } else if ((m = line.match(/^(?:let|var)\s+(\w+)/))) {
      lets.push(m[1]);
    }
  }
  return { functions, consts, lets };
}

/** Build the window-export code block appended inside the IIFE. */
function buildExports(names) {
  const lines = [];

  // Functions and consts: plain assignment (immutable)
  for (const n of [...names.functions, ...names.consts]) {
    lines.push(`window['${n}'] = ${n};`);
  }

  // Lets: defineProperty with getter/setter for live access
  for (const n of names.lets) {
    lines.push(
      `Object.defineProperty(window, '${n}', {` +
      `  get() { return ${n}; },` +
      `  set(v) { ${n} = v; },` +
      `  configurable: true,` +
      `});`
    );
  }

  return lines.join('\n');
}

/**
 * Strip ES module import lines from source and return replacement
 * destructuring statements that pull the same names from window.
 *
 * e.g. `import { foo, bar } from './modules/x.js';`
 *   -> `const { foo, bar } = window;`
 */
function replaceImports(src) {
  // Replace both single-line and multi-line import statements with
  // window destructuring so the eval'd code can access imported symbols.
  return src.replace(
    /^import\s+\{([^}]+)\}\s+from\s+['"][^'"]+['"];?/gm,
    (_match, names) => {
      // Collapse multi-line name lists to a single line
      const cleaned = names.replace(/\n/g, ' ').replace(/\s+/g, ' ').trim();
      return `const { ${cleaned} } = window;`;
    }
  );
}

/**
 * Dynamically import all modules under modules/ and place their
 * exports on window so eval'd app.js code can reference them.
 */
async function preloadModules() {
  const modulesDir = resolve(__dirname, 'modules');
  const { readdirSync } = await import('fs');
  const files = readdirSync(modulesDir).filter(f => f.endsWith('.js') && !f.endsWith('.test.js'));
  for (const file of files) {
    const mod = await import(resolve(modulesDir, file));
    for (const [name, value] of Object.entries(mod)) {
      window[name] = value;
    }
  }
}

export async function loadApp() {
  // Reset relevant globals between tests
  window.__settings = {};

  // Provide IntersectionObserver (jsdom doesn't include it)
  window.IntersectionObserver = MockIntersectionObserver;

  // Stub fetch (tests that need it can override)
  window.fetch = () => Promise.resolve({
    ok: true,
    json: () => Promise.resolve({}),
    text: () => Promise.resolve(''),
  });

  // Provide a minimal articles-list so DOMContentLoaded doesn't crash
  document.body.innerHTML = `
    <div class="articles-view">
      <div id="articles-list" class="articles-list"></div>
      <div class="end-of-articles" id="end-of-articles">
        <span class="end-of-articles-dot"></span>
        <span>You're all caught up</span>
        <span class="end-of-articles-dot"></span>
      </div>
    </div>
  `;

  // Pre-load extracted modules onto window
  await preloadModules();

  // Reset module state between tests (modules persist across loadApp calls)
  if (window._resetArticleActionsState) window._resetArticleActionsState();
  if (window._resetPaginationState) window._resetPaginationState();
  if (window._resetArticlesState) window._resetArticlesState();

  // Expose internal module state as window properties for legacy tests.
  // These use getter/setter accessors exported from modules with _ prefix.
  // Once tests are migrated to direct module imports (Phase 4), remove these.
  if (window._getAutoMarkReadObserver) {
    Object.defineProperty(window, 'autoMarkReadObserver', {
      get: () => window._getAutoMarkReadObserver(),
      set: (v) => window._setAutoMarkReadObserver(v),
      configurable: true,
    });
  }
  if (window._getMarkReadQueue) {
    Object.defineProperty(window, '_markReadQueue', {
      get: () => window._getMarkReadQueue(),
      configurable: true,
    });
  }
  // Pagination state shims
  if (window._getPaginationCursorTime) {
    Object.defineProperty(window, 'paginationCursorTime', {
      get: () => window._getPaginationCursorTime(),
      set: (v) => window.setPaginationState({ cursorTime: v }),
      configurable: true,
    });
    Object.defineProperty(window, 'paginationCursorId', {
      get: () => window._getPaginationCursorId(),
      set: (v) => window.setPaginationState({ cursorId: v }),
      configurable: true,
    });
    Object.defineProperty(window, 'paginationDone', {
      get: () => window._getPaginationDone(),
      set: (v) => window.setPaginationState({ done: v }),
      configurable: true,
    });
    Object.defineProperty(window, 'paginationLoading', {
      get: () => window._getPaginationLoading(),
      set: (v) => window.setPaginationState({ loading: v }),
      configurable: true,
    });
  }
  // showingHiddenArticles shim
  if (window.getShowingHiddenArticles) {
    Object.defineProperty(window, 'showingHiddenArticles', {
      get: () => window.getShowingHiddenArticles(),
      set: (v) => window.setShowingHiddenArticles(v),
      configurable: true,
    });
  }

  let src = readFileSync(resolve(__dirname, 'app.js'), 'utf-8');
  // Replace import statements with window destructuring
  src = replaceImports(src);
  const names = extractTopLevelNames(src);
  const exports = buildExports(names);

  const wrapped = `(function() {\n${src}\n${exports}\n})();`;

  const script = new Function(wrapped);
  script.call(window);
}

// Re-export for testing the helper itself
export { extractTopLevelNames };
