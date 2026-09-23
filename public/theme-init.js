// Applied synchronously before first paint to avoid a flash of the wrong
// theme. Kept as an external file (not `is:inline`) so it's served
// same-origin and can pass a strict `script-src 'self'` CSP (see
// nginx.conf) — inline scripts would otherwise be blocked without
// 'unsafe-inline' or a hash/nonce.
(function () {
  const stored = localStorage.getItem('theme');
  let theme;
  if (stored === 'light' || stored === 'dark') {
    theme = stored;
  } else {
    // system default — respect OS preference
    theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  document.documentElement.dataset.theme = theme;
})();
