// Minimal service worker for PWA installability.
// No caching — all requests go straight to the network.

self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', (event) => {
  // Clear any caches left over from previous versions
  event.waitUntil(
    caches.keys().then((names) =>
      Promise.all(names.map((name) => caches.delete(name)))
    )
  );
  self.clients.claim();
});
