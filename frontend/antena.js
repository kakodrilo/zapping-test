// ── ANTENA · shared client-side helpers ──────────────────────────
// Loaded by every page. Handles theme bootstrapping (so the right
// background paints on the FIRST frame, no flash) plus token + small
// utilities.

(function () {
  // Apply theme attribute as early as possible — before stylesheets paint.
  // Defaults to oscuro if nothing is stored.
  try {
    const saved = localStorage.getItem('antena-theme');
    const theme = saved === 'claro' || saved === 'oscuro' ? saved : 'oscuro';
    document.documentElement.setAttribute('data-theme', theme);
  } catch (_) {
    document.documentElement.setAttribute('data-theme', 'oscuro');
  }
})();

// Public utilities (used by per-page scripts).
window.AntenaTheme = {
  current() { return document.documentElement.getAttribute('data-theme') || 'oscuro'; },
  set(name) {
    const t = name === 'claro' ? 'claro' : 'oscuro';
    document.documentElement.setAttribute('data-theme', t);
    try { localStorage.setItem('antena-theme', t); } catch (_) {}
  },
  toggle() {
    const next = this.current() === 'oscuro' ? 'claro' : 'oscuro';
    this.set(next);
    return next;
  },
};

// Tiny DOM helper.
window.$ = (id) => document.getElementById(id);

// HTML-escape for safe injection of API-supplied strings.
window.escapeHtml = (s) =>
  String(s).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));

// Fire-and-forget toast helper.
window.showToast = (msg) => {
  document.querySelectorAll('.toast').forEach((el) => el.remove());
  const el = document.createElement('div');
  el.className = 'toast';
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 1400);
};
