// Applies the saved theme/density before first paint. Loaded as a classic,
// render-blocking script from <head> because the CSP forbids inline scripts.
// Keep the keys and rules in sync with src/theme/theme.ts, which takes over
// after boot (and follows OS changes while "system" is selected).
(function () {
  var root = document.documentElement;
  var pref = 'system';
  var density = 'comfortable';
  try {
    pref = localStorage.getItem('nm-theme') || 'system';
    density = localStorage.getItem('nm-density') || 'comfortable';
  } catch (e) { /* storage blocked: use defaults */ }
  var light = pref === 'light' || (pref === 'system' && !!window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches);
  root.setAttribute('data-theme', light ? 'light' : 'dark');
  if (density === 'compact') root.setAttribute('data-density', 'compact');
})();
