(() => {
  'use strict';

  const stage = document.querySelector('.geometry-stage');
  const canvas = document.getElementById('geometryCanvas');
  const control = document.getElementById('motionControl');
  const toggle = document.getElementById('motionToggle');
  if (!stage || !canvas || !control || !toggle) return;

  const query = (value) => window.matchMedia(value);
  const reducedMotion = query('(prefers-reduced-motion: reduce)');
  const forcedColors = query('(forced-colors: active)');
  const increasedContrast = query('(prefers-contrast: more)');
  const reducedTransparency = query('(prefers-reduced-transparency: reduce)');
  const connection = navigator.connection;
  let renderer = null;
  let idleHandle = 0;
  let resizeFrame = 0;
  let sceneVisible = false;
  let initializing = false;
  let pageLoaded = document.readyState === 'complete';

  const userPaused = () => {
    try { return localStorage.getItem('taawun-decorative-motion') === 'off'; } catch { return false; }
  };
  const setUserPaused = (paused) => {
    try {
      if (paused) localStorage.setItem('taawun-decorative-motion', 'off');
      else localStorage.removeItem('taawun-decorative-motion');
    } catch { /* The visible control remains authoritative for this page. */ }
  };
  const environmentEligible = () => !reducedMotion.matches && !forcedColors.matches && !increasedContrast.matches && !reducedTransparency.matches &&
    !Boolean(connection && connection.saveData) && window.innerWidth > 840 &&
    (!navigator.deviceMemory || navigator.deviceMemory > 4) && (!navigator.hardwareConcurrency || navigator.hardwareConcurrency > 4) &&
    Boolean(navigator.gpu) && sceneVisible;
  const capable = () => environmentEligible() && !userPaused();

  const stop = ({ keepControl = false } = {}) => {
    renderer?.destroy();
    renderer = null;
    stage.removeAttribute('data-enhanced');
    control.hidden = !keepControl;
    toggle.checked = false;
    toggle.disabled = false;
  };

  const rendererFailed = () => {
    renderer = null;
    stage.removeAttribute('data-enhanced');
    control.hidden = true;
    toggle.checked = false;
    toggle.disabled = false;
  };

  const initialize = async () => {
    if (!capable() || document.hidden || renderer || initializing) return;
    initializing = true;
    toggle.disabled = true;
    try {
      const module = await import('/geometric-renderer.js');
      if (!capable() || document.hidden) return;
      const candidate = await module.createGeometricRenderer({ canvas, stage, onFailure: rendererFailed });
      if (!candidate || !candidate.active() || !capable() || document.hidden) {
        candidate?.destroy();
        return;
      }
      renderer = candidate;
      control.hidden = false;
      toggle.checked = true;
    } catch {
      rendererFailed();
    } finally {
      initializing = false;
      toggle.disabled = false;
    }
  };

  const schedule = () => {
    if (!pageLoaded || !capable() || idleHandle || initializing) return;
    const run = () => { idleHandle = 0; initialize(); };
    if ('requestIdleCallback' in window) idleHandle = window.requestIdleCallback(run, { timeout: 1500 });
    else idleHandle = window.setTimeout(run, 250);
  };

  toggle.addEventListener('change', () => {
    if (!toggle.checked) {
      setUserPaused(true);
      stop({ keepControl: environmentEligible() });
      return;
    }
    setUserPaused(false);
    control.hidden = true;
    schedule();
  });

  const reconcilePreference = () => {
    if (!environmentEligible()) stop();
    else if (userPaused()) stop({ keepControl: true });
    else schedule();
  };
  reducedMotion.addEventListener?.('change', reconcilePreference);
  forcedColors.addEventListener?.('change', reconcilePreference);
  increasedContrast.addEventListener?.('change', reconcilePreference);
  reducedTransparency.addEventListener?.('change', reconcilePreference);
  connection?.addEventListener?.('change', reconcilePreference);
  document.addEventListener('visibilitychange', () => { if (document.hidden) renderer?.pause(); else if (renderer) renderer.resume(); else reconcilePreference(); });
  window.addEventListener('resize', () => {
    if (resizeFrame) return;
    resizeFrame = window.requestAnimationFrame(() => {
      resizeFrame = 0;
      if (!environmentEligible()) stop();
      else if (renderer) renderer.resize();
      else reconcilePreference();
    });
  }, { passive: true });

  const observer = new IntersectionObserver((entries) => {
    sceneVisible = entries.some((entry) => entry.isIntersecting) || [...document.querySelectorAll('[data-geometry-state]')].some((section) => {
      const rect = section.getBoundingClientRect();
      return rect.bottom > 0 && rect.top < window.innerHeight;
    });
    if (!sceneVisible) renderer?.pause();
    else if (renderer) renderer.resume();
    else reconcilePreference();
  }, { rootMargin: '80px 0px', threshold: 0 });
  document.querySelectorAll('[data-geometry-state]').forEach((section) => observer.observe(section));
  if (!pageLoaded) window.addEventListener('load', () => { pageLoaded = true; reconcilePreference(); }, { once: true });

  window.addEventListener('pagehide', () => {
    if (idleHandle) {
      if ('cancelIdleCallback' in window) window.cancelIdleCallback(idleHandle); else window.clearTimeout(idleHandle);
    }
    if (resizeFrame) window.cancelAnimationFrame(resizeFrame);
    observer.disconnect();
    stop();
  }, { once: true });
})();
