import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { openModal, closeModal } from './modal.js';

describe('modal', () => {
    let modal;
    let closeFn;

    beforeEach(() => {
        document.body.innerHTML = `
            <button id="trigger">Open</button>
            <div id="test-modal" class="modal" style="display: flex">
                <div class="modal-backdrop"></div>
                <div class="modal-content">
                    <button id="close-btn">Close</button>
                    <input id="input1" type="text">
                    <input id="input2" type="text">
                    <button id="save-btn">Save</button>
                </div>
            </div>
        `;
        modal = document.getElementById('test-modal');
        closeFn = vi.fn();
    });

    afterEach(() => {
        closeModal();
    });

    it('sets role and aria-modal on open', () => {
        openModal(modal, closeFn);
        expect(modal.getAttribute('role')).toBe('dialog');
        expect(modal.getAttribute('aria-modal')).toBe('true');
    });

    it('calls closeFn on Escape', () => {
        openModal(modal, closeFn);
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        expect(closeFn).toHaveBeenCalledOnce();
    });

    it('does not call closeFn after closeModal', () => {
        openModal(modal, closeFn);
        closeModal();
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        expect(closeFn).not.toHaveBeenCalled();
    });

    it('focuses specified element on open', async () => {
        const input = document.getElementById('input1');
        openModal(modal, closeFn, input);
        // requestAnimationFrame is used internally
        await new Promise(r => requestAnimationFrame(r));
        expect(document.activeElement).toBe(input);
    });

    it('restores focus on close', () => {
        const trigger = document.getElementById('trigger');
        trigger.focus();
        openModal(modal, closeFn);
        closeModal();
        expect(document.activeElement).toBe(trigger);
    });

    it('traps Tab at the end of modal', () => {
        openModal(modal, closeFn);
        const saveBtn = document.getElementById('save-btn');
        saveBtn.focus();
        const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(true);
    });

    it('traps Shift+Tab at the start of modal', () => {
        openModal(modal, closeFn);
        const closeBtn = document.getElementById('close-btn');
        closeBtn.focus();
        const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(true);
    });

    it('does nothing when modal is null', () => {
        openModal(null, closeFn);
        // Should not throw, should not register keyHandler
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        expect(closeFn).not.toHaveBeenCalled();
    });

    it('focuses first focusable element when no focusEl is specified', async () => {
        openModal(modal, closeFn);
        await new Promise(r => requestAnimationFrame(r));
        const closeBtn = document.getElementById('close-btn');
        expect(document.activeElement).toBe(closeBtn);
    });

    it('closeModal is safe to call with no active modal', () => {
        // No openModal called first — should not throw
        closeModal();
    });

    it('Tab does not prevent default when not at last element', () => {
        openModal(modal, closeFn);
        const input1 = document.getElementById('input1');
        input1.focus();
        const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(false);
    });

    it('Shift+Tab does not prevent default when not at first element', () => {
        openModal(modal, closeFn);
        const input2 = document.getElementById('input2');
        input2.focus();
        const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(false);
    });

    it('trapFocus falls back to modal itself when no .modal-content', () => {
        document.body.innerHTML = `
            <div id="bare-modal" style="display: flex">
                <button id="only-btn">OK</button>
            </div>
        `;
        const bareModal = document.getElementById('bare-modal');
        openModal(bareModal, closeFn);
        const btn = document.getElementById('only-btn');
        btn.focus();
        // Tab from only focusable element should wrap
        const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(true);
    });

    it('trapFocus does nothing when no focusable elements', () => {
        document.body.innerHTML = `
            <div id="empty-modal" style="display: flex">
                <div class="modal-content">
                    <p>No focusable elements here</p>
                </div>
            </div>
        `;
        const emptyModal = document.getElementById('empty-modal');
        openModal(emptyModal, closeFn);
        const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(false);
    });

    it('Escape calls preventDefault', () => {
        openModal(modal, closeFn);
        const event = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(event.defaultPrevented).toBe(true);
    });

    it('other keys are ignored', () => {
        openModal(modal, closeFn);
        const event = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true });
        document.dispatchEvent(event);
        expect(closeFn).not.toHaveBeenCalled();
        expect(event.defaultPrevented).toBe(false);
    });
});
