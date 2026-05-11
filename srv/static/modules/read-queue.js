// Durable sessionStorage-backed read queue.
//
// Replaces the ephemeral _markReadQueue array and the pendingReadIds helpers
// in nav-state.js with a single persistent store. IDs survive tab close and
// full-page navigations; replay() re-POSTs them on the next page load.

const STORAGE_KEY = 'feedreader:read-queue';
const API_URL = '/api/articles/batch-read';

// --- Module-scope fallback for when sessionStorage is unavailable ---
let _memoryFallback = null; // Set<number> | null; non-null means storage failed
let _warnedAboutStorage = false;

// --- Internal storage helpers ---

function _readFromStorage() {
    if (_memoryFallback !== null) return new Set(_memoryFallback);
    try {
        const raw = sessionStorage.getItem(STORAGE_KEY);
        if (raw === null) return new Set();
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return new Set();
        return new Set(parsed.filter(id => Number.isFinite(id)));
    } catch {
        return new Set();
    }
}

function _writeToStorage(set) {
    if (_memoryFallback !== null) {
        _memoryFallback = new Set(set);
        return;
    }
    try {
        sessionStorage.setItem(STORAGE_KEY, JSON.stringify([...set]));
    } catch (e) {
        if (!_warnedAboutStorage) {
            console.warn('[read-queue] sessionStorage unavailable, falling back to memory:', e);
            _warnedAboutStorage = true;
        }
        _memoryFallback = new Set(set);
    }
}

// --- Public API ---

/**
 * Enqueue an article ID. Idempotent — calling twice with the same id is a no-op.
 * Persists immediately to sessionStorage (or memory fallback).
 */
export function enqueueRead(id) {
    const num = Number(id);
    if (!Number.isFinite(num)) return;
    const current = _readFromStorage();
    if (current.has(num)) return;
    current.add(num);
    _writeToStorage(current);
}

/**
 * Return a Set<number> snapshot of currently pending ids. Does not modify storage.
 */
export function peekIds() {
    return _readFromStorage();
}

/**
 * Return the number of pending ids.
 */
export function size() {
    return _readFromStorage().size;
}

/**
 * Clear all pending ids from storage.
 */
export function clear() {
    if (_memoryFallback !== null) {
        _memoryFallback = new Set();
        return;
    }
    try {
        sessionStorage.removeItem(STORAGE_KEY);
    } catch {
        // ignore
    }
}

/**
 * POST pending ids to the server.
 *
 * Locked semantics:
 * 1. Empty queue → resolve true immediately, no fetch.
 * 2. Snapshot-and-remove: snapshot ids at start, remove only those on 2xx.
 *    IDs added during the in-flight POST are preserved.
 * 3. {keepalive:true} adds keepalive to fetch options.
 * 4. On failure, do NOT modify storage. Return false.
 * 5. No concurrency lock — snapshot-and-remove is sufficient.
 *
 * @param {object} [options]
 * @param {boolean} [options.keepalive=false]
 * @returns {Promise<boolean>}
 */
export function flush({ keepalive = false } = {}) {
    const snapshot = _readFromStorage();
    if (snapshot.size === 0) return Promise.resolve(true);

    const ids = [...snapshot];
    console.debug(`[auto-mark-read] flushing batch of ${ids.length} article(s):`, ids);
    const fetchOptions = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest',
        },
        body: JSON.stringify({ ids }),
    };
    if (keepalive) fetchOptions.keepalive = true;

    return fetch(API_URL, fetchOptions)
        .then(res => {
            if (!res.ok) return false;
            // Remove only the snapshotted ids; preserve any added during flight
            const current = _readFromStorage();
            for (const id of ids) {
                current.delete(id);
            }
            _writeToStorage(current);
            return true;
        })
        .catch(() => false);
}

/**
 * Alias for flush({keepalive:false}). Intended for Back-navigation replay.
 * @returns {Promise<boolean>}
 */
export function replay() {
    return flush({ keepalive: false });
}

// --- Test-only reset ---
export function _resetReadQueueState() {
    _memoryFallback = null;
    _warnedAboutStorage = false;
    try {
        sessionStorage.removeItem(STORAGE_KEY);
    } catch {
        // ignore
    }
}
