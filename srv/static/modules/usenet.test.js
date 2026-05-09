import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
    initUsenetCredentialsSection,
    initUsenetSettingsListeners,
    initUsenetGroupsSection,
    initUsenetFeedsListeners,
} from './usenet.js';

// Mock the api module
vi.mock('./api.js', () => ({
    api: vi.fn(),
}));

// Mock toast
vi.mock('./toast.js', () => ({
    showToast: vi.fn(),
}));

import { api } from './api.js';
import { showToast } from './toast.js';

// ---------- helpers ----------

function setupSettingsDOM() {
    document.body.innerHTML = `
        <section id="usenet-section">
            <div id="usenet-credentials-status"></div>
            <div id="usenet-credentials-form" style="display:none">
                <form id="usenet-cred-form">
                    <input id="usenet-username">
                    <input id="usenet-password" type="password">
                    <button type="submit">Save</button>
                    <button type="button" id="usenet-delete-btn"
                            data-action="usenet-delete-credentials"
                            style="display:none">Remove</button>
                    <span id="usenet-cred-status"></span>
                </form>
            </div>
        </section>
    `;
}

function setupFeedsDOM() {
    document.body.innerHTML = `
        <div id="usenet-groups-section">
            <div id="usenet-no-credentials" style="display:none"></div>
            <div id="usenet-groups-content" style="display:none">
                <form id="usenet-add-group-form">
                    <input id="usenet-group-name">
                    <select id="usenet-group-category">
                        <option value="0">No folder</option>
                        <option value="42">Tech</option>
                    </select>
                    <button type="submit">Subscribe</button>
                    <span id="usenet-add-status"></span>
                </form>
                <div id="usenet-groups-list"></div>
            </div>
        </div>
    `;
}

// ---------- Settings page: credential loading ----------

describe('initUsenetCredentialsSection', () => {
    beforeEach(() => {
        setupSettingsDOM();
        vi.clearAllMocks();
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    it('is a no-op when the usenet section is absent', async () => {
        document.body.innerHTML = '';
        await initUsenetCredentialsSection();
        expect(api).not.toHaveBeenCalled();
    });

    it('renders configured status with username and key_version', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: true, username: 'alice', key_version: 'v1' });
        await initUsenetCredentialsSection();

        const statusEl = document.getElementById('usenet-credentials-status');
        expect(statusEl.textContent).toContain('Configured');
        expect(statusEl.textContent).toContain('alice');
        expect(statusEl.textContent).toContain('v1');
    });

    it('renders not-configured status', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: false, username: '', key_version: '' });
        await initUsenetCredentialsSection();

        const statusEl = document.getElementById('usenet-credentials-status');
        expect(statusEl.textContent).toContain('Not configured');
    });

    it('shows the form after loading', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: false, username: '', key_version: '' });
        await initUsenetCredentialsSection();

        const formEl = document.getElementById('usenet-credentials-form');
        expect(formEl.style.display).not.toBe('none');
    });

    it('hides delete button when not configured', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: false, username: '', key_version: '' });
        await initUsenetCredentialsSection();

        const deleteBtn = document.getElementById('usenet-delete-btn');
        expect(deleteBtn.style.display).toBe('none');
    });

    it('shows delete button when configured', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: true, username: 'alice', key_version: 'v1' });
        await initUsenetCredentialsSection();

        const deleteBtn = document.getElementById('usenet-delete-btn');
        expect(deleteBtn.style.display).not.toBe('none');
    });

    it('shows error when API fails', async () => {
        api.mockRejectedValueOnce(new Error('network error'));
        await initUsenetCredentialsSection();

        const statusEl = document.getElementById('usenet-credentials-status');
        expect(statusEl.textContent).toContain('network error');
    });

    it('pre-fills username when configured', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: true, username: 'bob', key_version: 'v1' });
        await initUsenetCredentialsSection();

        const usernameInput = document.getElementById('usenet-username');
        expect(usernameInput.value).toBe('bob');
    });
});

// ---------- Settings page: save credentials ----------

describe('initUsenetSettingsListeners - save credentials', () => {
    beforeEach(() => {
        setupSettingsDOM();
        vi.clearAllMocks();
        initUsenetSettingsListeners();
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    it('saves credentials on form submit', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: true, username: 'alice', key_version: 'v1' });

        document.getElementById('usenet-username').value = 'alice';
        document.getElementById('usenet-password').value = 'secret';
        document.getElementById('usenet-credentials-status').innerHTML = 'x'; // pre-populate
        document.getElementById('usenet-credentials-form').style.display = '';
        document.getElementById('usenet-delete-btn').style.display = 'none';

        document.getElementById('usenet-cred-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await vi.waitFor(() => expect(api).toHaveBeenCalledWith('PUT', '/api/usenet/credentials', {
            username: 'alice',
            password: 'secret',
        }), { interval: 1 });

        await vi.waitFor(() => expect(showToast).toHaveBeenCalledWith('Usenet credentials saved', 'success'), { interval: 1 });
    });

    it('shows error message when username is empty', async () => {
        document.getElementById('usenet-username').value = '';
        document.getElementById('usenet-password').value = 'secret';

        document.getElementById('usenet-cred-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await new Promise(r => setTimeout(r, 0));
        expect(api).not.toHaveBeenCalled();
        expect(document.getElementById('usenet-cred-status').textContent).toContain('required');
    });

    it('shows API error on save failure', async () => {
        api.mockRejectedValueOnce(new Error('bad request'));

        document.getElementById('usenet-username').value = 'alice';
        document.getElementById('usenet-password').value = 'secret';

        document.getElementById('usenet-cred-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await vi.waitFor(() =>
            expect(document.getElementById('usenet-cred-status').textContent).toContain('bad request'),
        { interval: 1 });
    });
});

