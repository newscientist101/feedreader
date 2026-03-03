import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { initQueuePage, queueNext } from './queue.js';

// Mock the api and offline modules
vi.mock('./api.js', () => ({
    api: vi.fn(),
}));

vi.mock('./offline.js', () => ({
    updateQueueCacheIfStandalone: vi.fn(),
}));

import { api } from './api.js';
import { updateQueueCacheIfStandalone } from './offline.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('initQueuePage', () => {
    it('does nothing when #queue-data is absent', () => {
        document.body.innerHTML = '<div>Not the queue page</div>';
        // Should not throw
        initQueuePage();
    });

    it('does nothing when queue-data contains invalid JSON', () => {
        document.body.innerHTML = '<script id="queue-data" type="application/json">not-json</script>';
        expect(() => initQueuePage()).not.toThrow();
    });

    it('does nothing when queue-data is an empty array', () => {
        document.body.innerHTML = '<script id="queue-data" type="application/json">[]</script>';
        initQueuePage();
        // No button wired up — no error
    });

    it('does nothing when queue-data is not an array', () => {
        document.body.innerHTML = '<script id="queue-data" type="application/json">{"ids": [1,2]}</script>';
        initQueuePage();
        // Non-array JSON should be treated as invalid
    });

    it('does not crash when IDs are present but .queue-next-btn is missing', () => {
        document.body.innerHTML = '<script id="queue-data" type="application/json">[10,20]</script>';
        initQueuePage(); // should not throw
    });

    it('wires click on .queue-next-btn when IDs present', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML =
            '<script id="queue-data" type="application/json">[10,20,30]</script>' +
            '<button class="queue-next-btn">Next</button>';

        // Mock location.href
        const origLocation = window.location;
        delete window.location;
        window.location = { href: '' };

        initQueuePage();

        const btn = document.querySelector('.queue-next-btn');
        btn.click();

        // Wait for async handler
        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('DELETE', '/api/articles/10/queue');
        });

        expect(window.location.href).toBe('/queue');

        // Restore
        window.location = origLocation;
    });
});

describe('queueNext', () => {
    it('does nothing with empty array', async () => {
        await queueNext([]);
        expect(api).not.toHaveBeenCalled();
    });

    it('does nothing with null', async () => {
        await queueNext(null);
        expect(api).not.toHaveBeenCalled();
    });

    it('deletes first article from queue and navigates', async () => {
        api.mockResolvedValue({});
        const origLocation = window.location;
        delete window.location;
        window.location = { href: '' };

        await queueNext([42, 99]);

        expect(api).toHaveBeenCalledWith('DELETE', '/api/articles/42/queue');
        expect(window.location.href).toBe('/queue');

        window.location = origLocation;
    });

    it('calls updateQueueCacheIfStandalone', async () => {
        api.mockResolvedValue({});
        const origLocation = window.location;
        delete window.location;
        window.location = { href: '' };

        await queueNext([7]);

        expect(updateQueueCacheIfStandalone).toHaveBeenCalled();

        window.location = origLocation;
    });

    it('propagates API errors', async () => {
        api.mockRejectedValue(new Error('Server error'));

        await expect(queueNext([42])).rejects.toThrow('Server error');
    });

    it('does nothing with undefined', async () => {
        await queueNext(undefined);
        expect(api).not.toHaveBeenCalled();
    });
});
