import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    initSettingsPage, runCleanup,
    loadNewsletterAddress, generateNewsletterAddress,
    showNewsletterAddress, copyNewsletterAddress,
    initSettingsPageListeners,
} from './settings-page.js';

// Mock the settings module
vi.mock('./settings.js', () => {
    const store = {};
    return {
        getSetting: vi.fn((key) => store[key]),
        saveSetting: vi.fn((key, val) => { store[key] = val; }),
        applyHideReadArticles: vi.fn(),
        applyHideEmptyFeeds: vi.fn(),
        _store: store,
    };
});

// Mock the api module
vi.mock('./api.js', () => ({
    api: vi.fn(),
}));

vi.mock('./toast.js', () => ({
    showToast: vi.fn(),
}));

import { getSetting, saveSetting, applyHideReadArticles, applyHideEmptyFeeds } from './settings.js';
import { api } from './api.js';
import { showToast } from './toast.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('initSettingsPage', () => {
    it('does nothing when not on settings page (no auto-mark-read toggle)', () => {
        document.body.innerHTML = '<div>Not settings</div>';
        // Should not throw
        initSettingsPage();
    });

    it('initializes auto-mark-read toggle from stored setting', () => {
        document.body.innerHTML = `
            <input type="checkbox" id="auto-mark-read">
            <input type="radio" name="hide-read" value="show">
            <input type="radio" name="hide-read" value="hide">
            <input type="radio" name="hide-empty" value="show">
            <input type="radio" name="hide-empty" value="hide">
            <input type="radio" name="folder-view" value="card">
            <input type="radio" name="folder-view" value="list">
            <input type="radio" name="feed-view" value="card">
            <input type="radio" name="feed-view" value="list">
        `;
        getSetting.mockImplementation((key) => {
            if (key === 'autoMarkRead') return 'true';
            return undefined;
        });

        initSettingsPage();

        expect(document.getElementById('auto-mark-read').checked).toBe(true);
    });

    it('checks the correct hide-read radio', () => {
        document.body.innerHTML = `
            <input type="checkbox" id="auto-mark-read">
            <input type="radio" name="hide-read" value="show">
            <input type="radio" name="hide-read" value="hide">
            <input type="radio" name="hide-empty" value="show">
            <input type="radio" name="folder-view" value="card">
            <input type="radio" name="feed-view" value="card">
        `;
        getSetting.mockImplementation((key) => {
            if (key === 'hideReadArticles') return 'hide';
            return undefined;
        });

        initSettingsPage();

        const hideRadio = document.querySelector('input[name="hide-read"][value="hide"]');
        expect(hideRadio.checked).toBe(true);
    });

    it('checks the correct default view radios', () => {
        document.body.innerHTML = `
            <input type="checkbox" id="auto-mark-read">
            <input type="radio" name="hide-read" value="show">
            <input type="radio" name="hide-empty" value="show">
            <input type="radio" name="folder-view" value="card">
            <input type="radio" name="folder-view" value="list">
            <input type="radio" name="feed-view" value="card">
            <input type="radio" name="feed-view" value="list">
        `;
        getSetting.mockImplementation((key) => {
            if (key === 'defaultFolderView') return 'list';
            if (key === 'defaultFeedView') return 'list';
            return undefined;
        });

        initSettingsPage();

        expect(document.querySelector('input[name="folder-view"][value="list"]').checked).toBe(true);
        expect(document.querySelector('input[name="feed-view"][value="list"]').checked).toBe(true);
    });

    it('uses defaults when getSetting returns falsy values', () => {
        document.body.innerHTML = `
            <input type="checkbox" id="auto-mark-read">
            <input type="radio" name="hide-read" value="show">
            <input type="radio" name="hide-read" value="hide">
            <input type="radio" name="hide-empty" value="show">
            <input type="radio" name="hide-empty" value="hide">
            <input type="radio" name="folder-view" value="card">
            <input type="radio" name="folder-view" value="list">
            <input type="radio" name="feed-view" value="card">
            <input type="radio" name="feed-view" value="list">
        `;
        // All settings return undefined — defaults should be applied
        getSetting.mockReturnValue(undefined);

        initSettingsPage();

        // autoMarkRead undefined !== 'true', so unchecked
        expect(document.getElementById('auto-mark-read').checked).toBe(false);
        // hideReadArticles defaults to 'show'
        expect(document.querySelector('input[name="hide-read"][value="show"]').checked).toBe(true);
        // hideEmptyFeeds defaults to 'show'
        expect(document.querySelector('input[name="hide-empty"][value="show"]').checked).toBe(true);
        // defaultFolderView defaults to 'card'
        expect(document.querySelector('input[name="folder-view"][value="card"]').checked).toBe(true);
        // defaultFeedView defaults to 'card'
        expect(document.querySelector('input[name="feed-view"][value="card"]').checked).toBe(true);
    });

    it('checks the correct hide-empty radio', () => {
        document.body.innerHTML = `
            <input type="checkbox" id="auto-mark-read">
            <input type="radio" name="hide-read" value="show">
            <input type="radio" name="hide-empty" value="show">
            <input type="radio" name="hide-empty" value="hide">
            <input type="radio" name="folder-view" value="card">
            <input type="radio" name="feed-view" value="card">
        `;
        getSetting.mockImplementation((key) => {
            if (key === 'hideEmptyFeeds') return 'hide';
            return undefined;
        });

        initSettingsPage();

        const hideRadio = document.querySelector('input[name="hide-empty"][value="hide"]');
        expect(hideRadio.checked).toBe(true);
    });

    it('handles missing radio for stored value gracefully', () => {
        document.body.innerHTML = `
            <input type="checkbox" id="auto-mark-read">
            <input type="radio" name="hide-read" value="show">
            <input type="radio" name="hide-empty" value="show">
            <input type="radio" name="folder-view" value="card">
            <input type="radio" name="feed-view" value="card">
        `;
        // Return a value that has no matching radio
        getSetting.mockImplementation((key) => {
            if (key === 'hideReadArticles') return 'nonexistent';
            return undefined;
        });

        // Should not throw when querySelector returns null
        expect(() => initSettingsPage()).not.toThrow();
    });

    it('calls loadNewsletterAddress on init', async () => {
        document.body.innerHTML = `
            <input type="checkbox" id="auto-mark-read">
            <input type="radio" name="hide-read" value="show">
            <input type="radio" name="hide-empty" value="show">
            <input type="radio" name="folder-view" value="card">
            <input type="radio" name="feed-view" value="card">
            <div id="newsletter-container"></div>
        `;
        getSetting.mockReturnValue(undefined);
        api.mockResolvedValue({});

        initSettingsPage();
        // loadNewsletterAddress should have been called (it calls api)
        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('GET', '/api/newsletter/address');
        });
    });
});

