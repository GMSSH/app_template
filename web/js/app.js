import { Globe } from './Globe.js'
import { Flat } from './Flat.js'
import { SERVER, POLL_MS, RADAR_RANGE } from './config.js'
import { Z } from './countries.js'
import {
  currentServerLoc, isMock, loadStore, saveStore, fetchServer, fetchGlobal, agg,
  fetchHeat, fetchPaths, GLOBAL_TTL, HEAT_TTL,
} from './api.js'

/* ============================ 状态 ============================ */
const S = {
  scope: 'global', view: '3d',
  raw: [], spots: [], sel: null,
  selC: null, selServer: false,
  gtab: 'country', kindF: '', metric: 'target',
  heat: null, paths: null,
  hov: null, tipXY: [0, 0],
  updated: 0, pollErr: '', gAt: 0, gLoad: false, gErr: '',
  heatAt: 0, heatErr: '', heatLoad: false,
  pAt: 0, pErr: '', pLoad: false,
  ready: false, hide: false, err: '', stepIdx: 0,
}

let globe = null, flat = null
const timers = []
let pauseT = 0, gTry = 0

const STEPS = ['正在初始化地图引擎……', '正在加载地理数据……', '正在获取全球攻击数据……', '正在构建攻击态势……', '初始化完成。']
const KL = { 'SSH 认证失败': 'SSH', 'Web 可疑请求': 'Web', 'PHP 漏洞探测': 'PHP 漏洞', 'Fail2ban 封禁': 'Fail2ban' }
const KINDS = Object.keys(KL)
const LV = ['', '低', '中', '高']

const $ = (id) => document.getElementById(id)
const el = {}
;['app', 'title', 'statusDot', 'statusText', 'globalCnt', 'topValue', 'topLabel',
  'serverHero', 'serverTotal', 'serverTag', 'serverTagName', 'serverTagIp', 'kindChips',
  'globalChips', 'heatTip', 'pathCount', 'listScroll', 'listBody', 'prov', 'provApi',
  'provRange', 'provMeta', 'cv3', 'cv2', 'topNav', 'nums', 'numEvents', 'numIps',
  'numCountries', 'legend', 'legendNote', 'serverTip', 'stIp', 'stTotal', 'stName',
  'stCoord', 'countryTip', 'ctZh', 'ctCc', 'ctValue', 'ctRankNote', 'ctPartners',
  'ctFooter', 'serverDetail', 'sdName', 'sdIp', 'sdLoc', 'sdCoord', 'sdIspLabel',
  'sdIsp', 'sdEvents', 'sdIps', 'sdCountries', 'countryDetail', 'cdName', 'cdCc',
  'cdMetricLabel', 'cdValue', 'cdRank', 'cdRange', 'cdPartnersBox', 'cdPartnersTitle',
  'eventDetail', 'edTitle', 'edFields', 'boot', 'bootBar', 'bootStep', 'bootErr',
  'bootErrMsg', 'bootRetry',
].forEach((k) => (el[k] = $(k)))

const zn = (cc, en) => Z[cc] || en || cc || '未知'
const lvName = (l) => LV[l] || '低'
const fmt = (ts, full = false) => ts
  ? new Date(ts).toLocaleString('zh-CN', {
      hour12: false,
      ...(full ? { month: '2-digit', day: '2-digit' } : {}),
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    })
  : '—'

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))