// ---------- Settings page: delete credentials ----------

describe('initUsenetSettingsListeners - delete credentials', () => {
    beforeEach(() => {
        setupSettingsDOM();
        vi.clearAllMocks();
        // Show the form and delete button.
        document.getElementById('usenet-credentials-form').style.display = '';
        document.getElementById('usenet-delete-btn').style.display = '';
        vi.stubGlobal('confirm', vi.fn(() => true));
        initUsenetSettingsListeners();
    });

    afterEach(() => {
        document.body.innerHTML = '';
        vi.unstubAllGlobals();
    });

    it('deletes credentials on button click', async () => {
        api.mockResolvedValueOnce({ status: 'ok' });

        document.querySelector('[data-action="usenet-delete-credentials"]').click();

        await vi.waitFor(() => expect(api).toHaveBeenCalledWith('DELETE', '/api/usenet/credentials'), { interval: 1 });
        await vi.waitFor(() => expect(showToast).toHaveBeenCalledWith('Usenet credentials removed'), { interval: 1 });
    });

    it('does not delete when user cancels the confirm dialog', async () => {
        vi.stubGlobal('confirm', vi.fn(() => false));

        document.querySelector('[data-action="usenet-delete-credentials"]').click();

        await new Promise(r => setTimeout(r, 0));
        expect(api).not.toHaveBeenCalled();
    });

    it('shows error on delete failure', async () => {
        api.mockRejectedValueOnce(new Error('server error'));

        document.querySelector('[data-action="usenet-delete-credentials"]').click();

        await vi.waitFor(() =>
            expect(document.getElementById('usenet-cred-status').textContent).toContain('server error'),
        { interval: 1 });
    });
});

// ---------- Feeds page: load groups with credentials ----------

describe('initUsenetGroupsSection - with credentials', () => {
    beforeEach(() => {
        setupFeedsDOM();
        vi.clearAllMocks();
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    it('is a no-op when usenet section is absent', async () => {
        document.body.innerHTML = '';
        await initUsenetGroupsSection();
        expect(api).not.toHaveBeenCalled();
    });

    it('shows no-credentials prompt when credentials are not configured', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: false });
        await initUsenetGroupsSection();

        expect(document.getElementById('usenet-no-credentials').style.display).not.toBe('none');
        expect(document.getElementById('usenet-groups-content').style.display).toBe('none');
    });

    it('shows groups content when credentials are configured', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: true });
        api.mockResolvedValueOnce([]); // GET /api/usenet/groups
        await initUsenetGroupsSection();

        expect(document.getElementById('usenet-groups-content').style.display).not.toBe('none');
        expect(document.getElementById('usenet-no-credentials').style.display).toBe('none');
    });

    it('renders subscribed groups', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: true });
        api.mockResolvedValueOnce([
            { feed_id: 1, group_name: 'comp.lang.go', provider: 'eternal-september', high_water_article_number: 42 },
            { feed_id: 2, group_name: 'misc.test', provider: 'eternal-september', high_water_article_number: 0 },
        ]);
        await initUsenetGroupsSection();

        const list = document.getElementById('usenet-groups-list');
        expect(list.textContent).toContain('comp.lang.go');
        expect(list.textContent).toContain('misc.test');
        expect(list.textContent).toContain('#42');
        expect(list.textContent).toContain('Not fetched yet');
    });

    it('renders empty-state message when no groups', async () => {
        api.mockResolvedValueOnce({ enabled: true, configured: true });
        api.mockResolvedValueOnce([]);
        await initUsenetGroupsSection();

        const list = document.getElementById('usenet-groups-list');
        expect(list.textContent).toContain('No newsgroups subscribed yet');
    });

    it('shows missing-credentials prompt on API error', async () => {
        api.mockRejectedValueOnce(new Error('network failure'));
        await initUsenetGroupsSection();

        const noCredsEl = document.getElementById('usenet-no-credentials');
        expect(noCredsEl.style.display).not.toBe('none');
    });
});

// ---------- Feeds page: add newsgroup ----------