describe('runCleanup', () => {
    it('calls cleanup API and updates status on success', async () => {
        document.body.innerHTML = `
            <span id="cleanup-status"></span>
            <span id="articles-to-delete">42</span>
        `;
        api.mockResolvedValue({ deleted: 42 });

        await runCleanup();

        expect(api).toHaveBeenCalledWith('POST', '/api/retention/cleanup');
        expect(document.getElementById('cleanup-status').textContent).toBe('Deleted 42 articles');
        expect(document.getElementById('cleanup-status').className).toBe('cleanup-status success');
        expect(document.getElementById('articles-to-delete').textContent).toBe('0');
    });

    it('shows error status on failure', async () => {
        document.body.innerHTML = `
            <span id="cleanup-status"></span>
            <span id="articles-to-delete">10</span>
        `;
        api.mockRejectedValue(new Error('Server error'));

        await runCleanup();

        expect(document.getElementById('cleanup-status').textContent).toBe('Cleanup failed: Server error');
        expect(document.getElementById('cleanup-status').className).toBe('cleanup-status error');
        // articles-to-delete should NOT be reset on error
        expect(document.getElementById('articles-to-delete').textContent).toBe('10');
    });

    it('shows intermediate loading state before API resolves', async () => {
        document.body.innerHTML = `
            <span id="cleanup-status"></span>
            <span id="articles-to-delete">5</span>
        `;
        let resolve;
        api.mockReturnValue(new Promise(r => { resolve = r; }));

        const promise = runCleanup();

        // Check intermediate state before resolution
        expect(document.getElementById('cleanup-status').textContent).toBe('Cleaning up...');
        expect(document.getElementById('cleanup-status').className).toBe('cleanup-status');

        resolve({ deleted: 5 });
        await promise;

        expect(document.getElementById('cleanup-status').textContent).toBe('Deleted 5 articles');
    });
});

