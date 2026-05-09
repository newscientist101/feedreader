// Navigation state shared by modules that need to coordinate across full-page
// article navigations and browser back/forward restores.

const RETURNING_FROM_ARTICLE_LIST_KEY = 'feedreader:returning-from-article-list';
const PENDING_READ_IDS_KEY = 'feedreader:article-list-pending-read-ids';

export function markReturningFromArticleList() {
    try {
        sessionStorage.setItem(RETURNING_FROM_ARTICLE_LIST_KEY, '1');
        return true;
    } catch {
        return false;
    }
}

export function consumeReturningFromArticleList() {
    try {
        const value = sessionStorage.getItem(RETURNING_FROM_ARTICLE_LIST_KEY);
        if (value !== null) {
            sessionStorage.removeItem(RETURNING_FROM_ARTICLE_LIST_KEY);
        }
        return value === '1';
    } catch {
        return false;
    }
}

// --- Pending read IDs: survive full-page article navigation ---

// Merge additional IDs into the stored pending-read set. Duplicates are ignored.
export function mergePendingReadIds(ids) {
    try {
        const existing = _loadPendingReadIds();
        for (const id of ids) {
            existing.add(id);
        }
        sessionStorage.setItem(PENDING_READ_IDS_KEY, JSON.stringify([...existing]));
        return true;
    } catch {
        return false;
    }
}

// Return the stored pending-read IDs without clearing them.
export function peekPendingReadIds() {
    return _loadPendingReadIds();
}

// Clear the pending-read IDs key.
export function clearPendingReadIds() {
    try {
        sessionStorage.removeItem(PENDING_READ_IDS_KEY);
        return true;
    } catch {
        return false;
    }
}

// Internal helper: load and parse the stored set, returning an empty Set on any error.
function _loadPendingReadIds() {
    try {
        const raw = sessionStorage.getItem(PENDING_READ_IDS_KEY);
        if (raw === null) return new Set();
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return new Set();
        return new Set(parsed.filter(id => typeof id === 'number'));
    } catch {
        return new Set();
    }
}