/* ============================ 派生数据 ============================ */
function shownList() {
  if (S.scope === 'global') return S.spots
  const lo = Date.now() - 864e5
  return agg(S.raw.filter((r) => r.ts >= lo && (!S.kindF || r.kind === S.kindF)))
}
function stats() {
  const list = shownList()
  const g = S.scope === 'global'
  return [
    ['安全事件', list.reduce((s, e) => s + (g ? e.reports || 0 : e.n), 0)],
    ['来源 IP', list.length],
    ['涉及国家', new Set(list.filter((e) => e.src).map((e) => e.src.name)).size],
  ]
}
const rank = () => S.heat?.list ?? []
const topOf = () => rank()[0] ?? null
const pathList = () => (S.paths?.list ?? []).slice().sort((a, b) => b.value - a.value)
const rangeText = () => {
  const r = S.heat?.range || RADAR_RANGE
  const m = r.match(/^(\d+)([dw])$/)
  return m ? (m[2] === 'd' ? `${m[1]} 天` : `${m[1]} 周`) : r
}
const selRow = () => (S.selC ? rank().find((c) => c.cc === S.selC) ?? null : null)
const selRank = () => (S.selC ? rank().findIndex((c) => c.cc === S.selC) + 1 : 0)
const partnersOf = (cc, n = 3) => pathList()
  .filter((p) => (S.metric === 'target' ? p.to === cc : p.from === cc)).slice(0, n)

function statusText() {
  if (S.scope === 'global') {
    if (S.heatLoad || S.pLoad || S.gLoad) return '正在获取全球态势……'
    if (S.heatErr) return '热区数据源不可用，仅显示来源热点'
    return `Cloudflare Radar · 更新于 ${fmt(S.heatAt, true)}`
  }
  return S.pollErr ? '数据源连接失败，将自动重试' : `最近更新 ${fmt(S.updated)}`
}

/* ============================ 渲染：头部 ============================ */
function renderHeader() {
  el.statusText.textContent = statusText()
  el.statusDot.classList.toggle('bad', !!(S.pollErr || S.heatErr))

  const g = S.scope === 'global'
  el.globalCnt.hidden = !g
  el.serverHero.hidden = g
  el.nums.hidden = g
  el.prov.hidden = !g
  el.legend.hidden = !(g && S.heat && !S.heatErr)

  if (g) {
    const top = topOf()
    el.topValue.textContent = top ? top.value.toFixed(1) + '%' : '—'
    el.topLabel.textContent = top ? `最高占比 · ${zn(top.cc, top.name)}` : '正在获取全球数据'
  } else {
    const st = stats()
    el.serverTotal.textContent = st[0][1]
    el.numEvents.textContent = st[0][1]
    el.numIps.textContent = st[1][1]
    el.numCountries.textContent = st[2][1]
    el.serverTagName.textContent = currentServerLoc.confirmed
      ? (currentServerLoc.name || SERVER.name)
      : '位置探测中'
    el.serverTagIp.textContent = currentServerLoc.ip || ''
    el.serverTag.title = currentServerLoc.confirmed
      ? `点击查看详情 · 经纬度: ${(currentServerLoc.lat || SERVER.lat).toFixed(4)}, ${(currentServerLoc.lon || SERVER.lon).toFixed(4)}`
      : '点击查看详情（位置探测中）'
  }

  el.kindChips.hidden = g
  el.globalChips.hidden = g
  el.legendNote.textContent = `${S.metric === 'target' ? '被攻击占比' : '攻击来源占比'} · 弧线＝主要攻击路径`

  // 全球侧栏的小状态
  if (!g) {
    const cnt = {}
    for (const r of S.raw) cnt[r.kind] = (cnt[r.kind] || 0) + 1
    el.kindChips.querySelectorAll('button').forEach((b) => {
      const k = b.dataset.kind
      b.classList.toggle('on', S.kindF === k)
      const small = b.querySelector('small')
      if (small) small.textContent = cnt[k] || 0
    })
  } else {
    el.heatTip.textContent = S.heatLoad ? '加载中…'
      : S.heatErr ? '热区不可用'
      : S.heat ? `${S.heat.list.length} 国 · 近${rangeText()}` : ''
    el.heatTip.classList.toggle('bad', !!S.heatErr)
    el.heatTip.title = S.heatErr || ''
    el.globalChips.querySelectorAll('[data-metric]').forEach((b) => b.classList.toggle('on', b.dataset.metric === S.metric))
    el.globalChips.querySelectorAll('[data-tab]').forEach((b) => b.classList.toggle('on', b.dataset.tab === S.gtab))
    const pl = pathList()
    el.pathCount.hidden = !pl.length
    el.pathCount.textContent = pl.length
    el.provApi.textContent = `top/locations/${S.metric}`
    el.provRange.textContent = `近 ${rangeText()}`
    el.provMeta.textContent = `${S.heat?.cached ? '本地缓存' : '实时请求'}${S.heat?.ms ? ` · ${S.heat.ms}ms` : ''}`
  }
}