describe('showNewsletterAddress', () => {
    it('shows address and hides generate button', () => {
        document.body.innerHTML = `
            <div id="newsletter-no-address">Generate</div>
            <div id="newsletter-has-address" style="display: none">
                <span id="newsletter-address"></span>
            </div>
        `;

        showNewsletterAddress('test@feed.example.com');

        expect(document.getElementById('newsletter-no-address').style.display).toBe('none');
        expect(document.getElementById('newsletter-has-address').style.display).toBe('');
        expect(document.getElementById('newsletter-address').textContent).toBe('test@feed.example.com');
    });

    it('handles missing elements gracefully', () => {
        document.body.innerHTML = '<div>No newsletter</div>';
        expect(() => showNewsletterAddress('test@example.com')).not.toThrow();
    });
});

describe('loadNewsletterAddress', () => {
    it('does nothing when newsletter container not found', async () => {
        document.body.innerHTML = '<div>No container</div>';

        await loadNewsletterAddress();

        expect(api).not.toHaveBeenCalled();
    });

    it('loads and shows existing address', async () => {
        document.body.innerHTML = `
            <div id="newsletter-container">
                <div id="newsletter-no-address">Generate</div>
                <div id="newsletter-has-address" style="display: none">
                    <span id="newsletter-address"></span>
                </div>
            </div>
        `;
        api.mockResolvedValue({ address: 'news@feed.example.com' });

        await loadNewsletterAddress();

        expect(api).toHaveBeenCalledWith('GET', '/api/newsletter/address');
        expect(document.getElementById('newsletter-address').textContent).toBe('news@feed.example.com');
    });

    it('does not throw when API returns no address', async () => {
        document.body.innerHTML = `
            <div id="newsletter-container">
                <div id="newsletter-no-address" style="display: block">Generate</div>
                <div id="newsletter-has-address" style="display: none"></div>
            </div>
        `;
        api.mockResolvedValue({});

        await loadNewsletterAddress();

        expect(api).toHaveBeenCalled();
        // should NOT show the address section
        expect(document.getElementById('newsletter-has-address').style.display).toBe('none');
    });

    it('does not show address when API returns empty string', async () => {
        document.body.innerHTML = `
            <div id="newsletter-container">
                <div id="newsletter-no-address" style="display: block">Generate</div>
                <div id="newsletter-has-address" style="display: none">
                    <span id="newsletter-address"></span>
                </div>
            </div>
        `;
        api.mockResolvedValue({ address: '' });

        await loadNewsletterAddress();

        // Empty string is falsy, so address section should stay hidden
        expect(document.getElementById('newsletter-has-address').style.display).toBe('none');
    });

    it('handles API errors gracefully', async () => {
        document.body.innerHTML = '<div id="newsletter-container"></div>';
        api.mockRejectedValue(new Error('fail'));

        await expect(loadNewsletterAddress()).resolves.toBeUndefined();
    });
});

