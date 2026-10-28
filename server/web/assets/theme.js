/* Light / dark switch shared by every page. Loaded in <head> so the theme is set before first paint. */
(function () {
  var K = 'tp_theme', d = document.documentElement;
  function stored() { try { var v = localStorage.getItem(K); return v === 'dark' || v === 'light' ? v : null; } catch (e) { return null; } }
  function sys() { return window.matchMedia && matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'; }
  function apply(v) {
    d.setAttribute('data-theme', v);
    var m = document.querySelector('meta[name=theme-color]');
    if (m) m.setAttribute('content', v === 'dark' ? '#0C0D0C' : '#EFF0EE');
  }
  apply(stored() || sys());
  if (window.matchMedia) {
    try { matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function () { if (!stored()) apply(sys()); }); } catch (e) {}
  }
  window.tpTheme = {
    get: function () { return d.getAttribute('data-theme'); },
    toggle: function () {
      var v = d.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
      try { localStorage.setItem(K, v); } catch (e) {}
      apply(v);
    },
    // returns a ready-made round toggle button (sun in dark mode, moon in light mode)
    button: function (cls) {
      var b = document.createElement('button');
      b.type = 'button'; b.className = (cls || 'round') + ' theme-btn'; b.setAttribute('aria-label', 'Switch light / dark mode'); b.title = 'Light / dark';
      b.innerHTML = '<svg class="i-moon" viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1111.2 3a7 7 0 009.8 9.8z"/></svg>' +
        '<svg class="i-sun" viewBox="0 0 24 24" fill="none" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>';
      b.addEventListener('click', window.tpTheme.toggle);
      return b;
    }
  };
})();
