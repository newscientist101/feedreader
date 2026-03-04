import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { updateCounts, updateFeedStatusCell, updateFeedErrors } from './counts.js';

vi.mock('./articles.js');
vi.mock('./feed-errors.js');
vi.mock('./toast.js');

import { applyUserPreferences } from './articles.js';
import { showFeedErrorBanner, removeFeedErrorBanner } from './feed-errors.js';
import { showToast } from './toast.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('updateCounts', () => {
    it('updates unread, starred, queue, and alerts badges', async () => {
        document.body.innerHTML = `
            <span data-count="unread">5</span>
            <span data-count="starred">2</span>
            <span data-count="queue">1</span>
            <span data-count="alerts">0</span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 10, starred: 3, queue: 2, alerts: 4,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="unread"]').textContent).toBe('10');
        expect(document.querySelector('[data-count="starred"]').textContent).toBe('3');
        expect(document.querySelector('[data-count="queue"]').textContent).toBe('2');
        expect(document.querySelector('[data-count="alerts"]').textContent).toBe('4');
    });

    it('updates category counts and zeros missing ones', async () => {
        document.body.innerHTML = `
            <span data-count="category-1">5</span>
            <span data-count="category-2">3</span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: { '1': 7 },
                feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="category-1"]').textContent).toBe('7');
        expect(document.querySelector('[data-count="category-2"]').textContent).toBe('');
    });

    it('updates feed counts and zeros missing ones', async () => {
        document.body.innerHTML = `
            <span data-count="feed-10">2</span>
            <span data-count="feed-20">4</span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {},
                feeds: { '10': 5 },
                feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="feed-10"]').textContent).toBe('5');
        expect(document.querySelector('[data-count="feed-20"]').textContent).toBe('');
    });

    it('does not zero feed badges with pending class', async () => {
        document.body.innerHTML = '<span data-count="feed-10" class="pending">...</span>';
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="feed-10"]').textContent).toBe('...');
    });

    it('calls applyUserPreferences after updating counts', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(applyUserPreferences).toHaveBeenCalled();
    });

    it('handles API errors gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await updateCounts(); // should not throw

        expect(console.error).toHaveBeenCalledWith('Failed to update counts:', expect.any(Error));
    });

    it('sets aria-label attributes on unread, queue, and alerts badges', async () => {
        document.body.innerHTML = `
            <span data-count="unread"></span>
            <span data-count="queue"></span>
            <span data-count="alerts"></span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 7, starred: 0, queue: 3, alerts: 2,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="unread"]').getAttribute('aria-label')).toBe('7 unread articles');
        expect(document.querySelector('[data-count="queue"]').getAttribute('aria-label')).toBe('3 queued articles');
        expect(document.querySelector('[data-count="alerts"]').getAttribute('aria-label')).toBe('2 alerts');
    });

    it('clears aria-label when counts are zero', async () => {
        document.body.innerHTML = `
            <span data-count="unread" aria-label="5 unread articles"></span>
            <span data-count="queue" aria-label="2 queued articles"></span>
            <span data-count="alerts" aria-label="1 alerts"></span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0, alerts: 0,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="unread"]').getAttribute('aria-label')).toBe('');
        expect(document.querySelector('[data-count="queue"]').getAttribute('aria-label')).toBe('');
        expect(document.querySelector('[data-count="alerts"]').getAttribute('aria-label')).toBe('');
    });

    it('does not throw when badge elements are missing from DOM', async () => {
        document.body.innerHTML = ''; // no badges at all
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 5, starred: 2, queue: 1, alerts: 3,
                categories: { '1': 10 }, feeds: { '10': 5 }, feedErrors: {},
            }),
        });

        await updateCounts(); // should not throw

        expect(applyUserPreferences).toHaveBeenCalled();
    });

    it('updates pending feed badge when count is positive and removes pending class', async () => {
        document.body.innerHTML = '<span data-count="feed-10" class="pending">...</span>';
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {}, feeds: { '10': 8 }, feedErrors: {},
            }),
        });

        await updateCounts();

        const badge = document.querySelector('[data-count="feed-10"]');
        expect(badge.textContent).toBe('8');
        expect(badge.classList.contains('pending')).toBe(false);
        expect(badge.getAttribute('aria-label')).toBe('8 unread');
    });

    it('updates multiple badges for the same feed', async () => {
        document.body.innerHTML = `
            <span data-count="feed-10">0</span>
            <span data-count="feed-10">0</span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {}, feeds: { '10': 3 }, feedErrors: {},
            }),
        });

        await updateCounts();

        const badges = document.querySelectorAll('[data-count="feed-10"]');
        badges.forEach(badge => {
            expect(badge.textContent).toBe('3');
        });
    });

    it('sets category aria-label on nonzero and clears on zero', async () => {
        document.body.innerHTML = `
            <span data-count="category-1">5</span>
            <span data-count="category-2">3</span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: { '1': 7 },
                feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="category-1"]').getAttribute('aria-label')).toBe('7 unread');
        expect(document.querySelector('[data-count="category-2"]').getAttribute('aria-label')).toBe('');
    });

    it('shows toast on API error', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await updateCounts();

        expect(showToast).toHaveBeenCalledWith('Failed to update counts');
    });

    it('passes feedErrors to updateFeedErrors', async () => {
        document.body.innerHTML = `
            <div class="feed-item" data-feed-id="5">
                <span data-error></span>
            </div>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {}, feeds: {},
                feedErrors: { '5': 'Timeout' },
            }),
        });

        await updateCounts();

        const item = document.querySelector('[data-feed-id="5"]');
        expect(item.classList.contains('has-error')).toBe(true);
    });

    it('clears badges when counts are zero', async () => {
        document.body.innerHTML = `
            <span data-count="unread">5</span>
            <span data-count="starred">2</span>
            <span data-count="queue">1</span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="unread"]').textContent).toBe('');
        expect(document.querySelector('[data-count="starred"]').textContent).toBe('');
        expect(document.querySelector('[data-count="queue"]').textContent).toBe('');
    });
});

describe('updateFeedStatusCell', () => {
    it('sets error status on feed row', () => {
        document.body.innerHTML = `
            <table><tbody>
                <tr data-feed-id="5">
                    <td>Feed name</td>
                    <td class="status-cell">OK</td>
                    <td>Actions</td>
                </tr>
            </tbody></table>
        `;

        updateFeedStatusCell(5, 'Connection timeout');

        const row = document.querySelector('tr[data-feed-id="5"]');
        const statusCell = row.querySelectorAll('td')[1];
        expect(statusCell.innerHTML).toContain('status-error');
        expect(statusCell.innerHTML).toContain('Connection timeout');
        expect(row.dataset.hasError).toBe('true');
    });

    it('sets OK status and removes hasError', () => {
        document.body.innerHTML = `
            <table><tbody>
                <tr data-feed-id="5" data-has-error="true">
                    <td>Feed name</td>
                    <td class="status-cell">Error</td>
                    <td>Actions</td>
                </tr>
            </tbody></table>
        `;

        updateFeedStatusCell(5, null);

        const row = document.querySelector('tr[data-feed-id="5"]');
        const statusCell = row.querySelectorAll('td')[1];
        expect(statusCell.innerHTML).toContain('status-ok');
        expect(row.dataset.hasError).toBeUndefined();
    });

    it('does nothing when row not found', () => {
        document.body.innerHTML = '<table><tbody></tbody></table>';
        updateFeedStatusCell(999, 'error'); // should not throw
    });

    it('does nothing when row has fewer than 2 cells', () => {
        document.body.innerHTML = `
            <table><tbody>
                <tr data-feed-id="5"><td>Only cell</td></tr>
            </tbody></table>
        `;
        // statusCell would be cells[-1] which is undefined
        updateFeedStatusCell(5, 'error'); // should not throw
    });
});

describe('updateFeedErrors', () => {
    it('adds error class and icon to feed items with errors', () => {
        document.body.innerHTML = `
            <div class="feed-item" data-feed-id="1">
                <span data-error></span>
            </div>
            <div class="feed-item" data-feed-id="2">
                <span data-error></span>
            </div>
        `;

        updateFeedErrors({ '1': 'Timeout' });

        const item1 = document.querySelector('[data-feed-id="1"]');
        const item2 = document.querySelector('[data-feed-id="2"]');
        expect(item1.classList.contains('has-error')).toBe(true);
        expect(item1.title).toBe('Error: Timeout');
        expect(item1.querySelector('[data-error]').textContent).toBe('\u26A0');
        expect(item2.classList.contains('has-error')).toBe(false);
        expect(item2.title).toBe('');
    });

    it('removes error class when error clears', () => {
        document.body.innerHTML = `
            <div class="feed-item has-error" data-feed-id="1" title="Error: old">
                <span data-error>\u26A0</span>
            </div>
        `;

        updateFeedErrors({});

        const item = document.querySelector('[data-feed-id="1"]');
        expect(item.classList.contains('has-error')).toBe(false);
        expect(item.title).toBe('');
        expect(item.querySelector('[data-error]').textContent).toBe('');
    });

    it('updates error banner on current feed page', () => {
        document.body.innerHTML = '<button data-feed-id="3">Refresh</button>';

        updateFeedErrors({ '3': 'Network error' });

        expect(showFeedErrorBanner).toHaveBeenCalledWith('3', 'Network error');
    });

    it('removes error banner when current feed has no errors', () => {
        document.body.innerHTML = '<button data-feed-id="3">Refresh</button>';

        updateFeedErrors({});

        expect(removeFeedErrorBanner).toHaveBeenCalled();
    });

    it('handles feed items without data-error span', () => {
        document.body.innerHTML = '<div class="feed-item" data-feed-id="1"></div>';

        updateFeedErrors({ '1': 'Timeout' });

        const item = document.querySelector('[data-feed-id="1"]');
        expect(item.classList.contains('has-error')).toBe(true);
        expect(item.title).toBe('Error: Timeout');
    });

    it('does not update banner when no button[data-feed-id] on page', () => {
        document.body.innerHTML = `
            <div class="feed-item" data-feed-id="1">
                <span data-error></span>
            </div>
        `;

        updateFeedErrors({ '1': 'Timeout' });

        expect(showFeedErrorBanner).not.toHaveBeenCalled();
        expect(removeFeedErrorBanner).not.toHaveBeenCalled();
    });
});
