/**
 * Test helper: loads app.js into the jsdom global scope.
 *
 * app.js is a plain <script> (not an ES module), so we eval it inside
 * an IIFE.  This file auto-scans app.js for top-level declarations
 * (function, const, let, var) and generates window assignments so
 * every symbol is accessible in tests — no manual list to maintain.
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

export function loadApp() {
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

  const src = readFileSync(resolve(__dirname, 'app.js'), 'utf-8');
  const names = extractTopLevelNames(src);
  const exports = buildExports(names);

  const wrapped = `(function() {\n${src}\n${exports}\n})();`;

  const script = new Function(wrapped);
  script.call(window);
}

// Re-export for testing the helper itself
export { extractTopLevelNames };