/* ============================ 渲染：左侧列表 ============================ */
function renderList() {
  const g = S.scope === 'global'
  const list = shownList()

  if (g && S.gtab === 'country') {
    const rk = rank()
    const max = rk[0]?.value || 1
    el.listBody.innerHTML = rk.length
      ? `<ul class="list rk">${rk.map((c, i) => `
          <li data-cc="${esc(c.cc)}" class="${S.selC === c.cc ? 'on' : ''}">
            <i class="rkb" style="width:${((c.value / max) * 100).toFixed(2)}%"></i>
            <span class="rkn">${i + 1}</span>
            <span class="rkt">${esc(zn(c.cc, c.name))}</span>
            <span class="rkv">${c.value.toFixed(2)}%</span>
          </li>`).join('')}</ul>`
      : `<div class="empty">${esc(S.heatErr || '暂无热区数据')}<br />${S.heatErr ? '<button class="retry" data-act="retry-heat">重试</button>' : ''}</div>`
    return
  }

  if (g && S.gtab === 'path') {
    const pl = pathList()
    el.listBody.innerHTML = pl.length
      ? `<ul class="list pp">${pl.map((p) => `
          <li data-from="${esc(p.from)}" data-to="${esc(p.to)}">
            <div class="route"><b>${esc(zn(p.from, p.fromName))}</b><em>→</em><span>${esc(zn(p.to, p.toName))}</span></div>
            <div class="meta"><span>占总攻击 ${p.value.toFixed(2)}%</span></div>
          </li>`).join('')}</ul>`
      : `<div class="empty">${esc(S.pErr || '暂无配对数据')}<br />${S.pErr ? '<button class="retry" data-act="retry-paths">重试</button>' : ''}</div>`
    return
  }

  if (g) {
    // 全球 · 来源 IP
    el.listBody.innerHTML = list.length
      ? `<ul class="list">${list.map((e) => `
          <li data-eid="${esc(e.id)}" class="${S.sel?.id === e.id ? 'on' : ''}">
            <div class="route"><b>${esc(e.src?.name ?? '位置未知')}</b><em>→</em><span>全球传感器</span></div>
            <div class="meta"><span>${e.reports ?? 0} 次上报</span><span class="lv" data-l="${e.level}">${lvName(e.level)}</span><time>${esc(e.ip)}</time></div>
          </li>`).join('')}</ul>`
      : `<div class="empty">${esc(S.gErr || '暂无来源数据')}${S.gErr ? '<br /><button class="retry" data-act="retry-global">重试</button>' : ''}</div>`
    return
  }

  // 本机
  const city = currentServerLoc.city || currentServerLoc.country || SERVER.name
  el.listBody.innerHTML = list.length
    ? `<ul class="list server-list">${list.map((e) => `
        <li data-eid="${esc(e.id)}" class="${S.sel?.id === e.id ? 'on' : ''}">
          <div class="route">
            <span class="dot-status" data-l="${e.level}"></span>
            <b>${esc(e.src?.name ?? '位置未知')}</b>
            <em>→</em>
            <span>${esc(city)}</span>
          </div>
          <div class="meta">
            <span class="event-badge">${esc(e.kind)}</span>
            <span class="lv" data-l="${e.level}">${lvName(e.level)}危</span>
            <time>${esc(fmt(e.ts))}</time>
          </div>
        </li>`).join('')}</ul>`
    : `<div class="empty">暂无事件<br /><small>${isMock() ? '正在使用模拟数据（未读取到真实日志）' : '未发现相关记录，或无权限读取日志'}</small></div>`
}

