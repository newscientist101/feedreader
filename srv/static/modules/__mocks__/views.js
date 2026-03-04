import { vi } from 'vitest';
export const getViewScope = vi.fn(() => '');
export const setView = vi.fn();
export const migrateLegacyViewDefaults = vi.fn();
export const getDefaultViewForScope = vi.fn(() => 'card');
export const applyDefaultViewForScope = vi.fn();
export const initView = vi.fn();
export const initViewListeners = vi.fn();
