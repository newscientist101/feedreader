/**
 * Test helper: loads app.js into the jsdom global scope.
 *
 * app.js is a plain <script> (not an ES module), so we eval it in the
 * global context.  Before loading we stub out browser APIs that the
 * DOMContentLoaded handler needs but jsdom doesn't fully support.
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

  // Wrap in an IIFE. We expose selected names to window via a final block.
  const wrapped = `
    (function() {
      ${src}

      // Expose functions & state to the global scope for tests.
      // Use a proxy object so getters read live closure values.
      window.formatTimeAgo = formatTimeAgo;
      window.stripHtml = stripHtml;
      window.truncateText = truncateText;
      window.getSetting = getSetting;
      window.saveSetting = saveSetting;
      window.initAutoMarkRead = initAutoMarkRead;
      window.observeNewArticles = observeNewArticles;
      window.flushMarkReadQueue = flushMarkReadQueue;
      window.markReadSilent = markReadSilent;
      window.renderArticles = renderArticles;
      window.buildArticleCardHtml = buildArticleCardHtml;
      window.applyUserPreferences = applyUserPreferences;
      window.updateEndOfArticlesIndicator = updateEndOfArticlesIndicator;
      window.getPaginationUrl = getPaginationUrl;
      window.updateAllReadMessage = updateAllReadMessage;
      window.showReadArticles = showReadArticles;
      window.api = api;
      window.openArticle = openArticle;
      window.openArticleExternal = openArticleExternal;
      window.PAGE_SIZE = PAGE_SIZE;
      window.PREVIEW_TEXT_LIMIT = PREVIEW_TEXT_LIMIT;

      // Live getters/setters for mutable state
      Object.defineProperty(window, 'autoMarkReadObserver', {
        get() { return autoMarkReadObserver; },
        set(v) { autoMarkReadObserver = v; },
        configurable: true,
      });
      Object.defineProperty(window, '_markReadQueue', {
        get() { return _markReadQueue; },
        configurable: true,
      });
      Object.defineProperty(window, '_markReadTimer', {
        get() { return _markReadTimer; },
        configurable: true,
      });
      Object.defineProperty(window, 'paginationDone', {
        get() { return paginationDone; },
        set(v) { paginationDone = v; },
        configurable: true,
      });
      Object.defineProperty(window, 'paginationLoading', {
        get() { return paginationLoading; },
        set(v) { paginationLoading = v; },
        configurable: true,
      });
      Object.defineProperty(window, 'paginationOffset', {
        get() { return paginationOffset; },
        set(v) { paginationOffset = v; },
        configurable: true,
      });
    })();
  `;

  const script = new Function(wrapped);
  script.call(window);
}
