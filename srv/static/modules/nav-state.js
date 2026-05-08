// Navigation state shared by modules that need to coordinate across full-page
// article navigations and browser back/forward restores.

const RETURNING_FROM_ARTICLE_LIST_KEY = 'feedreader:returning-from-article-list';

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
