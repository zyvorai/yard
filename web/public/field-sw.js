const CACHE = "yard-shell-v1";

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (url.origin !== self.location.origin || url.pathname.startsWith("/api/")) return;
  event.respondWith((async () => {
    try {
      const res = await fetch(event.request);
      if (res.ok && event.request.method === "GET") {
        const cache = await caches.open(CACHE);
        await cache.put(event.request, res.clone());
      }
      return res;
    } catch {
      const cache = await caches.open(CACHE);
      const hit = await cache.match(event.request);
      if (hit) return hit;
      if (event.request.mode === "navigate") {
        const shell = await cache.match("/index.html");
        if (shell) return shell;
      }
      throw new Error("offline");
    }
  })());
});
