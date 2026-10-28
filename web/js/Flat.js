// @ts-nocheck
import { SERVER } from './config.js';
import { heatColor } from './Globe.js';
const ease = (t) => 1 - Math.pow(1 - t, 3);
export class Flat {
    el;
    c;
    tex = null;
    spots = [];
    arcs = [];
    pairs = [];
    pairT0 = 0;
    sel = null;
    hov = null;
    serverPos;
    constructor(el) {
        this.el = el;
        this.serverPos = { lat: SERVER.lat, lon: SERVER.lon, name: SERVER.name || '本机服务器' };
        this.c = el.getContext('2d');
        this.ro = new ResizeObserver(() => this.resize());
        this.ro.observe(el.parentElement);
        this.resize();
        this.bind();
        this.loop = this.loop.bind(this);
        this.raf = requestAnimationFrame(this.loop);
    }
    setServerLocation(lat, lon, name) {
        if (typeof lat !== 'number' || typeof lon !== 'number' || isNaN(lat) || isNaN(lon))
            return;
        this.serverPos = { lat, lon, name: name || '本机服务器' };
    }
    resize() {
        const p = this.el.parentElement, dpr = Math.min(devicePixelRatio, 2);
        this.w = p.clientWidth;
        this.h = p.clientHeight;
        this.el.width = this.w * dpr;
        this.el.height = this.h * dpr;
        this.c.setTransform(dpr, 0, 0, dpr, 0, 0);
    }
    pt(lat, lon) {
        const x = (lon + 180) / 360 * this.w;
        const y = (90 - lat) / 180 * this.h;
        return [x, y];
    }
    toLL(x, y) {
        return [90 - (y / this.h) * 180, (x / this.w) * 360 - 180];
    }
    bind() {
        const el = this.el;
        el.addEventListener('pointermove', e => {
            const r = el.getBoundingClientRect(), x = e.clientX - r.left, y = e.clientY - r.top;
            const [lat, lon] = this.toLL(x, y);
            if (Math.hypot(lat - this.serverPos.lat, lon - this.serverPos.lon) < 4.0) {
                if (this.onHover)
                    this.onHover({ is_server: true, lat: this.serverPos.lat, lon: this.serverPos.lon, name: this.serverPos.name }, e.clientX, e.clientY);
                return;
            }
            if (this.onHover)
                this.onHover({ lat, lon }, e.clientX, e.clientY);
        });
        el.addEventListener('pointerleave', () => { if (this.onHover)
            this.onHover(null); });
        el.addEventListener('click', e => {
            const r = el.getBoundingClientRect();
            const [lat, lon] = this.toLL(e.clientX - r.left, e.clientY - r.top);
            if (Math.hypot(lat - this.serverPos.lat, lon - this.serverPos.lon) < 4.0) {
                if (this.onPick)
                    this.onPick({ is_server: true, lat: this.serverPos.lat, lon: this.serverPos.lon, name: this.serverPos.name });
                return;
            }
            if (this.onPick)
                this.onPick({ lat, lon });
        });
    }
    setSpots(l) { this.spots = l; }
    setPairs(pairs = []) {
        this.pairs = pairs.map(p => ({ ...p, a: p.a, b: p.b, k: p.k }));
        this.pairT0 = performance.now();
    }
    setPairFocus(cc) { this.foc = cc || null; }
    fire(ev, strong = ev.level) {
        if (!ev.src)
            return;
        this.arcs.push({ ev, k: .35 + strong * .2, t0: performance.now() });
    }
    select(id) { this.sel = id; }
    loop(now) {
        this.raf = requestAnimationFrame(this.loop);
        const g = this.c, W = this.w, H = this.h;
        g.clearRect(0, 0, W, H);
        if (this.tex) {
            g.drawImage(this.tex, 0, 0, W, H);
        }
        // 国家间攻击路径
        this.pairs.forEach((p, i) => {
            const [x0, y0] = this.pt(p.a[0], p.a[1]), [x1, y1] = this.pt(p.b[0], p.b[1]);
            const dx = x1 - x0, dy = y1 - y0, mx = (x0 + x1) / 2 - dy * .18, my = (y0 + y1) / 2 + dx * .18;
            const age = now - (this.pairT0 || 0) - i * 55;
            if (age < 0)
                return;
            const grow = Math.min(1, age / 800);
            const dim = this.foc ? (p.from === this.foc || p.to === this.foc ? 1 : .1) : 1;
            const col = heatColor(.45 + p.k * .55);
            const bez = u => { const v = 1 - u; return [v * v * x0 + 2 * v * u * mx + u * u * x1, v * v * y0 + 2 * v * u * my + u * u * y1]; };
            g.strokeStyle = col;
            g.globalAlpha = (.22 + p.k * .3) * grow * dim;
            g.lineWidth = .9 + p.k * 1.8;
            g.beginPath();
            g.moveTo(x0, y0);
            g.quadraticCurveTo(mx, my, x1, y1);
            g.stroke();
            // 流动光段
            const u1 = ((age / (2800 - p.k * 1100)) + i * .17) % 1, u0 = Math.max(0, u1 - .22);
            if (u1 > u0) {
                g.globalAlpha = .95 * dim;
                g.lineWidth = 1.6 + p.k * 2.2;
                g.lineCap = 'round';
                g.shadowColor = col;
                g.shadowBlur = 8 + p.k * 8;
                g.beginPath();
                const [ax, ay] = bez(u0);
                g.moveTo(ax, ay);
                for (let j = 1; j <= 10; j++) {
                    const [bx, by] = bez(u0 + (u1 - u0) * j / 10);
                    g.lineTo(bx, by);
                }
                g.stroke();
                g.shadowBlur = 0;
                const [hx0, hy0] = bez(u1);
                g.fillStyle = '#fff';
                g.globalAlpha = dim;
                g.beginPath();
                g.arc(hx0, hy0, 1.6 + p.k * 1.8, 0, 6.3);
                g.fill();
            }
            // 落点脉冲环
            const rt = ((age / (2800 - p.k * 1100)) + i * .17) % 1;
            if (rt > .9) {
                const q = (rt - .9) / .1;
                g.globalAlpha = (1 - q) * .85 * dim;
                g.strokeStyle = col;
                g.lineWidth = 1.4;
                g.beginPath();
                g.arc(x1, y1, 3 + q * (12 + p.k * 14), 0, 6.3);
                g.stroke();
            }
            g.globalAlpha = 1;
        });
        for (const sp of this.spots) {
            const [x, y] = this.pt(sp.lat, sp.lon);
            g.fillStyle = `rgba(255,255,255,${.3 + .6 * sp.w})`;
            g.beginPath();
            g.arc(x, y, 1.5 + 5 * sp.w, 0, 6.3);
            g.fill();
        }
        const [hx, hy] = this.pt(this.serverPos.lat, this.serverPos.lon);
        // 本机服务器 2D 涟漪波纹
        const sec = now / 1000;
        for (let i = 0; i < 3; i++) {
            const u = (sec * 0.55 + i * 0.33) % 1;
            const r = 4 + u * 32;
            const op = (1 - u) * 0.75;
            g.strokeStyle = `rgba(0, 243, 255, ${op})`;
            g.lineWidth = 1.5;
            g.beginPath();
            g.arc(hx, hy, r, 0, Math.PI * 2);
            g.stroke();
        }
        this.arcs = this.arcs.filter(a => {
            const t = (now - a.t0) / 1000, pinned = this.sel === a.ev.id, fade = pinned ? 1 : Math.max(0, 1 - Math.max(0, t - 6) / 3), dim = this.sel && !pinned ? .2 : 1;
            if (fade <= 0)
                return false;
            const [x0, y0] = this.pt(a.ev.src.lat, a.ev.src.lon), cx = (x0 + hx) / 2, cy = (y0 + hy) / 2 - Math.hypot(hx - x0, hy - y0) * .25;
            const fly = Math.min(1, t / 1.3), n = Math.max(2, Math.floor(ease(fly) * 48));
            let px = x0, py = y0;
            g.strokeStyle = `rgba(255,255,255,${a.k * fade * dim})`;
            g.lineWidth = 1 + a.k;
            g.beginPath();
            g.moveTo(x0, y0);
            for (let i = 1; i <= n; i++) {
                const u = i / 48, v = 1 - u;
                px = v * v * x0 + 2 * v * u * cx + u * u * hx;
                py = v * v * y0 + 2 * v * u * cy + u * u * hy;
                g.lineTo(px, py);
            }
            g.stroke();
            if (fly < 1) {
                g.fillStyle = '#fff';
                g.beginPath();
                g.arc(px, py, 2.5, 0, 6.3);
                g.fill();
            }
            const rt = t - 1.3;
            if (rt > 0 && rt < 1.6) {
                g.strokeStyle = `rgba(255,255,255,${(1 - rt / 1.6) * fade * dim})`;
                g.lineWidth = 1.2;
                g.beginPath();
                g.arc(hx, hy, 4 + rt * 30 * (1 + a.k), 0, 6.3);
                g.stroke();
            }
            return true;
        });
        // 核心高亮圆点
        g.shadowColor = '#00f3ff';
        g.shadowBlur = 10;
        g.fillStyle = '#00f3ff';
        g.beginPath();
        g.arc(hx, hy, 4.5, 0, 6.3);
        g.fill();
        g.shadowBlur = 0;
    }
    // 2D 平面图没有镜头，以下三个方法保持空实现（与 3D 版接口对齐，避免调用方报错）
    clear() { this.arcs = []; this.sel = null }
    focus() { /* no-op */ }
    focusLL() { /* no-op */ }
    focusHome() { /* no-op */ }
    dispose() { cancelAnimationFrame(this.raf); this.ro.disconnect(); }
}