describe('generateNewsletterAddress', () => {
    it('generates and shows new address', async () => {
        document.body.innerHTML = `
            <div id="newsletter-no-address">Generate</div>
            <div id="newsletter-has-address" style="display: none">
                <span id="newsletter-address"></span>
            </div>
        `;
        api.mockResolvedValue({ address: 'new@feed.example.com' });

        await generateNewsletterAddress();

        expect(api).toHaveBeenCalledWith('POST', '/api/newsletter/generate-address');
        expect(document.getElementById('newsletter-address').textContent).toBe('new@feed.example.com');
    });

    it('shows toast on failure', async () => {
        api.mockRejectedValue(new Error('rate limit'));

        await generateNewsletterAddress();

        expect(showToast).toHaveBeenCalledWith('Failed to generate address: rate limit');
    });

    it('does not show address when API returns no address field', async () => {
        document.body.innerHTML = `
            <div id="newsletter-no-address" style="display: block">Generate</div>
            <div id="newsletter-has-address" style="display: none">
                <span id="newsletter-address"></span>
            </div>
        `;
        api.mockResolvedValue({});

        await generateNewsletterAddress();

        // The address section should remain hidden
        expect(document.getElementById('newsletter-has-address').style.display).toBe('none');
    });
});

describe('copyNewsletterAddress', () => {
    it('does nothing when address element not found', async () => {
        document.body.innerHTML = '<div>No address</div>';

        await copyNewsletterAddress();
        // Should not throw
    });

    it('copies address to clipboard', async () => {
        document.body.innerHTML = `
            <span id="newsletter-address">copy@example.com</span>
            <button>Copy</button>
        `;
        const writeText = vi.fn().mockResolvedValue(undefined);
        Object.defineProperty(navigator, 'clipboard', {
            value: { writeText },
            writable: true,
            configurable: true,
        });

        await copyNewsletterAddress();

        expect(writeText).toHaveBeenCalledWith('copy@example.com');
    });

    it('shows checkmark icon temporarily after successful copy', async () => {
        vi.useFakeTimers();
        document.body.innerHTML = `
            <span id="newsletter-address">copy@example.com</span>
            <button id="copy-btn">Original</button>
        `;
        const writeText = vi.fn().mockResolvedValue(undefined);
        Object.defineProperty(navigator, 'clipboard', {
            value: { writeText },
            writable: true,
            configurable: true,
        });

        await copyNewsletterAddress();

        const btn = document.getElementById('copy-btn');
        // Button should show checkmark SVG
        expect(btn.innerHTML).toContain('polyline');

        // After 1500ms, button should revert to original
        vi.advanceTimersByTime(1500);
        expect(btn.innerHTML).toBe('Original');

        vi.useRealTimers();
    });

    it('handles missing sibling button gracefully', async () => {
        document.body.innerHTML = `
            <span id="newsletter-address">copy@example.com</span>
        `;
        const writeText = vi.fn().mockResolvedValue(undefined);
        Object.defineProperty(navigator, 'clipboard', {
            value: { writeText },
            writable: true,
            configurable: true,
        });

        // nextElementSibling is null — should not throw
        await copyNewsletterAddress();

        expect(writeText).toHaveBeenCalledWith('copy@example.com');
    });

    it('falls back to text selection when clipboard API fails', async () => {
        document.body.innerHTML = `
            <span id="newsletter-address">copy@example.com</span>
        `;
        Object.defineProperty(navigator, 'clipboard', {
            value: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
            writable: true,
            configurable: true,
        });

        // Should not throw
        await copyNewsletterAddress();
    });
});

