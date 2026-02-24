import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from './api.js';

describe('api', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
    });

    it('makes a GET request and returns JSON', async () => {
        const mockData = { id: 1, title: 'test' };
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve(mockData),
        });

        const result = await api('GET', '/api/test');
        expect(result).toEqual(mockData);
        expect(fetch).toHaveBeenCalledWith('/api/test', {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' },
        });
    });

    it('sends JSON body for POST requests', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });

        await api('POST', '/api/test', { name: 'foo' });
        expect(fetch).toHaveBeenCalledWith('/api/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: 'foo' }),
        });
    });

    it('throws on non-ok response with JSON error', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'Not found' })),
        });

        await expect(api('GET', '/api/missing')).rejects.toThrow('Not found');
    });

    it('throws on non-ok response with plain text', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve('Server error'),
        });

        await expect(api('GET', '/api/broken')).rejects.toThrow('Server error');
    });

    it('throws generic message for empty error response', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(''),
        });

        await expect(api('GET', '/api/broken')).rejects.toThrow('Request failed');
    });
});
