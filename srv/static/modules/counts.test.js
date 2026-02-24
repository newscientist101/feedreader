import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { updateCounts, updateFeedStatusCell, updateFeedErrors, setCountsDeps } from './counts.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    setCountsDeps({
        showFeedErrorBanner: vi.fn(),
        removeFeedErrorBanner: vi.fn(),
        applyUserPreferences: vi.fn(),
    });
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('updateCounts', () => {
    it('updates unread, starred, and queue badges', async () => {
        document.body.innerHTML = `
            <span data-count="unread">5</span>
            <span data-count="starred">2</span>
            <span data-count="queue">1</span>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 10, starred: 3, queue: 2,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(document.querySelector('[data-count="unread"]').textContent).toBe('10');
        expect(document.querySelector('[data-count="starred"]').textContent).toBe('3');
        expect(document.querySelector('[data-count="queue"]').textContent).toBe('2');
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
        const mockApply = vi.fn();
        setCountsDeps({
            showFeedErrorBanner: vi.fn(),
            removeFeedErrorBanner: vi.fn(),
            applyUserPreferences: mockApply,
        });
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({
                unread: 0, starred: 0, queue: 0,
                categories: {}, feeds: {}, feedErrors: {},
            }),
        });

        await updateCounts();

        expect(mockApply).toHaveBeenCalled();
    });

    it('handles API errors gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));
        vi.spyOn(console, 'error').mockImplementation(() => {});

        await updateCounts(); // should not throw

        expect(console.error).toHaveBeenCalledWith('Failed to update counts:', expect.any(Error));
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
        const mockShowBanner = vi.fn();
        const mockRemoveBanner = vi.fn();
        setCountsDeps({
            showFeedErrorBanner: mockShowBanner,
            removeFeedErrorBanner: mockRemoveBanner,
            applyUserPreferences: vi.fn(),
        });
        document.body.innerHTML = '<button data-feed-id="3">Refresh</button>';

        updateFeedErrors({ '3': 'Network error' });

        expect(mockShowBanner).toHaveBeenCalledWith('3', 'Network error');
    });

    it('removes error banner when current feed has no errors', () => {
        const mockRemoveBanner = vi.fn();
        setCountsDeps({
            showFeedErrorBanner: vi.fn(),
            removeFeedErrorBanner: mockRemoveBanner,
            applyUserPreferences: vi.fn(),
        });
        document.body.innerHTML = '<button data-feed-id="3">Refresh</button>';

        updateFeedErrors({});

        expect(mockRemoveBanner).toHaveBeenCalled();
    });
});
