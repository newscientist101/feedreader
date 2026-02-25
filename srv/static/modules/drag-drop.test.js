import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    initFolderDragDrop, initDragDrop, syncFolderOrder,
    reorderElements, getDragAfterElementAmongSiblings,
} from './drag-drop.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('getDragAfterElementAmongSiblings', () => {
    it('returns element below the cursor', () => {
        const el1 = document.createElement('div');
        const el2 = document.createElement('div');
        // Mock getBoundingClientRect
        el1.getBoundingClientRect = () => ({ top: 0, height: 50 });
        el2.getBoundingClientRect = () => ({ top: 50, height: 50 });

        // y=10 is above the midpoint of el1 (25), so el1 should be the after element
        const result = getDragAfterElementAmongSiblings([el1, el2], 10);
        expect(result).toBe(el1);
    });

    it('returns undefined when cursor is below all elements', () => {
        const el1 = document.createElement('div');
        el1.getBoundingClientRect = () => ({ top: 0, height: 50 });

        // y=100 is well below el1's midpoint
        const result = getDragAfterElementAmongSiblings([el1], 100);
        expect(result).toBeUndefined();
    });

    it('skips elements with dragging class', () => {
        const el1 = document.createElement('div');
        el1.classList.add('dragging');
        el1.getBoundingClientRect = () => ({ top: 0, height: 50 });
        const el2 = document.createElement('div');
        el2.getBoundingClientRect = () => ({ top: 50, height: 50 });

        // Even though y=10 is above el1 midpoint, el1 is dragging so skip it
        // y=10 is above el2 midpoint (75), so el2
        const result = getDragAfterElementAmongSiblings([el1, el2], 10);
        expect(result).toBe(el2);
    });

    it('returns closest element above cursor', () => {
        const el1 = document.createElement('div');
        const el2 = document.createElement('div');
        const el3 = document.createElement('div');
        el1.getBoundingClientRect = () => ({ top: 0, height: 40 });
        el2.getBoundingClientRect = () => ({ top: 40, height: 40 });
        el3.getBoundingClientRect = () => ({ top: 80, height: 40 });

        // y=50 is above el2 midpoint (60) and el3 midpoint (100), but closer to el2
        const result = getDragAfterElementAmongSiblings([el1, el2, el3], 50);
        expect(result).toBe(el2);
    });

    it('handles empty array', () => {
        const result = getDragAfterElementAmongSiblings([], 50);
        expect(result).toBeUndefined();
    });
});

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
