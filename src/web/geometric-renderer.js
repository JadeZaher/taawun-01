const shaderSource = `
  struct Uniforms { viewportScroll: vec4f, stageMotion: vec4f }
  @group(0) @binding(0) var<uniform> u: Uniforms;

  @vertex fn vertexMain(@builtin(vertex_index) i: u32) -> @builtin(position) vec4f {
    var p = array<vec2f, 3>(vec2f(-1.0, -3.0), vec2f(-1.0, 1.0), vec2f(3.0, 1.0));
    return vec4f(p[i], 0.0, 1.0);
  }
  fn rotate2(v: vec2f, a: f32) -> vec2f { let c = cos(a); let s = sin(a); return mat2x2f(c, -s, s, c) * v; }
  fn starDistance(p: vec2f) -> f32 {
    let angle = atan2(p.y, p.x);
    let radius = length(p);
    let boundary = mix(0.19, 0.47, pow(0.5 + 0.5 * cos(angle * 8.0), 1.8));
    return abs(radius - boundary);
  }
  fn lattice(point: vec2f, scale: f32, turn: f32) -> vec3f {
    let cell = fract(rotate2(point, turn) * scale) - vec2f(0.5);
    let line = min(starDistance(cell), abs(abs(cell.x) + abs(cell.y) - 0.49) * 0.62);
    return vec3f(smoothstep(0.055, 0.004, line), smoothstep(0.018, 0.002, line), line);
  }
  @fragment fn fragmentMain(@builtin(position) position: vec4f) -> @location(0) vec4f {
    let resolution = max(u.viewportScroll.xy, vec2f(1.0));
    let scroll = u.viewportScroll.z;
    let phase = u.viewportScroll.w;
    let section = u.stageMotion.x;
    var uv = (position.xy * 2.0 - resolution) / resolution.y;
    uv /= 1.0 + 0.10 * sin(uv.y * 2.2 + section * 0.7);
    let turn = (section - 2.0) * 0.026 + phase * 0.045;
    let baseLayer = lattice(uv + vec2f(scroll * 0.035, -scroll * 0.02), 3.4 + section * 0.12, turn);
    let deepLayer = lattice(uv * 1.12 + vec2f(0.08, -0.05), 4.3, -turn * 0.7);
    let lensCenter = vec2f(0.28 * sin(scroll * 3.4), -0.18 + phase * 0.26);
    let lensDistance = length(uv - lensCenter);
    let lens = smoothstep(0.66, 0.04, lensDistance);
    let refractedPoint = uv + normalize(uv - lensCenter + vec2f(0.001)) * lens * 0.04;
    let refracted = lattice(refractedPoint, 3.4 + section * 0.12, turn + lens * 0.045);
    let refractionEdge = abs(refracted.x - baseLayer.x);
    let caustic = 0.5 + 0.5 * sin(uv.x * 5.0 - uv.y * 3.0 - phase * 5.0);
    var color = vec3f(0.018, 0.052, 0.045);
    color += vec3f(0.16, 0.57, 0.45) * mix(baseLayer.x, refracted.x, lens) * (0.42 + caustic * 0.28);
    color += vec3f(0.95, 0.68, 0.24) * baseLayer.y * (0.44 + lens * 0.72);
    color += vec3f(0.16, 0.57, 0.45) * deepLayer.x * 0.2;
    color += vec3f(0.78, 0.24, 0.08) * refractionEdge * 1.1;
    color += vec3f(0.95, 0.68, 0.24) * pow(max(0.0, 1.0 - lensDistance), 5.0) * 0.14;
    return vec4f(color, 0.96);
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

  const teardown = ({ notify = false } = {}) => {
    if (!alive) return;
    alive = false;
    window.removeEventListener('scroll', onScroll);
    if (frame) cancelAnimationFrame(frame);
    if (delayedFrame) window.clearTimeout(delayedFrame);
    frame = 0;
    delayedFrame = 0;
    stage.removeAttribute('data-enhanced');
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

  const draw = () => {
    frame = 0;
    if (!alive || paused || document.hidden) return;
    try {
      const started = performance.now();
      const maxScroll = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
      const scroll = Math.min(1, Math.max(0, window.scrollY / maxScroll));
      let nearest = 0;
      let nearestDistance = Number.POSITIVE_INFINITY;
      for (const section of sections) {
        const rect = section.getBoundingClientRect();
        const distance = Math.abs(rect.top + rect.height * 0.5 - window.innerHeight * 0.5);
        if (distance < nearestDistance) { nearestDistance = distance; nearest = Number(section.dataset.geometryState || 0); }
      }
      const sectionRect = sections.find((section) => Number(section.dataset.geometryState || 0) === nearest)?.getBoundingClientRect();
      const phase = sectionRect ? Math.min(1, Math.max(0, 1 - sectionRect.top / Math.max(1, window.innerHeight))) : scroll;
      device.queue.writeBuffer(uniformBuffer, 0, new Float32Array([canvas.width, canvas.height, scroll, phase, nearest, 0, 0, 0]));
      const encoder = device.createCommandEncoder();
      const pass = encoder.beginRenderPass({ colorAttachments: [{ view: context.getCurrentTexture().createView(), clearValue: { r: 0.02, g: 0.055, b: 0.045, a: 1 }, loadOp: 'clear', storeOp: 'store' }] });
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
  const onScroll = () => requestDraw();
  window.addEventListener('scroll', onScroll, { passive: true });
  device.lost.then(() => teardown({ notify: true }));
  resize();
  if (!alive) return null;
  requestDraw();
  return {
    active() { return alive; },
    pause() { paused = true; if (frame) cancelAnimationFrame(frame); if (delayedFrame) window.clearTimeout(delayedFrame); frame = 0; delayedFrame = 0; },
    resume() { paused = false; requestDraw(); },
    resize() { resize(); requestDraw(); },
    destroy() { teardown(); },
  };
  } catch {
    try { device.destroy(); } catch { /* Static fallback remains available. */ }
    stage.removeAttribute('data-enhanced');
    delete canvas.dataset.renderer;
    return null;
  }
}
