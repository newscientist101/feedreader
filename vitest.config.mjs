import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['srv/static/**/*.test.js'],
    reporters: ['default'],
    coverage: {
      provider: 'v8',
      include: ['srv/static/**/*.js'],
      exclude: ['srv/static/**/*.test.js', 'srv/static/test-helper.js', 'srv/static/sw.js'],
      reporter: ['text', 'text-summary'],
    },
  },
});
