import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    setView,
    getViewScope,
    initView,
    migrateLegacyViewDefaults,
    getDefaultViewForScope,
    applyDefaultViewForScope,
} from './views.js';

// Mock settings module
const store = {};
vi.mock('./settings.js', () => ({
    getSetting: vi.fn((key, def) => store[key] !== undefined ? store[key] : (def || '')),
    saveSetting: vi.fn((key, val) => { store[key] = val; }),
}));

import { getSetting, saveSetting } from './settings.js';

function clearStore() {
    for (const key of Object.keys(store)) delete store[key];
}

describe('views', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        clearStore();
        document.body.innerHTML = '';
        localStorage.clear();
    });

    describe('getViewScope', () => {
        it('returns "all" when no articles-view element', () => {
            expect(getViewScope()).toBe('all');
        });

        it('returns scope from data attribute', () => {
            document.body.innerHTML = '<div class="articles-view" data-view-scope="feed"></div>';
            expect(getViewScope()).toBe('feed');
        });

        it('returns "all" when data-view-scope is empty', () => {
            document.body.innerHTML = '<div class="articles-view"></div>';
            expect(getViewScope()).toBe('all');
        });
    });

    describe('setView', () => {
        beforeEach(() => {
            document.body.innerHTML = `
                <div class="articles-view" data-view-scope="all">
                    <div class="view-toggle">
                        <button data-view="card">Card</button>
                        <button data-view="list">List</button>
                        <button data-view="magazine">Magazine</button>
                        <button data-view="expanded">Expanded</button>
                    </div>
                    <div id="articles-list" class="view-card"></div>
                </div>
            `;
        });

        it('switches to the specified view class', () => {
            setView('list');
            const list = document.getElementById('articles-list');
            expect(list.classList.contains('view-list')).toBe(true);
            expect(list.classList.contains('view-card')).toBe(false);
        });

        it('updates active button', () => {
            setView('magazine');
            const btns = document.querySelectorAll('.view-toggle button');
            const active = [...btns].filter(b => b.classList.contains('active'));
            expect(active).toHaveLength(1);
            expect(active[0].dataset.view).toBe('magazine');
        });

        it('falls back from compact to list', () => {
            setView('compact');
            const list = document.getElementById('articles-list');
            expect(list.classList.contains('view-list')).toBe(true);
        });

        it('saves setting for "all" scope as defaultView', () => {
            setView('expanded');
            expect(saveSetting).toHaveBeenCalledWith('defaultView', 'expanded');
        });

        it('saves setting for folder scope as defaultFolderView', () => {
            document.querySelector('.articles-view').dataset.viewScope = 'folder';
            setView('list');
            expect(saveSetting).toHaveBeenCalledWith('defaultFolderView', 'list');
        });

        it('saves setting for feed scope as defaultFeedView', () => {
            document.querySelector('.articles-view').dataset.viewScope = 'feed';
            setView('card');
            expect(saveSetting).toHaveBeenCalledWith('defaultFeedView', 'card');
        });

        it('does not save when save: false', () => {
            setView('list', { save: false });
            expect(saveSetting).not.toHaveBeenCalled();
        });

        it('does nothing when articles-list is missing', () => {
            document.body.innerHTML = '';
            // Should not throw
            setView('card');
        });
    });

    describe('getDefaultViewForScope', () => {
        it('returns "card" as default for all scopes', () => {
            expect(getDefaultViewForScope('all')).toBe('card');
            expect(getDefaultViewForScope('folder')).toBe('card');
            expect(getDefaultViewForScope('feed')).toBe('card');
        });

        it('returns saved folder view', () => {
            store.defaultFolderView = 'list';
            expect(getDefaultViewForScope('folder')).toBe('list');
        });

        it('returns saved feed view', () => {
            store.defaultFeedView = 'magazine';
            expect(getDefaultViewForScope('feed')).toBe('magazine');
        });

        it('returns saved default view', () => {
            store.defaultView = 'expanded';
            expect(getDefaultViewForScope('all')).toBe('expanded');
        });
    });

    describe('applyDefaultViewForScope', () => {
        beforeEach(() => {
            document.body.innerHTML = `
                <div class="articles-view" data-view-scope="all">
                    <div class="view-toggle">
                        <button data-view="card">Card</button>
                        <button data-view="list">List</button>
                    </div>
                    <div id="articles-list" class="view-card"></div>
                </div>
            `;
        });

        it('applies saved view without saving again', () => {
            store.defaultView = 'list';
            applyDefaultViewForScope('all');
            const list = document.getElementById('articles-list');
            expect(list.classList.contains('view-list')).toBe(true);
            expect(saveSetting).not.toHaveBeenCalled();
        });
    });

    describe('migrateLegacyViewDefaults', () => {
        it('migrates localStorage keys to settings', () => {
            localStorage.setItem('feedreader-view', 'list');
            localStorage.setItem('feedreader-view-folder-default', 'magazine');
            migrateLegacyViewDefaults();
            expect(saveSetting).toHaveBeenCalledWith('defaultView', 'list');
            expect(saveSetting).toHaveBeenCalledWith('defaultFolderView', 'magazine');
            // localStorage should be cleaned up
            expect(localStorage.getItem('feedreader-view')).toBeNull();
            expect(localStorage.getItem('feedreader-view-folder-default')).toBeNull();
        });

        it('does not overwrite existing settings', () => {
            store.defaultView = 'card';
            localStorage.setItem('feedreader-view', 'list');
            migrateLegacyViewDefaults();
            // Should not have called saveSetting for defaultView since it already exists
            const calls = saveSetting.mock.calls.filter(c => c[0] === 'defaultView');
            expect(calls).toHaveLength(0);
        });
    });
});
