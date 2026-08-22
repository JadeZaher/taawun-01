const shaderSource = `
  struct Uniforms { viewportScroll: vec4f, stageMotion: vec4f }
  @group(0) @binding(0) var<uniform> u: Uniforms;

  @vertex fn vertexMain(@builtin(vertex_index) i: u32) -> @builtin(position) vec4f {
    var p = array<vec2f, 3>(vec2f(-1.0, -3.0), vec2f(-1.0, 1.0), vec2f(3.0, 1.0));
    return vec4f(p[i], 0.0, 1.0);
  }
  fn rotate2(v: vec2f, a: f32) -> vec2f { let c = cos(a); let s = sin(a); return mat2x2f(c, -s, s, c) * v; }
  fn segmentDistance(p: vec2f, a: vec2f, b: vec2f) -> f32 {
    let edge = b - a;
    let projection = clamp(dot(p - a, edge) / max(dot(edge, edge), 0.00001), 0.0, 1.0);
    return length(p - (a + edge * projection));
  }
  fn starOutline(p: vec2f) -> f32 {
    let outerSector = 0.78539816;
    let halfSector = 0.39269908;
    let angle = atan2(p.y, p.x);
    let foldedAngle = abs((fract(angle / outerSector + 0.5) - 0.5) * outerSector);
    let foldedPoint = vec2f(cos(foldedAngle), sin(foldedAngle)) * length(p);
    let outerPoint = vec2f(0.445, 0.0);
    let innerPoint = vec2f(cos(halfSector), sin(halfSector)) * 0.205;
    return segmentDistance(foldedPoint, outerPoint, innerPoint);
  }
  fn lattice(point: vec2f, scale: f32, turn: f32, softness: f32) -> vec3f {
    let cell = fract(rotate2(point, turn) * scale) - vec2f(0.5);
    let star = starOutline(cell);
    let diagonalA = abs(abs(cell.x + cell.y) - 0.5) * 0.70710678;
    let diagonalB = abs(abs(cell.x - cell.y) - 0.5) * 0.70710678;
    let straps = min(diagonalA, diagonalB);
    let innerRosette = starOutline(rotate2(cell, 0.39269908) * 1.42) / 1.42;
    let primaryDistance = min(star, straps);
    let secondaryDistance = innerRosette;
    let primaryAA = max(fwidth(primaryDistance) * softness, 0.0008);
    let secondaryAA = max(fwidth(secondaryDistance) * softness, 0.0006);
    let primary = 1.0 - smoothstep(0.010, 0.010 + primaryAA, primaryDistance);
    let secondary = 1.0 - smoothstep(0.004, 0.004 + secondaryAA, secondaryDistance);
    return vec3f(primary, secondary, primaryDistance);
  }
  @fragment fn fragmentMain(@builtin(position) position: vec4f) -> @location(0) vec4f {
    let resolution = max(u.viewportScroll.xy, vec2f(1.0));
    let scroll = u.viewportScroll.z;
    let phase = u.viewportScroll.w;
    let section = u.stageMotion.x;
    let side = u.stageMotion.y;
    let transition = u.stageMotion.z;
    var uv = (position.xy * 2.0 - resolution) / resolution.y;
    uv.x -= side * (0.48 - transition * 0.14);
    let softness = mix(1.75, 1.0, u.stageMotion.w);
    let turn = (section - 2.0) * 0.052 + phase * 0.03 + transition * 0.08;
    let basePoint = rotate2(uv + vec2f(scroll * 0.045, -scroll * 0.025), transition * 0.025);
    let baseLayer = lattice(basePoint, 3.25 + section * 0.1, turn, softness);
    let mirrorPoint = vec2f(-uv.x, uv.y) * 1.1 + vec2f(0.08, -0.05);
    let deepLayer = lattice(mirrorPoint, 4.25, -turn * 0.7, softness * 1.1);
    let lensCenter = vec2f(side * 0.08 + 0.32 * sin(scroll * 3.4 + transition), -0.18 + phase * 0.34);
    let lensDistance = length(uv - lensCenter);
    let lens = 1.0 - smoothstep(0.04, 0.66, lensDistance);
    let refractedPoint = uv + normalize(uv - lensCenter + vec2f(0.001)) * lens * 0.04;
    let refracted = lattice(refractedPoint, 3.25 + section * 0.1, turn + lens * 0.035, softness);
    let refractionEdge = abs(refracted.x - baseLayer.x);
    let caustic = 0.5 + 0.5 * sin(uv.x * 5.0 - uv.y * 3.0 - phase * 5.0);
    var color = vec3f(0.012, 0.04, 0.034);
    color += vec3f(0.16, 0.57, 0.45) * mix(baseLayer.x, refracted.x, lens) * (0.42 + caustic * 0.28);
    color += vec3f(0.95, 0.68, 0.24) * baseLayer.y * (0.44 + lens * 0.72);
    color += vec3f(0.16, 0.57, 0.45) * deepLayer.x * 0.2;
    color += vec3f(0.78, 0.24, 0.08) * refractionEdge * 1.1;
    color += vec3f(0.95, 0.68, 0.24) * pow(max(0.0, 1.0 - lensDistance), 5.0) * 0.14;
    let field = clamp(baseLayer.x * 0.34 + baseLayer.y * 0.72 + deepLayer.x * 0.18 + lens * 0.08 + refractionEdge * 0.35, 0.0, 1.0);
    let edgeFade = 1.0 - smoothstep(0.16, 1.65, length(uv));
    let alpha = clamp(field * edgeFade * (0.52 + transition * 0.22) * mix(0.82, 1.0, u.stageMotion.w), 0.0, 0.82);
    return vec4f(color * alpha, alpha);
  }
`;

