import { describe, it, expect } from 'vitest';
import {
    SVG_MARK_READ,
    SVG_MARK_UNREAD,
    SVG_STAR_FILLED,
    SVG_STAR_EMPTY,
    SVG_EXTERNAL,
    SVG_QUEUE_ADD,
    SVG_QUEUE_REMOVE,
} from './icons.js';

describe('icons', () => {
    const icons = {
        SVG_MARK_READ,
        SVG_MARK_UNREAD,
        SVG_STAR_FILLED,
        SVG_STAR_EMPTY,
        SVG_EXTERNAL,
        SVG_QUEUE_ADD,
        SVG_QUEUE_REMOVE,
    };

    it('exports 7 icon constants', () => {
        expect(Object.keys(icons)).toHaveLength(7);
    });

    for (const [name, svg] of Object.entries(icons)) {
        describe(name, () => {
            it('is a non-empty string', () => {
                expect(typeof svg).toBe('string');
                expect(svg.length).toBeGreaterThan(0);
            });

            it('is valid SVG markup', () => {
                expect(svg).toMatch(/^<svg[\s>]/);
                expect(svg).toMatch(/<\/svg>$/);
            });

            it('has viewBox, width, and height attributes', () => {
                expect(svg).toContain('viewBox=');
                expect(svg).toContain('width=');
                expect(svg).toContain('height=');
            });

            it('contains a path element', () => {
                expect(svg).toContain('<path');
            });
        });
    }
});
