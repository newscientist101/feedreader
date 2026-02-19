import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['srv/static/**/*.test.js'],
    reporters: ['default'],
  },
});