export async function createGeometricRenderer({ canvas, stage, onFailure = () => {} }) {
  const adapter = await navigator.gpu.requestAdapter({ powerPreference: 'low-power' });
  if (!adapter || adapter.isFallbackAdapter) return null;
  const device = await adapter.requestDevice();
  try {
  const context = canvas.getContext('webgpu');
  if (!context) { device.destroy(); return null; }
  const format = navigator.gpu.getPreferredCanvasFormat();
  const shader = device.createShaderModule({ code: shaderSource });
  const compilation = await shader.getCompilationInfo();
  if (compilation.messages.some((message) => message.type === 'error')) { device.destroy(); return null; }
  const pipeline = device.createRenderPipeline({
    layout: 'auto',
    vertex: { module: shader, entryPoint: 'vertexMain' },
    fragment: { module: shader, entryPoint: 'fragmentMain', targets: [{ format }] },
    primitive: { topology: 'triangle-list' },
  });
  const uniformBuffer = device.createBuffer({ size: 32, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
  const bindGroup = device.createBindGroup({ layout: pipeline.getBindGroupLayout(0), entries: [{ binding: 0, resource: { buffer: uniformBuffer } }] });
  const sections = [...document.querySelectorAll('[data-geometry-state]')];
  let alive = true;
  let paused = false;
  let frame = 0;
  let delayedFrame = 0;
  let lastDraw = 0;
  let lastFrameCostExceeded = false;
  let resolutionScale = 1;
  let configuredWidth = 0;
  let configuredHeight = 0;
  const FIXED_STEP_SECONDS = 1 / 60;
  const MAX_DT_SECONDS = 0.05;
  const SPRING_STIFFNESS = 72;
  const SPRING_DAMPING = 10.5;
  const MAX_VELOCITY = 0.9;
  const SETTLE_DISTANCE = 0.0002;
  const SETTLE_VELOCITY = 0.0005;
  const readScrollTarget = () => {
    const maxScroll = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
    return Math.min(1, Math.max(0, window.scrollY / maxScroll));
  };
  let targetScroll = readScrollTarget();
  let springScroll = targetScroll;
  let springVelocity = 0;
  let physicsAccumulator = 0;
  let lastPhysicsTime = 0;
  let settling = false;

  const teardown = ({ notify = false } = {}) => {
    if (!alive) return;
    alive = false;
    window.removeEventListener('scroll', onScroll);
    if (frame) cancelAnimationFrame(frame);
    if (delayedFrame) window.clearTimeout(delayedFrame);
    frame = 0;
    delayedFrame = 0;
    stage.removeAttribute('data-enhanced');
    stage.removeAttribute('data-spring');
    delete canvas.dataset.renderer;
    try { uniformBuffer.destroy(); } catch { /* Already unavailable. */ }
    try { device.destroy(); } catch { /* Device loss is already the fallback. */ }
    if (notify) queueMicrotask(onFailure);
  };

  const resize = () => {
    if (!alive) return false;
    try {
      const dpr = Math.min(window.devicePixelRatio || 1, 1.5) * resolutionScale;
      let width = Math.max(1, Math.round(canvas.clientWidth * dpr));
      let height = Math.max(1, Math.round(canvas.clientHeight * dpr));
      const pixels = width * height;
      if (pixels > 1_500_000) {
        const scale = Math.sqrt(1_500_000 / pixels);
        width = Math.round(width * scale);
        height = Math.round(height * scale);
      }
      if (configuredWidth === width && configuredHeight === height) return false;
      canvas.width = width;
      canvas.height = height;
      context.configure({ device, format, alphaMode: 'premultiplied' });
      configuredWidth = width;
      configuredHeight = height;
      return true;
    } catch {
      teardown({ notify: true });
      return false;
    }
  };

  const advanceSpring = (timestamp) => {
    const elapsed = lastPhysicsTime ? Math.min(MAX_DT_SECONDS, Math.max(0, (timestamp - lastPhysicsTime) / 1000)) : FIXED_STEP_SECONDS;
    lastPhysicsTime = timestamp;
    physicsAccumulator = Math.min(MAX_DT_SECONDS, physicsAccumulator + elapsed);
    while (physicsAccumulator >= FIXED_STEP_SECONDS) {
      const acceleration = SPRING_STIFFNESS * (targetScroll - springScroll) - SPRING_DAMPING * springVelocity;
      springVelocity = Math.max(-MAX_VELOCITY, Math.min(MAX_VELOCITY, springVelocity + acceleration * FIXED_STEP_SECONDS));
      springScroll += springVelocity * FIXED_STEP_SECONDS;
      physicsAccumulator -= FIXED_STEP_SECONDS;
    }
    if (Math.abs(targetScroll - springScroll) <= SETTLE_DISTANCE && Math.abs(springVelocity) <= SETTLE_VELOCITY) {
      springScroll = targetScroll;
      springVelocity = 0;
      physicsAccumulator = 0;
      settling = false;
      lastPhysicsTime = 0;
      stage.dataset.spring = 'settled';
    } else {
      settling = true;
      stage.dataset.spring = 'settling';
    }
    return settling;
  };

  const draw = (timestamp = performance.now()) => {
    frame = 0;
    if (!alive || paused || document.hidden) return;
    try {
      const started = performance.now();
      const maxScroll = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
      const moving = advanceSpring(timestamp);
      const scroll = Math.min(1, Math.max(0, springScroll));
      const anchor = scroll * maxScroll + window.innerHeight * 0.68;
      const centers = sections.map((section) => section.offsetTop + section.offsetHeight * 0.5);
      let nextIndex = centers.findIndex((center) => center > anchor);
      if (nextIndex < 0) nextIndex = centers.length;
      nextIndex = Math.min(Math.max(nextIndex, 1), Math.max(0, centers.length - 1));
      const currentIndex = Math.max(0, nextIndex - 1);
      const span = Math.max(1, (centers[nextIndex] || anchor) - (centers[currentIndex] || anchor));
      const rawProgress = currentIndex === nextIndex ? 0 : Math.min(1, Math.max(0, (anchor - centers[currentIndex]) / span));
      const earlyProgress = Math.min(1, Math.max(0, (rawProgress - 0.08) / 0.84));
      const progress = earlyProgress * earlyProgress * (3 - 2 * earlyProgress);
      const current = sections[currentIndex] || sections[0];
      const next = sections[nextIndex] || current;
      const currentState = Number(current?.dataset.geometryState || 0);
      const nextState = Number(next?.dataset.geometryState || currentState);
      const currentSide = current?.dataset.geometrySide === 'left' ? -1 : 1;
      const nextSide = next?.dataset.geometrySide === 'left' ? -1 : 1;
      const section = currentState + (nextState - currentState) * progress;
      const side = currentSide + (nextSide - currentSide) * progress;
      const transition = Math.sin(Math.PI * earlyProgress);
      const phase = currentIndex + progress;
      const selected = progress < 0.5 ? current : next;
      stage.dataset.state = selected?.dataset.geometryState || String(Math.round(section));
      stage.dataset.side = (progress < 0.5 ? currentSide : nextSide) < 0 ? 'left' : 'right';
      if (transition > 0.24) stage.dataset.transition = 'true'; else stage.removeAttribute('data-transition');
      device.queue.writeBuffer(uniformBuffer, 0, new Float32Array([canvas.width, canvas.height, scroll, phase, section, side, transition, moving ? 0 : 1]));
      const encoder = device.createCommandEncoder();
      const pass = encoder.beginRenderPass({ colorAttachments: [{ view: context.getCurrentTexture().createView(), clearValue: { r: 0, g: 0, b: 0, a: 0 }, loadOp: 'clear', storeOp: 'store' }] });
      pass.setPipeline(pipeline);
      pass.setBindGroup(0, bindGroup);
      pass.draw(3);
      pass.end();
      device.queue.submit([encoder.finish()]);
      stage.dataset.enhanced = 'true';
      canvas.dataset.renderer = 'webgpu';
      lastDraw = performance.now();
      const elapsed = performance.now() - started;
      if (elapsed > 50 || (elapsed > 20 && lastFrameCostExceeded)) { teardown({ notify: true }); return; }
      if (elapsed > 20) { resolutionScale = 0.7; lastFrameCostExceeded = true; configuredWidth = 0; configuredHeight = 0; resize(); }
      if (moving) requestDraw();
    } catch {
      teardown({ notify: true });
    }
  };

  const requestDraw = () => {
    if (frame || delayedFrame || !alive || paused) return;
    const wait = 33 - (performance.now() - lastDraw);
    if (lastDraw && wait > 0) {
      delayedFrame = window.setTimeout(() => { delayedFrame = 0; requestDraw(); }, wait);
      return;
    }
    frame = requestAnimationFrame(draw);
  };
  const onScroll = () => {
    const nextTarget = readScrollTarget();
    if (Math.abs(nextTarget - targetScroll) > Number.EPSILON) {
      targetScroll = nextTarget;
      settling = true;
      stage.dataset.spring = 'settling';
      requestDraw();
    }
  };
  window.addEventListener('scroll', onScroll, { passive: true });
  device.lost.then(() => teardown({ notify: true }));
  resize();
  if (!alive) return null;
  requestDraw();
  return {
    active() { return alive; },
    pause() { paused = true; if (frame) cancelAnimationFrame(frame); if (delayedFrame) window.clearTimeout(delayedFrame); frame = 0; delayedFrame = 0; lastPhysicsTime = 0; },
    resume() { paused = false; requestDraw(); },
    resize() { targetScroll = readScrollTarget(); settling = Math.abs(targetScroll - springScroll) > SETTLE_DISTANCE; lastPhysicsTime = 0; resize(); requestDraw(); },
    destroy() { teardown(); },
  };
  } catch {
    try { device.destroy(); } catch { /* Static fallback remains available. */ }
    stage.removeAttribute('data-enhanced');
    delete canvas.dataset.renderer;
    return null;
  }
}
