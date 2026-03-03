import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
    toggleSidebar,
    setSidebarActive,
    navigateFolder,
    toggleFolderCollapse,
    collapseFolder,
    setSidebarLoadCategory,
    initSidebarListeners,
    initSidebarMobileClose,
} from './sidebar.js';

describe('sidebar', () => {
    beforeEach(() => {
        document.body.innerHTML = '';
        document.body.style.overflow = '';
        setSidebarLoadCategory(null);
    });

    describe('toggleSidebar', () => {
        beforeEach(() => {
            document.body.innerHTML = `
                <div class="sidebar"></div>
                <div class="sidebar-overlay"></div>
            `;
        });

        it('opens the sidebar', () => {
            toggleSidebar();
            expect(document.querySelector('.sidebar').classList.contains('open')).toBe(true);
            expect(document.querySelector('.sidebar-overlay').classList.contains('active')).toBe(true);
            expect(document.body.style.overflow).toBe('hidden');
        });

        it('closes an open sidebar', () => {
            toggleSidebar(); // open
            toggleSidebar(); // close
            expect(document.querySelector('.sidebar').classList.contains('open')).toBe(false);
            expect(document.querySelector('.sidebar-overlay').classList.contains('active')).toBe(false);
            expect(document.body.style.overflow).toBe('');
        });
    });

    describe('setSidebarActive', () => {
        beforeEach(() => {
            document.body.innerHTML = `
                <div class="sidebar">
                    <div class="nav-item active" id="nav1">All</div>
                    <div class="feed-item" id="feed1">Feed 1</div>
                    <div class="folder-item" id="folder1">Folder 1</div>
                </div>
            `;
        });

        it('clears existing active states and sets new one', () => {
            const feed1 = document.getElementById('feed1');
            setSidebarActive(feed1);
            expect(document.getElementById('nav1').classList.contains('active')).toBe(false);
            expect(feed1.classList.contains('active')).toBe(true);
        });

        it('handles null element (just clears active)', () => {
            setSidebarActive(null);
            expect(document.querySelectorAll('.sidebar .active')).toHaveLength(0);
        });
    });

    describe('navigateFolder', () => {
        it('returns true when not on articles page', () => {
            document.body.innerHTML = '<div>Other page</div>';
            const event = { preventDefault: vi.fn() };
            const result = navigateFolder(event, 1);
            expect(result).toBe(true);
            expect(event.preventDefault).not.toHaveBeenCalled();
        });

        it('calls loadCategoryArticles when set and on articles page', () => {
            const loadFn = vi.fn();
            setSidebarLoadCategory(loadFn);
            document.body.innerHTML = `
                <div class="articles-view"><div id="articles-list"></div></div>
                <div class="folder-item" data-category-id="42">
                    <span class="folder-name">Tech</span>
                </div>
            `;
            const event = { preventDefault: vi.fn() };
            navigateFolder(event, 42);
            expect(event.preventDefault).toHaveBeenCalled();
            expect(loadFn).toHaveBeenCalledWith(42, 'Tech');
        });

        it('returns false when folder not found on articles page', () => {
            document.body.innerHTML = '<div class="articles-view"><div id="articles-list"></div></div>';
            const event = { preventDefault: vi.fn() };
            const result = navigateFolder(event, 999);
            expect(result).toBe(false);
        });

        it('does not call loadCategoryArticles when it is null', () => {
            // _loadCategoryArticles is null (set in beforeEach)
            document.body.innerHTML = `
                <div class="articles-view"><div id="articles-list"></div></div>
                <div class="folder-item" data-category-id="42">
                    <span class="folder-name">Tech</span>
                </div>
            `;
            const event = { preventDefault: vi.fn() };
            // Should not throw even though _loadCategoryArticles is null
            navigateFolder(event, 42);
            expect(event.preventDefault).toHaveBeenCalled();
        });

        it('falls back to "Category" when folder-name span is absent', () => {
            const loadFn = vi.fn();
            setSidebarLoadCategory(loadFn);
            document.body.innerHTML = `
                <div class="articles-view"><div id="articles-list"></div></div>
                <div class="folder-item" data-category-id="42"></div>
            `;
            const event = { preventDefault: vi.fn() };
            navigateFolder(event, 42);
            expect(loadFn).toHaveBeenCalledWith(42, 'Category');
        });
    });

    describe('toggleFolderCollapse', () => {
        beforeEach(() => {
            document.body.innerHTML = `
                <div class="folder-item" data-category-id="1" id="f1">
                    <div class="folder-item" data-category-id="2" id="f2"></div>
                </div>
            `;
        });

        it('expands a collapsed folder', () => {
            toggleFolderCollapse(1);
            expect(document.getElementById('f1').classList.contains('expanded')).toBe(true);
        });

        it('collapses an expanded folder', () => {
            document.getElementById('f1').classList.add('expanded');
            toggleFolderCollapse(1);
            expect(document.getElementById('f1').classList.contains('expanded')).toBe(false);
        });

        it('does nothing for non-existent folder', () => {
            // Should not throw
            toggleFolderCollapse(999);
        });
    });

    describe('collapseFolder', () => {
        it('removes expanded class and clears nested active/expanded states', () => {
            document.body.innerHTML = `
                <div class="folder-item expanded" id="parent">
                    <div class="folder-item expanded active" id="child">
                        <div class="feed-item active" id="nested-feed">Feed</div>
                    </div>
                </div>
            `;
            const parent = document.getElementById('parent');
            collapseFolder(parent);

            expect(parent.classList.contains('expanded')).toBe(false);
            expect(document.getElementById('child').classList.contains('expanded')).toBe(false);
            expect(document.getElementById('child').classList.contains('active')).toBe(false);
            expect(document.getElementById('nested-feed').classList.contains('active')).toBe(false);
        });
    });

    describe('initSidebarListeners', () => {
        // Call once — event listeners on document persist across tests in this block.
        // Using a flag to ensure we only init once.
        let inited = false;
        beforeEach(() => {
            if (!inited) {
                initSidebarListeners();
                inited = true;
            }
        });

        it('toggle-sidebar: clicking menu toggle opens sidebar', () => {
            document.body.innerHTML = `
                <button data-action="toggle-sidebar">Menu</button>
                <div class="sidebar"></div>
                <div class="sidebar-overlay"></div>
            `;
            document.querySelector('[data-action="toggle-sidebar"]').click();
            expect(document.querySelector('.sidebar').classList.contains('open')).toBe(true);
        });

        it('toggle-sidebar: clicking overlay closes sidebar', () => {
            document.body.innerHTML = `
                <div class="sidebar open"></div>
                <div class="sidebar-overlay active" data-action="toggle-sidebar"></div>
            `;
            document.querySelector('.sidebar-overlay').click();
            expect(document.querySelector('.sidebar').classList.contains('open')).toBe(false);
        });

        it('toggle-folder: clicking chevron toggles folder collapse', () => {
            document.body.innerHTML = `
                <div class="folder-item" data-category-id="5">
                    <button data-action="toggle-folder" data-category-id="5">▶</button>
                </div>
            `;
            document.querySelector('[data-action="toggle-folder"]').click();
            expect(document.querySelector('.folder-item').classList.contains('expanded')).toBe(true);
        });

        it('toggle-folder: stopPropagation prevents parent click', () => {
            const parentClicked = vi.fn();
            document.body.innerHTML = `
                <div class="folder-item" data-category-id="5" id="parent">
                    <button data-action="toggle-folder" data-category-id="5">▶</button>
                </div>
            `;
            document.getElementById('parent').addEventListener('click', parentClicked);
            document.querySelector('[data-action="toggle-folder"]').click();
            // The delegated handler calls stopPropagation on the event
            expect(parentClicked).not.toHaveBeenCalled();
        });

        it('navigate-folder: on articles page, calls loadCategoryArticles', () => {
            const loadFn = vi.fn();
            setSidebarLoadCategory(loadFn);
            document.body.innerHTML = `
                <div class="articles-view"><div id="articles-list"></div></div>
                <div class="folder-item" data-category-id="7">
                    <a href="/category/7" data-action="navigate-folder" data-category-id="7">
                        <span class="folder-name">News</span>
                    </a>
                </div>
            `;
            const link = document.querySelector('[data-action="navigate-folder"]');
            link.click();
            expect(loadFn).toHaveBeenCalledWith(7, 'News');
        });

        it('navigate-folder: on non-articles page, allows normal navigation', () => {
            document.body.innerHTML = `
                <div class="folder-item" data-category-id="7">
                    <a href="/category/7" data-action="navigate-folder" data-category-id="7">
                        <span class="folder-name">News</span>
                    </a>
                </div>
            `;
            const link = document.querySelector('[data-action="navigate-folder"]');
            const event = new MouseEvent('click', { bubbles: true, cancelable: true });
            link.dispatchEvent(event);
            // navigateFolder returns true (no articles-view), so preventDefault is NOT called
            expect(event.defaultPrevented).toBe(false);
        });

        it('toggle-folder: ignores button with zero/invalid categoryId', () => {
            document.body.innerHTML = `
                <div class="folder-item" data-category-id="0">
                    <button data-action="toggle-folder" data-category-id="0">▶</button>
                </div>
            `;
            // catId 0 is falsy, so toggleFolderCollapse should not be called
            document.querySelector('[data-action="toggle-folder"]').click();
            // Folder should remain collapsed (not expanded)
            expect(document.querySelector('.folder-item').classList.contains('expanded')).toBe(false);
        });

        it('toggle-folder: ignores click with missing data-category-id', () => {
            document.body.innerHTML = `
                <div class="folder-item" data-category-id="5">
                    <button data-action="toggle-folder">▶</button>
                </div>
            `;
            // No data-category-id on the button, Number(undefined) = NaN which is falsy
            document.querySelector('[data-action="toggle-folder"]').click();
            expect(document.querySelector('.folder-item').classList.contains('expanded')).toBe(false);
        });

        it('navigate-folder: preventDefault called on articles page SPA navigation', () => {
            const loadFn = vi.fn();
            setSidebarLoadCategory(loadFn);
            document.body.innerHTML = `
                <div class="articles-view"><div id="articles-list"></div></div>
                <div class="folder-item" data-category-id="3">
                    <a href="/category/3" data-action="navigate-folder" data-category-id="3">
                        <span class="folder-name">Science</span>
                    </a>
                </div>
            `;
            const link = document.querySelector('[data-action="navigate-folder"]');
            const event = new MouseEvent('click', { bubbles: true, cancelable: true });
            link.dispatchEvent(event);
            expect(event.defaultPrevented).toBe(true);
            expect(loadFn).toHaveBeenCalledWith(3, 'Science');
        });
    });

    describe('initSidebarMobileClose', () => {
        it('does nothing when sidebar is absent', () => {
            document.body.innerHTML = '<div>No sidebar</div>';
            initSidebarMobileClose(); // should not throw
        });

        it('toggles sidebar on link click when width <= 768', () => {
            document.body.innerHTML = `
                <div class="sidebar open">
                    <a href="/feed/1">Feed 1</a>
                    <a href="/feed/2">Feed 2</a>
                </div>
                <div class="sidebar-overlay active"></div>
            `;
            Object.defineProperty(window, 'innerWidth', { value: 600, configurable: true });
            initSidebarMobileClose();

            document.querySelector('a[href="/feed/1"]').click();
            expect(document.querySelector('.sidebar').classList.contains('open')).toBe(false);
        });

        it('does not toggle sidebar on link click when width > 768', () => {
            document.body.innerHTML = `
                <div class="sidebar open">
                    <a href="/feed/1">Feed 1</a>
                </div>
                <div class="sidebar-overlay active"></div>
            `;
            Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
            initSidebarMobileClose();

            document.querySelector('a[href="/feed/1"]').click();
            // Sidebar should remain open on wide screens
            expect(document.querySelector('.sidebar').classList.contains('open')).toBe(true);
        });
    });
});
