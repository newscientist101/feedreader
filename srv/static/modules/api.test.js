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
            headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
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
            headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
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

    it('throws generic message when JSON error field is missing', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ status: 'error' })),
        });

        await expect(api('GET', '/api/broken')).rejects.toThrow('Request failed');
    });

    it('propagates network errors from fetch', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'));

        await expect(api('GET', '/api/test')).rejects.toThrow('Failed to fetch');
    });

    it('does not include body for GET requests with null data', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });

        await api('GET', '/api/test', null);
        const callOptions = fetch.mock.calls[0][1];
        expect(callOptions.body).toBeUndefined();
    });

    it('sends body for PUT and DELETE methods', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });

        await api('PUT', '/api/item/1', { name: 'updated' });
        expect(fetch).toHaveBeenCalledWith('/api/item/1', expect.objectContaining({
            method: 'PUT',
            body: JSON.stringify({ name: 'updated' }),
        }));

        await api('DELETE', '/api/item/1', { id: 1 });
        expect(fetch).toHaveBeenCalledWith('/api/item/1', expect.objectContaining({
            method: 'DELETE',
            body: JSON.stringify({ id: 1 }),
        }));
    });

    it('throws generic message when JSON error field is empty string', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: '' })),
        });

        await expect(api('GET', '/api/broken')).rejects.toThrow('Request failed');
    });
});
