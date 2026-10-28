// @ts-nocheck
import * as THREE from '../vendor/three.module.js';
import { OrbitControls } from '../vendor/OrbitControls.js';
import { SERVER } from './config.js';
import { C, Z } from './countries.js';
const MAX = 40, ease = (t) => 1 - Math.pow(1 - t, 3);
// 热区配色：低→高 依次为 暗黄 → 亮黄 → 橙 → 红
const STOPS = [[0, [110, 92, 16]], [0.35, [222, 176, 32]], [0.65, [238, 118, 26]], [1, [255, 42, 42]]];
export function heatColor(t) {
    const k = Math.min(1, Math.max(0, t));
    let i = 0;
    while (i < STOPS.length - 2 && k > STOPS[i + 1][0])
        i++;
    const [p0, c0] = STOPS[i], [p1, c1] = STOPS[i + 1], u = (k - p0) / (p1 - p0 || 1);
    return 'rgb(' + Math.round(c0[0] + (c1[0] - c0[0]) * u) + ',' + Math.round(c0[1] + (c1[1] - c0[1]) * u) + ',' + Math.round(c0[2] + (c1[2] - c0[2]) * u) + ')';
}
function ll(lat, lon, r = 1) {
    const p = (90 - lat) * Math.PI / 180, t = (lon + 180) * Math.PI / 180;
    return new THREE.Vector3(-r * Math.sin(p) * Math.cos(t), r * Math.cos(p), r * Math.sin(p) * Math.sin(t));
}
function toLL(p) {
    const v = p.clone().normalize();
    const lat = 90 - Math.acos(Math.max(-1, Math.min(1, v.y))) * 180 / Math.PI;
    let lon = Math.atan2(v.z, -v.x) * 180 / Math.PI - 180;
    while (lon < -180)
        lon += 360;
    while (lon > 180)
        lon -= 360;
    return [lat, lon];
}
export class Globe {
    el;
    r;
    s;
    cam;
    ctl;
    earth;
    earthMat;
    arcs = [];
    reduce;
    last;
    serverPos;
    homeGroup;
    homeRings = [];
    constructor(el) {
        this.el = el;
        this.serverPos = { lat: SERVER.lat, lon: SERVER.lon, name: SERVER.name || '本机服务器' };
        this.reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
        this.r = new THREE.WebGLRenderer({ canvas: el, antialias: true, alpha: true });
        this.r.setPixelRatio(Math.min(devicePixelRatio, 2));
        this.s = new THREE.Scene();
        this.cam = new THREE.PerspectiveCamera(40, 1, 0.1, 50);
        this.cam.position.set(0, 0.6, 9);
        this.ctl = new OrbitControls(this.cam, el);
        Object.assign(this.ctl, { enablePan: false, enableDamping: true, minDistance: 1.6, maxDistance: 6, rotateSpeed: 0.5, zoomSpeed: 0.6, autoRotate: !this.reduce, autoRotateSpeed: 0.35 });
        this.ctl.addEventListener('start', () => { this.focusT = 1; this.ctl.autoRotate = false; });
        this.ctl.addEventListener('end', () => { this.idle = performance.now(); });
        this.earthMat = new THREE.MeshBasicMaterial({ color: 0x0a0a0a });
        this.earth = new THREE.Mesh(new THREE.SphereGeometry(1, 96, 96), this.earthMat);
        this.s.add(this.earth);
        this.s.add(new THREE.Mesh(new THREE.SphereGeometry(1.14, 64, 64), new THREE.ShaderMaterial({
            transparent: true, side: THREE.BackSide, blending: THREE.AdditiveBlending, depthWrite: false,
            vertexShader: 'varying vec3 n;void main(){n=normalize(normalMatrix*normal);gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}',
            fragmentShader: 'varying vec3 n;void main(){float i=pow(max(0.,0.68-dot(n,vec3(0.,0.,1.))),3.);gl_FragColor=vec4(vec3(.85),1.)*i;}'
        })));
        // 创建本机服务器位置标记及动态涟漪扩散环
        this.homeGroup = new THREE.Group();
        this.createHomeMarker();
        this.s.add(this.homeGroup);
        this.ro = new ResizeObserver(() => this.resize());
        this.ro.observe(el.parentElement);
        this.resize();
        this.ray = new THREE.Raycaster();
        this.hlG = new THREE.Group();
        this.s.add(this.hlG);
        this.bindPointer();
        this.t0 = performance.now();
        this.last = this.t0;
        this.loop = this.loop.bind(this);
        this.raf = requestAnimationFrame(this.loop);
    }
    createHomeMarker() {
        while (this.homeGroup.children.length > 0) {
            const obj = this.homeGroup.children[0];
            this.homeGroup.remove(obj);
            obj.geometry?.dispose();
            obj.material?.dispose();
        }
        this.homeRings = [];
        const p = ll(this.serverPos.lat, this.serverPos.lon, 1.006);
        const norm = p.clone().normalize();
        // 核心高亮白点
        const core = new THREE.Mesh(new THREE.SphereGeometry(0.016, 16, 16), new THREE.MeshBasicMaterial({ color: 0x00f3ff }));
        core.position.copy(p);
        this.homeGroup.add(core);
        // 发光柱（直插云霄的光束，方便远距离一眼认出服务器位置）
        const beamGeo = new THREE.CylinderGeometry(0.002, 0.008, 0.22, 16);
        beamGeo.translate(0, 0.11, 0);
        beamGeo.rotateX(Math.PI / 2);
        const beamMat = new THREE.MeshBasicMaterial({ color: 0x00e1ff, transparent: true, opacity: 0.65, blending: THREE.AdditiveBlending, depthWrite: false });
        const beam = new THREE.Mesh(beamGeo, beamMat);
        beam.position.copy(p);
        beam.lookAt(p.clone().add(norm));
        this.homeGroup.add(beam);
        // 3层连续向外扩散的涟漪环
        for (let i = 0; i < 3; i++) {
            const ringGeo = new THREE.RingGeometry(0.012, 0.02, 32);
            const ringMat = new THREE.MeshBasicMaterial({
                color: 0x00f3ff,
                transparent: true,
                opacity: 0.8,
                side: THREE.DoubleSide,
                depthWrite: false,
                blending: THREE.AdditiveBlending
            });
            const ring = new THREE.Mesh(ringGeo, ringMat);
            ring.position.copy(p.clone().multiplyScalar(1.002));
            ring.lookAt(p.clone().add(norm));
            ring.userData = { phase: i * 0.33, speed: 0.55 };
            this.homeGroup.add(ring);
            this.homeRings.push(ring);
        }
    }
    // 动态更新服务器经纬度与名称
    setServerLocation(lat, lon, name) {
        if (typeof lat !== 'number' || typeof lon !== 'number' || isNaN(lat) || isNaN(lon))
            return;
        this.serverPos = { lat, lon, name: name || '本机服务器' };
        this.createHomeMarker();
    }
    // ---- 指针拾取：射线打到球面 → 经纬度 → 查 ID 贴图得到国家 ----
    bindPointer() {
        const el = this.el;
        el.addEventListener('pointerdown', e => { this.down = [e.clientX, e.clientY]; this.moved = false; });
        el.addEventListener('pointerup', () => { this.down = null; if (this.moved) {
            this.moved = false;
            this.emitHover(null);
        } });
        el.addEventListener('pointermove', e => {
            if (this.down && Math.hypot(e.clientX - this.down[0], e.clientY - this.down[1]) > 6)
                this.moved = true;
            this.xy = [e.clientX, e.clientY];
            if (this.hRaf)
                return;
            this.hRaf = requestAnimationFrame(() => { this.hRaf = 0; this.probe(); });
        });
        el.addEventListener('pointerleave', () => { this.down = null; this.emitHover(null); });
        el.addEventListener('click', () => { if (!this.moved && this.hov && this.onPick)
            this.onPick(this.hov); });
    }
    probe() {
        if (!this.ic || !this.xy || this.paused || this.down)
            return;
        const r = this.el.getBoundingClientRect(), [cx, cy] = this.xy;
        if (cx < r.left || cx > r.right || cy < r.top || cy > r.bottom)
            return this.emitHover(null);
        const ndc = new THREE.Vector2(((cx - r.left) / r.width) * 2 - 1, -((cy - r.top) / r.height) * 2 + 1);
        this.ray.setFromCamera(ndc, this.cam);
        const hit = this.ray.intersectObject(this.earth, false)[0];
        if (!hit)
            return this.emitHover(null);
        const [lat, lon] = toLL(hit.point);
        // 优先检测是否悬浮/点击到了本机服务器坐标（阈值 3.5 度内）
        if (this.serverPos && Math.hypot(lat - this.serverPos.lat, lon - this.serverPos.lon) < 3.5) {
            this.emitHover({ is_server: true, lat: this.serverPos.lat, lon: this.serverPos.lon, name: this.serverPos.name }, cx, cy);
            return;
        }
        this.emitHover(this.pick(lat, lon), cx, cy);
    }
    // 在 ID 贴图上采样（7×7 取众数，抗边界混色）
    pick(lat, lon) {
        if (!this.ig || !this.coll)
            return null;
        const px = Math.round((lon + 180) / 360 * this.w), py = Math.round((90 - lat) / 180 * this.h);
        const sx = Math.max(0, Math.min(this.w - 7, px - 3)), sy = Math.max(0, Math.min(this.h - 7, py - 3));
        const d = this.ig.getImageData(sx, sy, 7, 7).data, cnt = new Map();
        for (let i = 0; i < d.length; i += 4) {
            const r = d[i], g = d[i + 1], b = d[i + 2];
            if (!r && !g && !b)
                continue;
            const k = (r - 1) + (g - 1) * 250 + (b - 1) * 62500;
            cnt.set(k, (cnt.get(k) || 0) + 1);
        }
        let best = -1, bn = 0;
        for (const [k, v] of cnt)
            if (v > bn) {
                bn = v;
                best = k;
            }
        if (best < 0)
            return null;
        const f = this.coll.features[best];
        if (!f || f.id == null || !C[f.id])
            return null;
        const [cc, lat0, lon0, en] = C[f.id];
        return { id: f.id, cc, en, zh: Z[cc] || en, lat: lat0, lon: lon0 };
    }
    emitHover(info, cx, cy) {
        const cur = this.hov?.cc ?? null, next = info?.cc ?? null;
        if (cur !== next) {
            this.hov = info;
            this.highlight(info);
            if (this.onHover)
                this.onHover(info, cx, cy);
        }
        else if (info && this.onMove && cx != null) {
            this.onMove(cx, cy);
        }
    }
    // 高亮悬浮国家轮廓：在 3D 球面上画白色轮廓线
    highlight(info) {
        while (this.hlG.children.length) {
            const o = this.hlG.children.pop();
            o.geometry.dispose();
            o.material.dispose();
        }
        if (!info || !this.topo || !this.coll)
            return;
        const f = this.coll.features.find((x) => x.id === info.id);
        if (!f)
            return;
        const rings = [];
        const g = f.geometry;
        if (g.type === 'Polygon')
            rings.push(...g.coordinates);
        else if (g.type === 'MultiPolygon')
            g.coordinates.forEach((p) => rings.push(...p));
        for (const ring of rings) {
            if (ring.length < 3)
                continue;
            const pts = ring.map(([lo, la]) => ll(la, lo, 1.004));
            this.hlG.add(new THREE.Line(new THREE.BufferGeometry().setFromPoints(pts), new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, opacity: .9, depthWrite: false })));
        }
    }
    async loadGeo() {
        const [{ feature, mesh }, { geoEquirectangular, geoPath }] = [window.topojson, window.d3]
    const topo = await (await fetch('./vendor/countries-110m.json', { cache: 'force-cache' })).json()
        const t = topo, W = 4096, H = 2048, c = document.createElement('canvas');
        c.width = W;
        c.height = H;
        const g = c.getContext('2d');
        this.topo = t;
        this.feat = feature;
        this.mesh = mesh;
        this.cv = c;
        this.cg = g;
        this.w = W;
        this.h = H;
        const proj = geoEquirectangular().fitSize([W, H], { type: 'Sphere' });
        this.proj = proj;
        this.geoPathFn = geoPath;
        this.path = geoPath(proj, g);
        this.coll = feature(t, t.objects.countries);
        // ID 贴图：每个国家填唯一颜色，用于悬浮拾取
        const ic = document.createElement('canvas');
        ic.width = W;
        ic.height = H;
        const ig = ic.getContext('2d', { willReadFrequently: true });
        ig.fillStyle = '#000';
        ig.fillRect(0, 0, W, H);
        const ipath = geoPath(proj, ig);
        this.ic = ic;
        this.ig = ig;
        this.coll.features.forEach((f, i) => {
            const r = (i % 250) + 1, gg = (Math.floor(i / 250) % 250) + 1, b = (Math.floor(i / 62500) % 250) + 1;
            ig.beginPath();
            ipath(f);
            ig.fillStyle = `rgb(${r},${gg},${b})`;
            ig.fill();
        });
        // alpha2 → 中心坐标，供攻击路径与镜头聚焦使用
        this.llByCC = new Map();
        for (const f of this.coll.features) {
            const cc = f.id != null && C[f.id] ? C[f.id][0] : '';
            if (cc && !this.llByCC.has(cc))
                this.llByCC.set(cc, [C[f.id][1], C[f.id][2]]);
        }
        const tex = new THREE.CanvasTexture(c);
        tex.anisotropy = 8;
        tex.colorSpace = THREE.SRGBColorSpace;
        this.texObj = tex;
        this.tex = c;
        this.earthMat.map = tex;
        this.earthMat.color.set(0xffffff);
        this.earthMat.needsUpdate = true;
        this.paint();
    }
    // 重绘底图纹理：heat 为 alpha2 → 强度（0..max），null 恢复默认底色
    paint(heat) {
        const g = this.cg, W = this.w, H = this.h, path = this.path;
        if (!g || !this.coll)
            return;
        g.clearRect(0, 0, W, H);
        g.fillStyle = '#070707';
        g.fillRect(0, 0, W, H);
        const mx = heat && heat.size ? Math.max(...heat.values()) || 1 : 1;
        for (const f of this.coll.features) {
            const cc = f.id != null && C[f.id] ? C[f.id][0] : '';
            const v = heat && cc ? heat.get(cc) : undefined;
            g.beginPath();
            path(f);
            g.fillStyle = v === undefined ? '#1c1c1c' : heatColor(v / mx);
            g.fill();
        }
        g.beginPath();
        path(this.mesh(this.topo, this.topo.objects.countries, (a, b) => a !== b));
        g.strokeStyle = 'rgba(255,255,255,.22)';
        g.lineWidth = 1.4;
        g.stroke();
        g.beginPath();
        path(this.feat(this.topo, this.topo.objects.land));
        g.strokeStyle = 'rgba(255,255,255,.55)';
        g.lineWidth = 2;
        g.stroke();
        if (this.texObj)
            this.texObj.needsUpdate = true;
    }
    setHeat(heat) { this.heat = heat; this.paint(heat); }
    // 国家轮廓（贴图坐标系 4096×2048 的 Path2D），供 2D 地图按缩放描边高亮
    outline(cc) {
        if (!this.geoPathFn || !this.coll)
            return null;
        const f = this.coll.features.find(x => x.id != null && C[x.id] && C[x.id][0] === cc);
        if (!f)
            return null;
        if (!this._p2)
            this._p2 = new WeakMap();
        let p = this._p2.get(f);
        if (!p) {
            p = new Path2D();
            this.geoPathFn(this.proj, p)(f);
            this._p2.set(f, p);
        }
        return p;
    }
    resize() { const p = this.el.parentElement; this.r.setSize(p.clientWidth, p.clientHeight, false); this.cam.aspect = p.clientWidth / p.clientHeight; this.cam.updateProjectionMatrix(); }
    intro() { this.introT = performance.now(); this.from = 9; this.to = 3.4; }
    fire(ev, strong = ev.level) {
        if (!ev.src)
            return;
        let a = this.arcs.find(x => !x.on) || this.arcs.length < MAX && this.make() || this.arcs.reduce((o, x) => x.t0 < o.t0 ? x : o);
        const p0 = ll(ev.src.lat, ev.src.lon), p1 = ll(this.serverPos.lat, this.serverPos.lon);
        const mid = p0.clone().add(p1).multiplyScalar(.5).normalize().multiplyScalar(1.08 + p0.distanceTo(p1) * .28);
        a.pts = new THREE.QuadraticBezierCurve3(p0, mid, p1).getPoints(63);
        a.geo.setFromPoints(a.pts);
        a.ev = ev;
        a.on = true;
        a.t0 = performance.now();
        a.k = .35 + strong * .2;
        a.ring.position.copy(p1.clone().multiplyScalar(1.003));
        a.ring.lookAt(p1.clone().multiplyScalar(2));
    }
    make() {
        const geo = new THREE.BufferGeometry(), mat = new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, depthWrite: false });
        const line = new THREE.Line(geo, mat), dot = new THREE.Mesh(new THREE.SphereGeometry(.011, 12, 12), new THREE.MeshBasicMaterial({ color: 0xffffff }));
        const ring = new THREE.Mesh(new THREE.RingGeometry(.97, 1, 48), new THREE.MeshBasicMaterial({ color: 0xffffff, transparent: true, side: THREE.DoubleSide, depthWrite: false }));
        this.s.add(line, dot, ring);
        const a = { geo, mat, line, dot, ring, on: false, t0: 0, pts: [] };
        this.arcs.push(a);
        return a;
    }
    select(id) { this.sel = id; }
    focus(ev, solo) {
        if (!ev?.src)
            return;
        this.ctl.autoRotate = false;
        this.f0 = this.cam.position.clone();
        this.f1 = (solo ? ll(ev.src.lat, ev.src.lon) : ll(ev.src.lat, ev.src.lon).add(ll(this.serverPos.lat, this.serverPos.lon))).normalize().multiplyScalar(2.7);
        this.focusT = 0;
        this.fs = performance.now();
    }
    focusLL(lat, lon, dist = 2.55) {
        this.ctl.autoRotate = false;
        this.f0 = this.cam.position.clone();
        this.f1 = ll(lat, lon).normalize().multiplyScalar(dist);
        this.focusT = 0;
        this.fs = performance.now();
    }
    // 平滑聚焦到本机服务器位置
    focusHome(dist = 2.4) {
        this.focusLL(this.serverPos.lat, this.serverPos.lon, dist);
    }
    clear() { this.arcs.forEach(a => (a.on = false)); }
    // ---- 国家间攻击路径：管状光带 + 流动彗尾 + 落点脉冲 ----
    setPairs(pairs = []) {
        this.disposePairs();
        if (!pairs.length || !this.llByCC)
            return;
        const mx = Math.max(...pairs.map(p => p.value)) || 1;
        const g = new THREE.Group();
        this.s.add(g);
        this.pairG = g;
        const items = [];
        pairs.forEach((p, i) => {
            const a = this.llByCC.get(p.from), b = this.llByCC.get(p.to);
            if (!a || !b || p.from === p.to)
                return;
            const k = Math.min(1, p.value / mx);
            const p0 = ll(a[0], a[1], 1.004), p1 = ll(b[0], b[1], 1.004);
            const d = p0.distanceTo(p1);
            const mid = p0.clone().add(p1).normalize().multiplyScalar(1 + .08 + d * .36);
            const curve = new THREE.QuadraticBezierCurve3(p0, mid, p1);
            const col = new THREE.Color(k > .7 ? 0xff3b2f : k > .35 ? 0xff7a1f : 0xffb02e);
            const tubeMat = new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: 0, blending: THREE.AdditiveBlending, depthWrite: false });
            const tube = new THREE.Mesh(new THREE.TubeGeometry(curve, 76, .0016 + k * .0028, 6, false), tubeMat);
            const N = 24, pos = new Float32Array(N * 3), al = new Float32Array(N);
            const pg = new THREE.BufferGeometry();
            pg.setAttribute('position', new THREE.BufferAttribute(pos, 3));
            pg.setAttribute('a', new THREE.BufferAttribute(al, 1));
            const pm = new THREE.ShaderMaterial({
                transparent: true, depthWrite: false, blending: THREE.AdditiveBlending,
                uniforms: { c: { value: col }, pr: { value: this.r.getPixelRatio() } },
                vertexShader: 'attribute float a;varying float v;uniform float pr;void main(){v=a;gl_PointSize=(1.6+4.2*a)*pr;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}',
                fragmentShader: 'varying float v;uniform vec3 c;void main(){float d=length(gl_PointCoord-.5);if(d>.5)discard;gl_FragColor=vec4(c, v*(1.-d*1.8));}',
            });
            const pts = new THREE.Points(pg, pm);
            const ring = new THREE.Mesh(new THREE.RingGeometry(.86, 1, 40), new THREE.MeshBasicMaterial({ color: col, transparent: true, opacity: 0, side: THREE.DoubleSide, depthWrite: false, blending: THREE.AdditiveBlending }));
            ring.position.copy(p1);
            ring.lookAt(p1.clone().multiplyScalar(2));
            g.add(tube, pts, ring);
            items.push({ curve, tubeMat, pts, pg, pos, al, ring, k, dur: 2800 - k * 1100, phase: (i * .27) % 1, delay: i * 55, cc: [p.from, p.to] });
        });
        this.pairItems = items;
        this.pairT0 = performance.now();
    }
    disposePairs() {
        if (this.pairG) {
            this.s.remove(this.pairG);
            this.pairG.traverse(o => { o.geometry?.dispose(); o.material?.dispose(); });
        }
        this.pairG = null;
        this.pairItems = [];
    }
    setPairFocus(cc) { this.pairFocus = cc || null; }
    drawPairs(now) {
        const items = this.pairItems;
        if (!items || !items.length)
            return;
        const foc = this.pairFocus, reduce = this.reduce, N = 24;
        for (const it of items) {
            const age = now - this.pairT0 - it.delay;
            if (age < 0) {
                it.tubeMat.opacity = 0;
                it.pts.visible = it.ring.visible = false;
                continue;
            }
            const grow = reduce ? 1 : Math.min(1, age / 800);
            const dim = foc ? (it.cc[0] === foc || it.cc[1] === foc ? 1 : .1) : 1;
            const pulse = reduce ? 1 : .8 + .2 * Math.sin(now / 620 + it.phase * 7);
            it.tubeMat.opacity = (.38 + it.k * .52) * grow * dim * pulse;
            if (reduce) {
                it.pts.visible = it.ring.visible = false;
                continue;
            }
            const u = ((age / it.dur) + it.phase) % 1;
            for (let i = 0; i < N; i++) {
                const uu = u - i * .014;
                if (uu < 0) {
                    it.al[i] = 0;
                    continue;
                }
                const v = it.curve.getPoint(uu);
                it.pos[i * 3] = v.x;
                it.pos[i * 3 + 1] = v.y;
                it.pos[i * 3 + 2] = v.z;
                it.al[i] = Math.pow(1 - i / N, 1.5) * (.55 + it.k * .45) * grow * dim;
            }
            it.pts.visible = true;
            it.pg.attributes.position.needsUpdate = true;
            it.pg.attributes.a.needsUpdate = true;
            const r = .9 + .1 * ((age / it.dur + it.phase) % 1);
            const rt = (r - .9) / .1;
            it.ring.visible = true;
            it.ring.scale.setScalar(.012 + rt * (.045 + it.k * .05));
            it.ring.material.opacity = (1 - rt) * dim * .9;
        }
    }
    setSpots(list) {
        if (this.spots) {
            this.s.remove(this.spots);
            this.spots.geometry.dispose();
            this.spots.material.dispose();
            this.spots = null;
        }
        if (!list.length)
            return;
        const p = new Float32Array(list.length * 3), z = new Float32Array(list.length);
        list.forEach((e, i) => { ll(e.lat, e.lon, 1.004).toArray(p, i * 3); z[i] = e.w; });
        const g = new THREE.BufferGeometry();
        g.setAttribute('position', new THREE.BufferAttribute(p, 3));
        g.setAttribute('z', new THREE.BufferAttribute(z, 1));
        this.spots = new THREE.Points(g, new THREE.ShaderMaterial({ transparent: true, depthWrite: false,
            vertexShader: 'attribute float z;varying float a;void main(){a=.35+.65*z;gl_PointSize=(3.+9.*z)*' + this.r.getPixelRatio().toFixed(1) + ';gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}',
            fragmentShader: 'varying float a;void main(){float d=length(gl_PointCoord-.5);if(d>.5)discard;gl_FragColor=vec4(1.,1.,1.,a*(1.-d*1.6));}' }));
        this.s.add(this.spots);
    }
    loop(now) {
        this.raf = requestAnimationFrame(this.loop);
        if (document.hidden || this.paused)
            return;
        this.last = now;
        if (this.introT) {
            const k = Math.min(1, (now - this.introT) / 2600);
            const d = this.from + (this.to - this.from) * ease(k);
            this.cam.position.setLength(d);
            if (k >= 1) {
                this.introT = 0;
                this.ctl.autoRotate = !this.reduce;
            }
        }
        if (this.focusT < 1) {
            this.focusT = Math.min(1, (now - this.fs) / 1400);
            const k = ease(this.focusT);
            const dir = this.f0.clone().normalize().lerp(this.f1.clone().normalize(), k).normalize();
            this.cam.position.copy(dir.multiplyScalar(this.f0.length() + (this.f1.length() - this.f0.length()) * k));
        }
        else if (!this.ctl.autoRotate && !this.introT && this.idle && now - this.idle > 8000 && !this.reduce) {
            this.ctl.autoRotate = true;
            this.idle = 0;
        }
        // 更新服务器标记涟漪扩散动画
        if (this.homeRings && this.homeRings.length > 0) {
            const sec = now / 1000;
            this.homeRings.forEach(ring => {
                const u = ((sec * ring.userData.speed + ring.userData.phase) % 1);
                // 缩放从小扩展到大 (0.5 到 3.6 倍)
                const scale = 0.5 + u * 3.1;
                ring.scale.set(scale, scale, scale);
                // 透明度从明显淡出到 0
                ring.material.opacity = (1 - u) * 0.85;
            });
        }
        for (const a of this.arcs) {
            if (!a.on) {
                a.line.visible = a.dot.visible = a.ring.visible = false;
                continue;
            }
            const t = (now - a.t0) / 1000, fly = Math.min(1, t / 1.3), pinned = this.sel === a.ev.id;
            const fade = pinned ? 1 : Math.max(0, 1 - Math.max(0, t - 6) / 3), dim = this.sel && !pinned ? .2 : 1;
            if (fade <= 0) {
                a.on = false;
                continue;
            }
            const n = Math.max(2, Math.floor(ease(fly) * 64));
            a.geo.setDrawRange(0, n);
            a.mat.opacity = a.k * fade * dim;
            a.line.visible = true;
            a.dot.visible = fly < 1;
            a.dot.position.copy(a.pts[Math.min(63, n - 1)]);
            const rt = t - 1.3;
            a.ring.visible = rt > 0 && rt < 1.6;
            if (a.ring.visible) {
                const s = .02 + rt * .055 * (1 + a.k);
                a.ring.scale.setScalar(s);
                a.ring.material.opacity = (1 - rt / 1.6) * fade * dim;
            }
        }
        this.drawPairs(now);
        this.ctl.update();
        this.r.render(this.s, this.cam);
    }
    dispose() {
        cancelAnimationFrame(this.raf);
        this.ro.disconnect();
        this.ctl.dispose();
        this.s.traverse(o => { o.geometry?.dispose(); const m = o.material; if (m) {
            m.map?.dispose();
            m.dispose();
        } });
        this.r.dispose();
    }
}
