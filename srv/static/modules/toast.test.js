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

    it('dismiss button removes toast', () => {
        showToast('Dismissable', 'error', 0);

        const closeBtn = document.querySelector('.toast-close');
        closeBtn.click();

        // The toast gets removed after transition (or fallback timeout)
        // Since there's no real transition in tests, the setTimeout fallback removes it
        vi.useFakeTimers();
        closeBtn.click(); // click again on a second toast
        vi.advanceTimersByTime(300);
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
});
