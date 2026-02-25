import { describe, it, expect, beforeEach } from 'vitest';
import { updateReadButton } from './read-button.js';

beforeEach(() => {
    document.body.innerHTML = '';
});

describe('updateReadButton', () => {
    function makeCard(isRead) {
        const card = document.createElement('div');
        card.className = 'article-card';
        const btn = document.createElement('button');
        btn.className = 'btn-read-toggle';
        btn.dataset.isRead = isRead ? '1' : '0';
        btn.title = isRead ? 'Mark unread' : 'Mark read';
        btn.innerHTML = isRead ? 'unread-icon' : 'read-icon';
        card.appendChild(btn);
        document.body.appendChild(card);
        return card;
    }

    it('sets button to "mark unread" state when isRead is true', () => {
        const card = makeCard(false);
        updateReadButton(card, true);
        const btn = card.querySelector('.btn-read-toggle');
        expect(btn.dataset.isRead).toBe('1');
        expect(btn.title).toBe('Mark unread');
        expect(btn.innerHTML).toContain('<svg');
    });

    it('sets button to "mark read" state when isRead is false', () => {
        const card = makeCard(true);
        updateReadButton(card, false);
        const btn = card.querySelector('.btn-read-toggle');
        expect(btn.dataset.isRead).toBe('0');
        expect(btn.title).toBe('Mark read');
        expect(btn.innerHTML).toContain('<svg');
    });

    it('does nothing when card is null', () => {
        updateReadButton(null, true); // should not throw
    });

    it('does nothing when card has no btn-read-toggle', () => {
        const card = document.createElement('div');
        updateReadButton(card, true); // should not throw
    });

    it('uses different SVG icons for read vs unread', () => {
        const card = makeCard(false);
        updateReadButton(card, true);
        const readIcon = card.querySelector('.btn-read-toggle').innerHTML;

        updateReadButton(card, false);
        const unreadIcon = card.querySelector('.btn-read-toggle').innerHTML;

        expect(readIcon).not.toBe(unreadIcon);
    });
});
