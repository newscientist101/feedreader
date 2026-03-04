import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getSetting, saveSetting, applyHideReadArticles, applyHideEmptyFeeds } from './settings.js';

vi.mock('./toast.js');

import { showToast } from './toast.js';

describe('settings', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
        vi.clearAllMocks();
        // Stub fetch globally so saveSetting doesn't produce noisy errors
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true });
        window.__settings = {};
        document.body.innerHTML = '';
    });

    describe('getSetting', () => {
        it('returns value from __settings', () => {
            window.__settings = { theme: 'dark' };
            expect(getSetting('theme')).toBe('dark');
        });

        it('returns default when key is missing', () => {
            expect(getSetting('missing', 'fallback')).toBe('fallback');
        });

        it('returns empty string when key is missing and no default', () => {
            expect(getSetting('missing')).toBe('');
        });

        it('returns the value even if falsy (e.g. 0)', () => {
            window.__settings = { count: 0 };
            expect(getSetting('count', 5)).toBe(0);
        });

        it('returns empty string value rather than default', () => {
            window.__settings = { name: '' };
            // empty string is not undefined, so it should be returned
            expect(getSetting('name', 'fallback')).toBe('');
        });

        it('uses default when value is explicitly undefined', () => {
            window.__settings = { key: undefined };
            expect(getSetting('key', 'default')).toBe('default');
        });

        it('handles missing __settings gracefully', () => {
            delete window.__settings;
            expect(getSetting('key', 'default')).toBe('default');
        });
    });

    describe('saveSetting', () => {
        it('updates __settings locally', () => {
            saveSetting('theme', 'light');
            expect(window.__settings.theme).toBe('light');
        });

        it('creates __settings if missing', () => {
            delete window.__settings;
            saveSetting('key', 'val');
            expect(window.__settings.key).toBe('val');
        });

        it('sends PUT request to /api/settings', () => {
            const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue({ ok: true });
            saveSetting('theme', 'dark');
            expect(fetchSpy).toHaveBeenCalledWith('/api/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
                body: JSON.stringify({ theme: 'dark' }),
            });
        });

        it('logs error and shows toast on fetch failure', async () => {
            const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
            vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('network'));
            saveSetting('key', 'val');
            // Wait for the promise rejection to be caught
            await new Promise(r => setTimeout(r, 10));
            expect(consoleSpy).toHaveBeenCalledWith('Failed to save setting:', expect.any(Error));
            expect(showToast).toHaveBeenCalledWith('Failed to save setting');
        });
    });

    describe('applyHideReadArticles', () => {
        beforeEach(() => {
            document.body.innerHTML = `
                <div class="article-card read" id="read1">Read</div>
                <div class="article-card read" id="read2">Read</div>
                <div class="article-card" id="unread1">Unread</div>
            `;
        });

        it('hides read articles when value is "hide"', () => {
            applyHideReadArticles('hide');
            expect(document.getElementById('read1').style.display).toBe('none');
            expect(document.getElementById('read2').style.display).toBe('none');
            expect(document.getElementById('unread1').style.display).toBe('');
        });

        it('shows read articles when value is not "hide"', () => {
            // First hide them
            applyHideReadArticles('hide');
            // Then show
            applyHideReadArticles('show');
            expect(document.getElementById('read1').style.display).toBe('');
            expect(document.getElementById('read2').style.display).toBe('');
        });

        it('does nothing when no read articles exist', () => {
            document.body.innerHTML = '<div class="article-card" id="unread">Unread</div>';
            applyHideReadArticles('hide');
            expect(document.getElementById('unread').style.display).toBe('');
        });

        it('does not affect unread articles', () => {
            applyHideReadArticles('hide');
            expect(document.getElementById('unread1').style.display).toBe('');
        });
    });

    describe('applyHideEmptyFeeds', () => {
        beforeEach(() => {
            document.body.innerHTML = `
                <div class="feed-item" id="feed-with-count">
                    <span class="badge">5</span>
                </div>
                <div class="feed-item" id="feed-empty">
                    <span class="badge">0</span>
                </div>
                <div class="feed-item" id="feed-no-badge">
                </div>
                <div class="feed-item" id="feed-never-fetched" data-never-fetched="true">
                </div>
            `;
        });

        it('hides empty feeds when value is "hide"', () => {
            applyHideEmptyFeeds('hide');
            expect(document.getElementById('feed-empty').style.display).toBe('none');
            expect(document.getElementById('feed-no-badge').style.display).toBe('none');
        });

        it('does not hide feeds with unread count', () => {
            applyHideEmptyFeeds('hide');
            expect(document.getElementById('feed-with-count').style.display).toBe('');
        });

        it('does not hide never-fetched feeds', () => {
            applyHideEmptyFeeds('hide');
            expect(document.getElementById('feed-never-fetched').style.display).toBe('');
        });

        it('shows all feeds when value is not "hide"', () => {
            applyHideEmptyFeeds('hide');
            applyHideEmptyFeeds('show');
            expect(document.getElementById('feed-empty').style.display).toBe('');
            expect(document.getElementById('feed-no-badge').style.display).toBe('');
        });

        it('hides feed with empty badge text', () => {
            document.body.innerHTML = '<div class="feed-item" id="empty-badge"><span class="badge"></span></div>';
            applyHideEmptyFeeds('hide');
            expect(document.getElementById('empty-badge').style.display).toBe('none');
        });

        it('hides feed with non-numeric badge text (NaN parses to 0)', () => {
            document.body.innerHTML = '<div class="feed-item" id="nan-badge"><span class="badge">abc</span></div>';
            applyHideEmptyFeeds('hide');
            // parseInt('abc', 10) is NaN, which is falsy, so count=0
            expect(document.getElementById('nan-badge').style.display).toBe('none');
        });

        it('always shows feeds with positive count regardless of value', () => {
            applyHideEmptyFeeds('show');
            expect(document.getElementById('feed-with-count').style.display).toBe('');
        });
    });
});