/* ============================ 渲染：悬浮 / 详情 ============================ */
function renderTips() {
  const h = S.hov
  if (h?.is_server) {
    el.countryTip.hidden = true
    el.serverTip.hidden = false
    el.stIp.textContent = currentServerLoc.ip || '本机'
    el.stTotal.textContent = `${stats()[0][1]} 次`
    el.stName.textContent = currentServerLoc.confirmed ? (currentServerLoc.name || SERVER.name) : '正在探测公网位置'
    el.stCoord.textContent = currentServerLoc.confirmed
      ? `${(currentServerLoc.lat || SERVER.lat).toFixed(4)}, ${(currentServerLoc.lon || SERVER.lon).toFixed(4)}`
      : '—'
    el.serverTip.style.left = S.tipXY[0] + 'px'
    el.serverTip.style.top = S.tipXY[1] + 'px'
    return
  }
  el.serverTip.hidden = true
  if (!h) { el.countryTip.hidden = true; return }

  const list = rank()
  const i = list.findIndex((c) => c.cc === h.cc)
  const partners = partnersOf(h.cc, 3)
  el.countryTip.hidden = false
  el.ctZh.textContent = Z[h.cc] || h.zh || h.cc
  el.ctCc.textContent = h.cc
  el.ctValue.textContent = i >= 0 ? list[i].value.toFixed(2) + '%' : '无数据'
  el.ctRankNote.textContent = i >= 0
    ? `${S.metric === 'target' ? '被攻击占比' : '攻击来源占比'} · 第 ${i + 1}/${list.length} 位`
    : '未进入前 100 名（或该国不在 110m 底图内）'
  el.ctPartners.innerHTML = partners.length
    ? `<span>${S.metric === 'target' ? '主要攻击来自' : '主要攻击目标'}</span>` +
      partners.map((p) => `<em>${esc(S.metric === 'target' ? zn(p.from, p.fromName) : zn(p.to, p.toName))} ${p.value.toFixed(2)}%</em>`).join('')
    : ''
  el.ctFooter.textContent = `Cloudflare Radar · 近 ${rangeText()} · 点击查看详情`
  el.countryTip.style.left = S.tipXY[0] + 'px'
  el.countryTip.style.top = S.tipXY[1] + 'px'
}

