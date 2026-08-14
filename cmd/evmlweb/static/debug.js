// Optional debug aid — NOT app logic. Logs Datastar fetch/actions and lifecycle
// events to the browser console for correlation with server session logs.
// Enabled only when the page URL contains ?debug=1 (or window.EVML_DEBUG = true).
(() => {
  const params = new URLSearchParams(window.location.search);
  if (!params.has("debug") && window.EVML_DEBUG !== true) return;

  const tag = "[evmlweb:debug]";
  const log = (...args) => console.debug(tag, ...args);

  const origFetch = window.fetch;
  window.fetch = async (input, init) => {
    const method = (init && init.method) || "GET";
    const url = typeof input === "string" ? input : input.url;
    const start = performance.now();
    let body;
    if (init && init.body) {
      try {
        body = typeof init.body === "string" ? JSON.parse(init.body) : String(init.body);
      } catch {
        body = init.body;
      }
    }
    log("fetch ->", method, url, body);
    try {
      const resp = await origFetch(input, init);
      log("fetch <-", method, url, resp.status, `${(performance.now() - start).toFixed(0)}ms`);
      return resp;
    } catch (err) {
      log("fetch ERROR", method, url, err);
      throw err;
    }
  };

  for (const evt of ["datastar-ready", "datastar-fetch", "datastar-signal-patch", "datastar-prop-change"]) {
    document.addEventListener(evt, (e) => log(evt, e.detail));
  }

  log("client debug logging enabled");
})();
