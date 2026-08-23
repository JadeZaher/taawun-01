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
  fn octagonMetric(p: vec2f) -> f32 {
    let q = abs(p);
    return max(max(q.x, q.y), (q.x + q.y) * 0.70710678);
  }
  fn stroke(distance: f32, width: f32, softness: f32) -> f32 {
    let aa = max(fwidth(distance) * softness, 0.0006);
    return 1.0 - smoothstep(width, width + aa, distance);
  }
  fn lattice(point: vec2f, scale: f32, turn: f32, softness: f32, travel: f32) -> vec3f {
    let tiledPoint = rotate2(point, turn) * scale;
    let cell = fract(tiledPoint) - vec2f(0.5);
    let outerStarScale = 1.0 - travel * 0.035;
    let innerRosetteScale = 1.0 + travel * 0.045;
    let star = starOutline(cell / outerStarScale) * outerStarScale;
    let diagonalA = abs(abs(cell.x + cell.y) - 0.5) * 0.70710678;
    let diagonalB = abs(abs(cell.x - cell.y) - 0.5) * 0.70710678;
    let innerRosette = starOutline(rotate2(cell, 0.39269908) * 1.42 / innerRosetteScale) * innerRosetteScale / 1.42;
    let junctionPoint = vec2f(0.5) - abs(cell);
    let junctionMetric = octagonMetric(junctionPoint);
    let junctionInterior = 1.0 - smoothstep(0.105, 0.135, junctionMetric);
    let connector = stroke(abs(junctionMetric - 0.145), 0.008, softness);
    let diagonalBridgeAxis = abs(abs(cell.x) - abs(cell.y)) * 0.70710678;
    let bridgePairDistance = abs(diagonalBridgeAxis - 0.012);
    let bridgeStarGate = smoothstep(0.392, 0.410, length(cell));
    let bridgeConnectorGate = smoothstep(0.132, 0.150, junctionMetric);
    let bridgeRails = stroke(bridgePairDistance, 0.004, softness) * bridgeStarGate * bridgeConnectorGate;
    let crossing = stroke(max(diagonalA, diagonalB), 0.018, softness);
    let verticalCrossing = abs(cell.x) > abs(cell.y);
    let verticalCrossingID = round(tiledPoint.x) + floor(tiledPoint.y);
    let horizontalCrossingID = floor(tiledPoint.x) + round(tiledPoint.y);
    let crossingID = select(horizontalCrossingID, verticalCrossingID, verticalCrossing);
    let overUnder = step(0.5, fract(crossingID * 0.5));
    let strapGate = 1.0 - junctionInterior;
    let strapA = stroke(diagonalA, 0.008, softness) * strapGate * (1.0 - crossing * (1.0 - overUnder));
    let strapB = stroke(diagonalB, 0.008, softness) * strapGate * (1.0 - crossing * overUnder);
    let primary = max(stroke(star, 0.009, softness), max(connector, max(bridgeRails, max(strapA, strapB))));
    let secondary = stroke(innerRosette, 0.004, softness);
    return vec3f(primary, secondary, min(star, min(diagonalA, diagonalB)));
  }
  @fragment fn fragmentMain(@builtin(position) position: vec4f) -> @location(0) vec4f {
    let resolution = max(u.viewportScroll.xy, vec2f(1.0));
    let phase = u.viewportScroll.w;
    let section = u.stageMotion.x;
    let side = u.stageMotion.y;
    let transition = u.stageMotion.z;
    let canvasUV = (position.xy * 2.0 - resolution) / resolution.y;
    var uv = canvasUV;
    uv.x -= side * 0.34;
    let softness = mix(1.75, 1.0, u.stageMotion.w);
    let travel = 1.0 - u.stageMotion.w;
    let sceneTravel = travel * transition;
    let internalPulse = travel * (0.58 + 0.42 * sin(phase));
    let kaleidoscopeTravel = max(sceneTravel, internalPulse * 0.45);
    let stableTurn = 0.018;
    let turn = stableTurn + sceneTravel * 0.012;
    let basePoint = rotate2(uv, sceneTravel * 0.006);
    let latticeScale = 3.45;
    let baseLayer = lattice(basePoint, latticeScale, turn, softness, kaleidoscopeTravel);
    let lensCenter = vec2f(side * 0.08 + 0.26 * sin(phase * 0.72 + transition), -0.18 + 0.12 * cos(phase * 0.54));
    let lensDistance = length(uv - lensCenter);
    let lens = 1.0 - smoothstep(0.04, 0.66, lensDistance);
    let refractionVector = uv - lensCenter + vec2f(0.001);
    let refractionDirection = refractionVector / max(length(refractionVector), 0.0001);
    let bandCellOffset = 0.010 + lens * 0.010 + internalPulse * 0.0008 + sceneTravel * 0.0012;
    let bandMagnitude = bandCellOffset / latticeScale;
    let bandVector = refractionDirection + vec2f(0.35, -0.2);
    let bandDirection = bandVector / max(length(bandVector), 0.0001);
    let bandOffset = bandDirection * bandMagnitude;
    let mirrorOffset = vec2f(-bandOffset.y, bandOffset.x) * (0.72 + internalPulse * 0.06);
    let emeraldBand = lattice(basePoint + bandOffset, latticeScale, turn, softness, kaleidoscopeTravel);
    let rustBand = lattice(basePoint - bandOffset, latticeScale, turn, softness, kaleidoscopeTravel);
    let mirrorBand = lattice(basePoint + mirrorOffset, latticeScale, turn, softness * 1.05, kaleidoscopeTravel);
    let refractionEdge = abs(emeraldBand.x - rustBand.x);
    let chromaticEnvelope = 0.07 + lens * 0.68;
    let caustic = 0.5 + 0.5 * sin(uv.x * 5.0 - uv.y * 3.0 - phase * 1.4);
    var color = vec3f(0.012, 0.04, 0.034);
    color += vec3f(0.95, 0.68, 0.24) * baseLayer.x * (0.48 + caustic * 0.18);
    color += vec3f(0.16, 0.57, 0.45) * emeraldBand.x * chromaticEnvelope * 0.48;
    color += vec3f(0.78, 0.24, 0.08) * rustBand.x * chromaticEnvelope * 0.44;
    color += vec3f(0.95, 0.68, 0.24) * baseLayer.y * (0.4 + lens * 0.38);
    color += vec3f(0.16, 0.57, 0.45) * mirrorBand.y * (0.03 + lens * 0.14);
    color += vec3f(0.78, 0.24, 0.08) * refractionEdge * (0.05 + lens * 0.4);
    color += vec3f(0.95, 0.68, 0.24) * pow(max(0.0, 1.0 - lensDistance), 5.0) * 0.14;
    let field = clamp(baseLayer.x * 0.4 + baseLayer.y * 0.62 + (emeraldBand.x * 0.18 + rustBand.x * 0.16) * chromaticEnvelope + mirrorBand.y * (0.03 + lens * 0.09) + lens * 0.05 + refractionEdge * (0.04 + lens * 0.14), 0.0, 1.0);
    let edgeFade = 1.0 - smoothstep(0.16, 1.65, length(uv));
    let viewportHalfExtent = vec2f(resolution.x / resolution.y, 1.0);
    let viewportEdgeDistance = min(viewportHalfExtent.x - abs(canvasUV.x), viewportHalfExtent.y - abs(canvasUV.y));
    let viewportFeather = smoothstep(0.015, 0.10, viewportEdgeDistance);
    let alpha = clamp(field * edgeFade * viewportFeather * (0.52 + transition * 0.22) * mix(0.82, 1.0, u.stageMotion.w), 0.0, 0.82);
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
  const landingMain = document.querySelector('main');
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
  const SPRING_DAMPING = 2 * Math.sqrt(SPRING_STIFFNESS);
  const INTERNAL_PHASE_RATE = 0.55;
  const INTERNAL_PHASE_EASE = 0.18;
  const INTERNAL_PHASE_SETTLE_VELOCITY = 0.001;
  const SCROLL_ACTIVITY_DECAY = 0.12;
  const SCROLL_ACTIVITY_SETTLE = 0.004;
  const SCROLL_ACTIVITY_STRENGTH = 0.32;
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
  let internalPhase = 0;
  let internalPhaseVelocity = 0;
  let scrollActivity = 0;

  const teardown = ({ notify = false } = {}) => {
    if (!alive) return;
    alive = false;
    window.removeEventListener('scroll', onScroll);
    if (frame) cancelAnimationFrame(frame);
    if (delayedFrame) window.clearTimeout(delayedFrame);
    frame = 0;
    delayedFrame = 0;
    stage.removeAttribute('data-spring');
    delete canvas.dataset.renderer;
    try { uniformBuffer.destroy(); } catch { /* Already unavailable. */ }
    try { device.destroy(); } catch { /* Device loss is already the fallback. */ }
    if (notify) queueMicrotask(() => {
      try { onFailure(); } finally { stage.removeAttribute('data-enhanced'); }
    });
    else stage.removeAttribute('data-enhanced');
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
      const priorDistance = targetScroll - springScroll;
      const internalMotionActive = Math.abs(priorDistance) > SETTLE_DISTANCE || Math.abs(springVelocity) > SETTLE_VELOCITY || scrollActivity > SCROLL_ACTIVITY_SETTLE;
      const acceleration = SPRING_STIFFNESS * (targetScroll - springScroll) - SPRING_DAMPING * springVelocity;
      springVelocity = Math.max(-MAX_VELOCITY, Math.min(MAX_VELOCITY, springVelocity + acceleration * FIXED_STEP_SECONDS));
      springScroll += springVelocity * FIXED_STEP_SECONDS;
      const internalPhaseTarget = internalMotionActive ? INTERNAL_PHASE_RATE : 0;
      internalPhaseVelocity += (internalPhaseTarget - internalPhaseVelocity) * INTERNAL_PHASE_EASE;
      if (!internalMotionActive && Math.abs(internalPhaseVelocity) <= INTERNAL_PHASE_SETTLE_VELOCITY) internalPhaseVelocity = 0;
      internalPhase = (internalPhase + internalPhaseVelocity * FIXED_STEP_SECONDS) % (Math.PI * 2);
      scrollActivity *= 1 - SCROLL_ACTIVITY_DECAY;
      if (scrollActivity <= SCROLL_ACTIVITY_SETTLE) scrollActivity = 0;
      physicsAccumulator -= FIXED_STEP_SECONDS;
      if (priorDistance !== 0 && priorDistance * (targetScroll - springScroll) <= 0) {
        springScroll = targetScroll;
        springVelocity = 0;
        break;
      }
    }
    const springSettled = Math.abs(targetScroll - springScroll) <= SETTLE_DISTANCE && Math.abs(springVelocity) <= SETTLE_VELOCITY;
    if (springSettled) {
      springScroll = targetScroll;
      springVelocity = 0;
    }
    settling = !springSettled || internalPhaseVelocity !== 0 || scrollActivity !== 0;
    if (!settling) {
      physicsAccumulator = 0;
      lastPhysicsTime = 0;
    }
    stage.dataset.spring = settling ? 'settling' : 'settled';
    return settling;
  };

  const draw = (timestamp = performance.now()) => {
    frame = 0;
    if (!alive || paused || document.hidden) return;
    try {
      const started = performance.now();
      const maxScroll = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
      const moving = advanceSpring(timestamp);
      const springMotionAmount = Math.abs(springVelocity) / MAX_VELOCITY * 0.55 + Math.abs(targetScroll - springScroll) * 8;
      const motionAmount = moving ? Math.min(1, Math.max(springMotionAmount, scrollActivity * SCROLL_ACTIVITY_STRENGTH)) : 0;
      const settledMix = 1 - motionAmount;
      const scroll = Math.min(1, Math.max(0, springScroll));
      const scrollTop = scroll * maxScroll;
      const anchor = scrollTop + window.innerHeight * 0.68;
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
      const section = currentState + (nextState - currentState) * progress;
      const mainStart = landingMain?.offsetTop || 0;
      const mainScrollSpan = Math.max(1, (landingMain?.offsetHeight || document.documentElement.scrollHeight) - window.innerHeight);
      const mainProgress = Math.min(1, Math.max(0, (scrollTop - mainStart) / mainScrollSpan));
      const lateralAngle = Math.PI * 2 * mainProgress;
      const side = Math.cos(lateralAngle);
      const transition = Math.abs(Math.sin(lateralAngle));
      const phase = internalPhase;
      const selected = progress < 0.5 ? current : next;
      stage.dataset.state = selected?.dataset.geometryState || String(Math.round(section));
      stage.dataset.side = side < 0 ? 'left' : 'right';
      if (moving && transition > 0.24) stage.dataset.transition = 'true'; else stage.removeAttribute('data-transition');
      device.queue.writeBuffer(uniformBuffer, 0, new Float32Array([canvas.width, canvas.height, scroll, phase, section, side, transition, settledMix]));
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
      let replacementDrawRequired = false;
      if (elapsed > 20) { resolutionScale = 0.7; lastFrameCostExceeded = true; configuredWidth = 0; configuredHeight = 0; resize(); replacementDrawRequired = true; }
      else lastFrameCostExceeded = false;
      if (moving || replacementDrawRequired) requestDraw();
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
      scrollActivity = 1;
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
