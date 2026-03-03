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

    it('does nothing when articles-view is absent and no prior banner', () => {
        document.body.innerHTML = '<div>No articles view</div>';

        showFeedErrorBanner(1, 'Error');

        // Banner created internally but never inserted (no .articles-view to attach to)
        expect(document.querySelector('.feed-error-banner')).toBeNull();
    });

    it('escapes HTML in error messages', () => {
        document.body.innerHTML = '<div class="articles-view"></div>';

        showFeedErrorBanner(1, '<script>alert("xss")</script>');

        const banner = document.querySelector('.feed-error-banner');
        expect(banner.innerHTML).not.toContain('<script>');
        expect(banner.innerHTML).toContain('&lt;script&gt;');
    });

    it('updates existing banner even when outside articles-view', () => {
        // Banner exists in DOM but not inside .articles-view
        document.body.innerHTML = `
            <div class="feed-error-banner">Old error</div>
            <div class="articles-view"></div>
        `;

        showFeedErrorBanner(99, 'Updated error');

        const banners = document.querySelectorAll('.feed-error-banner');
        expect(banners).toHaveLength(1);
        expect(banners[0].innerHTML).toContain('Updated error');
        expect(banners[0].innerHTML).toContain('data-feed-id="99"');
    });

    it('includes SVG warning icon in the banner', () => {
        document.body.innerHTML = '<div class="articles-view"></div>';

        showFeedErrorBanner(1, 'Error');

        const banner = document.querySelector('.feed-error-banner');
        expect(banner.querySelector('svg')).not.toBeNull();
    });

    it('includes retry button with correct data attributes', () => {
        document.body.innerHTML = '<div class="articles-view"></div>';

        showFeedErrorBanner(55, 'Timeout');

        const btn = document.querySelector('.feed-error-banner button');
        expect(btn).not.toBeNull();
        expect(btn.dataset.action).toBe('refresh-feed');
        expect(btn.dataset.feedId).toBe('55');
        expect(btn.textContent).toBe('Retry');
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