function renderDetails() {
  const st = stats()

  // 本机服务器
  el.serverDetail.hidden = !S.selServer
  if (S.selServer) {
    const ok = currentServerLoc.confirmed
    el.sdName.textContent = ok ? (currentServerLoc.name || SERVER.name) : '本机服务器'
    el.sdIp.textContent = currentServerLoc.ip || '—'
    el.sdLoc.textContent = ok
      ? [currentServerLoc.country, currentServerLoc.region, currentServerLoc.city].filter(Boolean).join(' · ') || '—'
      : '正在探测公网位置…'
    el.sdCoord.textContent = ok
      ? `${(currentServerLoc.lat || SERVER.lat).toFixed(4)}, ${(currentServerLoc.lon || SERVER.lon).toFixed(4)}`
      : '—'
    el.sdIspLabel.hidden = !currentServerLoc.isp
    el.sdIsp.hidden = !currentServerLoc.isp
    el.sdIsp.textContent = currentServerLoc.isp || ''
    el.sdEvents.textContent = `${st[0][1]} 次尝试`
    el.sdIps.textContent = `${st[1][1]} 个独立 IP`
    el.sdCountries.textContent = `${st[2][1]} 个地区`
  }

  // 国家
  el.countryDetail.hidden = !S.selC
  if (S.selC) {
    const row = selRow()
    const parts = S.selC ? partnersOf(S.selC, 5) : []
    el.cdName.textContent = zn(S.selC)
    el.cdCc.textContent = S.selC
    el.cdMetricLabel.textContent = S.metric === 'target' ? '被攻击占比' : '攻击来源占比'
    el.cdValue.textContent = row ? row.value.toFixed(2) + '%' : '未进前 100'
    el.cdRank.textContent = row ? `第 ${selRank()} / ${rank().length} 位` : '—'
    el.cdRange.textContent = `近 ${rangeText()}`
    el.cdPartnersBox.hidden = !parts.length
    el.cdPartnersTitle.textContent = S.metric === 'target' ? '主要攻击来源' : '主要攻击目标'
    el.cdPartnersBox.querySelectorAll('span').forEach((n) => n.remove())
    parts.forEach((p) => {
      const span = document.createElement('span')
      const em = document.createElement('em')
      em.textContent = S.metric === 'target' ? zn(p.from, p.fromName) : zn(p.to, p.toName)
      span.append(em, `${p.value.toFixed(2)}%`)
      el.cdPartnersBox.append(span)
    })
  }

  // 事件
  const ev = S.sel
  el.eventDetail.hidden = !ev
  if (ev) {
    el.edTitle.textContent = `${ev.src?.name ?? '位置未知'} → ${S.scope === 'global' ? '全球传感器' : SERVER.name}`
    const rows = S.scope === 'global'
      ? [['来源 IP', ev.ip], ['类型', ev.kind], ['上报次数', ev.reports], ['目标数', ev.targets], ['快照时间', fmt(S.gAt, true)]]
      : [['来源 IP', ev.ip], ['类型', ev.kind], ['尝试次数', ev.n], ['最近发生', fmt(ev.ts, true)], ['风险', lvName(ev.level)]]
    el.edFields.innerHTML = rows.map(([k, v]) => `<dt>${esc(k)}</dt><dd>${esc(v)}</dd>`).join('')
  }
}

function renderBoot() {
  el.boot.classList.toggle('out', S.ready)
  el.boot.hidden = S.hide
  el.bootBar.style.width = `${(S.stepIdx / 4) * 100}%`
  el.bootStep.hidden = !!S.err
  el.bootStep.textContent = STEPS[S.stepIdx] || ''
  el.bootErr.hidden = !S.err
  el.bootErrMsg.textContent = S.err
}

function renderAll() {
  el.app.classList.toggle('ready', S.ready)
  renderHeader()
  renderList()
  renderTips()
  renderDetails()
  renderBoot()
}

/* ============================ 视图切换 ============================ */
function applyView() {
  el.cv3.classList.toggle('off', S.view === '2d')
  el.cv2.classList.toggle('off', S.view === '3d')
  el.topNav.querySelectorAll('[data-view]').forEach((b) => b.classList.toggle('on', b.dataset.view === S.view))
  el.topNav.querySelectorAll('[data-scope]').forEach((b) => b.classList.toggle('on', b.dataset.scope === S.scope))
}

/* ============================ 地图动作 ============================ */
function fireEv(e, strong) { if (S.scope !== 'server') return; globe?.fire(e, strong); flat?.fire(e, strong) }

function fireRows(rows, max) {
  const m = new Map()
  for (const r of rows) {
    if (!r.src || (S.kindF && r.kind !== S.kindF)) continue
    const x = m.get(r.ip)
    if (x) x[1]++
    else m.set(r.ip, [r, 1])
  }
  ;[...m.values()].sort((a, b) => b[1] - a[1]).slice(0, max)
    .forEach(([r, n], i) => setTimeout(() => fireEv(
      { id: r.ip + '|' + r.kind, ts: r.ts, ip: r.ip, kind: r.kind, n, level: 1, src: r.src },
      Math.min(3, 1 + Math.floor(Math.log2(n))),
    ), i * 140))
}

