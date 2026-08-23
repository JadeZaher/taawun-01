const shaderSource = `
  struct Uniforms {
    viewportPanel: vec4f,
    panelMotion: vec4f,
    interaction: vec4f,
    renderMetrics: vec4f,
  }
  @group(0) @binding(0) var<uniform> u: Uniforms;

  @vertex fn vertexMain(@builtin(vertex_index) i: u32) -> @builtin(position) vec4f {
    var p = array<vec2f, 3>(vec2f(-1.0, -3.0), vec2f(-1.0, 1.0), vec2f(3.0, 1.0));
    return vec4f(p[i], 0.0, 1.0);
  }
  fn rotate2(v: vec2f, a: f32) -> vec2f { let c = cos(a); let s = sin(a); return mat2x2f(c, -s, s, c) * v; }
  fn hash2(p: vec2f) -> vec2f {
    let k = vec2f(dot(p, vec2f(127.1, 311.7)), dot(p, vec2f(269.5, 183.3)));
    return fract(sin(k) * 43758.5453);
  }
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
  // Nearest-rail distance across the whole tessellation; the lattice coordinates stay rigid.
  fn railDistance(cellInput: vec2f, tiledPoint: vec2f, shapeMotion: f32, shapePhase: f32) -> vec2f {
    let cell = cellInput;
    // At full local strength the outer star rotates half a sector and the rosette
    // counter-rotates to zero: the pair swaps symmetric poses instead of drifting.
    let outerStarTurn = shapeMotion * 0.39269908;
    let outerStarScale = 1.0 - shapeMotion * 0.07;
    let star = starOutline(rotate2(cell, outerStarTurn) / outerStarScale) * outerStarScale;
    let innerRosetteTurn = 0.39269908 * (1.0 - shapeMotion);
    let innerRosetteScale = 1.0 + shapeMotion * 0.11;
    let innerRosette = starOutline(rotate2(cell, innerRosetteTurn) * 1.42 / innerRosetteScale) * innerRosetteScale / 1.42;
    let diagonalA = abs(abs(cell.x + cell.y) - 0.5) * 0.70710678;
    let diagonalB = abs(abs(cell.x - cell.y) - 0.5) * 0.70710678;
    let junctionPoint = vec2f(0.5) - abs(cell);
    let junctionMetric = octagonMetric(junctionPoint);
    let junctionInterior = 1.0 - smoothstep(0.105, 0.135, junctionMetric);
    let connector = abs(junctionMetric - 0.145);
    let diagonalBridgeAxis = abs(abs(cell.x) - abs(cell.y)) * 0.70710678;
    let bridgePairDistance = abs(diagonalBridgeAxis - 0.012);
    let bridgeStarGate = smoothstep(0.392, 0.410, length(cell));
    let bridgeConnectorGate = smoothstep(0.132, 0.150, junctionMetric);
    let bridgeGate = bridgeStarGate * bridgeConnectorGate;
    let crossing = step(max(diagonalA, diagonalB), 0.018);
    let verticalCrossing = abs(cell.x) > abs(cell.y);
    let verticalCrossingID = round(tiledPoint.x) + floor(tiledPoint.y);
    let horizontalCrossingID = floor(tiledPoint.x) + round(tiledPoint.y);
    let crossingID = select(horizontalCrossingID, verticalCrossingID, verticalCrossing);
    let overUnder = step(0.5, fract(crossingID * 0.5));
    let strapGate = 1.0 - junctionInterior;
    // Rails carry per-family widths; over/under gaps cut the yielding strap, keeping the interlock.
    let far = 9.0;
    let strapA = select(far, diagonalA, strapGate > 0.5 && !(crossing > 0.5 && overUnder < 0.5));
    let strapB = select(far, diagonalB, strapGate > 0.5 && !(crossing > 0.5 && overUnder > 0.5));
    let bridge = select(far, bridgePairDistance, bridgeGate > 0.5);
    // Normalize each family by its authored half-width so one bevel field serves all rails.
    let starT = star / 0.03;
    let rosetteT = innerRosette / 0.015;
    let connectorT = connector / 0.022;
    let strapT = min(strapA, strapB) / 0.024;
    let bridgeT = bridge / 0.008;
    let nearest = min(starT, min(rosetteT, min(connectorT, min(strapT, bridgeT))));
    let coreDistance = min(star, min(innerRosette, min(connector, min(min(strapA, strapB), bridge))));
    return vec2f(nearest, coreDistance);
  }
  // Metalheart studio: sharp horizon flash, zenith white, emerald and gold bands, a rust sliver.
  fn environmentColor(direction: vec3f, lightDirection: vec3f, lightEnergy: f32) -> vec3f {
    let elevation = direction.y;
    var color = mix(vec3f(0.012, 0.03, 0.026), vec3f(0.34, 0.42, 0.4), smoothstep(-0.02, 0.3, elevation));
    color = mix(color, vec3f(0.97, 0.99, 0.98), smoothstep(0.34, 0.72, elevation));
    color = mix(color, vec3f(0.05, 0.1, 0.09), smoothstep(-0.1, -0.42, elevation));
    let horizonFlash = exp(-abs(elevation - 0.05) * 42.0);
    color += vec3f(1.0, 1.0, 0.98) * horizonFlash * 0.85;
    color += vec3f(0.92, 0.66, 0.28) * horizonFlash * smoothstep(0.08, 0.55, direction.x) * 0.8;
    color += vec3f(0.16, 0.62, 0.46) * horizonFlash * smoothstep(-0.08, -0.55, direction.x) * 0.8;
    color += vec3f(0.09, 0.46, 0.34) * exp(-abs(elevation + 0.4) * 7.5) * 1.1;
    color += vec3f(0.9, 0.64, 0.25) * exp(-abs(elevation - 0.58) * 17.0) * 0.6;
    color += vec3f(0.66, 0.26, 0.1) * exp(-abs(dot(direction.xz, vec2f(0.94, 0.34)) - 0.6) * 20.0) * 0.24;
    color += vec3f(1.0, 0.99, 0.95) * pow(max(dot(direction, lightDirection), 0.0), 26.0) * lightEnergy;
    return color;
  }
  fn luminance(c: vec3f) -> f32 { return dot(c, vec3f(0.299, 0.587, 0.114)); }
  fn crispCenterStroke(distance: f32, softness: f32) -> f32 {
    let pixelFootprint = clamp(fwidth(distance), 0.0002, 0.011);
    let halfCore = pixelFootprint * max(u.renderMetrics.x, 1.0) * 1.25;
    return 1.0 - smoothstep(halfCore, halfCore + pixelFootprint * softness, distance);
  }
  @fragment fn fragmentMain(@builtin(position) position: vec4f) -> @location(0) vec4f {
    let resolution = max(u.viewportPanel.xy, vec2f(1.0));
    let panelLift = u.viewportPanel.z;
    let panelAngle = u.viewportPanel.w;
    let panelX = u.panelMotion.x;
    let canvasUV = (position.xy * 2.0 - resolution) / resolution.y;
    // One rigid body: the whole tessellation translates and rotates together.
    let p = rotate2(canvasUV - vec2f(panelX, panelLift), -panelAngle);
    let strength = clamp(u.interaction.z, 0.0, 1.0);
    let phase = u.interaction.w;
    let pointerLight = rotate2(u.interaction.xy - vec2f(panelX, panelLift), -panelAngle);
    let authoredLight = vec2f(0.22, -0.26);
    let lightPoint = mix(authoredLight, pointerLight, strength * 0.9);
    let lightEnergy = mix(0.75, 1.5, strength);
    let lightDirection = normalize(vec3f(lightPoint - p, 0.8));

    let latticeScale = 3.0;
    let tiledPoint = p * latticeScale;
    let cell = fract(tiledPoint) - vec2f(0.5);
    let tiledCellCenter = floor(tiledPoint) + vec2f(0.5);
    let cellCenter = tiledCellCenter / latticeScale;
    let interactionDistance = length(cellCenter - lightPoint);
    let nearField = 1.0 - smoothstep(0.0, 0.14, interactionDistance);
    let interactionWave = mix(0.72 + 0.28 * cos(interactionDistance * 20.0 - phase), 1.0, nearField);
    let interactionEnvelope = (1.0 - smoothstep(0.035, 0.54, interactionDistance)) * strength * interactionWave;

    let rails = railDistance(cell, tiledPoint, interactionEnvelope, phase);
    let railField = clamp(1.0 - rails.x, 0.0, 1.0);
    let railAA = clamp(fwidth(rails.x), 0.001, 0.35);
    let coverage = 1.0 - smoothstep(1.0 - railAA, 1.0 + railAA, rails.x);
    // Molten zone: reflections smear and the bevel rounds, while every rail stays put.
    let liquid = (1.0 - smoothstep(0.0, 0.16, length(p - lightPoint))) * strength;
    let bevelHeight = sqrt(clamp(railField * (2.0 - railField), 0.0, 1.0));
    let bevelSlope = vec2f(dpdx(bevelHeight), dpdy(bevelHeight)) * resolution.y * 0.014 * (1.0 - liquid * 0.45);
    let railNormal = normalize(vec3f(-bevelSlope, 1.0));
    var railReflect = reflect(vec3f(0.0, 0.0, -1.0), railNormal);
    let flow = vec2f(sin(p.y * 34.0 + phase * 3.1 + p.x * 9.0), cos(p.x * 30.0 - phase * 2.6 + p.y * 7.0));
    railReflect = normalize(railReflect + vec3f(flow * liquid * 0.4, 0.0));
    var railColor = environmentColor(railReflect, lightDirection, lightEnergy) * vec3f(0.92, 0.98, 0.96);
    let fresnel = pow(1.0 - clamp(railNormal.z, 0.0, 1.0), 2.0);
    railColor = railColor * (0.72 + 0.5 * fresnel);
    // Palette lives in the reflections: gold catches one bevel flank, emerald the other.
    railColor += vec3f(0.58, 0.4, 0.15) * max(railNormal.x, 0.0) * 0.55;
    railColor += vec3f(0.07, 0.32, 0.23) * max(-railNormal.x, 0.0) * 0.6;
    railColor = clamp(railColor * railColor * (vec3f(3.0) - 2.0 * railColor), vec3f(0.0), vec3f(1.0));

    // Ayeneh-kari ground: diamond mirror panes whose corners sit exactly on star
    // centers and whose edges run along the strap diagonals (L1 tiling of the
    // cell-center checkerboard).
    let paneAxes = tiledPoint - vec2f(0.5);
    let paneId = vec2f(round((paneAxes.x + paneAxes.y) * 0.5), round((paneAxes.x - paneAxes.y) * 0.5));
    let paneHash = hash2(paneId);
    let paneNormal = normalize(vec3f((paneHash - vec2f(0.5)) * 0.5, 1.0));
    var paneReflect = reflect(vec3f(0.0, 0.0, -1.0), paneNormal);
    paneReflect = normalize(paneReflect + vec3f(flow * liquid * 0.35, 0.0));
    let facetEnv = environmentColor(paneReflect, lightDirection, lightEnergy * 0.7);
    let glint = pow(max(dot(paneReflect, lightDirection), 0.0), 60.0) * 1.6 * lightEnergy;
    let lightProximity = 1.0 - smoothstep(0.05, 0.85, length(p - lightPoint));
    let facetAlphaRaw = (0.05 + luminance(facetEnv) * 0.07 + glint * 0.35) * (0.6 + 0.4 * lightProximity);

    let edgeFade = 1.0 - smoothstep(0.18, 1.6, length(p));
    let visibleHalfExtent = max(u.renderMetrics.yz, vec2f(0.001));
    let viewportEdgeDistance = min(visibleHalfExtent.x - abs(canvasUV.x), visibleHalfExtent.y - abs(canvasUV.y));
    let viewportFeather = smoothstep(0.0, 0.12, viewportEdgeDistance);
    let envelope = edgeFade * viewportFeather;

    let facetAlpha = clamp(facetAlphaRaw, 0.0, 0.6) * envelope * (1.0 - coverage);
    let facetPremultiplied = facetEnv * facetAlpha + vec3f(0.9, 0.97, 0.94) * glint * 0.25 * envelope * (1.0 - coverage);

    let softness = 1.0;
    let core = crispCenterStroke(rails.y, softness);
    // The exact 2.5 CSS px core stays fully opaque and resolves as polished mirror face.
    let coreColor = environmentColor(vec3f(0.0, 0.06, -1.0), lightDirection, lightEnergy) * vec3f(0.95, 0.99, 0.97) + vec3f(0.28, 0.3, 0.29);
    let railAlpha = clamp(max(coverage * 0.88, core), 0.0, 1.0) * envelope;
    var railOut = mix(railColor, clamp(coreColor, vec3f(0.0), vec3f(1.0)), core * 0.8);
    railOut = clamp(railOut, vec3f(0.0), vec3f(1.0));
    let railPremultiplied = railOut * railAlpha;

    // Anamorphic glare: one restrained horizontal streak riding the live light.
    let glareDelta = p - lightPoint;
    let glare = exp(-abs(glareDelta.y) * 44.0) * exp(-abs(glareDelta.x) * 6.5) * 0.3 * strength * envelope;
    let glarePremultiplied = vec3f(0.96, 0.98, 0.94) * glare;

    var outputPremultiplied = facetPremultiplied * (1.0 - railAlpha) + railPremultiplied;
    var outputAlpha = facetAlpha * (1.0 - railAlpha) + railAlpha;
    outputPremultiplied += glarePremultiplied;
    outputAlpha = clamp(outputAlpha + glare * 0.5, 0.0, 1.0);
    let grain = fract(sin(dot(position.xy, vec2f(12.9898, 78.233))) * 43758.5453);
    outputPremultiplied *= 0.985 + 0.015 * grain;
    return vec4f(outputPremultiplied, outputAlpha);
  }
`;

