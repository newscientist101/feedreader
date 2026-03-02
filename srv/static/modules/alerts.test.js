import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    initAlertsPage, initAlertDetailPage,
    dismissAllMatches, dismissArticleAlert, updateAlertsBadge,
    submitCreateAlert, openCreateAlertModal, closeCreateAlertModal,
    saveAlert, deleteAlert,
    dismissArticleAlertDetail, undismissArticleAlertDetail, toggleDismissState,
} from './alerts.js';

vi.mock('./api.js', () => ({ api: vi.fn() }));
vi.mock('./toast.js', () => ({ showToast: vi.fn() }));
vi.mock('./modal.js', () => ({ openModal: vi.fn(), closeModal: vi.fn() }));

import { api } from './api.js';
import { showToast } from './toast.js';

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('initAlertsPage', () => {
    it('does nothing when .alerts-view is absent', () => {
        document.body.innerHTML = '<div>Not alerts page</div>';
        expect(() => initAlertsPage()).not.toThrow();
    });

    it('wires create-alert button click', () => {
        document.body.innerHTML = '<div class="alerts-view"><button data-action="create-alert">Create</button></div>';
        initAlertsPage();
        const btn = document.querySelector('[data-action="create-alert"]');
        btn.click();
        // Modal should be created
        expect(document.getElementById('create-alert-modal')).not.toBeNull();
    });

    it('wires dismiss-all-alert delegated click', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="alerts-view">
                <div class="alert-group" data-alert-id="5">
                    <button data-action="dismiss-all-alert" data-alert-id="5">Dismiss All</button>
                    <div class="article-alert-item" data-article-alert-id="10"></div>
                </div>
            </div>`;
        initAlertsPage();
        document.querySelector('[data-action="dismiss-all-alert"]').click();
        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('POST', '/api/alerts/5/dismiss');
        });
    });

    it('wires dismiss-article-alert delegated click', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="alerts-view">
                <div class="alert-group" data-alert-id="5">
                    <div class="article-alert-item" data-article-alert-id="42">
                        <button data-action="dismiss-article-alert" data-article-alert-id="42">X</button>
                    </div>
                </div>
            </div>`;
        initAlertsPage();
        document.querySelector('[data-action="dismiss-article-alert"]').click();
        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('POST', '/api/article-alerts/42/dismiss');
        });
    });
});

