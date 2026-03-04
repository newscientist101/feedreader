// Dropdown toggle and click-outside close behavior with keyboard navigation.

function closeAllDropdowns() {
    document.querySelectorAll('.dropdown.open').forEach(d => {
        d.classList.remove('open');
        const toggle = d.querySelector('.dropdown-toggle');
        if (toggle) toggle.setAttribute('aria-expanded', 'false');
    });
}

export function toggleDropdown(btn) {
    const dropdown = btn.closest('.dropdown');
    const wasOpen = dropdown.classList.contains('open');

    // Close all dropdowns
    closeAllDropdowns();

    // Toggle this one
    if (!wasOpen) {
        dropdown.classList.add('open');
        btn.setAttribute('aria-expanded', 'true');
        // Focus the first menu item
        const firstItem = dropdown.querySelector('.dropdown-menu [role="menuitem"]');
        if (firstItem) firstItem.focus();
    } else {
        btn.setAttribute('aria-expanded', 'false');
    }
}

// Register the click-outside listener to close open dropdowns.
let _closeListenerAC = null;
export function initDropdownCloseListener() {
    if (_closeListenerAC) _closeListenerAC.abort();
    _closeListenerAC = new AbortController();
    const signal = _closeListenerAC.signal;

    document.addEventListener('click', (e) => {
        if (!e.target.closest('.dropdown')) {
            closeAllDropdowns();
        }
    }, { signal });
}

// Delegated listener for dropdown toggle buttons (replaces inline onclick).
let _dropdownListenerAC = null;
export function initDropdownListeners() {
    if (_dropdownListenerAC) _dropdownListenerAC.abort();
    _dropdownListenerAC = new AbortController();
    const signal = _dropdownListenerAC.signal;

    document.addEventListener('click', (e) => {
        const btn = e.target.closest('.dropdown-toggle');
        if (btn) {
            toggleDropdown(btn);
        }
    }, { signal });
}

// Keyboard navigation for dropdowns.
let _keyboardNavAC = null;
export function initDropdownKeyboardNav() {
    if (_keyboardNavAC) _keyboardNavAC.abort();
    _keyboardNavAC = new AbortController();
    const signal = _keyboardNavAC.signal;

    document.addEventListener('keydown', (e) => {
        // Escape closes any open dropdown and returns focus to the toggle
        if (e.key === 'Escape') {
            const openDropdown = document.querySelector('.dropdown.open');
            if (openDropdown) {
                e.preventDefault();
                const toggle = openDropdown.querySelector('.dropdown-toggle');
                closeAllDropdowns();
                if (toggle) toggle.focus();
            }
            return;
        }

        // Arrow keys navigate within an open dropdown menu
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            const openDropdown = document.querySelector('.dropdown.open');
            if (!openDropdown) return;

            e.preventDefault();
            const items = Array.from(openDropdown.querySelectorAll('.dropdown-menu [role="menuitem"]'));
            if (items.length === 0) return;

            const currentIndex = items.indexOf(document.activeElement);
            let nextIndex;
            if (e.key === 'ArrowDown') {
                nextIndex = currentIndex < items.length - 1 ? currentIndex + 1 : 0;
            } else {
                nextIndex = currentIndex > 0 ? currentIndex - 1 : items.length - 1;
            }
            items[nextIndex].focus();
        }

        // Enter/Space activates focused menu items
        if (e.key === 'Enter' || e.key === ' ') {
            const openDropdown = document.querySelector('.dropdown.open');
            if (!openDropdown) return;

            const activeItem = document.activeElement;
            if (activeItem && activeItem.matches('.dropdown-menu [role="menuitem"]')) {
                e.preventDefault();
                activeItem.click();
                closeAllDropdowns();
            }
        }
    }, { signal });
}
