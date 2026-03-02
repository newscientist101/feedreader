// Modal keyboard support: Escape to close, Tab trapping.

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

let keyHandler = null;
let previousFocus = null;

/**
 * Open a modal with keyboard support.
 * @param {HTMLElement} modal - The modal element.
 * @param {Function} closeFn - Function to call when Escape is pressed.
 * @param {HTMLElement} [focusEl] - Element to focus inside the modal (defaults to first focusable).
 */
export function openModal(modal, closeFn, focusEl) {
    if (!modal) return;
    previousFocus = document.activeElement;
    modal.setAttribute('role', 'dialog');
    modal.setAttribute('aria-modal', 'true');

    keyHandler = (e) => {
        if (e.key === 'Escape') {
            e.preventDefault();
            closeFn();
            return;
        }
        if (e.key === 'Tab') {
            trapFocus(modal, e);
        }
    };
    document.addEventListener('keydown', keyHandler);

    // Focus the target element or first focusable
    requestAnimationFrame(() => {
        if (focusEl) {
            focusEl.focus();
        } else {
            const first = modal.querySelector('.modal-content ' + FOCUSABLE);
            if (first) first.focus();
        }
    });
}

/**
 * Close the active modal and clean up keyboard handlers.
 */
export function closeModal() {
    if (keyHandler) {
        document.removeEventListener('keydown', keyHandler);
        keyHandler = null;
    }
    if (previousFocus && typeof previousFocus.focus === 'function') {
        previousFocus.focus();
    }
    previousFocus = null;
}

function trapFocus(modal, e) {
    const content = modal.querySelector('.modal-content') || modal;
    const focusable = Array.from(content.querySelectorAll(FOCUSABLE));
    if (focusable.length === 0) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    if (e.shiftKey) {
        if (document.activeElement === first) {
            e.preventDefault();
            last.focus();
        }
    } else {
        if (document.activeElement === last) {
            e.preventDefault();
            first.focus();
        }
    }
}