describe('initUsenetFeedsListeners - add newsgroup', () => {
    beforeEach(() => {
        setupFeedsDOM();
        vi.clearAllMocks();
        document.getElementById('usenet-groups-content').style.display = '';
        initUsenetFeedsListeners();
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    it('is a no-op when feeds section is absent', () => {
        document.body.innerHTML = '';
        // should not throw
        expect(() => initUsenetFeedsListeners()).not.toThrow();
    });

    it('subscribes to a newsgroup on form submit', async () => {
        api.mockResolvedValueOnce({ feed_id: 5, group_name: 'comp.lang.go' }); // POST
        api.mockResolvedValueOnce([{ feed_id: 5, group_name: 'comp.lang.go', high_water_article_number: 0 }]); // GET refresh

        document.getElementById('usenet-group-name').value = 'comp.lang.go';
        document.getElementById('usenet-group-category').value = '0';

        document.getElementById('usenet-add-group-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await vi.waitFor(() => expect(api).toHaveBeenCalledWith('POST', '/api/usenet/groups', {
            group_name: 'comp.lang.go',
            category_id: 0,
        }), { interval: 1 });

        await vi.waitFor(() => expect(showToast).toHaveBeenCalledWith('Subscribed to comp.lang.go', 'success'), { interval: 1 });
    });

    it('sends category_id when folder is selected', async () => {
        api.mockResolvedValueOnce({ feed_id: 6, group_name: 'alt.test' }); // POST
        api.mockResolvedValueOnce([]); // GET refresh

        document.getElementById('usenet-group-name').value = 'alt.test';
        document.getElementById('usenet-group-category').value = '42';

        document.getElementById('usenet-add-group-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await vi.waitFor(() => expect(api).toHaveBeenCalledWith('POST', '/api/usenet/groups', {
            group_name: 'alt.test',
            category_id: 42,
        }), { interval: 1 });
    });

    it('shows error when group name is empty', async () => {
        document.getElementById('usenet-group-name').value = '';

        document.getElementById('usenet-add-group-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await new Promise(r => setTimeout(r, 0));
        expect(api).not.toHaveBeenCalled();
        expect(document.getElementById('usenet-add-status').textContent).toContain('required');
    });

    it('shows API validation error on duplicate/invalid group', async () => {
        api.mockRejectedValueOnce(new Error('Already subscribed to comp.lang.go'));

        document.getElementById('usenet-group-name').value = 'comp.lang.go';

        document.getElementById('usenet-add-group-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await vi.waitFor(() =>
            expect(document.getElementById('usenet-add-status').textContent).toContain('Already subscribed'),
        { interval: 1 });
    });

    it('clears the group name input after successful subscribe', async () => {
        api.mockResolvedValueOnce({ feed_id: 7, group_name: 'comp.lang.go' });
        api.mockResolvedValueOnce([]);

        document.getElementById('usenet-group-name').value = 'comp.lang.go';
        document.getElementById('usenet-add-group-form').dispatchEvent(new Event('submit', { bubbles: true }));

        await vi.waitFor(() => expect(api).toHaveBeenCalledWith('POST', '/api/usenet/groups', expect.any(Object)), { interval: 1 });
        await vi.waitFor(() => {
            expect(document.getElementById('usenet-group-name').value).toBe('');
        }, { interval: 1 });
    });
});

// ---------- Feeds page: remove newsgroup ----------

describe('initUsenetFeedsListeners - remove newsgroup', () => {
    beforeEach(() => {
        setupFeedsDOM();
        vi.clearAllMocks();
        vi.stubGlobal('confirm', vi.fn(() => true));
        document.getElementById('usenet-groups-content').style.display = '';
        // Pre-populate the list with one group.
        document.getElementById('usenet-groups-list').innerHTML = `
            <div class="usenet-group-row" data-feed-id="3">
                <span class="usenet-group-name">sci.physics</span>
                <button data-action="usenet-remove-group"
                        data-feed-id="3" data-group-name="sci.physics">Remove</button>
            </div>
        `;
        initUsenetFeedsListeners();
    });

    afterEach(() => {
        document.body.innerHTML = '';
        vi.unstubAllGlobals();
    });

    it('removes newsgroup on button click', async () => {
        api.mockResolvedValueOnce({ status: 'ok' }); // DELETE
        api.mockResolvedValueOnce([]); // GET refresh

        document.querySelector('[data-action="usenet-remove-group"]').click();

        await vi.waitFor(() =>
            expect(api).toHaveBeenCalledWith('DELETE', '/api/usenet/groups/3'),
        { interval: 1 });

        await vi.waitFor(() => expect(showToast).toHaveBeenCalledWith('Unsubscribed from sci.physics'), { interval: 1 });
    });

    it('does not remove when user cancels the confirm dialog', async () => {
        vi.stubGlobal('confirm', vi.fn(() => false));

        document.querySelector('[data-action="usenet-remove-group"]').click();

        await new Promise(r => setTimeout(r, 0));
        expect(api).not.toHaveBeenCalled();
    });

    it('shows error toast on remove failure', async () => {
        api.mockRejectedValueOnce(new Error('not found'));

        document.querySelector('[data-action="usenet-remove-group"]').click();

        await vi.waitFor(() =>
            expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to remove')),
        { interval: 1 });
    });
});
