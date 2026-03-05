import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'url';
import path from 'path';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

export default defineConfig({
  test: {
    root,
    environment: 'happy-dom',
    setupFiles: ['./tests/config/vitest.setup.mjs'],
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
