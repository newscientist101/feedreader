import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'happy-dom',
    setupFiles: ['./vitest.setup.mjs'],
    include: ['srv/static/**/*.test.js'],
    exclude: ['srv/static/**/*.browser.test.js'],
    reporters: ['default'],
    coverage: {
      provider: 'v8',
      include: ['srv/static/**/*.js'],
      exclude: ['srv/static/**/*.test.js', 'srv/static/sw.js'],
      reporter: ['text', 'text-summary'],
    },
  },
});