describe('initSettingsPageListeners', () => {
    beforeEach(() => {
        initSettingsPageListeners();
    });

    it('delegates data-setting checkbox change', () => {
        document.body.innerHTML = `
            <input type="checkbox" data-setting="autoMarkRead" data-setting-type="checkbox">
        `;
        const input = document.querySelector('[data-setting]');
        input.checked = true;

        input.dispatchEvent(new Event('change', { bubbles: true }));

        expect(saveSetting).toHaveBeenCalledWith('autoMarkRead', 'true');
    });

    it('delegates data-setting radio change', () => {
        document.body.innerHTML = `
            <input type="radio" name="hide-read" value="hide" data-setting="hideReadArticles" data-apply="hideReadArticles">
        `;
        const input = document.querySelector('[data-setting]');
        input.checked = true;

        input.dispatchEvent(new Event('change', { bubbles: true }));

        expect(saveSetting).toHaveBeenCalledWith('hideReadArticles', 'hide');
        expect(applyHideReadArticles).toHaveBeenCalledWith('hide');
    });

    it('delegates data-apply hideEmptyFeeds', () => {
        document.body.innerHTML = `
            <input type="radio" value="hide" data-setting="hideEmptyFeeds" data-apply="hideEmptyFeeds">
        `;
        const input = document.querySelector('[data-setting]');
        input.checked = true;

        input.dispatchEvent(new Event('change', { bubbles: true }));

        expect(saveSetting).toHaveBeenCalledWith('hideEmptyFeeds', 'hide');
        expect(applyHideEmptyFeeds).toHaveBeenCalledWith('hide');
    });

    it('delegates view default radio without apply', () => {
        document.body.innerHTML = `
            <input type="radio" value="list" data-setting="defaultFolderView">
        `;
        const input = document.querySelector('[data-setting]');
        input.checked = true;

        input.dispatchEvent(new Event('change', { bubbles: true }));

        expect(saveSetting).toHaveBeenCalledWith('defaultFolderView', 'list');
        // No apply function should be called
        expect(applyHideReadArticles).not.toHaveBeenCalled();
        expect(applyHideEmptyFeeds).not.toHaveBeenCalled();
    });

    it('delegates run-cleanup click', async () => {
        document.body.innerHTML = `
            <button data-action="run-cleanup">Cleanup</button>
            <span id="cleanup-status"></span>
            <span id="articles-to-delete">5</span>
        `;
        api.mockResolvedValue({ deleted: 5 });

        document.querySelector('[data-action="run-cleanup"]').click();
        await new Promise(r => setTimeout(r, 10));

        expect(api).toHaveBeenCalledWith('POST', '/api/retention/cleanup');
    });

    it('delegates generate-newsletter click', async () => {
        document.body.innerHTML = `
            <button data-action="generate-newsletter">Generate</button>
            <div id="newsletter-no-address"></div>
            <div id="newsletter-has-address" style="display:none">
                <span id="newsletter-address"></span>
            </div>
        `;
        api.mockResolvedValue({ address: 'test@example.com' });

        document.querySelector('[data-action="generate-newsletter"]').click();
        await new Promise(r => setTimeout(r, 10));

        expect(api).toHaveBeenCalledWith('POST', '/api/newsletter/generate-address');
    });

    it('delegates copy-newsletter click', async () => {
        document.body.innerHTML = `
            <span id="newsletter-address">test@example.com</span>
            <button data-action="copy-newsletter">Copy</button>
        `;
        const writeText = vi.fn().mockResolvedValue(undefined);
        Object.defineProperty(navigator, 'clipboard', {
            value: { writeText },
            writable: true,
            configurable: true,
        });

        document.querySelector('[data-action="copy-newsletter"]').click();
        await new Promise(r => setTimeout(r, 10));

        expect(writeText).toHaveBeenCalledWith('test@example.com');
    });

    it('ignores change events without data-setting', () => {
        document.body.innerHTML = '<input type="text" id="other-input">';

        document.getElementById('other-input').dispatchEvent(new Event('change', { bubbles: true }));

        expect(saveSetting).not.toHaveBeenCalled();
    });

    it('delegates click from nested child of data-action button', async () => {
        document.body.innerHTML = `
            <button data-action="run-cleanup"><span id="inner">Cleanup</span></button>
            <span id="cleanup-status"></span>
            <span id="articles-to-delete">3</span>
        `;
        api.mockResolvedValue({ deleted: 3 });

        // Click the inner span — .closest() should find the button
        document.getElementById('inner').click();
        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('POST', '/api/retention/cleanup');
        });
    });

    it('ignores clicks that do not match any data-action', () => {
        document.body.innerHTML = '<button id="random">Random</button>';

        document.getElementById('random').click();

        expect(api).not.toHaveBeenCalled();
    });
});
