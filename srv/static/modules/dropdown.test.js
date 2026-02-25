import { describe, it, expect, beforeEach, vi } from 'vitest';
import { toggleDropdown, initDropdownCloseListener, initDropdownListeners } from './dropdown.js';

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
