// Drag-and-drop reordering for folders/categories.

import { api } from './api.js';
import { showToast } from './toast.js';

/**
 * Initialize drag-drop on sidebar folders and feeds-page category cards.
 */
export function initFolderDragDrop() {
    // Sidebar folders
    const foldersContainer = document.querySelector('.folders-list');
    if (foldersContainer) {
        initDragDrop(foldersContainer, '.folder-item', 'data-category-id');
    }

    // Feeds page category cards
    const categoriesGrid = document.querySelector('.categories-grid');
    if (categoriesGrid) {
        initDragDrop(categoriesGrid, '.category-card[data-id]', 'data-id');
    }
}

/**
 * Set up drag-and-drop reordering within a container.
 */
export function initDragDrop(container, itemSelector, idAttr) {
    let draggedItem = null;
    let placeholder = null;
    let dropTarget = null; // For nesting
    let draggedParentId = null; // Track parent of dragged item

    // Helper to get parent ID from item
    function getParentId(item) {
        const parentAttr = item.dataset.parentId;
        return parentAttr ? parseInt(parentAttr) : null;
    }

    // Helper to get siblings (items with same parent)
    function getSiblings(parentId) {
        return Array.from(container.querySelectorAll(itemSelector)).filter(item => {
            return getParentId(item) === parentId;
        });
    }

    container.addEventListener('dragstart', (e) => {
        const item = e.target.closest(itemSelector);
        if (!item) return;

        draggedItem = item;
        draggedParentId = getParentId(item);
        item.classList.add('dragging');
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', item.getAttribute(idAttr));

        // Create placeholder
        placeholder = document.createElement('div');
        placeholder.className = 'drag-placeholder';
        placeholder.style.height = item.offsetHeight + 'px';
        // Copy indentation from dragged item
        if (item.style.paddingLeft) {
            placeholder.style.marginLeft = item.style.paddingLeft;
        }
    });

    container.addEventListener('dragend', () => {
        if (draggedItem) {
            draggedItem.classList.remove('dragging');
            draggedItem = null;
        }
        if (placeholder && placeholder.parentNode) {
            placeholder.remove();
        }
        placeholder = null;
        draggedParentId = null;

        // Remove any remaining drag-over classes
        container.querySelectorAll('.drag-over, .nest-target').forEach(el => {
            el.classList.remove('drag-over', 'nest-target');
        });
        dropTarget = null;
    });

    container.addEventListener('dragover', (e) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';

        const targetItem = e.target.closest(itemSelector);

        // Check if we're hovering over another folder (for nesting)
        // Only if holding Shift key
        if (e.shiftKey && targetItem && targetItem !== draggedItem) {
            // Show nest target indicator
            container.querySelectorAll('.nest-target').forEach(el => el.classList.remove('nest-target'));
            targetItem.classList.add('nest-target');
            dropTarget = targetItem;
            if (placeholder.parentNode) placeholder.remove();
            return;
        } else {
            container.querySelectorAll('.nest-target').forEach(el => el.classList.remove('nest-target'));
            dropTarget = null;
        }

        // Get position among siblings only
        const siblings = getSiblings(draggedParentId);
        const afterElement = getDragAfterElementAmongSiblings(siblings, e.clientY);

        if (placeholder) {
            if (afterElement) {
                container.insertBefore(placeholder, afterElement);
            } else {
                // Find where to insert at end of siblings
                if (siblings.length > 0) {
                    const lastSibling = siblings[siblings.length - 1];
                    if (lastSibling.nextSibling) {
                        container.insertBefore(placeholder, lastSibling.nextSibling);
                    } else {
                        // Check for add-category card
                        const addCard = container.querySelector('.add-category');
                        if (addCard) {
                            container.insertBefore(placeholder, addCard);
                        } else {
                            container.appendChild(placeholder);
                        }
                    }
                } else {
                    const addCard = container.querySelector('.add-category');
                    if (addCard) {
                        container.insertBefore(placeholder, addCard);
                    } else {
                        container.appendChild(placeholder);
                    }
                }
            }
        }
    });

    container.addEventListener('drop', async (e) => {
        e.preventDefault();

        if (!draggedItem) return;

        const draggedId = parseInt(draggedItem.getAttribute(idAttr));

        // Check if nesting (Shift was held and we have a target)
        if (dropTarget && dropTarget !== draggedItem) {
            const parentId = parseInt(dropTarget.getAttribute(idAttr));

            // Set parent via API
            try {
                await api('POST', `/api/categories/${draggedId}/parent`, {
                    parent_id: parentId,
                    sort_order: 0
                });
                // Reload to show new hierarchy
                location.reload();
            } catch (err) {
                console.error('Failed to nest folder:', err);
                showToast('Failed to move folder');
            }
            return;
        }

        if (!placeholder) return;

        // Insert the dragged item where the placeholder is
        placeholder.replaceWith(draggedItem);

        // Get new order - only for siblings with the same parent
        const siblings = getSiblings(draggedParentId);
        const order = siblings
            .map(item => parseInt(item.getAttribute(idAttr)))
            .filter(id => !isNaN(id));

        // Save new order to server (include parent_id so server knows context)
        try {
            await api('POST', '/api/categories/reorder', {
                order,
                parent_id: draggedParentId
            });
            // Sync the other container
            syncFolderOrder(order, container, draggedParentId);
        } catch (err) {
            console.error('Failed to save folder order:', err);
            showToast('Failed to save folder order');
        }
    });
}