function applyHeat() {
  const h = S.heat
  globe?.setHeat(S.scope === 'global' && !S.heatErr && h ? new Map(h.list.map((c) => [c.cc, c.value])) : new Map())
}
function applyPairs() {
  const list = S.scope === 'global' && !S.pErr ? pathList() : []
  const mx = Math.max(1, ...list.map((p) => p.value))
  globe?.setPairs(list.map((p) => ({ from: p.from, to: p.to, value: p.value })))
  flat?.setPairs(list.map((p) => {
    const a = globe?.llByCC?.get(p.from), b = globe?.llByCC?.get(p.to)
    return a && b ? { from: p.from, to: p.to, a, b, k: Math.min(1, p.value / mx) } : null
  }).filter(Boolean))
}
function pushSpots() {
  const l = S.spots.filter((e) => e.src)
  const mx = Math.max(1, ...l.map((e) => e.reports || 0))
  const pts = l.map((e) => ({ lat: e.src.lat, lon: e.src.lon, w: Math.log(1 + (e.reports || 0)) / Math.log(1 + mx) }))
  globe?.setSpots(pts)
  if (flat) flat.spots = pts
}
function syncOutline(cc) { if (flat) flat.hlPath = cc ? globe?.outline(cc) ?? null : null }
function closeDetails() {
  S.sel = null; S.selServer = false
  globe?.select(null); flat?.select(null)
}
function pickEvent(e) {
  S.sel = e; S.selC = null; syncOutline(null)
  const g = S.scope === 'global'
  globe?.select(e.id); flat?.select(e.id)
  if (!g) fireEv(e)
  globe?.focus(e, g); flat?.focus(e, g)
}
function pickServer() {
  closeDetails()
  S.selC = null
  S.selServer = true
  if (currentServerLoc.lat && currentServerLoc.lon) {
    globe?.focusLL(currentServerLoc.lat, currentServerLoc.lon, 2.8)
    flat?.focusLL(currentServerLoc.lat, currentServerLoc.lon, 3.2)
  }
}
function pickCountry(cc) {
  closeDetails()
  S.selC = cc; S.gtab = 'country'; syncOutline(cc)
  const p = globe?.llByCC?.get(cc)
  if (p) globe?.focusLL(p[0], p[1])
  flat?.focusLL(p?.[0] ?? 0, p?.[1] ?? 0, 3)
}
function pickPath(p) {
  const a = globe?.llByCC?.get(p.from), b = globe?.llByCC?.get(p.to)
  S.selC = p.to
  if (a && b) {
    const lat = (a[0] + b[0]) / 2, lon = (a[1] + b[1]) / 2
    globe?.focusLL(lat, lon, 2.7); flat?.focusLL(lat, lon, 2.4)
  }
}
function onHover(info, x, y) {
  S.hov = info
  if (x != null && y != null) {
    const w = 268, h = 190
    S.tipXY = [Math.min(x + 18, window.innerWidth - w - 12), Math.min(y + 18, window.innerHeight - h - 12)]
  }
  if (info?.is_server) {
    syncOutline(null); globe?.setPairFocus(null); flat?.setPairFocus(null)
  } else {
    syncOutline(info?.cc ?? null)
    globe?.setPairFocus(info?.cc ?? null); flat?.setPairFocus(info?.cc ?? null)
  }
  renderTips()
}
function onPick(info) {
  if (info?.is_server) return pickServer()
  if (info?.cc) pickCountry(info.cc)
}

