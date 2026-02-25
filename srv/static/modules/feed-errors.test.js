import { describe, it, expect, beforeEach } from 'vitest';
import { showFeedErrorBanner, removeFeedErrorBanner } from './feed-errors.js';

beforeEach(() => {
    document.body.innerHTML = '';
});

describe('showFeedErrorBanner', () => {
    it('creates an error banner if none exists', () => {
        document.body.innerHTML = '<div class="articles-view"></div>';

        showFeedErrorBanner(42, 'Connection timeout');

        const banner = document.querySelector('.feed-error-banner');
        expect(banner).not.toBeNull();
        expect(banner.innerHTML).toContain('Connection timeout');
        expect(banner.innerHTML).toContain('data-action="refresh-feed"');
        expect(banner.innerHTML).toContain('data-feed-id="42"');
    });

    it('updates existing banner', () => {
        document.body.innerHTML = `
            <div class="articles-view">
                <div class="feed-error-banner">Old error</div>
            </div>
        `;

        showFeedErrorBanner(7, 'New error');

        const banners = document.querySelectorAll('.feed-error-banner');
        expect(banners).toHaveLength(1);
        expect(banners[0].innerHTML).toContain('New error');
    });

    it('inserts banner as first child of articles-view', () => {
        document.body.innerHTML = '<div class="articles-view"><p>Existing content</p></div>';

        showFeedErrorBanner(1, 'Error msg');

        const view = document.querySelector('.articles-view');
        expect(view.firstElementChild.classList.contains('feed-error-banner')).toBe(true);
    });

    it('does nothing when articles-view is absent', () => {
        document.body.innerHTML = '<div>No articles view</div>';

        showFeedErrorBanner(1, 'Error');

        // Should still find the banner if it was already in DOM
        expect(document.querySelector('.feed-error-banner')).toBeNull();
    });
});

describe('removeFeedErrorBanner', () => {
    it('removes the banner if present', () => {
        document.body.innerHTML = '<div class="feed-error-banner">Error</div>';

        removeFeedErrorBanner();

        expect(document.querySelector('.feed-error-banner')).toBeNull();
    });

    it('does nothing if no banner exists', () => {
        document.body.innerHTML = '<div>Content</div>';
        removeFeedErrorBanner(); // should not throw
    });
});
