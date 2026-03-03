import { describe, it, expect, vi } from 'vitest';
import {
    PREVIEW_TEXT_LIMIT,
    formatTimeAgo,
    formatLocalDate,
    stripHtml,
    truncateText,
    getArticleSortTime,
    escapeHtml,
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

        it('returns "1m ago" at exactly 1 minute', () => {
            vi.useFakeTimers({ now: new Date('2024-06-15T12:01:00Z') });
            expect(formatTimeAgo('2024-06-15T12:00:00Z')).toBe('1m ago');
            vi.useRealTimers();
        });

        it('returns "1h ago" at exactly 1 hour', () => {
            vi.useFakeTimers({ now: new Date('2024-06-15T13:00:00Z') });
            expect(formatTimeAgo('2024-06-15T12:00:00Z')).toBe('1h ago');
            vi.useRealTimers();
        });

        it('returns "1d ago" at exactly 1 day', () => {
            vi.useFakeTimers({ now: new Date('2024-06-16T12:00:00Z') });
            expect(formatTimeAgo('2024-06-15T12:00:00Z')).toBe('1d ago');
            vi.useRealTimers();
        });

        it('returns locale date at exactly 7 days', () => {
            vi.useFakeTimers({ now: new Date('2024-06-22T12:00:00Z') });
            const result = formatTimeAgo('2024-06-15T12:00:00Z');
            expect(result).not.toContain('d ago');
            expect(result).not.toContain('h ago');
            expect(result).not.toContain('m ago');
            vi.useRealTimers();
        });

        it('handles invalid date string', () => {
            const result = formatTimeAgo('not-a-date');
            // Invalid Date produces NaN diffs, all comparisons false, falls through to toLocaleDateString
            expect(result).toBe('Invalid Date');
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

        it('strips nested and mixed tags', () => {
            expect(stripHtml('<div><p>A <em>B <strong>C</strong></em></p></div>')).toBe('A B C');
        });

        it('handles HTML entities', () => {
            expect(stripHtml('&amp; &lt; &gt;')).toBe('& < >');
        });

        it('handles plain text with no tags', () => {
            expect(stripHtml('just plain text')).toBe('just plain text');
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

        it('returns text unchanged when length equals maxLen', () => {
            expect(truncateText('hello', 5)).toBe('hello');
        });

        it('truncates at maxLen of 0', () => {
            expect(truncateText('hello', 0)).toBe('...');
        });

        it('handles empty string', () => {
            expect(truncateText('', 5)).toBe('');
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

        it('falls back to fetched_at when published_at is empty string', () => {
            expect(getArticleSortTime({
                published_at: '',
                fetched_at: '2024-02-01',
            })).toBe('2024-02-01');
        });

        it('returns undefined when both are missing', () => {
            expect(getArticleSortTime({})).toBeUndefined();
        });
    });

    describe('escapeHtml', () => {
        it('escapes ampersands', () => {
            expect(escapeHtml('A&B')).toBe('A&amp;B');
        });

        it('escapes angle brackets', () => {
            expect(escapeHtml('<script>alert(1)</script>')).toBe('&lt;script&gt;alert(1)&lt;/script&gt;');
        });

        it('escapes double quotes', () => {
            expect(escapeHtml('a"b')).toBe('a&quot;b');
        });

        it('escapes single quotes', () => {
            expect(escapeHtml("a'b")).toBe('a&#39;b');
        });

        it('returns empty string for falsy input', () => {
            expect(escapeHtml('')).toBe('');
            expect(escapeHtml(null)).toBe('');
            expect(escapeHtml(undefined)).toBe('');
        });

        it('handles strings with no special characters', () => {
            expect(escapeHtml('hello world')).toBe('hello world');
        });

        it('handles multiple special characters', () => {
            expect(escapeHtml('<img onerror="alert(\'xss\')">')).toBe('&lt;img onerror=&quot;alert(&#39;xss&#39;)&quot;&gt;');
        });

        it('coerces non-string input to string', () => {
            expect(escapeHtml(42)).toBe('42');
            expect(escapeHtml(true)).toBe('true');
        });

        it('handles string with only special characters', () => {
            expect(escapeHtml('<>&"\'')).toBe('&lt;&gt;&amp;&quot;&#39;');
        });
    });
});
