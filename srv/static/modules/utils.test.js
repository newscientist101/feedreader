import { describe, it, expect, vi } from 'vitest';
import {
    PREVIEW_TEXT_LIMIT,
    formatTimeAgo,
    formatLocalDate,
    stripHtml,
    truncateText,
    getArticleSortTime,
} from './utils.js';

describe('utils', () => {
    describe('PREVIEW_TEXT_LIMIT', () => {
        it('is 500', () => {
            expect(PREVIEW_TEXT_LIMIT).toBe(500);
        });
    });

    describe('formatTimeAgo', () => {
        it('returns empty string for falsy input', () => {
            expect(formatTimeAgo('')).toBe('');
            expect(formatTimeAgo(null)).toBe('');
            expect(formatTimeAgo(undefined)).toBe('');
        });

        it('returns "just now" for very recent dates', () => {
            const now = new Date().toISOString();
            expect(formatTimeAgo(now)).toBe('just now');
        });

        it('returns minutes ago', () => {
            const date = new Date(Date.now() - 5 * 60000).toISOString();
            expect(formatTimeAgo(date)).toBe('5m ago');
        });

        it('returns hours ago', () => {
            const date = new Date(Date.now() - 3 * 3600000).toISOString();
            expect(formatTimeAgo(date)).toBe('3h ago');
        });

        it('returns days ago', () => {
            const date = new Date(Date.now() - 2 * 86400000).toISOString();
            expect(formatTimeAgo(date)).toBe('2d ago');
        });

        it('returns locale date for older dates', () => {
            const date = new Date(Date.now() - 14 * 86400000).toISOString();
            const result = formatTimeAgo(date);
            // Should be a locale date string, not "Xd ago"
            expect(result).not.toContain('d ago');
        });
    });

    describe('formatLocalDate', () => {
        it('returns a formatted date string', () => {
            const result = formatLocalDate('2024-01-15T10:30:00Z');
            expect(result).toContain('2024');
            expect(result).toContain('January');
        });
    });

    describe('stripHtml', () => {
        it('removes HTML tags', () => {
            expect(stripHtml('<p>Hello <b>world</b></p>')).toBe('Hello world');
        });

        it('handles empty input', () => {
            expect(stripHtml('')).toBe('');
        });
    });

    describe('truncateText', () => {
        it('returns text unchanged if shorter than max', () => {
            expect(truncateText('hello', 10)).toBe('hello');
        });

        it('truncates and adds ellipsis', () => {
            expect(truncateText('hello world', 5)).toBe('hello...');
        });

        it('handles null/undefined', () => {
            expect(truncateText(null, 5)).toBeNull();
            expect(truncateText(undefined, 5)).toBeUndefined();
        });
    });

    describe('getArticleSortTime', () => {
        it('prefers published_at', () => {
            expect(getArticleSortTime({
                published_at: '2024-01-01',
                fetched_at: '2024-02-01',
            })).toBe('2024-01-01');
        });

        it('falls back to fetched_at', () => {
            expect(getArticleSortTime({
                published_at: null,
                fetched_at: '2024-02-01',
            })).toBe('2024-02-01');
        });
    });
});
