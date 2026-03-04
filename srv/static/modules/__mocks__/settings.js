import { vi } from 'vitest';
export const getSetting = vi.fn((key, defaultValue) => defaultValue);
export const saveSetting = vi.fn();
export const applyHideReadArticles = vi.fn();
export const applyHideEmptyFeeds = vi.fn();