describe('dismissAllMatches', () => {
    it('calls API and removes group from DOM', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="alert-group" data-alert-id="7">
                <div class="article-alert-item"></div>
            </div>
            <span data-count="alerts">3</span>`;
        await dismissAllMatches('7');
        expect(api).toHaveBeenCalledWith('POST', '/api/alerts/7/dismiss');
        expect(document.querySelector('.alert-group[data-alert-id="7"]')).toBeNull();
    });

    it('shows toast on error', async () => {
        api.mockRejectedValue(new Error('fail'));
        await dismissAllMatches('7');
        expect(showToast).toHaveBeenCalledWith('Failed to dismiss matches');
    });
});

describe('dismissArticleAlert', () => {
    it('calls API and removes item from DOM', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="alert-group" data-alert-id="1">
                <div class="article-alert-item" data-article-alert-id="99"></div>
                <div class="article-alert-item" data-article-alert-id="100"></div>
            </div>
            <span data-count="alerts">2</span>`;
        await dismissArticleAlert('99');
        expect(api).toHaveBeenCalledWith('POST', '/api/article-alerts/99/dismiss');
        expect(document.querySelector('[data-article-alert-id="99"]')).toBeNull();
        // Group still has one item
        expect(document.querySelector('.alert-group')).not.toBeNull();
    });

    it('removes group when last item dismissed', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="alert-group" data-alert-id="1">
                <div class="article-alert-item" data-article-alert-id="99"></div>
            </div>
            <span data-count="alerts">1</span>`;
        await dismissArticleAlert('99');
        expect(document.querySelector('.alert-group')).toBeNull();
    });
});

describe('updateAlertsBadge', () => {
    it('sets badge to count of remaining items', () => {
        document.body.innerHTML = `
            <div class="article-alert-item"></div>
            <div class="article-alert-item"></div>
            <span data-count="alerts">5</span>`;
        updateAlertsBadge();
        expect(document.querySelector('[data-count="alerts"]').textContent).toBe('2');
    });

    it('clears badge when no items', () => {
        document.body.innerHTML = '<span data-count="alerts">5</span>';
        updateAlertsBadge();
        expect(document.querySelector('[data-count="alerts"]').textContent).toBe('');
    });
});

describe('submitCreateAlert', () => {
    it('calls API with form values and reloads', async () => {
        api.mockResolvedValue({});
        const origLocation = window.location;
        delete window.location;
        window.location = { reload: vi.fn(), href: '' };

        document.body.innerHTML = `
            <div class="alerts-view">
                <input id="alert-name" value="My Alert">
                <input id="alert-pattern" value="breaking">
                <input id="alert-is-regex" type="checkbox">
                <select id="alert-match-field"><option value="title" selected>Title</option></select>
            </div>`;

        await submitCreateAlert();
        expect(api).toHaveBeenCalledWith('POST', '/api/alerts', {
            name: 'My Alert',
            pattern: 'breaking',
            is_regex: false,
            match_field: 'title',
        });

        window.location = origLocation;
    });

    it('does nothing with empty name', async () => {
        document.body.innerHTML = `
            <input id="alert-name" value="">
            <input id="alert-pattern" value="test">
            <input id="alert-is-regex" type="checkbox">
            <select id="alert-match-field"><option value="title" selected>Title</option></select>`;
        await submitCreateAlert();
        expect(api).not.toHaveBeenCalled();
    });

    it('shows toast and does not call API for invalid regex', async () => {
        document.body.innerHTML = `
            <input id="alert-name" value="Bad Regex">
            <input id="alert-pattern" value="[invalid">
            <input id="alert-is-regex" type="checkbox" checked>
            <select id="alert-match-field"><option value="title" selected>Title</option></select>`;
        await submitCreateAlert();
        expect(api).not.toHaveBeenCalled();
        expect(showToast).toHaveBeenCalledWith('Invalid regular expression');
    });
});

describe('initAlertDetailPage', () => {
    it('does nothing when .alert-detail-view is absent', () => {
        document.body.innerHTML = '<div>Not the detail page</div>';
        expect(() => initAlertDetailPage()).not.toThrow();
    });

    it('does nothing when data-alert-id is missing', () => {
        document.body.innerHTML = '<div class="alert-detail-view"></div>';
        expect(() => initAlertDetailPage()).not.toThrow();
    });

    it('wires edit form submission', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="alert-detail-view" data-alert-id="3">
                <form id="edit-alert-form">
                    <input id="edit-alert-name" value="Updated">
                    <input id="edit-alert-pattern" value="new pattern">
                    <input id="edit-alert-is-regex" type="checkbox" checked>
                    <select id="edit-alert-match-field"><option value="content" selected>Content</option></select>
                    <button type="submit">Save</button>
                </form>
            </div>`;
        initAlertDetailPage();
        const form = document.getElementById('edit-alert-form');
        form.dispatchEvent(new Event('submit', { cancelable: true }));
        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('PUT', '/api/alerts/3', {
                name: 'Updated',
                pattern: 'new pattern',
                is_regex: true,
                match_field: 'content',
            });
        });
    });

    it('wires delete button', async () => {
        api.mockResolvedValue({});
        vi.stubGlobal('confirm', vi.fn(() => true));
        const origLocation = window.location;
        delete window.location;
        window.location = { href: '' };

        document.body.innerHTML = `
            <div class="alert-detail-view" data-alert-id="3">
                <button data-action="delete-alert">Delete</button>
            </div>`;
        initAlertDetailPage();
        document.querySelector('[data-action="delete-alert"]').click();
        await vi.waitFor(() => {
            expect(api).toHaveBeenCalledWith('DELETE', '/api/alerts/3');
        });
        expect(window.location.href).toBe('/alerts');

        window.location = origLocation;
    });
});

