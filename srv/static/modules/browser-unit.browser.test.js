/**
 * Browser-mode unit tests for modules that benefit from real browser layout.
 *
 * Modules tested:
 *   - drag-drop.js: real getBoundingClientRect for drag positioning
 *   - pagination.js: real scroll measurements for infinite scroll detection
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


let fetchSpy;
const pendingTimers = [];

beforeEach(() => {
    document.body.innerHTML = '';
    _resetPaginationState();
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
