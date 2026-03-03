import { describe, it, expect, beforeEach, vi } from 'vitest';
import { showToast } from './toast.js';

beforeEach(() => {
    document.body.innerHTML = '';
});

describe('showToast', () => {
    it('creates a toast container and toast element', () => {
        showToast('Something failed', 'error', 0);

        const container = document.querySelector('.toast-container');
        expect(container).not.toBeNull();
        expect(container.getAttribute('aria-live')).toBe('polite');

        const toast = container.querySelector('.toast');
        expect(toast).not.toBeNull();
        expect(toast.classList.contains('toast-error')).toBe(true);
        expect(toast.querySelector('.toast-message').textContent).toBe('Something failed');
    });

    it('supports success type', () => {
        showToast('Done!', 'success', 0);

        const toast = document.querySelector('.toast-success');
        expect(toast).not.toBeNull();
    });

    it('supports info type', () => {
        showToast('FYI', 'info', 0);

        const toast = document.querySelector('.toast-info');
        expect(toast).not.toBeNull();
    });

    it('escapes HTML in messages', () => {
        showToast('<script>alert(1)</script>', 'error', 0);

        const msg = document.querySelector('.toast-message');
        expect(msg.textContent).toBe('<script>alert(1)</script>');
        expect(msg.innerHTML).not.toContain('<script>');
    });

    it('reuses existing container', () => {
        showToast('First', 'error', 0);
        showToast('Second', 'error', 0);

        const containers = document.querySelectorAll('.toast-container');
        expect(containers).toHaveLength(1);

        const toasts = document.querySelectorAll('.toast');
        expect(toasts).toHaveLength(2);
    });

    it('dismiss button removes toast after fallback timeout', () => {
        vi.useFakeTimers();

        showToast('Dismissable', 'error', 0);
        const toast = document.querySelector('.toast');
        expect(toast).not.toBeNull();
        expect(toast.classList.contains('toast-visible')).toBe(true);

        const closeBtn = toast.querySelector('.toast-close');
        closeBtn.click();

        // After click, toast-visible class should be removed
        expect(toast.classList.contains('toast-visible')).toBe(false);

        // Toast is still in DOM until fallback timeout fires
        vi.advanceTimersByTime(300);
        expect(document.querySelector('.toast')).toBeNull();

        vi.useRealTimers();
    });

    it('auto-dismisses after duration', () => {
        vi.useFakeTimers();

        showToast('Temporary', 'error', 2000);
        expect(document.querySelector('.toast')).not.toBeNull();

        vi.advanceTimersByTime(2000);
        // After timeout, class is removed; element removed after transition
        vi.advanceTimersByTime(300);

        expect(document.querySelector('.toast')).toBeNull();

        vi.useRealTimers();
    });

    it('adds toast-visible class for animation', () => {
        const toast = showToast('Animate me', 'error', 0);
        expect(toast.classList.contains('toast-visible')).toBe(true);
    });

    it('uses default type error and auto-dismiss when called with message only', () => {
        vi.useFakeTimers();

        showToast('Defaults');
        const toast = document.querySelector('.toast');
        expect(toast.classList.contains('toast-error')).toBe(true);

        // Default duration is 4000ms
        vi.advanceTimersByTime(4000);
        vi.advanceTimersByTime(300); // fallback removal
        expect(document.querySelector('.toast')).toBeNull();

        vi.useRealTimers();
    });

    it('recreates container if previous one was removed from DOM', () => {
        showToast('First', 'info', 0);
        const firstContainer = document.querySelector('.toast-container');
        expect(firstContainer).not.toBeNull();

        // Remove the container from the DOM (simulates external DOM manipulation)
        firstContainer.remove();
        expect(document.querySelector('.toast-container')).toBeNull();

        showToast('Second', 'info', 0);
        const newContainer = document.querySelector('.toast-container');
        expect(newContainer).not.toBeNull();
        expect(newContainer.querySelector('.toast-message').textContent).toBe('Second');
    });

    it('dismiss is idempotent — double dismiss does not throw', () => {
        vi.useFakeTimers();

        const toast = showToast('Double dismiss', 'error', 0);
        const closeBtn = toast.querySelector('.toast-close');

        closeBtn.click();
        vi.advanceTimersByTime(300);
        expect(document.querySelector('.toast')).toBeNull();

        // Clicking again after removal should not throw
        expect(() => closeBtn.click()).not.toThrow();

        vi.useRealTimers();
    });

    it('sets role=status on each toast for accessibility', () => {
        const toast = showToast('Accessible', 'success', 0);
        expect(toast.getAttribute('role')).toBe('status');
    });

    it('returns the toast DOM element', () => {
        const toast = showToast('Return value', 'error', 0);
        expect(toast).toBeInstanceOf(HTMLElement);
        expect(toast.querySelector('.toast-message').textContent).toBe('Return value');
    });
});
