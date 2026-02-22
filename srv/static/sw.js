// Service Worker for FeedReader PWA
// Supports offline queue reading when installed as a PWA.

const STATIC_CACHE = 'fr-static-v1';
const QUEUE_CACHE = 'fr-queue-v1';

let offlineEnabled = false;
let pendingActions = [];

self.addEventListener('install', () => self.skipWaiting());

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((names) =>
      Promise.all(
        names
          .filter((n) => n !== STATIC_CACHE && n !== QUEUE_CACHE)
          .map((n) => caches.delete(n))
      )
    )
  );
  self.clients.claim();
});

// Listen for messages from the app
self.addEventListener('message', (event) => {
  const { type, data } = event.data || {};

  switch (type) {
    case 'ENABLE_OFFLINE':
      offlineEnabled = true;
      // Pre-cache static assets
      cacheStaticAssets(data?.staticUrls || []);
      if (event.source) {
        event.source.postMessage({ type: 'OFFLINE_ENABLED' });
      }
      break;

    case 'CACHE_QUEUE':
      cacheQueueData(data?.articles || []);
      break;

    case 'GET_PENDING_ACTIONS':
      if (event.source) {
        event.source.postMessage({
          type: 'PENDING_ACTIONS',
          actions: pendingActions.splice(0),
        });
      }
      break;

    case 'OFFLINE_DEQUEUE':
      if (data?.articleId) {
        pendingActions.push({
          type: 'dequeue',
          articleId: data.articleId,
          timestamp: Date.now(),
        });
      }
      break;
  }
});

async function cacheStaticAssets(urls) {
  try {
    const cache = await caches.open(STATIC_CACHE);
    // Cache each provided URL
    for (const url of urls) {
      try {
        const resp = await fetch(url);
        if (resp.ok) {
          await cache.put(url, resp);
        }
      } catch {
        // ignore individual failures
      }
    }
  } catch {
    // ignore
  }
}

async function cacheQueueData(articles) {
  try {
    const cache = await caches.open(QUEUE_CACHE);
    const resp = new Response(JSON.stringify(articles), {
      headers: { 'Content-Type': 'application/json' },
    });
    await cache.put('/api/queue', resp);
  } catch {
    // ignore
  }
}

// Fetch handler
self.addEventListener('fetch', (event) => {
  if (!offlineEnabled) return; // Pass through when not enabled

  const { request } = event;
  const url = new URL(request.url);

  // Only handle same-origin requests
  if (url.origin !== self.location.origin) return;

  // Static assets: network-first with cache fallback
  if (url.pathname.startsWith('/static/')) {
    event.respondWith(networkFirstStatic(request));
    return;
  }

  // Queue API: network-first with cache fallback
  if (url.pathname === '/api/queue' && request.method === 'GET') {
    event.respondWith(networkFirstQueue(request));
    return;
  }

  // Offline dequeue actions: store for later replay
  if (
    request.method === 'DELETE' &&
    url.pathname.match(/^\/api\/articles\/\d+\/queue$/)
  ) {
    event.respondWith(handleOfflineDequeue(request, url));
    return;
  }

  // Navigation requests when offline
  if (request.mode === 'navigate') {
    event.respondWith(handleNavigation(request, url));
    return;
  }
});

async function networkFirstStatic(request) {
  try {
    const resp = await fetch(request);
    if (resp.ok) {
      const cache = await caches.open(STATIC_CACHE);
      cache.put(request, resp.clone());
    }
    return resp;
  } catch {
    const cached = await findCachedStatic(request);
    if (cached) return cached;
    return new Response('', { status: 503 });
  }
}

async function findCachedStatic(request) {
  const cache = await caches.open(STATIC_CACHE);
  // Direct match first
  const direct = await cache.match(request);
  if (direct) return direct;
  // Try matching by pathname prefix (ignore query string/hash differences)
  const url = new URL(request.url);
  const keys = await cache.keys();
  for (const key of keys) {
    const keyUrl = new URL(key.url);
    if (keyUrl.pathname === url.pathname) {
      return cache.match(key);
    }
  }
  return null;
}

async function networkFirstQueue(request) {
  try {
    const resp = await fetch(request);
    if (resp.ok) {
      const cache = await caches.open(QUEUE_CACHE);
      cache.put('/api/queue', resp.clone());
    }
    return resp;
  } catch {
    const cache = await caches.open(QUEUE_CACHE);
    const cached = await cache.match('/api/queue');
    if (cached) return cached;
    return new Response('[]', {
      headers: { 'Content-Type': 'application/json' },
    });
  }
}

async function handleOfflineDequeue(request, url) {
  try {
    return await fetch(request);
  } catch {
    // Offline: store action for replay
    const match = url.pathname.match(/^\/api\/articles\/(\d+)\/queue$/);
    if (match) {
      pendingActions.push({
        type: 'dequeue',
        articleId: Number(match[1]),
        timestamp: Date.now(),
      });
    }
    return new Response(JSON.stringify({ status: 'ok' }), {
      headers: { 'Content-Type': 'application/json' },
    });
  }
}