/* ============================ 数据加载 ============================ */
async function loadGlobal(force = false) {
  S.gLoad = true; S.gErr = ''; gTry = Date.now(); renderHeader()
  try {
    const r = await fetchGlobal(force)
    S.gAt = r.at; S.spots = r.list
    if (S.scope === 'global' && S.gtab === 'ip') pushSpots()
  } catch (e) {
    S.gErr = e?.message || '获取失败'
  } finally {
    S.gLoad = false; renderAll()
  }
}
async function loadHeat(force = false) {
  S.heatLoad = true; S.heatErr = ''; renderHeader()
  try {
    const h = await fetchHeat(S.metric, force)
    S.heat = h; S.heatAt = h.at
  } catch (e) {
    S.heatErr = e?.message || '获取失败'; S.heat = null
  } finally {
    S.heatLoad = false; applyHeat(); renderAll()
  }
}
async function loadPaths(force = false) {
  S.pLoad = true; S.pErr = ''; renderHeader()
  try {
    const p = await fetchPaths(force)
    S.paths = p; S.pAt = p.at
  } catch (e) {
    S.pErr = e?.message || '获取失败'; S.paths = null
  } finally {
    S.pLoad = false; applyPairs(); renderAll()
  }
}

async function poll() {
  if (Date.now() - Math.max(S.gAt, gTry) > GLOBAL_TTL) loadGlobal(true)
  if (S.heatAt && Date.now() - S.heatAt > HEAT_TTL) loadHeat(true)
  if (S.pAt && Date.now() - S.pAt > HEAT_TTL) loadPaths(true)
  if (S.scope === 'global') return
  try {
    const ids = new Set(S.raw.map((r) => r.id))
    const all = await fetchServer(S.raw)
    fireRows(all.filter((r) => !ids.has(r.id)), 8)
    S.raw = saveStore(all); S.updated = Date.now(); S.pollErr = ''
  } catch (e) {
    S.pollErr = e?.message || 'err'
  }
  renderAll()
}

async function boot() {
  S.err = ''; S.stepIdx = 0; renderBoot()
  try {
    if (!globe) {
      globe = new Globe(el.cv3)
      flat = new Flat(el.cv2)
      globe.onHover = onHover
      globe.onMove = (x, y) => onHover(S.hov, x, y)
      globe.onPick = onPick
      flat.onHover = (p, x, y) => {
        const info = p ? globe.pick(p.lat, p.lon) ?? null : null
        if ((info?.cc ?? '') === (S.hov?.cc ?? '') && x != null) return onHover(S.hov, x, y)
        onHover(info, x, y)
      }
      flat.onPick = (p) => onPick(p ? globe.pick(p.lat, p.lon) ?? null : null)
      if (location.port) window.__radar = { globe, flat }
    }
    S.stepIdx = 1; renderBoot()
    await globe.loadGeo()
    flat.tex = globe.tex

    S.stepIdx = 2; renderBoot()
    await Promise.all([loadHeat(), loadPaths()])

    S.stepIdx = 3; S.updated = Date.now(); S.ready = true; renderAll()
    setTimeout(() => { S.hide = true; renderBoot() }, 450)
    globe.intro()
    S.stepIdx = 4

    loadGlobal()
    fetchServer(loadStore()).then((a) => {
      S.raw = saveStore(a); S.updated = Date.now()
      if (currentServerLoc.lat && currentServerLoc.lon) {
        globe?.setServerLocation(currentServerLoc.lat, currentServerLoc.lon, currentServerLoc.name)
        flat?.setServerLocation(currentServerLoc.lat, currentServerLoc.lon, currentServerLoc.name)
      }
      renderAll()
    }).catch((e) => { S.pollErr = e?.message || 'err'; renderAll() })

    timers.push(window.setInterval(poll, POLL_MS))
  } catch (e) {
    S.err = e?.message || '初始化失败'
    renderBoot()
  }
}

