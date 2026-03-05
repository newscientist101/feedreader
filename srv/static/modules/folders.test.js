import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
    openCreateFolderModal, closeCreateFolderModal, submitCreateFolder,
    renameCategory, unparentCategory, deleteCategory,
    initFoldersPageListeners, initCategorySettingsPage,
} from './folders.js';
import { showToast } from './toast.js';
import { openModal, closeModal } from './modal.js';

vi.mock('./toast.js');

vi.mock('./modal.js');

beforeEach(() => {
    document.body.innerHTML = '';
    vi.restoreAllMocks();
    vi.clearAllMocks();
    // Ensure dialog functions exist for happy-dom compatibility
    window.confirm ??= () => false;
    window.prompt ??= () => null;
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe('openCreateFolderModal', () => {
    it('shows the modal, resets fields, and calls openModal', () => {
        document.body.innerHTML = `
            <div id="create-folder-modal" style="display: none">
                <input id="new-folder-name" value="Old">
                <select id="new-folder-parent">
                    <option value="0">None</option>
                    <option value="5">Parent</option>
                </select>
            </div>
        `;
        // Pre-select a non-default option
        document.getElementById('new-folder-parent').value = '5';

        openCreateFolderModal();

        expect(document.getElementById('create-folder-modal').style.display).toBe('flex');
        expect(document.getElementById('new-folder-name').value).toBe('');
        expect(document.getElementById('new-folder-parent').value).toBe('0');
        expect(openModal).toHaveBeenCalledWith(
            document.getElementById('create-folder-modal'),
            expect.any(Function),
            document.getElementById('new-folder-name'),
        );
    });

    it('does nothing when modal not found', () => {
        document.body.innerHTML = '<div>Content</div>';
        openCreateFolderModal(); // should not throw
        expect(openModal).not.toHaveBeenCalled();
    });
});

describe('closeCreateFolderModal', () => {
    it('hides the modal and calls closeModal', () => {
        document.body.innerHTML = '<div id="create-folder-modal" style="display: flex"></div>';

        closeCreateFolderModal();

        expect(document.getElementById('create-folder-modal').style.display).toBe('none');
        expect(closeModal).toHaveBeenCalled();
    });

    it('calls closeModal even when modal element not found', () => {
        document.body.innerHTML = '<div>Content</div>';
        closeCreateFolderModal();
        expect(closeModal).toHaveBeenCalled();
    });
});

describe('submitCreateFolder', () => {
    beforeEach(() => {
        document.body.innerHTML = `
            <div id="create-folder-modal">
                <input id="new-folder-name" value="New Folder">
                <select id="new-folder-parent">
                    <option value="0">None</option>
                    <option value="5">Parent</option>
                </select>
            </div>
        `;
    });

    it('creates a folder with no parent and reloads', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ id: 10 }),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(event.preventDefault).toHaveBeenCalled();
        expect(fetch).toHaveBeenCalledOnce();
        expect(fetch).toHaveBeenCalledWith('/api/categories', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ name: 'New Folder' }),
        }));
        expect(reloadMock).toHaveBeenCalled();
        expect(showToast).not.toHaveBeenCalled();
    });

    it('creates a folder with parent and sets parent', async () => {
        document.getElementById('new-folder-parent').value = '5';
        const fetchMock = vi.spyOn(globalThis, 'fetch')
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({ id: 10 }),
            })
            .mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve({}),
            });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(fetchMock).toHaveBeenCalledTimes(2);
        // Second call sets the parent
        expect(fetchMock.mock.calls[1][0]).toBe('/api/categories/10/parent');
        expect(reloadMock).toHaveBeenCalled();
    });

    it('skips parent call when cat.id is falsy', async () => {
        document.getElementById('new-folder-parent').value = '5';
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}), // no id in response
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        // Only one call — the create, no parent call
        expect(fetch).toHaveBeenCalledOnce();
        expect(reloadMock).toHaveBeenCalled();
    });

    it('does nothing when name is empty', async () => {
        document.getElementById('new-folder-name').value = '  ';
        vi.spyOn(globalThis, 'fetch');
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(event.preventDefault).toHaveBeenCalled();
        expect(fetch).not.toHaveBeenCalled();
    });

    it('shows toast on error', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'Duplicate' })),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });
        const event = { preventDefault: vi.fn() };

        await submitCreateFolder(event);

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to create folder'));
        expect(reloadMock).not.toHaveBeenCalled();
    });
});