async function handleNavigation(request, url) {
  // Queue page: serve offline version if network fails
  if (url.pathname === '/queue') {
    try {
      return await fetch(request);
    } catch {
      return buildOfflineQueuePage();
    }
  }

  // All other pages: try network, fall back to offline notice
  try {
    return await fetch(request);
  } catch {
    return buildOfflineFallbackPage();
  }
}

async function getQueueArticles() {
  try {
    const cache = await caches.open(QUEUE_CACHE);
    const resp = await cache.match('/api/queue');
    if (resp) {
      return await resp.json();
    }
  } catch {
    // ignore
  }
  return [];
}

async function getCssUrl() {
  const cache = await caches.open(STATIC_CACHE);
  const keys = await cache.keys();
  for (const key of keys) {
    const keyUrl = new URL(key.url);
    if (keyUrl.pathname === '/static/style.css') {
      return key.url;
    }
  }
  return '/static/style.css';
}

async function buildOfflineQueuePage() {
  const articles = await getQueueArticles();
  const cssUrl = await getCssUrl();

  const articlesJson = JSON.stringify(articles)
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e');

  const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Queue (Offline) — FeedReader</title>
  <link rel="stylesheet" href="${cssUrl}">
  <style>
    .offline-banner {
      background: #e67e22;
      color: #fff;
      text-align: center;
      padding: 8px 16px;
      font-size: 14px;
      font-weight: 500;
      position: sticky;
      top: 0;
      z-index: 1000;
    }
    .offline-banner svg { vertical-align: middle; margin-right: 6px; }
    .offline-queue-app {
      max-width: 800px;
      margin: 0 auto;
      padding: 0 16px;
    }
    .offline-queue-app .article-body img { max-width: 100%; height: auto; }
    .offline-queue-app .article-body iframe,
    .offline-queue-app .article-body video,
    .offline-queue-app .article-body object,
    .offline-queue-app .article-body embed { display: none; }
    .offline-empty {
      text-align: center;
      padding: 80px 20px;
      color: var(--text-secondary, #999);
    }
    .offline-empty svg { opacity: 0.4; margin-bottom: 16px; }
    .offline-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 16px 0;
      border-bottom: 1px solid var(--border-color, #333);
      margin-bottom: 16px;
    }
    .offline-header .logo {
      display: flex;
      align-items: center;
      gap: 8px;
      font-weight: 600;
      font-size: 18px;
      color: var(--text-primary, #eee);
    }
    .offline-meta {
      display: flex;
      gap: 12px;
      align-items: center;
      flex-wrap: wrap;
      margin-bottom: 12px;
      font-size: 14px;
      color: var(--text-secondary, #999);
    }
    .offline-meta .feed-name {
      color: var(--accent-color, #6c5ce7);
      font-weight: 500;
    }
    .offline-queue-actions {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 12px 0;
      margin-bottom: 8px;
    }
    .offline-queue-actions .queue-counter {
      color: var(--text-secondary, #999);
      font-size: 14px;
    }
    .queue-next-btn {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      background: var(--accent-color, #6c5ce7);
      color: #fff;
      border: none;
      padding: 8px 20px;
      border-radius: 6px;
      font-size: 15px;
      font-weight: 500;
      cursor: pointer;
    }
    .queue-next-btn:hover { opacity: 0.9; }
  </style>
</head>
<body>
  <div class="offline-banner">
    <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
      <path d="M19.35 10.04C18.67 6.59 15.64 4 12 4c-1.48 0-2.85.43-4.01 1.17l1.46 1.46C10.21 6.23 11.08 6 12 6c3.04 0 5.5 2.46 5.5 5.5v.5H19c1.66 0 3 1.34 3 3 0 .99-.49 1.87-1.24 2.41l1.46 1.46C23.33 17.98 24 16.58 24 15c0-2.64-2.05-4.78-4.65-4.96zM3 5.27l2.75 2.74C2.56 8.15 0 10.77 0 14c0 3.31 2.69 6 6 6h11.73l2 2 1.27-1.27L4.27 4 3 5.27zM7.73 10l8 8H6c-2.21 0-4-1.79-4-4s1.79-4 4-4h1.73z"/>
    </svg>
    You're offline — reading from cached queue
  </div>
  <div class="offline-queue-app">
    <div class="offline-header">
      <div class="logo">
        <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
          <path d="M6.18 15.64a2.18 2.18 0 0 1 2.18 2.18C8.36 19 7.38 20 6.18 20C5 20 4 19 4 17.82a2.18 2.18 0 0 1 2.18-2.18M4 4.44A15.56 15.56 0 0 1 19.56 20h-2.83A12.73 12.73 0 0 0 4 7.27V4.44m0 5.66a9.9 9.9 0 0 1 9.9 9.9h-2.83A7.07 7.07 0 0 0 4 12.93V10.1Z"/>
        </svg>
        FeedReader
      </div>
    </div>
    <div id="queue-content"></div>
  </div>
  <script>
    const articles = ${articlesJson};
    let currentIndex = 0;
    const pendingDequeues = [];

    function escapeHtml(str) {
      if (!str) return '';
      const d = document.createElement('div');
      d.textContent = str;
      return d.innerHTML;
    }

    function timeAgo(dateStr) {
      if (!dateStr) return '';
      const d = new Date(dateStr);
      const now = new Date();
      const diffMs = now - d;
      const mins = Math.floor(diffMs / 60000);
      if (mins < 1) return 'just now';
      if (mins < 60) return mins + 'm ago';
      const hours = Math.floor(mins / 60);
      if (hours < 24) return hours + 'h ago';
      const days = Math.floor(hours / 24);
      if (days < 30) return days + 'd ago';
      return d.toLocaleDateString();
    }

    function render() {
      const el = document.getElementById('queue-content');
      if (articles.length === 0 || currentIndex >= articles.length) {
        el.innerHTML = '<div class="offline-empty">' +
          '<svg viewBox="0 0 24 24" width="64" height="64" fill="currentColor">' +
          '<path d="M4 6H2v14c0 1.1.9 2 2 2h14v-2H4V6zm16-4H8c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H8V4h12v12z"/>' +
          '</svg>' +
          '<h2>Queue is empty</h2>' +
          '<p>Your queued articles will appear here when you go back online.</p>' +
          '</div>';
        return;
      }

      const a = articles[currentIndex];
      const content = a.content || a.summary || '<p>No content available.</p>';

      el.innerHTML =
        '<div class="offline-queue-actions">' +
          '<span class="queue-counter">' + (currentIndex + 1) + ' of ' + articles.length + '</span>' +
          '<button class="queue-next-btn" onclick="nextArticle()">' +
            'Next ' +
            '<svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">' +
            '<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z"/>' +
            '</svg>' +
          '</button>' +
        '</div>' +
        '<div class="offline-meta">' +
          (a.feed_name ? '<span class="feed-name">' + escapeHtml(a.feed_name) + '</span>' : '') +
          (a.author ? '<span>by ' + escapeHtml(a.author) + '</span>' : '') +
          '<span>' + timeAgo(a.published_at) + '</span>' +
        '</div>' +
        '<h1>' + escapeHtml(a.title) + '</h1>' +
        '<div class="article-body">' + content + '</div>';

      window.scrollTo(0, 0);
    }

    function nextArticle() {
      if (currentIndex < articles.length) {
        const a = articles[currentIndex];
        pendingDequeues.push(a.id);
        // Tell SW about pending dequeue
        if (navigator.serviceWorker && navigator.serviceWorker.controller) {
          navigator.serviceWorker.controller.postMessage({
            type: 'OFFLINE_DEQUEUE',
            data: { articleId: a.id }
          });
        }
        currentIndex++;
        render();
      }
    }

    render();
  </script>
</body>
</html>`;

  return new Response(html, {
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
  });
}

function buildOfflineFallbackPage() {
  const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Offline — FeedReader</title>
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
      background: #1a1a2e;
      color: #e0e0e0;
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      margin: 0;
    }
    .offline-page {
      text-align: center;
      padding: 40px;
    }
    .offline-page svg { opacity: 0.5; margin-bottom: 20px; }
    .offline-page h1 { font-size: 24px; margin-bottom: 12px; }
    .offline-page p { color: #999; margin-bottom: 24px; }
    .offline-page a {
      display: inline-block;
      background: #6c5ce7;
      color: #fff;
      padding: 12px 28px;
      border-radius: 8px;
      text-decoration: none;
      font-weight: 500;
    }
    .offline-page a:hover { opacity: 0.9; }
  </style>
</head>
<body>
  <div class="offline-page">
    <svg viewBox="0 0 24 24" width="64" height="64" fill="currentColor">
      <path d="M19.35 10.04C18.67 6.59 15.64 4 12 4c-1.48 0-2.85.43-4.01 1.17l1.46 1.46C10.21 6.23 11.08 6 12 6c3.04 0 5.5 2.46 5.5 5.5v.5H19c1.66 0 3 1.34 3 3 0 .99-.49 1.87-1.24 2.41l1.46 1.46C23.33 17.98 24 16.58 24 15c0-2.64-2.05-4.78-4.65-4.96zM3 5.27l2.75 2.74C2.56 8.15 0 10.77 0 14c0 3.31 2.69 6 6 6h11.73l2 2 1.27-1.27L4.27 4 3 5.27zM7.73 10l8 8H6c-2.21 0-4-1.79-4-4s1.79-4 4-4h1.73z"/>
    </svg>
    <h1>You're offline</h1>
    <p>This page isn't available offline, but your reading queue is.</p>
    <a href="/queue">Go to Queue</a>
  </div>
</body>
</html>`;

  return new Response(html, {
    status: 200,
    headers: { 'Content-Type': 'text/html; charset=utf-8' },
  });
}
