// Minimal service worker so browsers offer "Install app". It never caches anything
// (payments and balances must always be live); it only passes requests through.
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', e => e.waitUntil(self.clients.claim()));
self.addEventListener('fetch', () => {});
