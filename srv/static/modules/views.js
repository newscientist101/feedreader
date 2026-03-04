// View mode switching: scope-aware view preferences (card/list/magazine/expanded).

import { getSetting, saveSetting } from './settings.js';

export function getViewScope() {
    const view = document.querySelector('.articles-view');
    if (!view) return 'all';
    return view.dataset.viewScope || 'all';
}

export function setView(view, { save = true } = {}) {
    // Compact view was removed; fall back to list
    if (view === 'compact') view = 'list';

    const list = document.getElementById('articles-list');
    if (!list) return;

    // Remove all view classes
    list.classList.remove('view-card', 'view-list', 'view-magazine', 'view-expanded');
    // Add the selected view class
    list.classList.add('view-' + view);

    // Update toggle buttons
    document.querySelectorAll('.view-toggle button').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.view === view);
    });

    // Save preference (scope-aware)
    if (save) {
        const scope = getViewScope();
        if (scope === 'folder') {
            saveSetting('defaultFolderView', view);
        } else if (scope === 'feed') {
            saveSetting('defaultFeedView', view);
        } else {
            saveSetting('defaultView', view);
        }
    }
}

export function migrateLegacyViewDefaults() {
    // Migrate any localStorage settings to the server
    const keys = ['autoMarkRead', 'hideReadArticles', 'hideEmptyFeeds', 'defaultFolderView', 'defaultFeedView'];
    const localMap = {
        'feedreader-view-folder-default': 'defaultFolderView',
        'feedreader-view-feed-default': 'defaultFeedView',
        'feedreader-view': 'defaultView',
    };
    for (const key of keys) {
        if (!getSetting(key) && localStorage.getItem(key)) {
            saveSetting(key, localStorage.getItem(key));
            localStorage.removeItem(key);
        }
    }
    for (const [oldKey, newKey] of Object.entries(localMap)) {
        if (!getSetting(newKey) && localStorage.getItem(oldKey)) {
            saveSetting(newKey, localStorage.getItem(oldKey));
            localStorage.removeItem(oldKey);
        }
    }
}

export function getDefaultViewForScope(scope) {
    if (scope === 'folder') {
        return getSetting('defaultFolderView') || 'card';
    }
    if (scope === 'feed') {
        return getSetting('defaultFeedView') || 'card';
    }
    return getSetting('defaultView') || 'card';
}

export function applyDefaultViewForScope(scope) {
    const savedView = getDefaultViewForScope(scope);
    setView(savedView, { save: false });
}

// Initialize view on page load
export function initView() {
    migrateLegacyViewDefaults();
    applyDefaultViewForScope(getViewScope());
}

let _viewListenerAC = null;

// Delegated listener for view toggle buttons (replaces inline onclick in index.html).
export function initViewListeners() {
    if (_viewListenerAC) _viewListenerAC.abort();
    _viewListenerAC = new AbortController();
    const signal = _viewListenerAC.signal;

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('.view-toggle [data-view]');
        if (btn) {
            setView(btn.dataset.view);
        }
    }, { signal });
}
