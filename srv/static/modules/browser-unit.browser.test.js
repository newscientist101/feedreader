/**
 * Browser-mode unit tests for modules that benefit from real browser layout.
 *
 * Modules tested:
 *   - drag-drop.js: real getBoundingClientRect for drag positioning
 *   - pagination.js: real scroll measurements for infinite scroll detection
 *   - article-actions.js: real IntersectionObserver for auto-mark-read
 *   - articles.js: real scrollTo and IntersectionObserver in renderArticles
 *
 * Note: vi.mock() causes process hangs in Vitest browser mode (v4.0.18).
 * We use vi.spyOn(globalThis, 'fetch') instead.
 *
 * Run with: npx vitest run --config vitest.browser-unit.config.mjs
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    initDragDrop, getDragAfterElementAmongSiblings,
} from './drag-drop.js';
import {
    checkScrollForMore, setPaginationState, getPaginationState,
    _resetPaginationState,
} from './pagination.js';
import {
    initAutoMarkRead, observeNewArticles,
    _resetArticleActionsState,
    _getAutoMarkReadObserver,
} from './article-actions.js';
import { renderArticles, _resetArticlesState } from './articles.js';


let fetchSpy;
const pendingTimers = [];

beforeEach(() => {
    document.body.innerHTML = '';
    _resetPaginationState();
    _resetArticleActionsState();
    _resetArticlesState();
    window.__settings = {};
    vi.restoreAllMocks();
    vi.clearAllMocks();
    fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({}),
        text: () => Promise.resolve('{}'),
    });
    // Track setTimeout calls so we can clear them in afterEach.
    // This prevents showToast's 4-second auto-dismiss timers from
    // keeping the browser process alive after tests complete.
    const origSetTimeout = globalThis.setTimeout;
    vi.spyOn(globalThis, 'setTimeout').mockImplementation((fn, delay, ...args) => {
        const id = origSetTimeout(fn, delay, ...args);
        pendingTimers.push(id);
        return id;
    });
});

afterEach(() => {
    // Clear any pending timers (e.g., showToast auto-dismiss)
    for (const id of pendingTimers) clearTimeout(id);
    pendingTimers.length = 0;
    vi.restoreAllMocks();
});

// --- Helpers ---

function createLayoutContainer(itemCount, { itemHeight = 50 } = {}) {
    const container = document.createElement('div');
    container.id = 'container';
    container.style.cssText = 'position: relative; width: 300px;';
    for (let i = 1; i <= itemCount; i++) {
        const item = document.createElement('div');
        item.className = 'item';
        item.setAttribute('data-id', String(i));
        item.draggable = true;
        item.textContent = `Item ${i}`;
        item.style.cssText = `display: block; height: ${itemHeight}px; width: 300px; box-sizing: border-box;`;
        container.appendChild(item);
    }
    document.body.appendChild(container);
    return container;
}

function startDrag(container, dataId) {
    const item = container.querySelector(`.item[data-id="${dataId}"]`);
    const event = new Event('dragstart', { bubbles: true });
    Object.defineProperty(event, 'dataTransfer', {
        value: { effectAllowed: '', setData: vi.fn() },
    });
    item.dispatchEvent(event);
    return item;
}

function createDropEvent() {
    const event = new Event('drop', { bubbles: true, cancelable: true });
    Object.defineProperty(event, 'dataTransfer', {
        value: { dropEffect: '' },
    });
    return event;
}

function mockFetchOk(data = {}) {
    fetchSpy.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(data),
        text: () => Promise.resolve(JSON.stringify(data)),
    });
}

function mockFetchFail(message = 'Server error') {
    fetchSpy.mockResolvedValueOnce({
        ok: false,
        status: 500,
        text: () => Promise.resolve(JSON.stringify({ error: message })),
    });
}

// --- Drag-drop: getDragAfterElementAmongSiblings ---

describe('getDragAfterElementAmongSiblings (real layout)', () => {
    it('returns the element whose midpoint is below the cursor Y', () => {
        const container = createLayoutContainer(3, { itemHeight: 50 });
        const items = Array.from(container.querySelectorAll('.item'));
        const result = getDragAfterElementAmongSiblings(items, 10);
        expect(result).toBe(items[0]);
    });

    it('returns undefined when cursor is below all elements', () => {
        const container = createLayoutContainer(2, { itemHeight: 50 });
        const items = Array.from(container.querySelectorAll('.item'));
        const result = getDragAfterElementAmongSiblings(items, 200);
        expect(result).toBeUndefined();
    });

    it('skips elements with dragging class', () => {
        const container = createLayoutContainer(2, { itemHeight: 50 });
        const items = Array.from(container.querySelectorAll('.item'));
        items[0].classList.add('dragging');
        const result = getDragAfterElementAmongSiblings(items, 10);
        expect(result).toBe(items[1]);
    });

    it('returns closest element when cursor is between items', () => {
        const container = createLayoutContainer(3, { itemHeight: 40 });
        const items = Array.from(container.querySelectorAll('.item'));
        const result = getDragAfterElementAmongSiblings(items, 50);
        expect(result).toBe(items[1]);
    });

    it('handles empty array', () => {
        const result = getDragAfterElementAmongSiblings([], 50);
        expect(result).toBeUndefined();
    });
});

// --- Drag-drop: placeholder positioning ---

describe('initDragDrop — dragover placeholder positioning (real layout)', () => {
    it('inserts placeholder before add-category card when cursor is below all items', () => {
        const container = createLayoutContainer(2, { itemHeight: 50 });
        const addCard = document.createElement('div');
        addCard.className = 'add-category';
        addCard.textContent = '+ Add';
        container.appendChild(addCard);

        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '1');

        const ev = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(ev, 'dataTransfer', { value: { dropEffect: '' } });
        Object.defineProperty(ev, 'shiftKey', { value: false });
        Object.defineProperty(ev, 'clientY', { value: 200 });
        container.dispatchEvent(ev);

        const placeholder = container.querySelector('.drag-placeholder');
        expect(placeholder).not.toBeNull();
        expect(placeholder.nextElementSibling).toBe(addCard);
    });

    it('inserts placeholder before an item when cursor is above its midpoint', () => {
        const container = createLayoutContainer(3, { itemHeight: 50 });
        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '3');

        const ev = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(ev, 'dataTransfer', { value: { dropEffect: '' } });
        Object.defineProperty(ev, 'shiftKey', { value: false });
        Object.defineProperty(ev, 'clientY', { value: 10 });
        container.querySelector('.item[data-id="1"]').dispatchEvent(ev);

        const placeholder = container.querySelector('.drag-placeholder');
        expect(placeholder).not.toBeNull();
        const children = Array.from(container.children);
        expect(children.indexOf(placeholder)).toBeLessThan(
            children.indexOf(container.querySelector('.item[data-id="1"]')),
        );
    });
});

// --- Drag-drop: drop reorder ---

describe('initDragDrop — drop reorder (real layout)', () => {
    it('calls api to save reorder after drag and drop', async () => {
        const container = createLayoutContainer(2, { itemHeight: 50 });
        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '2');

        const ev = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(ev, 'dataTransfer', { value: { dropEffect: '' } });
        Object.defineProperty(ev, 'shiftKey', { value: false });
        Object.defineProperty(ev, 'clientY', { value: 10 });
        container.querySelector('.item[data-id="1"]').dispatchEvent(ev);

        mockFetchOk({});
        container.dispatchEvent(createDropEvent());

        await vi.waitFor(() => {
            expect(fetchSpy).toHaveBeenCalledWith(
                '/api/categories/reorder',
                expect.objectContaining({ method: 'POST' }),
            );
        });
    });

    it('shows console.error on reorder api failure', async () => {
        const container = createLayoutContainer(2, { itemHeight: 50 });
        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '2');

        const ev = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(ev, 'dataTransfer', { value: { dropEffect: '' } });
        Object.defineProperty(ev, 'shiftKey', { value: false });
        Object.defineProperty(ev, 'clientY', { value: 10 });
        container.querySelector('.item[data-id="1"]').dispatchEvent(ev);

        mockFetchFail('Server error');
        const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

        container.dispatchEvent(createDropEvent());

        await vi.waitFor(() => {
            expect(consoleSpy).toHaveBeenCalledWith(
                'Failed to save folder order:',
                expect.any(Error),
            );
        });
    });
});

// --- Drag-drop: drop nesting ---

describe('initDragDrop — drop nesting (real layout)', () => {
    it('shows console.error on nesting api failure', async () => {
        const container = createLayoutContainer(2, { itemHeight: 50 });
        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '1');

        const target = container.querySelector('.item[data-id="2"]');
        const rect = target.getBoundingClientRect();
        const ev = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(ev, 'dataTransfer', { value: { dropEffect: '' } });
        Object.defineProperty(ev, 'shiftKey', { value: true });
        Object.defineProperty(ev, 'clientY', { value: rect.top + rect.height / 2 });
        target.dispatchEvent(ev);

        mockFetchFail('Server error');
        const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

        container.dispatchEvent(createDropEvent());

        await vi.waitFor(() => {
            expect(consoleSpy).toHaveBeenCalledWith(
                'Failed to nest folder:',
                expect.any(Error),
            );
        });
    });

    it('calls api to nest folder on shift+drop', async () => {
        const container = createLayoutContainer(2, { itemHeight: 50 });
        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '1');

        const target = container.querySelector('.item[data-id="2"]');
        const rect = target.getBoundingClientRect();
        const ev = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(ev, 'dataTransfer', { value: { dropEffect: '' } });
        Object.defineProperty(ev, 'shiftKey', { value: true });
        Object.defineProperty(ev, 'clientY', { value: rect.top + rect.height / 2 });
        target.dispatchEvent(ev);

        expect(target.classList.contains('nest-target')).toBe(true);

        // Use a never-resolving fetch to prevent location.reload() from
        // firing (which would reload the test iframe in browser mode).
        // We verify the fetch was initiated with the correct URL and body.
        fetchSpy.mockReturnValueOnce(new Promise(() => {}));
        container.dispatchEvent(createDropEvent());

        await vi.waitFor(() => {
            const calls = fetchSpy.mock.calls;
            const nestCall = calls.find(c => c[0].includes('/api/categories/1/parent'));
            expect(nestCall).toBeDefined();
            const body = JSON.parse(nestCall[1].body);
            expect(body.parent_id).toBe(2);
            expect(body.sort_order).toBe(0);
        });
    });
});

// --- Pagination: checkScrollForMore ---

describe('checkScrollForMore (real scroll measurements)', () => {
    it('does nothing when pagination is done', () => {
        setPaginationState({ done: true });
        checkScrollForMore();
        expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('does nothing when loading', () => {
        setPaginationState({ loading: true });
        checkScrollForMore();
        expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('does not load when page has tall content and not scrolled', () => {
        const tall = document.createElement('div');
        tall.style.height = '5000px';
        document.body.appendChild(tall);

        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });

        checkScrollForMore();
        expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('triggers load when near bottom of a short page', async () => {
        const short = document.createElement('div');
        short.style.height = '100px';
        document.body.appendChild(short);

        const list = document.createElement('div');
        list.id = 'articles-list';
        document.body.appendChild(list);

        setPaginationState({
            done: false,
            loading: false,
            cursorTime: '2025-01-01T00:00:00Z',
            cursorId: '999',
        });

        // Return empty articles so done=true (prevents recursive cascade)
        fetchSpy.mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ articles: [] }),
            text: () => Promise.resolve(JSON.stringify({ articles: [] })),
        });

        checkScrollForMore();
        await new Promise(resolve => setTimeout(resolve, 200));

        expect(fetchSpy).toHaveBeenCalled();
        expect(getPaginationState().done).toBe(true);
    });
});

// --- Article-actions: initAutoMarkRead (real IntersectionObserver) ---

describe('initAutoMarkRead (real IntersectionObserver)', () => {
    beforeEach(() => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
    });

    it('does nothing when autoMarkRead setting is not true', () => {
        window.__settings = { autoMarkRead: 'false' };
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).toBeNull();
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] disabled by setting');
    });

    it('creates a real IntersectionObserver when autoMarkRead is true', () => {
        window.__settings = { autoMarkRead: 'true' };
        const list = document.createElement('div');
        list.id = 'articles-list';
        list.innerHTML = '<div class="article-card" data-id="1"></div>';
        document.body.appendChild(list);

        initAutoMarkRead();
        const obs = _getAutoMarkReadObserver();
        expect(obs).not.toBeNull();
        expect(obs).toBeInstanceOf(IntersectionObserver);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 1 initial articles');
    });

    it('disconnects previous observer on re-init', () => {
        window.__settings = { autoMarkRead: 'true' };
        const list = document.createElement('div');
        list.id = 'articles-list';
        document.body.appendChild(list);

        initAutoMarkRead();
        const first = _getAutoMarkReadObserver();
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).not.toBe(first);
    });
});

// --- Article-actions: observeNewArticles (real IntersectionObserver) ---

describe('observeNewArticles (real IntersectionObserver)', () => {
    beforeEach(() => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
    });

    it('is a no-op when observer is null', () => {
        const container = document.createElement('div');
        container.innerHTML = '<div class="article-card"></div>';
        observeNewArticles(container);
        // Should not throw
    });

    it('observes new cards via real IntersectionObserver', () => {
        window.__settings = { autoMarkRead: 'true' };
        const list = document.createElement('div');
        list.id = 'articles-list';
        document.body.appendChild(list);

        initAutoMarkRead();
        const obs = _getAutoMarkReadObserver();
        const spy = vi.spyOn(obs, 'observe');
        const container = document.createElement('div');
        container.innerHTML = '<div class="article-card"></div><div class="article-card"></div>';
        observeNewArticles(container);
        expect(spy).toHaveBeenCalledTimes(2);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 new articles');
    });
});

// --- Article-actions: auto-mark-read integration (real IntersectionObserver) ---

describe('auto-mark-read integration (real IntersectionObserver)', () => {
    beforeEach(() => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        window.__settings = { autoMarkRead: 'true' };
    });

    it('observer works on initial page load articles', () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        list.innerHTML = `
            <article class="article-card" data-id="1"></article>
            <article class="article-card" data-id="2"></article>
        `;
        document.body.appendChild(list);

        const observeSpy = vi.spyOn(IntersectionObserver.prototype, 'observe');
        initAutoMarkRead();
        expect(_getAutoMarkReadObserver()).not.toBeNull();
        expect(_getAutoMarkReadObserver()).toBeInstanceOf(IntersectionObserver);
        expect(observeSpy).toHaveBeenCalledTimes(2);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 initial articles');
        observeSpy.mockRestore();
    });

    it('observer is re-created after renderArticles (client-side nav)', async () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        list.innerHTML = '<article class="article-card" data-id="1"></article>';
        document.body.appendChild(list);

        initAutoMarkRead();
        const initialObserver = _getAutoMarkReadObserver();

        // renderArticles calls scrollTo(0,0) — real browser supports it
        await renderArticles([
            { id: 100, title: 'VR Article', is_read: 0, is_starred: 0, feed_name: 'VR Feed' },
            { id: 101, title: 'VR News', is_read: 0, is_starred: 0, feed_name: 'VR Feed' },
        ]);

        expect(_getAutoMarkReadObserver()).not.toBe(initialObserver);
        expect(_getAutoMarkReadObserver()).not.toBeNull();
        expect(_getAutoMarkReadObserver()).toBeInstanceOf(IntersectionObserver);

        const cards = document.querySelectorAll('#articles-list .article-card');
        expect(cards.length).toBe(2);
        expect(cards[0].dataset.id).toBe('100');
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 initial articles');
    });

    it('new paginated articles are observed', () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        document.body.appendChild(list);

        initAutoMarkRead();
        const spy = vi.spyOn(_getAutoMarkReadObserver(), 'observe');

        const temp = document.createElement('div');
        temp.innerHTML = `
            <article class="article-card" data-id="50"></article>
            <article class="article-card" data-id="51"></article>
            <article class="article-card" data-id="52"></article>
        `;
        observeNewArticles(temp);

        expect(spy).toHaveBeenCalledTimes(3);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 3 new articles');
    });

    it('multiple navigations each get a fresh observer', async () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        document.body.appendChild(list);

        const observers = [];

        for (let i = 0; i < 3; i++) {
            await renderArticles([
                { id: i * 10 + 1, title: `Article ${i}`, is_read: 0, is_starred: 0 },
            ]);
            observers.push(_getAutoMarkReadObserver());
        }

        expect(observers[0]).not.toBe(observers[1]);
        expect(observers[1]).not.toBe(observers[2]);
        observers.forEach(obs => {
            expect(obs).not.toBeNull();
            expect(obs).toBeInstanceOf(IntersectionObserver);
        });
    });
});

// --- Articles: renderArticles (real scrollTo + IntersectionObserver) ---

describe('renderArticles (real browser)', () => {
    beforeEach(() => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
        window.__settings = { autoMarkRead: 'true' };
    });

    it('renders articles and calls real scrollTo', async () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        list.className = 'articles-list';
        document.body.appendChild(list);

        const scrollSpy = vi.spyOn(window, 'scrollTo');

        const articles = [
            { id: 1, title: 'Article 1', is_read: 0, is_starred: 0 },
            { id: 2, title: 'Article 2', is_read: 0, is_starred: 0 },
        ];
        await renderArticles(articles);

        expect(scrollSpy).toHaveBeenCalledWith(0, 0);
        const cards = document.querySelectorAll('#articles-list .article-card');
        expect(cards.length).toBe(2);
        expect(cards[0].dataset.id).toBe('1');
        expect(cards[1].dataset.id).toBe('2');
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 2 initial articles');
    });

    it('re-initializes real IntersectionObserver after render', async () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        list.className = 'articles-list';
        document.body.appendChild(list);

        initAutoMarkRead();
        const firstObserver = _getAutoMarkReadObserver();

        await renderArticles([
            { id: 10, title: 'New', is_read: 0, is_starred: 0 },
        ]);

        expect(_getAutoMarkReadObserver()).not.toBeNull();
        expect(_getAutoMarkReadObserver()).not.toBe(firstObserver);
        expect(_getAutoMarkReadObserver()).toBeInstanceOf(IntersectionObserver);
        expect(console.debug).toHaveBeenCalledWith('[auto-mark-read] observing 1 initial articles');
    });

    it('handles empty article list with real browser', async () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        list.className = 'articles-list';
        document.body.appendChild(list);

        await renderArticles([]);
        const cards = document.querySelectorAll('#articles-list .article-card');
        expect(cards.length).toBe(0);
        expect(document.querySelector('#articles-list .empty-state')).not.toBeNull();
    });

    it('sets aria-busy to false after rendering', async () => {
        const list = document.createElement('div');
        list.id = 'articles-list';
        list.className = 'articles-list';
        document.body.appendChild(list);

        await renderArticles([{ id: 1, title: 'T', is_read: 0, is_starred: 0 }]);
        expect(document.getElementById('articles-list').getAttribute('aria-busy')).toBe('false');
    });
});