describe('renameCategory', () => {
    it('renames on user input and reloads', async () => {
        vi.spyOn(window, 'prompt').mockReturnValue('Renamed Folder');
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await renameCategory(5, 'Old Name');

        expect(window.prompt).toHaveBeenCalledWith('Enter new name:', 'Old Name');
        expect(fetch).toHaveBeenCalledWith('/api/categories/5', expect.objectContaining({
            method: 'PUT',
            body: JSON.stringify({ name: 'Renamed Folder' }),
        }));
        expect(reloadMock).toHaveBeenCalled();
        expect(showToast).not.toHaveBeenCalled();
    });

    it('does nothing when user cancels prompt', async () => {
        vi.spyOn(window, 'prompt').mockReturnValue(null);
        vi.spyOn(globalThis, 'fetch');

        await renameCategory(5, 'Old Name');

        expect(fetch).not.toHaveBeenCalled();
    });

    it('does nothing when name is unchanged', async () => {
        vi.spyOn(window, 'prompt').mockReturnValue('Old Name');
        vi.spyOn(globalThis, 'fetch');

        await renameCategory(5, 'Old Name');

        expect(fetch).not.toHaveBeenCalled();
    });

    it('shows toast on error', async () => {
        vi.spyOn(window, 'prompt').mockReturnValue('New Name');
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'Server error' })),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await renameCategory(5, 'Old Name');

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to rename folder'));
        expect(reloadMock).not.toHaveBeenCalled();
    });
});

describe('unparentCategory', () => {
    it('moves category to top level and reloads', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await unparentCategory(5);

        expect(fetch).toHaveBeenCalledWith('/api/categories/5/parent', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ parent_id: null, sort_order: 999 }),
        }));
        expect(reloadMock).toHaveBeenCalled();
        expect(showToast).not.toHaveBeenCalled();
    });

    it('handles errors gracefully', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'fail' })),
        });
        vi.spyOn(console, 'error').mockImplementation(() => {});
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await unparentCategory(5);

        expect(console.error).toHaveBeenCalledWith('Failed to unparent category:', expect.any(Error));
        expect(showToast).toHaveBeenCalledWith('Failed to move folder to top level');
        expect(reloadMock).not.toHaveBeenCalled();
    });
});

describe('deleteCategory', () => {
    it('deletes on confirm and reloads', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await deleteCategory(5, 'Tech');

        expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Tech'));
        expect(fetch).toHaveBeenCalledWith('/api/categories/5', expect.objectContaining({ method: 'DELETE' }));
        expect(reloadMock).toHaveBeenCalled();
        expect(showToast).not.toHaveBeenCalled();
    });

    it('does nothing when user cancels', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(false);
        vi.spyOn(globalThis, 'fetch');

        await deleteCategory(5, 'Tech');

        expect(fetch).not.toHaveBeenCalled();
    });

    it('shows toast on error', async () => {
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'in use' })),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        await deleteCategory(5, 'Tech');

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to delete folder'));
        expect(reloadMock).not.toHaveBeenCalled();
    });
});

