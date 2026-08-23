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
  const interactionLayer = stage?.querySelector('.geometry-interaction');
  if (!stage || !canvas || !toggle) return;

  const sections = [...document.querySelectorAll('[data-geometry-state]')];
  const landingMain = document.querySelector('main');
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
    stage.removeAttribute('data-transition');
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
  let canvasRecoveryFrame = 0;
  let sceneVisible = false;
  let initializing = false;
  let pageLoaded = document.readyState === 'complete';
  let disposed = false;

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
  const interaction = {
    x: window.innerWidth * 0.5,
    y: window.innerHeight * 0.5,
    targetX: window.innerWidth * 0.5,
    targetY: window.innerHeight * 0.5,
    strength: 0,
    targetStrength: 0,
    phase: 0,
    lastTime: 0,
  };
  let interactionFrame = 0;
  const interactionAllowed = () => Boolean(interactionLayer) && motionControlEligible() && !userPaused() && !staticMotionPaused && !document.hidden;
  const renderInteraction = () => {
    const strength = Math.min(1, Math.max(0, interaction.strength));
    stage.style.setProperty('--geometry-interaction-x', `${interaction.x.toFixed(2)}px`);
    stage.style.setProperty('--geometry-interaction-y', `${interaction.y.toFixed(2)}px`);
    stage.style.setProperty('--geometry-interaction-opacity', (strength * 0.2).toFixed(5));
    stage.style.setProperty('--geometry-interaction-shift-x', `${(Math.sin(interaction.phase) * strength * 8).toFixed(3)}px`);
    stage.style.setProperty('--geometry-interaction-shift-y', `${(Math.cos(interaction.phase * 0.82) * strength * 6).toFixed(3)}px`);
    stage.style.setProperty('--geometry-interaction-scale', (1 + strength * 0.028).toFixed(5));
    stage.style.setProperty('--geometry-interaction-turn', `${(Math.sin(interaction.phase * 0.76) * strength * 3.8).toFixed(3)}deg`);
    stage.style.setProperty('--geometry-interaction-strength', strength.toFixed(5));
    renderer?.setInteraction({ x: interaction.x, y: interaction.y, strength, phase: interaction.phase });
  };
  const resetInteraction = () => {
    if (interactionFrame) window.cancelAnimationFrame(interactionFrame);
    interactionFrame = 0;
    interaction.strength = 0;
    interaction.targetStrength = 0;
    interaction.phase = 0;
    interaction.lastTime = 0;
    stage.dataset.interaction = 'idle';
    renderInteraction();
  };
  const queueInteractionFrame = () => {
    if (interactionFrame || !interactionAllowed()) return;
    stage.dataset.interaction = 'moving';
    interactionFrame = window.requestAnimationFrame(stepInteraction);
  };
  const stepInteraction = (time) => {
    interactionFrame = 0;
    if (!interactionAllowed()) {
      resetInteraction();
      return;
    }
    const elapsed = interaction.lastTime ? Math.min(64, Math.max(8, time - interaction.lastTime)) : 16;
    interaction.lastTime = time;
    const positionEase = 1 - Math.exp(-elapsed / 520);
    const strengthEase = 1 - Math.exp(-elapsed / (interaction.targetStrength ? 650 : 820));
    interaction.x += (interaction.targetX - interaction.x) * positionEase;
    interaction.y += (interaction.targetY - interaction.y) * positionEase;
    interaction.strength += (interaction.targetStrength - interaction.strength) * strengthEase;
    interaction.phase += elapsed / 1800;
    if (interaction.targetStrength === 0 && interaction.strength < 0.002) {
      resetInteraction();
      return;
    }
    const positionSettled = Math.abs(interaction.targetX - interaction.x) < 0.15 && Math.abs(interaction.targetY - interaction.y) < 0.15;
    const strengthSettled = Math.abs(interaction.targetStrength - interaction.strength) < 0.002;
    if (positionSettled && strengthSettled) {
      interaction.x = interaction.targetX;
      interaction.y = interaction.targetY;
      interaction.strength = interaction.targetStrength;
      interaction.lastTime = 0;
      renderInteraction();
      stage.dataset.interaction = interaction.targetStrength ? 'resting' : 'idle';
      return;
    }
    renderInteraction();
    queueInteractionFrame();
  };
  const activateInteraction = (event) => {
    if (!interactionAllowed() || event.isPrimary === false) return;
    const clientX = Number(event.clientX);
    const clientY = Number(event.clientY);
    if (!Number.isFinite(clientX) || !Number.isFinite(clientY)) return;
    const wasIdle = interaction.strength < 0.002 && interaction.targetStrength === 0;
    interaction.targetX = Math.min(window.innerWidth, Math.max(0, clientX));
    interaction.targetY = Math.min(window.innerHeight, Math.max(0, clientY));
    if (wasIdle) {
      interaction.x = interaction.targetX;
      interaction.y = interaction.targetY;
    }
    interaction.targetStrength = 1;
    queueInteractionFrame();
  };
  const settleInteraction = (event) => {
    if (event?.type === 'pointerup' && event.pointerType === 'mouse') return;
    interaction.targetStrength = 0;
    if (interactionAllowed()) queueInteractionFrame();
    else resetInteraction();
  };
  const setToggleState = (active) => {
    staticMotionPaused = !active;
    toggle.setAttribute('aria-pressed', String(active));
    stage.dataset.motion = active ? 'running' : 'paused';
    if (!active) {
      stage.removeAttribute('data-transition');
      resetInteraction();
    }
  };
  window.addEventListener('pointermove', activateInteraction, { passive: true });
  window.addEventListener('pointerdown', activateInteraction, { passive: true });
  window.addEventListener('pointerup', settleInteraction, { passive: true });
  window.addEventListener('pointercancel', settleInteraction, { passive: true });
  window.addEventListener('pointerout', (event) => {
    if (!event.relatedTarget) settleInteraction(event);
  }, { passive: true });
  window.addEventListener('blur', resetInteraction);
  resetInteraction();

  const clearCanvasRecovery = () => {
    if (canvasRecoveryFrame) window.cancelAnimationFrame(canvasRecoveryFrame);
    canvasRecoveryFrame = 0;
    canvas.style.removeProperty('transition');
    canvas.style.removeProperty('opacity');
  };

  const restoreEnhancedCanvasVisibility = () => {
    if (!renderer || stage.dataset.enhanced !== 'true' || !capable()) return;
    clearCanvasRecovery();
    canvas.style.transition = 'none';
    canvas.style.opacity = '1';
    void canvas.offsetWidth;
    canvasRecoveryFrame = window.requestAnimationFrame(() => {
      canvasRecoveryFrame = 0;
      canvas.style.removeProperty('transition');
    });
  };

  const stop = ({ motionActive = motionControlEligible() && !userPaused() } = {}) => {
    clearCanvasRecovery();
    updateStaticScene();
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
      renderInteraction();
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
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      resetInteraction();
      renderer?.pause();
    } else if (renderer) renderer.resume();
    else reconcilePreference();
  });
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
    sceneVisible = entries.some((entry) => entry.isIntersecting) || [landingMain].some((region) => {
      if (!region) return false;
      const rect = region.getBoundingClientRect();
      return rect.bottom > 0 && rect.top < window.innerHeight;
    });
    if (!sceneVisible) renderer?.pause();
    else if (renderer) renderer.resume();
    else reconcilePreference();
  }, { rootMargin: '80px 0px', threshold: 0 });
  if (landingMain) observer.observe(landingMain);
  else sections.forEach((section) => observer.observe(section));
  if (!pageLoaded) window.addEventListener('load', () => { pageLoaded = true; reconcilePreference(); }, { once: true });

  const cancelPendingFrames = () => {
    if (idleHandle) {
      if ('cancelIdleCallback' in window) window.cancelIdleCallback(idleHandle); else window.clearTimeout(idleHandle);
    }
    if (resizeFrame) window.cancelAnimationFrame(resizeFrame);
    if (sceneFrame) window.cancelAnimationFrame(sceneFrame);
    resetInteraction();
    clearCanvasRecovery();
    idleHandle = 0;
    resizeFrame = 0;
    sceneFrame = 0;
  };

  window.addEventListener('pagehide', (event) => {
    cancelPendingFrames();
    if (event.persisted) {
      renderer?.pause();
      return;
    }
    disposed = true;
    observer.disconnect();
    stop();
  });

  window.addEventListener('pageshow', (event) => {
    if (!event.persisted || disposed) return;
    pageLoaded = true;
    updateStaticScene();
    const visibleRegions = landingMain ? [landingMain] : sections;
    sceneVisible = visibleRegions.some((region) => {
      const rect = region.getBoundingClientRect();
      return rect.bottom > 0 && rect.top < window.innerHeight;
    });
    if (renderer && capable() && !document.hidden) {
      renderer.resume();
      restoreEnhancedCanvasVisibility();
    } else reconcilePreference();
  });
})();