/* ============================ 交互绑定 ============================ */
function setScope(s) {
  if (s === S.scope) return
  closeDetails(); S.selC = null; syncOutline(null); onHover(null)
  S.scope = s
  globe?.clear(); flat?.clear()
  applyView()
  if (s === 'global') {
    if (!S.heatAt || Date.now() - S.heatAt > HEAT_TTL) loadHeat(); else applyHeat()
    if (!S.pAt || Date.now() - S.pAt > HEAT_TTL) loadPaths(); else applyPairs()
    if (S.gtab === 'ip') (S.spots.length ? pushSpots() : loadGlobal())
  } else {
    applyHeat(); applyPairs()
    globe?.setSpots([]); if (flat) flat.spots = []
    if (currentServerLoc.lat && currentServerLoc.lon) {
      globe?.setServerLocation(currentServerLoc.lat, currentServerLoc.lon, currentServerLoc.name)
      flat?.setServerLocation(currentServerLoc.lat, currentServerLoc.lon, currentServerLoc.name)
      globe?.focusHome()
    }
    agg(S.raw).slice(0, 12).forEach((e, i) => setTimeout(() => fireEv(e), 600 + i * 140))
  }
  renderAll()
}

function setView(v) {
  if (v === S.view) return
  S.view = v
  if (globe) globe.paused = false
  if (flat) flat.paused = false
  clearTimeout(pauseT)
  pauseT = window.setTimeout(() => {
    if (globe) globe.paused = v === '2d'
    if (flat) flat.paused = v === '3d'
  }, 700)
  applyView()
}

function setMetric(m) {
  if (m === S.metric) return
  S.metric = m; S.heat = null; loadHeat()
}

function setKind(k) {
  S.kindF = k; closeDetails(); globe?.clear(); flat?.clear()
  shownList().slice(0, 12).forEach((e, i) => setTimeout(() => fireEv(e), 200 + i * 140))
  renderAll()
}

function bind() {
  el.topNav.addEventListener('click', (e) => {
    const b = e.target.closest('button')
    if (!b) return
    if (b.dataset.scope) setScope(b.dataset.scope)
    if (b.dataset.view) setView(b.dataset.view)
  })

  el.serverTag.addEventListener('click', pickServer)

  el.kindChips.addEventListener('click', (e) => {
    const b = e.target.closest('button[data-kind]')
    if (b) setKind(b.dataset.kind)
  })

  el.globalChips.addEventListener('click', (e) => {
    const b = e.target.closest('button')
    if (!b) return
    if (b.dataset.metric) return setMetric(b.dataset.metric)
    if (b.dataset.tab) {
      S.gtab = b.dataset.tab
      if (S.gtab === 'ip') pushSpots()
      renderAll()
    }
  })

  el.listBody.addEventListener('click', (e) => {
    const retry = e.target.closest('[data-act]')
    if (retry) {
      const a = retry.dataset.act
      if (a === 'retry-heat') loadHeat(true)
      if (a === 'retry-paths') loadPaths(true)
      if (a === 'retry-global') loadGlobal(true)
      return
    }
    const li = e.target.closest('li')
    if (!li) return
    if (li.dataset.cc) return pickCountry(li.dataset.cc)
    if (li.dataset.from) {
      const p = pathList().find((x) => x.from === li.dataset.from && x.to === li.dataset.to)
      if (p) pickPath(p)
      return
    }
    if (li.dataset.eid) {
      const ev = shownList().find((x) => x.id === li.dataset.eid)
      if (ev) { pickEvent(ev); renderAll() }
    }
  })

  document.addEventListener('click', (e) => {
    const btn = e.target.closest('[data-close]')
    if (btn) {
      const which = btn.dataset.close
      if (which === 'server') S.selServer = false
      if (which === 'country') { S.selC = null; syncOutline(null) }
      if (which === 'event') closeDetails()
      renderAll()
    }
  })

  el.bootRetry.addEventListener('click', boot)

  const ro = new ResizeObserver(() => applyView())
  ro.observe(el.app)

  window.addEventListener('beforeunload', () => {
    timers.forEach(clearInterval)
    clearTimeout(pauseT)
    globe?.dispose(); flat?.dispose()
  })
}

/* ============================ 启动 ============================ */
bind()
applyView()
renderAll()
boot()