export async function createGeometricRenderer({ canvas, stage, onFailure = () => {} }) {
  const adapter = await navigator.gpu.requestAdapter({ powerPreference: 'low-power' });
  if (!adapter || adapter.isFallbackAdapter || adapter.info?.isFallbackAdapter) return null;
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
  const uniformBuffer = device.createBuffer({ size: 64, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
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
  const BASE_TURN = 0.018;
  // The panel is an inertial body: scroll applies impulses, anchors pull softly, tilt follows velocity.
  const ANCHOR_STIFFNESS = 5.5;
  const ANCHOR_DAMPING = 2 * Math.sqrt(ANCHOR_STIFFNESS) * 1.1;
  const LIFT_STIFFNESS = 14;
  const LIFT_DAMPING = 2 * Math.sqrt(LIFT_STIFFNESS) * 1.2;
  const MAX_PANEL_VELOCITY = 0.55;
  const TILT_FROM_TRAVEL = 0.28;
  const TILT_FROM_LIFT = 0.3;
  const MAX_TILT = 0.02;
  const SETTLE_DISTANCE = 0.0004;
  const SETTLE_VELOCITY = 0.0006;
  const anchorOffset = () => (window.innerWidth <= 840 ? 0.42 : 0.34);
  const readTargetAnchor = () => {
    if (!sections.length) return anchorOffset();
    const anchor = window.scrollY + window.innerHeight * 0.68;
    const centers = sections.map((section) => section.offsetTop + section.offsetHeight * 0.5);
    let index = centers.findIndex((center) => center > anchor);
    if (index < 0) index = centers.length;
    const nextIndex = Math.min(Math.max(index, 1), centers.length - 1);
    const currentIndex = Math.max(0, nextIndex - 1);
    const span = Math.max(1, centers[nextIndex] - centers[currentIndex]);
    const progress = currentIndex === nextIndex ? 0 : Math.min(1, Math.max(0, (anchor - centers[currentIndex]) / span));
    const selected = sections[progress < 0.5 ? currentIndex : nextIndex];
    stage.dataset.state = selected?.dataset.geometryState || '0';
    const side = selected?.dataset.geometrySide === 'left' ? -1 : 1;
    stage.dataset.side = side < 0 ? 'left' : 'right';
    return side * anchorOffset();
  };
  const panel = { x: readTargetAnchor(), velocity: 0, lift: 0, liftVelocity: 0, angle: BASE_TURN };
  let targetAnchor = panel.x;
  let scrollImpulse = 0;
  let lastScrollTop = window.scrollY;
  let lastScrollTime = 0;
  let physicsAccumulator = 0;
  let lastPhysicsTime = 0;
  let settling = false;
  const interaction = { x: 0, y: 0, strength: 0, phase: 0 };

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
      const mobile = window.innerWidth <= 840;
      const dpr = Math.min(window.devicePixelRatio || 1, mobile ? 1.25 : 1.5) * resolutionScale;
      const pixelBudget = mobile ? 900_000 : 1_500_000;
      let width = Math.max(1, Math.round(canvas.clientWidth * dpr));
      let height = Math.max(1, Math.round(canvas.clientHeight * dpr));
      const pixels = width * height;
      if (pixels > pixelBudget) {
        const scale = Math.sqrt(pixelBudget / pixels);
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

  const advancePanel = (timestamp) => {
    const elapsed = lastPhysicsTime ? Math.min(MAX_DT_SECONDS, Math.max(0, (timestamp - lastPhysicsTime) / 1000)) : FIXED_STEP_SECONDS;
    lastPhysicsTime = timestamp;
    physicsAccumulator = Math.min(MAX_DT_SECONDS, physicsAccumulator + elapsed);
    while (physicsAccumulator >= FIXED_STEP_SECONDS) {
      const anchorForce = ANCHOR_STIFFNESS * (targetAnchor - panel.x) - ANCHOR_DAMPING * panel.velocity;
      panel.velocity = Math.max(-MAX_PANEL_VELOCITY, Math.min(MAX_PANEL_VELOCITY, panel.velocity + anchorForce * FIXED_STEP_SECONDS));
      panel.x += panel.velocity * FIXED_STEP_SECONDS;
      const liftTarget = Math.max(-0.022, Math.min(0.022, -scrollImpulse * 0.035));
      const liftForce = LIFT_STIFFNESS * (liftTarget - panel.lift) - LIFT_DAMPING * panel.liftVelocity;
      panel.liftVelocity += liftForce * FIXED_STEP_SECONDS;
      panel.lift += panel.liftVelocity * FIXED_STEP_SECONDS;
      scrollImpulse *= Math.exp(-FIXED_STEP_SECONDS / 0.24);
      const tiltTarget = BASE_TURN + Math.max(-MAX_TILT, Math.min(MAX_TILT, panel.velocity * TILT_FROM_TRAVEL + panel.liftVelocity * TILT_FROM_LIFT));
      panel.angle += (tiltTarget - panel.angle) * (1 - Math.exp(-FIXED_STEP_SECONDS / 0.42));
      physicsAccumulator -= FIXED_STEP_SECONDS;
    }
    const panelSettled = Math.abs(targetAnchor - panel.x) <= SETTLE_DISTANCE && Math.abs(panel.velocity) <= SETTLE_VELOCITY &&
      Math.abs(panel.lift) <= SETTLE_DISTANCE && Math.abs(panel.liftVelocity) <= SETTLE_VELOCITY &&
      Math.abs(panel.angle - BASE_TURN) <= 0.0008 && Math.abs(scrollImpulse) <= 0.002;
    if (panelSettled) {
      panel.x = targetAnchor;
      panel.velocity = 0;
      panel.lift = 0;
      panel.liftVelocity = 0;
      panel.angle = BASE_TURN;
      scrollImpulse = 0;
    }
    settling = !panelSettled;
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
      const moving = advancePanel(timestamp);
      const canvasRect = canvas.getBoundingClientRect();
      const referenceHeight = Math.max(1, canvasRect.height);
      const interactionX = ((interaction.x - canvasRect.left) * 2 - canvasRect.width) / referenceHeight;
      const interactionY = ((interaction.y - canvasRect.top) * 2 - canvasRect.height) / referenceHeight;
      const cssPixelRatio = canvas.height / referenceHeight;
      const visibleHalfWidth = window.innerWidth / referenceHeight;
      const visibleHalfHeight = window.innerHeight / referenceHeight;
      device.queue.writeBuffer(uniformBuffer, 0, new Float32Array([
        canvas.width, canvas.height, panel.lift, panel.angle,
        panel.x, 0, 0, 0,
        interactionX, interactionY, interaction.strength, interaction.phase,
        cssPixelRatio, visibleHalfWidth, visibleHalfHeight, 0,
      ]));
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
    const now = performance.now();
    const scrollTop = window.scrollY;
    const elapsed = lastScrollTime ? Math.min(0.25, Math.max(0.008, (now - lastScrollTime) / 1000)) : 0.016;
    const normalizedVelocity = ((scrollTop - lastScrollTop) / Math.max(1, window.innerHeight)) / elapsed;
    scrollImpulse = Math.max(-3, Math.min(3, scrollImpulse * 0.72 + normalizedVelocity * 0.28));
    lastScrollTop = scrollTop;
    lastScrollTime = now;
    targetAnchor = readTargetAnchor();
    settling = true;
    stage.dataset.spring = 'settling';
    requestDraw();
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
    resize() { targetAnchor = readTargetAnchor(); settling = true; lastPhysicsTime = 0; resize(); requestDraw(); },
    setInteraction(next = {}) {
      interaction.x = Number.isFinite(next.x) ? next.x : interaction.x;
      interaction.y = Number.isFinite(next.y) ? next.y : interaction.y;
      interaction.strength = Math.min(1, Math.max(0, Number(next.strength) || 0));
      interaction.phase = Number(next.phase) || 0;
      requestDraw();
    },
    destroy() { teardown(); },
  };
  } catch {
    try { device.destroy(); } catch { /* Static fallback remains available. */ }
    stage.removeAttribute('data-enhanced');
    delete canvas.dataset.renderer;
    return null;
  }
}
