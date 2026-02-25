// Dropdown toggle and click-outside close behavior.

export function toggleDropdown(btn) {
    const dropdown = btn.closest('.dropdown');
    const wasOpen = dropdown.classList.contains('open');

    // Close all dropdowns
    document.querySelectorAll('.dropdown.open').forEach(d => d.classList.remove('open'));

    // Toggle this one
    if (!wasOpen) {
        dropdown.classList.add('open');
    }
}

// Register the click-outside listener to close open dropdowns.
export function initDropdownCloseListener() {
    document.addEventListener('click', (e) => {
        if (!e.target.closest('.dropdown')) {
            document.querySelectorAll('.dropdown.open').forEach(d => d.classList.remove('open'));
        }
    });
}

// Delegated listener for dropdown toggle buttons (replaces inline onclick).
export function initDropdownListeners() {
    document.addEventListener('click', (e) => {
        const btn = e.target.closest('.dropdown-toggle');
        if (btn) {
            toggleDropdown(btn);
        }
    });
}