describe('saveAlert', () => {
    it('calls PUT and shows success toast', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <input id="edit-alert-name" value="Test">
            <input id="edit-alert-pattern" value="pat">
            <input id="edit-alert-is-regex" type="checkbox">
            <select id="edit-alert-match-field"><option value="title" selected>Title</option></select>`;
        await saveAlert('5');
        expect(api).toHaveBeenCalledWith('PUT', '/api/alerts/5', {
            name: 'Test',
            pattern: 'pat',
            is_regex: false,
            match_field: 'title',
        });
        expect(showToast).toHaveBeenCalledWith('Alert updated', 'success');
    });

    it('shows error toast on failure', async () => {
        api.mockRejectedValue(new Error('fail'));
        document.body.innerHTML = `
            <input id="edit-alert-name" value="Test">
            <input id="edit-alert-pattern" value="pat">
            <input id="edit-alert-is-regex" type="checkbox">
            <select id="edit-alert-match-field"><option value="title" selected>Title</option></select>`;
        await saveAlert('5');
        expect(showToast).toHaveBeenCalledWith('Failed to update alert');
    });

    it('shows toast and does not call API for invalid regex', async () => {
        document.body.innerHTML = `
            <input id="edit-alert-name" value="Test">
            <input id="edit-alert-pattern" value="(unclosed">
            <input id="edit-alert-is-regex" type="checkbox" checked>
            <select id="edit-alert-match-field"><option value="title" selected>Title</option></select>`;
        await saveAlert('5');
        expect(api).not.toHaveBeenCalled();
        expect(showToast).toHaveBeenCalledWith('Invalid regular expression');
    });
});

describe('deleteAlert', () => {
    it('does nothing if user cancels confirm', async () => {
        vi.stubGlobal('confirm', vi.fn(() => false));
        await deleteAlert('5');
        expect(api).not.toHaveBeenCalled();
    });

    it('calls DELETE and navigates to /alerts', async () => {
        api.mockResolvedValue({});
        vi.stubGlobal('confirm', vi.fn(() => true));
        const origLocation = window.location;
        delete window.location;
        window.location = { href: '' };

        await deleteAlert('5');
        expect(api).toHaveBeenCalledWith('DELETE', '/api/alerts/5');
        expect(window.location.href).toBe('/alerts');

        window.location = origLocation;
    });
});

describe('toggleDismissState', () => {
    it('adds dismissed class and swaps button action', () => {
        document.body.innerHTML = `
            <div class="article-alert-item" data-article-alert-id="10">
                <button data-action="dismiss-article-alert">Dismiss</button>
            </div>`;
        toggleDismissState('10', true);
        const item = document.querySelector('[data-article-alert-id="10"]');
        expect(item.classList.contains('dismissed')).toBe(true);
        const btn = item.querySelector('button');
        expect(btn.dataset.action).toBe('undismiss-article-alert');
        expect(btn.textContent).toBe('Undismiss');
    });

    it('removes dismissed class and swaps button action', () => {
        document.body.innerHTML = `
            <div class="article-alert-item dismissed" data-article-alert-id="10">
                <button data-action="undismiss-article-alert">Undismiss</button>
            </div>`;
        toggleDismissState('10', false);
        const item = document.querySelector('[data-article-alert-id="10"]');
        expect(item.classList.contains('dismissed')).toBe(false);
        const btn = item.querySelector('button');
        expect(btn.dataset.action).toBe('dismiss-article-alert');
        expect(btn.textContent).toBe('Dismiss');
    });

    it('does nothing if item not found', () => {
        document.body.innerHTML = '<div></div>';
        expect(() => toggleDismissState('999', true)).not.toThrow();
    });
});

describe('dismissArticleAlertDetail', () => {
    it('calls dismiss API and toggles state', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="article-alert-item" data-article-alert-id="20">
                <button data-action="dismiss-article-alert">Dismiss</button>
            </div>`;
        await dismissArticleAlertDetail('20');
        expect(api).toHaveBeenCalledWith('POST', '/api/article-alerts/20/dismiss');
        expect(document.querySelector('[data-article-alert-id="20"]').classList.contains('dismissed')).toBe(true);
    });
});

describe('undismissArticleAlertDetail', () => {
    it('calls undismiss API and toggles state', async () => {
        api.mockResolvedValue({});
        document.body.innerHTML = `
            <div class="article-alert-item dismissed" data-article-alert-id="20">
                <button data-action="undismiss-article-alert">Undismiss</button>
            </div>`;
        await undismissArticleAlertDetail('20');
        expect(api).toHaveBeenCalledWith('POST', '/api/article-alerts/20/undismiss');
        expect(document.querySelector('[data-article-alert-id="20"]').classList.contains('dismissed')).toBe(false);
    });
});

describe('openCreateAlertModal / closeCreateAlertModal', () => {
    it('creates and shows modal', () => {
        document.body.innerHTML = '<div class="alerts-view"></div>';
        openCreateAlertModal();
        const modal = document.getElementById('create-alert-modal');
        expect(modal).not.toBeNull();
        expect(modal.style.display).toBe('flex');
    });

    it('hides modal on close', () => {
        document.body.innerHTML = '<div class="alerts-view"></div>';
        openCreateAlertModal();
        closeCreateAlertModal();
        const modal = document.getElementById('create-alert-modal');
        expect(modal.style.display).toBe('none');
    });

    it('reuses existing modal', () => {
        document.body.innerHTML = '<div class="alerts-view"></div>';
        openCreateAlertModal();
        openCreateAlertModal();
        const modals = document.querySelectorAll('#create-alert-modal');
        expect(modals.length).toBe(1);
    });
});
