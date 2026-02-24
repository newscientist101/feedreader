import { describe, it, expect, beforeEach } from 'vitest';
import { initTimestampTooltips } from './timestamps.js';

describe('timestamps', () => {
    describe('initTimestampTooltips', () => {
        beforeEach(() => {
            document.body.innerHTML = '';
        });

        it('sets title attribute from data-timestamp', () => {
            document.body.innerHTML = `
                <span data-timestamp="2024-01-15T10:30:00Z" id="ts1">5h ago</span>
            `;
            initTimestampTooltips();
            const el = document.getElementById('ts1');
            expect(el.title).toContain('2024');
            expect(el.title).toContain('January');
        });

        it('handles multiple elements', () => {
            document.body.innerHTML = `
                <span data-timestamp="2024-01-15T10:30:00Z" id="ts1">5h ago</span>
                <span data-timestamp="2024-06-20T14:00:00Z" id="ts2">3d ago</span>
            `;
            initTimestampTooltips();
            expect(document.getElementById('ts1').title).toContain('January');
            expect(document.getElementById('ts2').title).toContain('June');
        });

        it('skips elements with empty data-timestamp', () => {
            document.body.innerHTML = `
                <span data-timestamp="" id="ts1">Unknown</span>
            `;
            initTimestampTooltips();
            expect(document.getElementById('ts1').title).toBe('');
        });

        it('does nothing when no elements have data-timestamp', () => {
            document.body.innerHTML = '<div>No timestamps here</div>';
            // Should not throw
            initTimestampTooltips();
        });
    });
});