describe('initFoldersPageListeners', () => {
    beforeEach(() => {
        initFoldersPageListeners();
    });

    it('delegates rename-category click', () => {
        document.body.innerHTML = `
            <button data-action="rename-category" data-category-id="5" data-category-name="Tech">Rename</button>
        `;
        vi.spyOn(window, 'prompt').mockReturnValue(null); // user cancels

        document.querySelector('[data-action="rename-category"]').click();

        expect(window.prompt).toHaveBeenCalledWith('Enter new name:', 'Tech');
    });

    it('delegates delete-category click', () => {
        document.body.innerHTML = `
            <button data-action="delete-category" data-category-id="5" data-category-name="Tech">Delete</button>
        `;
        vi.spyOn(window, 'confirm').mockReturnValue(false);

        document.querySelector('[data-action="delete-category"]').click();

        expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Tech'));
    });

    it('delegates open-create-folder click', () => {
        document.body.innerHTML = `
            <div data-action="open-create-folder">New Folder</div>
            <div id="create-folder-modal" style="display: none">
                <input id="new-folder-name" value="">
                <select id="new-folder-parent"><option value="0">None</option></select>
            </div>
        `;

        document.querySelector('[data-action="open-create-folder"]').click();

        expect(document.getElementById('create-folder-modal').style.display).toBe('flex');
    });

    it('delegates close-create-folder click', () => {
        document.body.innerHTML = `
            <div id="create-folder-modal" style="display: flex">
                <button data-action="close-create-folder">Close</button>
            </div>
        `;

        document.querySelector('[data-action="close-create-folder"]').click();

        expect(document.getElementById('create-folder-modal').style.display).toBe('none');
    });

    it('delegates create-folder-form submission', async () => {
        document.body.innerHTML = `
            <form id="create-folder-form">
                <input id="new-folder-name" value="Test">
                <select id="new-folder-parent"><option value="0">None</option></select>
                <button type="submit">Create</button>
            </form>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({ id: 10 }),
        });
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: vi.fn() },
            writable: true,
            configurable: true,
        });

        const form = document.getElementById('create-folder-form');
        form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await new Promise(r => setTimeout(r, 0));

        expect(fetch).toHaveBeenCalledWith('/api/categories', expect.objectContaining({
            method: 'POST',
        }));
    });

    it('ignores rename-category without id', () => {
        document.body.innerHTML = `
            <button data-action="rename-category" data-category-name="Tech">Rename</button>
        `;
        vi.spyOn(window, 'prompt');

        document.querySelector('[data-action="rename-category"]').click();

        expect(window.prompt).not.toHaveBeenCalled();
    });

    it('ignores delete-category without id', () => {
        document.body.innerHTML = `
            <button data-action="delete-category" data-category-name="Tech">Delete</button>
        `;
        vi.spyOn(window, 'confirm');

        document.querySelector('[data-action="delete-category"]').click();

        expect(window.confirm).not.toHaveBeenCalled();
    });

    it('delegates unparent-category click', async () => {
        document.body.innerHTML = `
            <button data-action="unparent-category" data-category-id="7">Move to top</button>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        document.querySelector('[data-action="unparent-category"]').click();
        await vi.waitFor(() => expect(fetch).toHaveBeenCalled(), { interval: 1 });

        expect(fetch).toHaveBeenCalledWith('/api/categories/7/parent', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ parent_id: null, sort_order: 999 }),
        }));
    });

    it('ignores unparent-category without id', () => {
        document.body.innerHTML = `
            <button data-action="unparent-category">Move to top</button>
        `;
        vi.spyOn(globalThis, 'fetch');

        document.querySelector('[data-action="unparent-category"]').click();

        expect(fetch).not.toHaveBeenCalled();
    });

    it('delegates delete-exclusion click with confirm', async () => {
        document.body.innerHTML = `
            <button data-action="delete-exclusion" data-exclusion-id="3">Delete</button>
        `;
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        document.querySelector('[data-action="delete-exclusion"]').click();
        await vi.waitFor(() => expect(fetch).toHaveBeenCalled(), { interval: 1 });

        expect(window.confirm).toHaveBeenCalledWith('Delete this exclusion rule?');
        expect(fetch).toHaveBeenCalledWith('/api/exclusions/3', expect.objectContaining({
            method: 'DELETE',
        }));
    });

    it('does not delete exclusion when user cancels', () => {
        document.body.innerHTML = `
            <button data-action="delete-exclusion" data-exclusion-id="3">Delete</button>
        `;
        vi.spyOn(window, 'confirm').mockReturnValue(false);
        vi.spyOn(globalThis, 'fetch');

        document.querySelector('[data-action="delete-exclusion"]').click();

        expect(window.confirm).toHaveBeenCalled();
        expect(fetch).not.toHaveBeenCalled();
    });

    it('ignores delete-exclusion without id', () => {
        document.body.innerHTML = `
            <button data-action="delete-exclusion">Delete</button>
        `;
        vi.spyOn(window, 'confirm');

        document.querySelector('[data-action="delete-exclusion"]').click();

        expect(window.confirm).not.toHaveBeenCalled();
    });

    it('shows toast on delete-exclusion error', async () => {
        document.body.innerHTML = `
            <button data-action="delete-exclusion" data-exclusion-id="3">Delete</button>
        `;
        vi.spyOn(window, 'confirm').mockReturnValue(true);
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'not found' })),
        });

        document.querySelector('[data-action="delete-exclusion"]').click();
        await vi.waitFor(() => expect(showToast).toHaveBeenCalled(), { interval: 1 });

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to delete rule'));
    });
});

