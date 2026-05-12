// Stub methods not implemented by jsdom to suppress noisy warnings during tests.
window.scrollTo = () => {};

// Suppress jsdom "Not implemented" stderr warnings (e.g., navigation, scrollTo).
// jsdom emits these via its virtual console which writes directly to stderr.
const _origStderrWrite = process.stderr.write.bind(process.stderr);
process.stderr.write = (chunk, ...rest) => {
    if (typeof chunk === 'string' && chunk.includes('Not implemented:')) return true;
    return _origStderrWrite(chunk, ...rest);
};

// Silence console.debug noise from production modules during tests.
// stderr (warn/error) is preserved.
console.debug = () => {};
