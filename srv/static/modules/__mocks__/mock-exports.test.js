import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';

/**
 * Validates that each __mocks__/*.js file exports the same names as the real
 * module it shadows. This catches structural drift (missing/extra exports)
 * so tests don't silently exercise wrong mocks.
 */

const mocksDir = path.resolve(import.meta.dirname, '.');
const realDir = path.resolve(mocksDir, '..');

/** Extract exported names from a JS source file via regex. */
function getExportNames(source) {
    const names = new Set();

    // export function name(  /  export async function name(
    for (const m of source.matchAll(/export\s+(?:async\s+)?function\s+(\w+)/g)) {
        names.add(m[1]);
    }
    // export const name  /  export let name
    for (const m of source.matchAll(/export\s+(?:const|let|var)\s+(\w+)/g)) {
        names.add(m[1]);
    }
    // export { name } from '...'  /  export { a, b }
    for (const m of source.matchAll(/export\s*\{([^}]+)\}/g)) {
        for (const part of m[1].split(',')) {
            const token = part.trim().split(/\s+as\s+/).pop().trim();
            if (token) names.add(token);
        }
    }
    return names;
}

const mockFiles = fs.readdirSync(mocksDir)
    .filter(f => f.endsWith('.js') && !f.endsWith('.test.js'));

describe('__mocks__ export validation', () => {
    it.each(mockFiles)('%s exports match real module', (filename) => {
        const mockSource = fs.readFileSync(path.join(mocksDir, filename), 'utf-8');
        const realSource = fs.readFileSync(path.join(realDir, filename), 'utf-8');

        const mockExports = getExportNames(mockSource);
        const realExports = getExportNames(realSource);

        const missingFromMock = [...realExports].filter(n => !mockExports.has(n));
        const extraInMock = [...mockExports].filter(n => !realExports.has(n));

        expect(missingFromMock, `Mock ${filename} is missing exports: ${missingFromMock.join(', ')}`).toEqual([]);
        expect(extraInMock, `Mock ${filename} has extra exports: ${extraInMock.join(', ')}`).toEqual([]);
    });
});
