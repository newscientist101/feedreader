import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    initFolderDragDrop, initDragDrop, syncFolderOrder,
    reorderElements, initDragPrevention,
} from './drag-drop.js';

vi.mock('./api.js');
vi.mock('./toast.js');

import { api } from './api.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

afterEach(() => {
    vi.restoreAllMocks();
});

// getDragAfterElementAmongSiblings tests migrated to browser-unit.browser.test.js
// (uses real browser layout instead of mocked getBoundingClientRect)

describe('reorderElements', () => {
    it('reorders elements to match the given order', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1">A</div>
                <div class="item" data-id="2">B</div>
                <div class="item" data-id="3">C</div>
            </div>
        `;
        const container = document.getElementById('container');

        reorderElements(container, '.item', 'data-id', [3, 1, 2]);

        const items = container.querySelectorAll('.item');
        expect(items[0].dataset.id).toBe('3');
        expect(items[1].dataset.id).toBe('1');
        expect(items[2].dataset.id).toBe('2');
    });

    it('only reorders items with matching parentId', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1">A</div>
                <div class="item" data-id="2" data-parent-id="5">B</div>
                <div class="item" data-id="3">C</div>
            </div>
        `;
        const container = document.getElementById('container');

        // Only reorder top-level items (parentId=null): [1, 3] → [3, 1]
        reorderElements(container, '.item', 'data-id', [3, 1], null);

        const items = container.querySelectorAll('.item');
        // Item 2 (child of parent 5) not touched; top-level items reordered
        expect(items[0].dataset.id).toBe('2'); // stays in its DOM position
        expect(items[1].dataset.id).toBe('3');
        expect(items[2].dataset.id).toBe('1');
    });

    it('does nothing for empty container', () => {
        document.body.innerHTML = '<div id="container"></div>';
        const container = document.getElementById('container');

        reorderElements(container, '.item', 'data-id', [1, 2, 3]);

        expect(container.children.length).toBe(0);
    });

    it('ignores items with non-numeric IDs', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="abc">X</div>
                <div class="item" data-id="1">A</div>
                <div class="item" data-id="2">B</div>
            </div>
        `;
        const container = document.getElementById('container');

        reorderElements(container, '.item', 'data-id', [2, 1]);

        const items = container.querySelectorAll('.item');
        // Non-numeric item stays, numeric items reordered
        expect(items[0].dataset.id).toBe('abc');
        expect(items[1].dataset.id).toBe('2');
        expect(items[2].dataset.id).toBe('1');
    });

    it('reorders only children of specified parentId', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1">Top A</div>
                <div class="item" data-id="10" data-parent-id="1">Child A</div>
                <div class="item" data-id="11" data-parent-id="1">Child B</div>
                <div class="item" data-id="2">Top B</div>
            </div>
        `;
        const container = document.getElementById('container');

        // Reorder children of parent 1
        reorderElements(container, '.item', 'data-id', [11, 10], 1);

        const items = container.querySelectorAll('.item');
        expect(items[0].dataset.id).toBe('1');
        expect(items[1].dataset.id).toBe('11');
        expect(items[2].dataset.id).toBe('10');
        expect(items[3].dataset.id).toBe('2');
    });

    it('handles partial order (some IDs not in order array)', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1">A</div>
                <div class="item" data-id="2">B</div>
                <div class="item" data-id="3">C</div>
            </div>
        `;
        const container = document.getElementById('container');

        // Order says [3, 1] — item 2 not in order array, stays in place
        reorderElements(container, '.item', 'data-id', [3, 1]);

        const items = container.querySelectorAll('.item');
        // Item 2 not moved; items 3 and 1 reordered relative to each other
        expect(items[0].dataset.id).toBe('2');
        expect(items[1].dataset.id).toBe('3');
        expect(items[2].dataset.id).toBe('1');
    });
});

describe('syncFolderOrder', () => {
    it('reorders sidebar folders when source is categories grid', () => {
        document.body.innerHTML = `
            <div class="folders-list">
                <div class="folder-item" data-category-id="1">A</div>
                <div class="folder-item" data-category-id="2">B</div>
                <div class="folder-item" data-category-id="3">C</div>
            </div>
            <div class="categories-grid">
                <div class="category-card" data-id="1">A</div>
            </div>
        `;
        const categoriesGrid = document.querySelector('.categories-grid');

        syncFolderOrder([3, 1, 2], categoriesGrid);

        const items = document.querySelectorAll('.folders-list .folder-item');
        expect(items[0].dataset.categoryId).toBe('3');
        expect(items[1].dataset.categoryId).toBe('1');
        expect(items[2].dataset.categoryId).toBe('2');
    });

    it('does not reorder source container', () => {
        document.body.innerHTML = `
            <div class="folders-list">
                <div class="folder-item" data-category-id="1">A</div>
                <div class="folder-item" data-category-id="2">B</div>
            </div>
        `;
        const foldersContainer = document.querySelector('.folders-list');

        // Source is the same as sidebar — should not reorder
        syncFolderOrder([2, 1], foldersContainer);

        const items = document.querySelectorAll('.folders-list .folder-item');
        expect(items[0].dataset.categoryId).toBe('1'); // unchanged
        expect(items[1].dataset.categoryId).toBe('2');
    });

    it('syncs categories grid when source is sidebar', () => {
        document.body.innerHTML = `
            <div class="folders-list">
                <div class="folder-item" data-category-id="1">A</div>
            </div>
            <div class="categories-grid">
                <div class="category-card" data-id="1">A</div>
                <div class="category-card" data-id="2">B</div>
                <div class="category-card" data-id="3">C</div>
            </div>
        `;
        const foldersContainer = document.querySelector('.folders-list');

        syncFolderOrder([3, 1, 2], foldersContainer);

        const items = document.querySelectorAll('.categories-grid .category-card');
        expect(items[0].dataset.id).toBe('3');
        expect(items[1].dataset.id).toBe('1');
        expect(items[2].dataset.id).toBe('2');
    });

    it('passes parentId through to reorderElements', () => {
        document.body.innerHTML = `
            <div class="folders-list">
                <div class="folder-item" data-category-id="1">A</div>
                <div class="folder-item" data-category-id="10" data-parent-id="5">B</div>
                <div class="folder-item" data-category-id="11" data-parent-id="5">C</div>
            </div>
            <div class="categories-grid"></div>
        `;
        const categoriesGrid = document.querySelector('.categories-grid');

        // Only reorder children of parent 5 in sidebar
        syncFolderOrder([11, 10], categoriesGrid, 5);

        const items = document.querySelectorAll('.folders-list .folder-item');
        // Top-level item (id=1) untouched, children of 5 reordered
        expect(items[0].dataset.categoryId).toBe('1');
        expect(items[1].dataset.categoryId).toBe('11');
        expect(items[2].dataset.categoryId).toBe('10');
    });

    it('handles missing containers gracefully', () => {
        document.body.innerHTML = '<div>Nothing here</div>';
        // Neither container exists — should not throw
        expect(() => syncFolderOrder([1, 2], document.createElement('div'))).not.toThrow();
    });
});

describe('initFolderDragDrop', () => {
    it('does not throw when no containers exist', () => {
        document.body.innerHTML = '<div>No folders</div>';
        expect(() => initFolderDragDrop()).not.toThrow();
    });

    it('initializes on sidebar folders container', () => {
        document.body.innerHTML = `
            <div class="folders-list">
                <div class="folder-item" data-category-id="1" draggable="true">Folder 1</div>
                <div class="folder-item" data-category-id="2" draggable="true">Folder 2</div>
            </div>
        `;
        // Should not throw
        expect(() => initFolderDragDrop()).not.toThrow();
    });

    it('initializes on categories grid', () => {
        document.body.innerHTML = `
            <div class="categories-grid">
                <div class="category-card" data-id="1" draggable="true">Cat 1</div>
                <div class="category-card" data-id="2" draggable="true">Cat 2</div>
            </div>
        `;
        expect(() => initFolderDragDrop()).not.toThrow();
    });

    it('initializes both containers when both exist', () => {
        document.body.innerHTML = `
            <div class="folders-list">
                <div class="folder-item" data-category-id="1" draggable="true">F1</div>
            </div>
            <div class="categories-grid">
                <div class="category-card" data-id="1" draggable="true">C1</div>
            </div>
        `;
        expect(() => initFolderDragDrop()).not.toThrow();
    });
});

describe('initDragDrop', () => {
    it('adds dragging class on dragstart', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
                <div class="item" data-id="2" draggable="true">B</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        const item = container.querySelector('.item[data-id="1"]');
        const event = new Event('dragstart', { bubbles: true });
        // Mock dataTransfer
        Object.defineProperty(event, 'dataTransfer', {
            value: {
                effectAllowed: '',
                setData: vi.fn(),
            },
        });
        item.dispatchEvent(event);

        expect(item.classList.contains('dragging')).toBe(true);
    });

    it('removes dragging class on dragend', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        const item = container.querySelector('.item[data-id="1"]');
        // First, start drag
        const startEvent = new Event('dragstart', { bubbles: true });
        Object.defineProperty(startEvent, 'dataTransfer', {
            value: { effectAllowed: '', setData: vi.fn() },
        });
        item.dispatchEvent(startEvent);
        expect(item.classList.contains('dragging')).toBe(true);

        // Then end drag
        const endEvent = new Event('dragend', { bubbles: true });
        container.dispatchEvent(endEvent);
        expect(item.classList.contains('dragging')).toBe(false);
    });

    it('prevents default on dragover', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        const event = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(event, 'dataTransfer', {
            value: { dropEffect: '' },
        });
        container.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(true);
    });
});

describe('initDragDrop — dragstart edge cases', () => {
    it('ignores dragstart when target is not a matching item', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
                <span class="other">Not an item</span>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        const other = container.querySelector('.other');
        const event = new Event('dragstart', { bubbles: true });
        Object.defineProperty(event, 'dataTransfer', {
            value: { effectAllowed: '', setData: vi.fn() },
        });
        other.dispatchEvent(event);

        // No item should have dragging class
        expect(container.querySelector('.dragging')).toBeNull();
    });

    it('copies paddingLeft to placeholder for indented items', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true" style="padding-left: 20px;">A</div>
                <div class="item" data-id="2" draggable="true">B</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        const item = container.querySelector('.item[data-id="1"]');
        const event = new Event('dragstart', { bubbles: true });
        Object.defineProperty(event, 'dataTransfer', {
            value: { effectAllowed: '', setData: vi.fn() },
        });
        item.dispatchEvent(event);

        expect(item.classList.contains('dragging')).toBe(true);
        expect(event.dataTransfer.setData).toHaveBeenCalledWith('text/plain', '1');
    });
});

describe('initDragDrop — dragend cleanup', () => {
    it('removes drag-over and nest-target classes on dragend', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item drag-over" data-id="1" draggable="true">A</div>
                <div class="item nest-target" data-id="2" draggable="true">B</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        container.dispatchEvent(new Event('dragend', { bubbles: true }));

        expect(container.querySelector('.drag-over')).toBeNull();
        expect(container.querySelector('.nest-target')).toBeNull();
    });

    it('handles dragend when no drag was started', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        // dragend without dragstart should not throw
        expect(() => {
            container.dispatchEvent(new Event('dragend', { bubbles: true }));
        }).not.toThrow();
    });
});

describe('initDragDrop — dragover shift-key nesting', () => {
    function startDrag(container, itemSelector, dataId) {
        const item = container.querySelector(`${itemSelector}[data-id="${dataId}"]`);
        const event = new Event('dragstart', { bubbles: true });
        Object.defineProperty(event, 'dataTransfer', {
            value: { effectAllowed: '', setData: vi.fn() },
        });
        item.dispatchEvent(event);
        return item;
    }

    it('adds nest-target class when shift key held over another item', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
                <div class="item" data-id="2" draggable="true">B</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '.item', '1');

        const target = container.querySelector('.item[data-id="2"]');
        const dragoverEvent = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(dragoverEvent, 'dataTransfer', {
            value: { dropEffect: '' },
        });
        Object.defineProperty(dragoverEvent, 'shiftKey', { value: true });
        Object.defineProperty(dragoverEvent, 'clientY', { value: 50 });
        target.dispatchEvent(dragoverEvent);

        expect(target.classList.contains('nest-target')).toBe(true);
    });

    it('removes nest-target when shift key not held', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
                <div class="item nest-target" data-id="2" draggable="true">B</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');
        startDrag(container, '.item', '1');

        const target = container.querySelector('.item[data-id="2"]');
        const dragoverEvent = new Event('dragover', { bubbles: true, cancelable: true });
        Object.defineProperty(dragoverEvent, 'dataTransfer', {
            value: { dropEffect: '' },
        });
        Object.defineProperty(dragoverEvent, 'shiftKey', { value: false });
        Object.defineProperty(dragoverEvent, 'clientY', { value: 50 });
        target.dispatchEvent(dragoverEvent);

        expect(target.classList.contains('nest-target')).toBe(false);
    });
});

// dragover placeholder positioning tests migrated to browser-unit.browser.test.js
// (uses real browser layout instead of mocked getBoundingClientRect)

// Drop reorder tests with real layout migrated to browser-unit.browser.test.js

describe('initDragDrop — drop reorder', () => {
    it('does nothing on drop when no drag started', () => {
        document.body.innerHTML = `
            <div id="container">
                <div class="item" data-id="1" draggable="true">A</div>
            </div>
        `;
        const container = document.getElementById('container');
        initDragDrop(container, '.item', 'data-id');

        const dropEvent = new Event('drop', { bubbles: true, cancelable: true });
        Object.defineProperty(dropEvent, 'dataTransfer', {
            value: { dropEffect: '' },
        });
        container.dispatchEvent(dropEvent);

        expect(api).not.toHaveBeenCalled();
    });
});

// Drop nesting (shift+drag) tests migrated to browser-unit.browser.test.js
// (uses real browser layout instead of mocked getBoundingClientRect)

describe('initDragPrevention', () => {
    it('prevents dragstart on folder chevrons', () => {
        document.body.innerHTML = `
            <div class="folder-item">
                <span class="folder-chevron">▶</span>
                <span class="folder-name">Folder</span>
            </div>
        `;
        initDragPrevention();

        const chevron = document.querySelector('.folder-chevron');
        const event = new Event('dragstart', { bubbles: true, cancelable: true });
        chevron.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(true);
    });

    it('does not prevent dragstart on non-chevron elements', () => {
        document.body.innerHTML = `
            <div class="folder-item" draggable="true">
                <span class="folder-chevron">▶</span>
                <span class="folder-name">Folder</span>
            </div>
        `;
        initDragPrevention();

        const folderName = document.querySelector('.folder-name');
        const event = new Event('dragstart', { bubbles: true, cancelable: true });
        folderName.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(false);
    });
});
