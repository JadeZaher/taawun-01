(() => {
  'use strict';

  const nav = document.querySelector('.nav-shell');
  const menuToggle = document.getElementById('menuToggle');
  const navigation = document.getElementById('primaryNavigation');
  if (nav && menuToggle && navigation) {
    const closeMenu = ({ restoreFocus = false } = {}) => {
      nav.removeAttribute('data-menu-open');
      menuToggle.setAttribute('aria-expanded', 'false');
      if (restoreFocus) menuToggle.focus();
    };
    const openMenu = () => {
      nav.dataset.menuOpen = 'true';
      menuToggle.setAttribute('aria-expanded', 'true');
      navigation.querySelector('a')?.focus();
    };
    nav.dataset.menuReady = 'true';
    menuToggle.hidden = false;
    menuToggle.addEventListener('click', () => {
      if (menuToggle.getAttribute('aria-expanded') === 'true') closeMenu();
      else openMenu();
    });
    navigation.addEventListener('click', (event) => {
      if (event.target.closest('a')) closeMenu({ restoreFocus: true });
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && menuToggle.getAttribute('aria-expanded') === 'true') closeMenu({ restoreFocus: true });
    });
    window.addEventListener('resize', () => {
      if (window.innerWidth > 840) closeMenu();
    }, { passive: true });
  }

  const stage = document.querySelector('.geometry-stage');
  const canvas = document.getElementById('geometryCanvas');
  const toggle = document.getElementById('motionToggle');
  if (!stage || !canvas || !toggle) return;

  const sections = [...document.querySelectorAll('[data-geometry-state]')];
  let sceneFrame = 0;
  let staticMotionPaused = false;
  const updateStaticScene = () => {
    sceneFrame = 0;
    if (!sections.length || staticMotionPaused) return;
    const anchor = window.scrollY + window.innerHeight * 0.68;
    const centers = sections.map((section) => section.offsetTop + section.offsetHeight * 0.5);
    let index = centers.findIndex((center) => center > anchor);
    if (index < 0) index = centers.length;
    const nextIndex = Math.min(Math.max(index, 1), centers.length - 1);
    const currentIndex = Math.max(0, nextIndex - 1);
    const span = Math.max(1, centers[nextIndex] - centers[currentIndex]);
    const progress = currentIndex === nextIndex ? 0 : Math.min(1, Math.max(0, (anchor - centers[currentIndex]) / span));
    const selected = progress < 0.5 ? currentIndex : nextIndex;
    const section = sections[selected];
    stage.dataset.state = section.dataset.geometryState || String(selected);
    stage.dataset.side = section.dataset.geometrySide === 'left' ? 'left' : 'right';
    if (progress > 0.12 && progress < 0.88) stage.dataset.transition = 'true';
    else stage.removeAttribute('data-transition');
  };
  const scheduleStaticScene = () => {
    if (!sceneFrame) sceneFrame = window.requestAnimationFrame(updateStaticScene);
  };
  updateStaticScene();
  window.addEventListener('scroll', scheduleStaticScene, { passive: true });

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
  const motionControlEligible = () => !reducedMotion.matches && !forcedColors.matches && !increasedContrast.matches && !reducedTransparency.matches;
  const capable = () => environmentEligible() && !userPaused();
  const setToggleState = (active) => {
    staticMotionPaused = !active;
    toggle.setAttribute('aria-pressed', String(active));
    stage.dataset.motion = active ? 'running' : 'paused';
    if (!active) stage.removeAttribute('data-transition');
  };

  const stop = ({ motionActive = motionControlEligible() && !userPaused() } = {}) => {
    renderer?.destroy();
    renderer = null;
    stage.removeAttribute('data-enhanced');
    toggle.hidden = !motionControlEligible();
    setToggleState(motionActive);
    toggle.disabled = false;
  };

  const rendererFailed = () => {
    stop();
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
        stop();
        return;
      }
      renderer = candidate;
      toggle.hidden = !motionControlEligible();
      setToggleState(true);
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

  toggle.addEventListener('click', () => {
    if (toggle.getAttribute('aria-pressed') === 'true') {
      setUserPaused(true);
      stop({ motionActive: false });
      return;
    }
    setUserPaused(false);
    setToggleState(true);
    toggle.disabled = true;
    updateStaticScene();
    if (environmentEligible()) schedule();
    else toggle.disabled = false;
  });

  const reconcilePreference = () => {
    if (!motionControlEligible()) stop({ motionActive: false });
    else if (userPaused()) stop({ motionActive: false });
    else if (!environmentEligible()) stop({ motionActive: true });
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
      updateStaticScene();
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
  sections.forEach((section) => observer.observe(section));
  if (!pageLoaded) window.addEventListener('load', () => { pageLoaded = true; reconcilePreference(); }, { once: true });

  window.addEventListener('pagehide', () => {
    if (idleHandle) {
      if ('cancelIdleCallback' in window) window.cancelIdleCallback(idleHandle); else window.clearTimeout(idleHandle);
    }
    if (resizeFrame) window.cancelAnimationFrame(resizeFrame);
    if (sceneFrame) window.cancelAnimationFrame(sceneFrame);
    observer.disconnect();
    stop();
  }, { once: true });
})();
