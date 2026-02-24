// Timestamp tooltip initialization — sets title attributes from data-timestamp.

import { formatLocalDate } from './utils.js';

export function initTimestampTooltips() {
    document.querySelectorAll('[data-timestamp]').forEach(el => {
        const timestamp = el.dataset.timestamp;
        if (timestamp) {
            el.title = formatLocalDate(timestamp);
        }
    });
}