/**
 * Sync reordering between sidebar and feeds page.
 */
export function syncFolderOrder(order, sourceContainer, parentId = null) {
    // Sync sidebar folders
    const sidebarFolders = document.querySelector('.folders-list');
    if (sidebarFolders && sidebarFolders !== sourceContainer) {
        reorderElements(sidebarFolders, '.folder-item', 'data-category-id', order, parentId);
    }

    // Sync feeds page category cards
    const categoriesGrid = document.querySelector('.categories-grid');
    if (categoriesGrid && categoriesGrid !== sourceContainer) {
        reorderElements(categoriesGrid, '.category-card[data-id]', 'data-id', order, parentId);
    }
}

/**
 * Reorder elements in a container to match the given order.
 */
export function reorderElements(container, itemSelector, idAttr, order, parentId = null) {
    // Get only items with the matching parent
    const items = Array.from(container.querySelectorAll(itemSelector)).filter(item => {
        const itemParentId = item.dataset.parentId ? parseInt(item.dataset.parentId) : null;
        return itemParentId === parentId;
    });

    const itemMap = new Map();
    items.forEach(item => {
        const id = parseInt(item.getAttribute(idAttr));
        if (!isNaN(id)) {
            itemMap.set(id, item);
        }
    });

    if (items.length === 0) return;

    // Find the first sibling's position to know where to insert
    const firstSibling = items[0];
    let insertPoint = firstSibling;

    // Reorder by inserting in order at the insertion point
    order.forEach((id, index) => {
        const item = itemMap.get(id);
        if (item) {
            if (index === 0) {
                // First item stays at the original position
                insertPoint = item.nextSibling;
            } else {
                container.insertBefore(item, insertPoint);
                insertPoint = item.nextSibling;
            }
        }
    });
}

/**
 * Get position among a specific set of sibling elements.
 */
export function getDragAfterElementAmongSiblings(siblings, y) {
    const nonDragging = siblings.filter(el => !el.classList.contains('dragging'));

    return nonDragging.reduce((closest, child) => {
        const box = child.getBoundingClientRect();
        const offset = y - box.top - box.height / 2;

        if (offset < 0 && offset > closest.offset) {
            return { offset, element: child };
        } else {
            return closest;
        }
    }, { offset: Number.NEGATIVE_INFINITY }).element;
}

/**
 * Prevent starting a drag when clicking folder chevrons.
 * Called once at module init time (top-level side effect from app.js).
 */
let _dragPreventionAC = null;
export function initDragPrevention() {
    if (_dragPreventionAC) _dragPreventionAC.abort();
    _dragPreventionAC = new AbortController();
    const signal = _dragPreventionAC.signal;
    document.addEventListener('dragstart', (event) => {
        if (event.target.closest('.folder-chevron')) {
            event.preventDefault();
        }
    }, { capture: true, signal });
}
