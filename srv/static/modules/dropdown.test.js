import { describe, it, expect, beforeEach, vi } from 'vitest';
import { toggleDropdown, initDropdownCloseListener, initDropdownListeners, initDropdownKeyboardNav } from './dropdown.js';

describe('dropdown', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div class="dropdown" id="dd1">
                <button class="dropdown-btn">Menu 1</button>
                <div class="dropdown-content">Content 1</div>
            </div>
            <div class="dropdown" id="dd2">
                <button class="dropdown-btn">Menu 2</button>
                <div class="dropdown-content">Content 2</div>
            </div>
            <div id="outside">Outside</div>
        `;
    });

    describe('toggleDropdown', () => {
        it('opens a closed dropdown', () => {
            const btn = document.querySelector('#dd1 .dropdown-btn');
            toggleDropdown(btn);
            expect(document.getElementById('dd1').classList.contains('open')).toBe(true);
        });

        it('closes an open dropdown', () => {
            const dd1 = document.getElementById('dd1');
            dd1.classList.add('open');
            const btn = dd1.querySelector('.dropdown-btn');
            toggleDropdown(btn);
            expect(dd1.classList.contains('open')).toBe(false);
        });

        it('closes other open dropdowns when opening one', () => {
            const dd1 = document.getElementById('dd1');
            const dd2 = document.getElementById('dd2');
            dd1.classList.add('open');

            const btn2 = dd2.querySelector('.dropdown-btn');
            toggleDropdown(btn2);

            expect(dd1.classList.contains('open')).toBe(false);
            expect(dd2.classList.contains('open')).toBe(true);
        });

        it('closes all dropdowns when toggling the only open one', () => {
            const dd1 = document.getElementById('dd1');
            dd1.classList.add('open');
            toggleDropdown(dd1.querySelector('.dropdown-btn'));

            expect(document.querySelectorAll('.dropdown.open')).toHaveLength(0);
        });
    });

    describe('initDropdownCloseListener', () => {
        it('closes open dropdowns when clicking outside', () => {
            initDropdownCloseListener();
            const dd1 = document.getElementById('dd1');
            dd1.classList.add('open');

            document.getElementById('outside').click();

            expect(dd1.classList.contains('open')).toBe(false);
        });

        it('does not close dropdowns when clicking inside one', () => {
            initDropdownCloseListener();
            const dd1 = document.getElementById('dd1');
            dd1.classList.add('open');

            dd1.querySelector('.dropdown-btn').click();

            // The click-outside listener shouldn't close it (clicked inside .dropdown)
            // Note: toggleDropdown is not wired here, so the listener alone keeps it open
            expect(dd1.classList.contains('open')).toBe(true);
        });
    });
});

describe('initDropdownListeners', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div class="dropdown" id="dd1">
                <button class="dropdown-toggle">Menu 1</button>
                <div class="dropdown-menu">Content 1</div>
            </div>
            <div class="dropdown" id="dd2">
                <button class="dropdown-toggle">Menu 2</button>
                <div class="dropdown-menu">Content 2</div>
            </div>
        `;
    });

    it('opens a dropdown when its toggle button is clicked', () => {
        initDropdownListeners();
        document.querySelector('#dd1 .dropdown-toggle').click();
        expect(document.getElementById('dd1').classList.contains('open')).toBe(true);
    });

    it('closes an open dropdown on second click', () => {
        initDropdownListeners();
        const toggle = document.querySelector('#dd1 .dropdown-toggle');
        toggle.click();
        toggle.click();
        expect(document.getElementById('dd1').classList.contains('open')).toBe(false);
    });

    it('closes other dropdowns when opening a new one', () => {
        initDropdownListeners();
        document.querySelector('#dd1 .dropdown-toggle').click();
        document.querySelector('#dd2 .dropdown-toggle').click();
        expect(document.getElementById('dd1').classList.contains('open')).toBe(false);
        expect(document.getElementById('dd2').classList.contains('open')).toBe(true);
    });
});

describe('initDropdownKeyboardNav', () => {
    // Only call initDropdownKeyboardNav once to avoid stacking listeners
    let initialized = false;
    beforeEach(() => {
        document.body.innerHTML = `
            <div class="dropdown" id="dd1">
                <button class="dropdown-toggle" aria-haspopup="true" aria-expanded="false">Menu 1</button>
                <div class="dropdown-menu" role="menu">
                    <button role="menuitem" id="item1">Item 1</button>
                    <button role="menuitem" id="item2">Item 2</button>
                    <button role="menuitem" id="item3">Item 3</button>
                </div>
            </div>
        `;
        if (!initialized) {
            initDropdownKeyboardNav();
            initialized = true;
        }
    });

    it('closes dropdown on Escape and focuses toggle', () => {
        const dd1 = document.getElementById('dd1');
        const toggle = dd1.querySelector('.dropdown-toggle');
        dd1.classList.add('open');
        toggle.setAttribute('aria-expanded', 'true');

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));

        expect(dd1.classList.contains('open')).toBe(false);
        expect(document.activeElement).toBe(toggle);
        expect(toggle.getAttribute('aria-expanded')).toBe('false');
    });

    it('navigates down with ArrowDown', () => {
        const dd1 = document.getElementById('dd1');
        dd1.classList.add('open');
        document.getElementById('item1').focus();

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));

        expect(document.activeElement.id).toBe('item2');
    });

    it('navigates up with ArrowUp', () => {
        const dd1 = document.getElementById('dd1');
        dd1.classList.add('open');
        document.getElementById('item2').focus();

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }));

        expect(document.activeElement.id).toBe('item1');
    });

    it('wraps from last to first on ArrowDown', () => {
        const dd1 = document.getElementById('dd1');
        dd1.classList.add('open');
        document.getElementById('item3').focus();

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));

        expect(document.activeElement.id).toBe('item1');
    });

    it('wraps from first to last on ArrowUp', () => {
        const dd1 = document.getElementById('dd1');
        dd1.classList.add('open');
        document.getElementById('item1').focus();

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true }));

        expect(document.activeElement.id).toBe('item3');
    });

    it('does nothing when no dropdown is open', () => {
        const item1 = document.getElementById('item1');
        item1.focus();

        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));

        // Focus should not change since no dropdown is open
        expect(document.activeElement).toBe(item1);
    });
});

describe('toggleDropdown aria-expanded', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div class="dropdown" id="dd1">
                <button class="dropdown-toggle" aria-haspopup="true" aria-expanded="false">Menu</button>
                <div class="dropdown-menu" role="menu">
                    <button role="menuitem">Item</button>
                </div>
            </div>
        `;
    });

    it('sets aria-expanded to true when opening', () => {
        const toggle = document.querySelector('#dd1 .dropdown-toggle');
        toggleDropdown(toggle);
        expect(toggle.getAttribute('aria-expanded')).toBe('true');
    });

    it('sets aria-expanded to false when closing', () => {
        const toggle = document.querySelector('#dd1 .dropdown-toggle');
        toggleDropdown(toggle);
        toggleDropdown(toggle);
        expect(toggle.getAttribute('aria-expanded')).toBe('false');
    });
});