describe('initCategorySettingsPage', () => {
    it('is a no-op when settings view is not present', () => {
        document.body.innerHTML = '<div>Not the settings page</div>';
        initCategorySettingsPage(); // should not throw
    });

    it('is a no-op when form is not present', () => {
        document.body.innerHTML = `
            <div class="settings-view" data-category-id="5"></div>
        `;
        initCategorySettingsPage(); // should not throw
    });

    it('submits exclusion form and reloads on success', async () => {
        document.body.innerHTML = `
            <div class="settings-view" data-category-id="5">
                <form id="add-exclusion-form">
                    <select id="exclusion-type"><option value="keyword">Keyword</option></select>
                    <input id="exclusion-pattern" value="spam">
                    <input id="exclusion-regex" type="checkbox">
                    <button type="submit">Add</button>
                </form>
            </div>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        initCategorySettingsPage();

        const form = document.getElementById('add-exclusion-form');
        form.dispatchEvent(new Event('submit', { cancelable: true }));
        await vi.waitFor(() => expect(fetch).toHaveBeenCalled(), { interval: 1 });

        expect(fetch).toHaveBeenCalledWith('/api/categories/5/exclusions', expect.objectContaining({
            method: 'POST',
            body: JSON.stringify({ type: 'keyword', pattern: 'spam', isRegex: false }),
        }));
        await vi.waitFor(() => expect(reloadMock).toHaveBeenCalled(), { interval: 1 });
    });

    it('submits exclusion form with regex checked', async () => {
        document.body.innerHTML = `
            <div class="settings-view" data-category-id="5">
                <form id="add-exclusion-form">
                    <select id="exclusion-type"><option value="author">Author</option></select>
                    <input id="exclusion-pattern" value="bot.*">
                    <input id="exclusion-regex" type="checkbox" checked>
                    <button type="submit">Add</button>
                </form>
            </div>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        initCategorySettingsPage();

        const form = document.getElementById('add-exclusion-form');
        form.dispatchEvent(new Event('submit', { cancelable: true }));
        await vi.waitFor(() => expect(fetch).toHaveBeenCalled(), { interval: 1 });

        expect(fetch).toHaveBeenCalledWith('/api/categories/5/exclusions', expect.objectContaining({
            body: JSON.stringify({ type: 'author', pattern: 'bot.*', isRegex: true }),
        }));
    });

    it('shows toast on exclusion form error', async () => {
        document.body.innerHTML = `
            <div class="settings-view" data-category-id="5">
                <form id="add-exclusion-form">
                    <select id="exclusion-type"><option value="keyword">Keyword</option></select>
                    <input id="exclusion-pattern" value="test">
                    <input id="exclusion-regex" type="checkbox">
                    <button type="submit">Add</button>
                </form>
            </div>
        `;
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: false,
            text: () => Promise.resolve(JSON.stringify({ error: 'Bad request' })),
        });
        const reloadMock = vi.fn();
        Object.defineProperty(window, 'location', {
            value: { ...window.location, reload: reloadMock },
            writable: true,
            configurable: true,
        });

        initCategorySettingsPage();

        const form = document.getElementById('add-exclusion-form');
        form.dispatchEvent(new Event('submit', { cancelable: true }));
        await vi.waitFor(() => expect(showToast).toHaveBeenCalled(), { interval: 1 });

        expect(showToast).toHaveBeenCalledWith(expect.stringContaining('Failed to add exclusion'));
        expect(reloadMock).not.toHaveBeenCalled();
    });
});
