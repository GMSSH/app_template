import { ICONS } from './icons.js'
import { executeDomainCommand } from './api.js'

// 9 个模块的定义及圆环角度顺序（顺时针排布）
export const MODULES_CONFIG = [
  { id: 'rdap', name: '域名信息', angle: -90, icon: 'rdap' },     // 正上方 (12点)
  { id: 'dns', name: 'DNS记录', angle: -50, icon: 'dns' },        // 1点半
  { id: 'multidns', name: '多源DNS', angle: -10, icon: 'multidns' }, // 3点
  { id: 'web', name: '网站访问', angle: 30, icon: 'web' },         // 4点半
  { id: 'ssl', name: 'SSL证书', angle: 70, icon: 'ssl' },          // 5点半
  { id: 'port', name: '端口连通', angle: 110, icon: 'port' },       // 6点半
  { id: 'redirect', name: '跳转链', angle: 150, icon: 'redirect' }, // 8点
  { id: 'mail', name: '邮件配置', angle: 190, icon: 'mail' },       // 9点半
  { id: 'dnssec', name: 'DNSSEC', angle: 230, icon: 'dnssec' },   // 10点半
]

// 各模块在详情面板中的文案
const MODULE_META = {
  rdap: { snippet: '注册信息（原始数据）', action: '查看 WHOIS 详情' },
  dns: { snippet: '解析记录（原始数据）', action: '查看全部记录' },
  multidns: { snippet: '多源对比（原始数据）', action: '查看对比详情' },
  web: { snippet: '响应详情（原始数据）', action: '查看访问详情' },
  ssl: { snippet: '证书详情（原始数据）', action: '查看证书详情' },
  port: { snippet: '端口探测（原始数据）', action: '查看端口详情' },
  redirect: { snippet: '跳转链路（原始数据）', action: '查看跳转详情' },
  mail: { snippet: '邮件记录（原始数据）', action: '查看邮件配置' },
  dnssec: { snippet: 'DNSSEC 记录（原始数据）', action: '查看 DNSSEC 详情' },
}

// 节点 DOM 复用表（moduleId -> element）
const nodeEls = {}
let breathRaf = null
let welcomeTimer = null

// 应用全局状态
const state = {
  currentDomain: '',
  selectedModuleId: 'rdap',
  overviewData: null,
  modules: {}, // moduleId -> ModuleEnvelope
  isLoading: false,
}

// DOM 元素缓存
const dom = {
  welcomeView: document.getElementById('welcomeView'),
  welcomeInput: document.getElementById('welcomeInput'),
  welcomeForm: document.getElementById('welcomeForm'),
  
  headerCapsule: document.getElementById('headerForm'),
  headerInput: document.getElementById('headerInput'),
  headerClear: document.getElementById('headerClear'),
  headerForm: document.getElementById('headerForm'),

  workbench: document.getElementById('workbench'),
  centerDomainText: document.getElementById('centerDomainText'),
  centerCore: document.getElementById('centerCore'),
  satelliteNodesWrap: document.getElementById('satelliteNodesWrap'),
  topologySvg: document.getElementById('topologySvg'),
  
  drawerIcon: document.getElementById('drawerIcon'),
  drawerTitle: document.getElementById('drawerTitle'),
  drawerBadge: document.getElementById('drawerBadge'),
  drawerTime: document.getElementById('drawerTime'),
  drawerSummary: document.getElementById('drawerSummary'),
  drawerMetricList: document.getElementById('drawerMetricList'),
  drawerSnippetBox: document.getElementById('drawerSnippetBox'),
  drawerSnippetContent: document.getElementById('drawerSnippetContent'),
  drawerSnippetTitle: document.getElementById('drawerSnippetTitle'),
  drawerCopySnippet: document.getElementById('drawerCopySnippet'),
  drawerActionBtn: document.getElementById('drawerActionBtn'),
  drawerActionText: document.getElementById('drawerActionText'),

  stage: document.getElementById('stage'),
  linkSvg: document.getElementById('linkSvg'),
  stars: document.getElementById('stars'),

  alertModal: document.getElementById('alertModal'),
  alertTitle: document.getElementById('alertTitle'),
  alertMessage: document.getElementById('alertMessage'),
  alertDetail: document.getElementById('alertDetail'),
  alertOk: document.getElementById('alertOk'),

  reportModal: document.getElementById('reportModal'),
  modalBackdrop: document.getElementById('modalBackdrop'),
  modalCloseBtn: document.getElementById('modalCloseBtn'),
  modalDomainTitle: document.getElementById('modalDomainTitle'),
  modalMeta: document.getElementById('modalMeta'),
  modalBody: document.getElementById('modalBody'),
  modalCopyBtn: document.getElementById('modalCopyBtn')
}

// 初始化
function init() {
  bindEvents()
  setupInitialView()
}

function setupInitialView() {
  renderStars()

  // 填充静态图标
  document.querySelectorAll('[data-icon]').forEach(el => {
    const iconName = el.getAttribute('data-icon')
    if (ICONS[iconName]) {
      el.innerHTML = ICONS[iconName]
    }
  })
}

function bindEvents() {
  // 欢迎页提交
  dom.welcomeForm.addEventListener('submit', (e) => {
    e.preventDefault()
    const domain = dom.welcomeInput.value.trim()
    if (domain) {
      startInspection(domain)
    }
  })

  // 顶栏搜索提交
  dom.headerForm.addEventListener('submit', (e) => {
    e.preventDefault()
    const domain = dom.headerInput.value.trim()
    if (domain) {
      startInspection(domain)
    }
  })

  // 输入变化控制清除按钮
  dom.headerInput.addEventListener('input', () => {
    dom.headerClear.style.display = dom.headerInput.value ? 'flex' : 'none'
  })
  dom.headerClear.addEventListener('click', () => {
    dom.headerInput.value = ''
    dom.headerClear.style.display = 'none'
    dom.headerInput.focus()
  })

  // 复制代码块
  dom.drawerCopySnippet.addEventListener('click', () => {
    const text = dom.drawerSnippetContent.textContent
    if (text) {
      copyToClipboard(text, dom.drawerCopySnippet)
    }
  })

  // 窗口重绘拓扑位置
  window.addEventListener('resize', () => {
    if (state.currentDomain) {
      renderTopologyLayout()
    }
  })

  // 打开完整检测报告
  dom.drawerActionBtn.addEventListener('click', () => {
    openReportModal()
  })

  // 关闭完整检测报告
  dom.modalCloseBtn.addEventListener('click', closeReportModal)
  dom.modalBackdrop.addEventListener('click', closeReportModal)

  // 失败弹窗：按钮 / 蒙层 / ESC 都可关闭
  dom.alertOk.addEventListener('click', closeAlert)
  dom.alertModal.addEventListener('click', (e) => {
    if (e.target === dom.alertModal) closeAlert()
  })

  window.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return
    if (dom.alertModal.classList.contains('active')) {
      closeAlert()
    } else if (dom.reportModal.classList.contains('active')) {
      closeReportModal()
    }
  })

  // 复制全量报告
  dom.modalCopyBtn.addEventListener('click', () => {
    const rawAll = JSON.stringify(state.overviewData || {}, null, 2)
    copyToClipboard(rawAll, dom.modalCopyBtn)
  })

}

// 查询失败弹窗：中文结论 + 原始报错小字
function showAlert(title, message, detail = '') {
  dom.alertTitle.textContent = title
  dom.alertMessage.textContent = message
  dom.alertDetail.textContent = detail
  dom.alertModal.classList.add('active')
  dom.alertOk.focus()
}

function closeAlert() {
  dom.alertModal.classList.remove('active')
}

// 把后端的英文报错翻成用户看得懂的一句话
function friendlyError(raw) {
  const msg = String(raw || '')
  if (/missing required option.*domain|域名无效:\s*$|domain is empty/i.test(msg)) {
    return '请输入要查询的域名'
  }
  if (/invalid domain|label|at least one dot/i.test(msg)) {
    return '域名格式不正确，请检查后重试，例如 example.com'
  }
  if (/blocked|private|loopback|reserved/i.test(msg)) {
    return '该地址指向内网或保留地址，出于安全考虑已阻断'
  }
  return '查询失败，请稍后重试'
}

// 开始全面查询
async function startInspection(domain) {
  domain = domain.replace(/^https?:\/\//i, '').replace(/\/.*$/, '').trim()
  if (!domain) return

  // 失败时要回滚到这次查询之前的样子
  const snapshot = {
    domain: state.currentDomain,
    modules: { ...state.modules },
    data: state.overviewData,
  }

  state.currentDomain = domain
  state.isLoading = true
  
  // 切换 UI 到工作台
  // 欢迎页先淡出，再彻底隐藏（定时器要留着，回滚时得能取消）
  dom.welcomeView.classList.add('leaving')
  clearTimeout(welcomeTimer)
  welcomeTimer = setTimeout(() => dom.welcomeView.classList.add('hidden'), 280)

  dom.headerCapsule.style.display = 'flex'
  dom.workbench.classList.add('active')

  // 进场动画只播一次；之后的重绘不再触发
  dom.workbench.classList.remove('entering')
  void dom.workbench.offsetWidth
  dom.workbench.classList.add('entering')
  setTimeout(() => dom.workbench.classList.remove('entering'), 1500)

  dom.headerInput.value = domain
  dom.headerClear.style.display = 'flex'
  dom.centerDomainText.textContent = domain

  // 初始化所有节点为 loading 状态
  MODULES_CONFIG.forEach(m => {
    state.modules[m.id] = {
      module: m.id,
      domain: domain,
      status: 'pending'
    }
  })

  renderTopologyLayout()
  updateDrawer()
  startNodeBreathing()

  try {
    // 调用后端的 overview 聚合命令
    const res = await executeDomainCommand('overview', domain)
    if (res && res.data) {
      state.overviewData = res.data
      state.modules = res.data.modules || {}
      renderTopologyLayout()
      updateDrawer()
    }
  } catch (err) {
    console.error('Query failed:', err)
    const raw = (err && err.message) || ''
    showAlert('查询失败', friendlyError(raw), raw)

    // 回滚：有旧结果就留着，没有就退回首页，别把界面晾在一片红叉里
    state.currentDomain = snapshot.domain
    state.modules = snapshot.modules
    state.overviewData = snapshot.data
    if (snapshot.data) {
      renderTopologyLayout()
      updateDrawer()
    } else {
      // 取消还没触发的隐藏定时器，否则回滚后欢迎页又被藏起来，只剩空白
      clearTimeout(welcomeTimer)
      dom.workbench.classList.remove('active', 'entering')
      dom.headerCapsule.style.display = 'none'
      dom.welcomeView.classList.remove('hidden', 'leaving')
    }
  } finally {
    state.isLoading = false
    stopNodeBreathing()
  }
}

// 动态布局中心辐射拓扑图与 SVG 连线（严格按设计图比例缩放）
function renderTopologyLayout() {
  const rect = dom.stage.getBoundingClientRect()
  if (!rect.width || !rect.height) return

  // 设计图基准：球体 250，节点 135，环半径 263（= 球半径 + 间隙 + 节点半径）
  const sphereSize = Math.round(Math.max(130, Math.min(rect.width * 0.26, rect.height * 0.30, 250)))
  const nodeSize = Math.round(sphereSize * 0.54)
  const ringRadius = sphereSize * 1.05
  const extent = ringRadius + nodeSize / 2      // 环 + 节点的最大外扩半径
  const gap = 14

  dom.stage.style.setProperty('--sphere-size', sphereSize + 'px')
  dom.stage.style.setProperty('--node-size', nodeSize + 'px')

  // 圆心居中（无遮挡物，纵向略高于正中，对齐设计图观感）
  const centerX = rect.width / 2
  const centerY = Math.min(
    Math.max(rect.height * 0.46, extent + gap),
    Math.max(rect.height - extent - gap, extent + gap)
  )

  dom.centerCore.style.left = `${centerX}px`
  dom.centerCore.style.top = `${centerY}px`

  // 1. 轨道环 + 中心到节点的连线
  let svgContent = `
    <defs>
      <radialGradient id="spokeFade" gradientUnits="userSpaceOnUse"
        cx="${centerX}" cy="${centerY}" r="${ringRadius}">
        <stop offset="0%" stop-color="#ffffff" stop-opacity="0.03" />
        <stop offset="52%" stop-color="#ffffff" stop-opacity="0.12" />
        <stop offset="100%" stop-color="#ffffff" stop-opacity="0.30" />
      </radialGradient>
    </defs>
    <circle class="orbit-ring-inner" cx="${centerX}" cy="${centerY}" r="${sphereSize * 0.72}" />
    <circle class="orbit-ring" cx="${centerX}" cy="${centerY}" r="${ringRadius}" />
    <circle class="orbit-ring-outer" cx="${centerX}" cy="${centerY}" r="${ringRadius + sphereSize * 0.16}" />
  `

  MODULES_CONFIG.forEach((m, mIndex) => {
    const rad = (m.angle * Math.PI) / 180
    const nodeX = centerX + ringRadius * Math.cos(rad)
    const nodeY = centerY + ringRadius * Math.sin(rad)

    const isSelected = state.selectedModuleId === m.id
    const badge = getStatusBadge(m.id, state.modules[m.id])

    // 连线从球体边缘起，到节点边缘止，不穿过球体
    const startX = centerX + (sphereSize / 2) * Math.cos(rad)
    const startY = centerY + (sphereSize / 2) * Math.sin(rad)
    const endX = centerX + (ringRadius - nodeSize / 2) * Math.cos(rad)
    const endY = centerY + (ringRadius - nodeSize / 2) * Math.sin(rad)

    svgContent += `
      <line class="spoke-line ${isSelected ? 'active' : ''}"
            stroke="${isSelected ? '#ffffff' : 'url(#spokeFade)'}"
            x1="${startX}" y1="${startY}" x2="${endX}" y2="${endY}" />
    `

    // 复用同一个 DOM 节点：重绘时只改位置与文案，呼吸动画不会被重置
    let nodeEl = nodeEls[m.id]
    if (!nodeEl) {
      nodeEl = document.createElement('div')
      nodeEl.className = 'satellite-node'
      nodeEl.style.setProperty('--i', mIndex)   // 进场按序号错峰
      nodeEl.addEventListener('click', () => {
        state.selectedModuleId = m.id
        renderTopologyLayout()
        updateDrawer()
      })
      dom.satelliteNodesWrap.appendChild(nodeEl)
      nodeEls[m.id] = nodeEl
    }

    nodeEl.classList.toggle('active', isSelected)
    nodeEl.style.left = `${nodeX}px`
    nodeEl.style.top = `${nodeY}px`
    nodeEl.innerHTML = `
      <div class="node-icon">${ICONS[m.icon] || ''}</div>
      <div class="node-name">${m.name}</div>
      <div class="node-badge ${badge.className}">${badge.label}</div>
    `
  })

  dom.topologySvg.innerHTML = svgContent

  // 2. 活动节点 → 详情面板的发光连接弧线
  const activeMod = MODULES_CONFIG.find((m) => m.id === state.selectedModuleId)
  if (activeMod) {
    const rad = (activeMod.angle * Math.PI) / 180
    const nx = centerX + ringRadius * Math.cos(rad)
    const ny = centerY + ringRadius * Math.sin(rad)
    // 从节点右缘出发，横向绕开节点后进入面板
    const sx = nx + nodeSize / 2 + 2
    const sy = ny
    const tx = rect.width + 40
    const ty = ny + 12
    const cx1 = sx + (tx - sx) * 0.42
    const cy1 = sy + 26
    const cx2 = sx + (tx - sx) * 0.78
    const cy2 = ty
    dom.linkSvg.innerHTML = `
      <defs>
        <linearGradient id="activeLink" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stop-color="#ffffff" stop-opacity="0.9" />
          <stop offset="100%" stop-color="#ffffff" stop-opacity="0.12" />
        </linearGradient>
        <filter id="activeLinkGlow" x="-40%" y="-200%" width="180%" height="500%">
          <feGaussianBlur stdDeviation="3.2" result="b" />
          <feMerge><feMergeNode in="b" /><feMergeNode in="SourceGraphic" /></feMerge>
        </filter>
      </defs>
      <path d="M ${sx} ${sy} C ${cx1} ${cy1}, ${cx2} ${cy2}, ${tx} ${ty}"
            fill="none" stroke="url(#activeLink)" stroke-width="1.6"
            stroke-linecap="round" filter="url(#activeLinkGlow)" />
    `
  } else {
    dom.linkSvg.innerHTML = ''
  }
}

// 背景星点（一次性生成，固定伪随机，避免重排抖动）
function renderStars() {
  let dots = ''
  let seed = 20240315
  const rnd = () => {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff
    return seed / 0x7fffffff
  }
  for (let i = 0; i < 46; i++) {
    const size = rnd() < 0.82 ? 1.4 : 2.2
    dots += `<span class="star" style="left:${(rnd() * 100).toFixed(2)}%;top:${(rnd() * 100).toFixed(2)}%;width:${size}px;height:${size}px;opacity:${(0.12 + rnd() * 0.35).toFixed(2)}"></span>`
  }
  dom.stars.innerHTML = dots
}

// 查询期间节点不规则涨缩：每个节点独立周期/振幅/相位，不同步才有生命感
function startNodeBreathing() {
  stopNodeBreathing()

  const targets = Object.values(nodeEls)
  if (!targets.length) return

  const params = targets.map((el) => {
    el.style.transition = 'none'
    return {
      el,
      period: 1300 + Math.random() * 1100,              // 1.3 ~ 2.4s
      amp: 0.055 + Math.random() * 0.075,               // 幅度 5.5% ~ 13%
      phase: Math.random() * Math.PI * 2,
      wobble: 1.4 + Math.random() * 0.7,                // 叠频倍率，破坏规则正弦
    }
  })

  const t0 = performance.now()
  const tick = (now) => {
    const elapsed = now - t0
    params.forEach((p) => {
      const w = (elapsed / p.period) * Math.PI * 2 + p.phase
      const s = 1 + p.amp * (Math.sin(w) * 0.72 + Math.sin(w * p.wobble + p.phase) * 0.28)
      p.el.style.scale = s.toFixed(4)
    })
    breathRaf = requestAnimationFrame(tick)
  }
  breathRaf = requestAnimationFrame(tick)
}

// 查询结束：从当前尺寸平滑过渡回 1，不硬切
function stopNodeBreathing() {
  if (breathRaf) {
    cancelAnimationFrame(breathRaf)
    breathRaf = null
  }
  Object.values(nodeEls).forEach((el) => {
    if (!el.style.scale) return
    el.style.transition = 'scale 0.55s cubic-bezier(0.16, 1, 0.3, 1)'
    el.style.scale = '1'
    // 过渡结束后交还给 CSS，避免残留内联样式影响 hover / active
    window.setTimeout(() => {
      el.style.transition = ''
      el.style.scale = ''
    }, 600)
  })
}

// 统一时间格式：YYYY-MM-DD HH:mm:ss
function formatTime(iso) {
  const t = new Date(iso)
  if (isNaN(t.getTime())) return '--'
  const p = (n) => String(n).padStart(2, '0')
  return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())} ${p(t.getHours())}:${p(t.getMinutes())}:${p(t.getSeconds())}`
}

// 状态与徽章计算映射
function getStatusBadge(moduleId, modData) {
  if (!modData || modData.status === 'pending') {
    return { label: '检测中', className: 'badge-loading' }
  }

  if (modData.status === 'not_configured') {
    return { label: '未配置', className: 'badge-inactive' }
  }

  if (modData.status === 'warning') {
    return { label: '警告', className: 'badge-warning' }
  }

  if (modData.status === 'error' || modData.status === 'unavailable') {
    return { label: '异常', className: 'badge-error' }
  }

  return { label: '正常', className: 'badge-success' }
}

// 更新右侧详情面板
function updateDrawer() {
  const selectedMod = MODULES_CONFIG.find(m => m.id === state.selectedModuleId)
  if (!selectedMod) return

  const modData = state.modules[selectedMod.id]
  const badge = getStatusBadge(selectedMod.id, modData)

  dom.drawerIcon.innerHTML = ICONS[selectedMod.icon] || ''
  dom.drawerTitle.textContent = selectedMod.name
  dom.drawerBadge.textContent = badge.label
  dom.drawerBadge.className = `node-badge ${badge.className}`
  
  const meta = MODULE_META[selectedMod.id] || {}
  dom.drawerSnippetTitle.textContent = meta.snippet || '原始检测数据'
  dom.drawerActionText.textContent = meta.action || '查看完整详情'

  if (modData && modData.finishedAt) {
    const t = new Date(modData.finishedAt)
    const pad = (n) => String(n).padStart(2, '0')
    dom.drawerTime.textContent = `${t.getFullYear()}-${pad(t.getMonth() + 1)}-${pad(t.getDate())} ${pad(t.getHours())}:${pad(t.getMinutes())}:${pad(t.getSeconds())}`
  } else {
    dom.drawerTime.textContent = '--:--:--'
  }

  renderDrawerDetails(selectedMod.id, modData)
}

// "200" + "200 OK" 会重复，统一成 "200 OK"
function statusLine(d) {
  const code = d.statusCode ? String(d.statusCode) : ''
  const text = d.statusText ? String(d.statusText) : ''
  if (!text) return code || '--'
  return text.startsWith(code) ? text : `${code} ${text}`.trim()
}

// 依据选中的模块动态渲染键值对与原始数据
function renderDrawerDetails(moduleId, modEnvelope) {
  const d = modEnvelope?.data || {}
  let summary = ''
  let metrics = []
  let rawSnippet = ''

  switch (moduleId) {
    case 'ssl':
      summary = d.statusLabel 
        ? `网站已正确配置 SSL 证书，验证状态：${d.statusLabel}。${d.verifyError ? ' (' + d.verifyError + ')' : ''}`
        : '网站已配置安全 SSL 证书。'
      metrics = [
        { k: '颁发机构 (CA)', v: d.issuer || '--' },
        { k: '生效时间', v: d.validFrom || '--' },
        { k: '到期时间', v: d.validTo || '--' },
        { k: '剩余天数', v: d.remainingDays !== undefined ? `${d.remainingDays} 天` : '--', highlight: true },
        { k: '验证状态', v: d.verified ? '✔ 正常' : (d.verifyError || '未通过') },
        { k: 'SAN (主题备用名)', list: d.dnsNames || [] },
        { k: 'TLS 协议版本', v: d.tlsVersion || '--' },
        { k: '加密套件', v: d.cipherSuite || '--' }
      ]
      rawSnippet = [
        ['Subject', d.subject],
        ['Issuer', d.issuer],
        ['Valid From', d.validFrom],
        ['Valid To', d.validTo],
        ['Signature', d.signatureAlg],
        ['Serial', d.serialNumber],
        ['SAN', (d.dnsNames || []).join(', ')],
      ].map(([k, v]) => `${(k + ':').padEnd(12)}${v || '--'}`).join('\n')
      if (d.rawCertificate) {
        rawSnippet += '\n\n' + d.rawCertificate
      }
      break

    case 'rdap':
      summary = `域名由 ${d.registrar || '注册局'} 管理` +
        (d.remainingDays !== null && d.remainingDays !== undefined
          ? `，距到期还有 ${d.remainingDays} 天。`
          : '。')
      metrics = [
        { k: '注册域名', v: d.registeredDomain || '--' },
        { k: '注册人', v: d.registrant || '未公开 / 隐私保护' },
        { k: '注册商', v: d.registrar || '--' },
        { k: '注册时间', v: d.registrationDate || '--' },
        { k: '更新时间', v: d.updatedDate || '--' },
        { k: '到期时间', v: d.expirationDate || '--' },
        { k: '到期剩余', v: d.remainingDays !== null && d.remainingDays !== undefined ? `${d.remainingDays} 天` : '--', highlight: true },
        { k: '域名状态', v: (d.status || []).join(', ') || '--' },
        { k: '权威名称服务器', v: (d.nameservers || []).join(', ') || '--' },
        { k: '数据来源', v: d.source || '--' }
      ]
      rawSnippet = JSON.stringify(d, null, 2)
      break

    case 'dns': {
      const failed = d.failedTypes || []
      // 查询超时的类型是"未知"，不能当成"没有记录"
      const stateOf = (type, has, yes, no) =>
        failed.includes(type) ? '未知（查询超时）' : (has ? yes : no)
      summary = `成功获取 ${d.totalCount || 0} 条 DNS 解析记录。` +
        (failed.length ? ` ${failed.join('/')} 记录查询超时，结果可能不完整。` : '')
      metrics = [
        { k: '查询解析器', v: d.nameserver || '--' },
        { k: '解析 A 记录 (IPv4)', v: d.primaryIP || (d.hasA ? '存在' : '无') },
        { k: '解析 AAAA 记录 (IPv6)', v: stateOf('AAAA', d.hasAAAA, '已配置', '未配置') },
        { k: 'CNAME 别名', v: stateOf('CNAME', d.hasCNAME, '已配置', '无') },
        { k: '总记录数', v: `${d.totalCount || 0} 条` }
      ]
      rawSnippet = (d.records || []).map(r => `[${r.type}] ${r.name} -> ${r.value} (TTL ${r.ttl})`).join('\n')
      break
    }

    case 'multidns':
      summary = d.summary || '从多个公共解析器获取结果对比。'
      metrics = (d.resolvers || []).map(r => {
        if (r.status && r.status !== 'success') {
          return { k: r.name, v: `请求失败 (${r.durationMs}ms)` }
        }
        return { k: r.name, v: `${r.ips && r.ips.length ? r.ips.join(', ') : '无 A 记录'} (${r.durationMs}ms)` }
      })
      rawSnippet = JSON.stringify(d.resolvers || [], null, 2)
      break

    case 'web':
      summary = `HTTP 响应 ${statusLine(d)}，TTFB 首字节耗时 ${d.latency?.ttfbMs || 0}ms。`
      metrics = [
        { k: '目标 URL', v: d.url || '--' },
        { k: '最终 URL', v: d.finalUrl || '--' },
        { k: 'HTTP 状态码', v: statusLine(d), highlight: true },
        { k: '协议版本', v: d.proto || '--' },
        { k: '服务器标识 (Server)', v: d.server || '隐藏/未提供' },
        { k: 'DNS 解析耗时', v: `${d.latency?.dnsLookupMs || 0} ms` },
        { k: 'TCP 连接耗时', v: `${d.latency?.tcpConnectMs || 0} ms` },
        { k: 'TLS 握手耗时', v: `${d.latency?.tlsHandshakeMs || 0} ms` },
        { k: '首字节响应 (TTFB)', v: `${d.latency?.ttfbMs || 0} ms` },
        { k: '总请求耗时', v: `${d.latency?.totalMs || 0} ms` }
      ]
      rawSnippet = JSON.stringify(d, null, 2)
      break

    case 'port':
      summary = '对 Web 基础端口 80 (HTTP) 与 443 (HTTPS) 进行连通测试。'
      metrics = (d.ports || []).map(p => ({
        k: `端口 TCP ${p.port}`,
        v: `${p.statusText} (${p.latencyMs}ms)`,
        highlight: p.open
      }))
      rawSnippet = JSON.stringify(d.ports || [], null, 2)
      break

    case 'redirect':
      summary = d.summary || 'HTTP 跳转追踪已完成。'
      metrics = [
        { k: '初始 URL', v: d.initialUrl || '--' },
        { k: '最终 URL', v: d.finalUrl || '--' },
        { k: '跳转次数', v: `${(d.hopCount || 1) - 1} 次` },
        { k: '跳转循环', v: d.hasLoop ? '⚠️ 检测到循环' : '正常 (无循环)' }
      ]
      rawSnippet = (d.hops || []).map(h => `Hop ${h.hopIndex}: ${h.url} -> HTTP ${h.statusCode} (${h.location || 'Terminal'})`).join('\n')
      break

    case 'mail':
      summary = d.summary || '邮件协议 DNS 相关安全配置。'
      metrics = [
        { k: 'MX 记录', v: d.hasMX ? `${d.mxRecords.length} 条 (首选: ${d.mxRecords[0]?.host})` : '未配置' },
        { k: 'SPF 记录', v: d.hasSPF ? '已配置 (保护发信域)' : '未配置' },
        { k: 'DMARC 策略', v: d.hasDMARC ? `已启用 (p=${d.dmarcPolicy})` : '未配置' },
        { k: 'DKIM 状态', v: d.dkimStatus || '--' }
      ]
      rawSnippet = `MX Records:\n${(d.mxRecords || []).map(m => `Priority ${m.preference}: ${m.host}`).join('\n')}\n\nSPF:\n${d.spfRecord || 'None'}\n\nDMARC:\n${d.dmarcRecord || 'None'}`
      break

    case 'dnssec':
      summary = d.summary || '域名安全扩展（DNSSEC）配置状态。'
      metrics = [
        { k: 'DS 记录', v: d.hasDS ? `已配置 (${d.dsRecords.length} 条)` : '未配置' },
        { k: 'DNSKEY 记录', v: d.hasDNSKEY ? `已配置 (${d.dnskeyRecords.length} 条)` : '未配置' },
        { k: 'RRSIG 签名', v: d.hasRRSIG ? '已检测到' : '无' },
        { k: '验证状态', v: d.statusLabel || '未启用', highlight: d.statusLabel === '验证通过' }
      ]
      rawSnippet = JSON.stringify(d, null, 2)
      break
  }

  // 模块失败时以错误原因为准，盖掉"一切正常"的模板文案
  if (modEnvelope?.error) summary = modEnvelope.error
  dom.drawerSummary.textContent = summary

  // 渲染指标列表（list 类长内容单独成行，用胶囊标签换行铺开）
  dom.drawerMetricList.innerHTML = metrics.map(m => {
    if (m.list) {
      const chips = m.list.length
        ? m.list.map(n => `<span class="metric-chip">${n}</span>`).join('')
        : '<span class="metric-val">--</span>'
      return `
        <div class="metric-item block">
          <span class="metric-key">${m.k}<span class="metric-count">（${m.list.length}）</span></span>
          <div class="metric-chips">${chips}</div>
        </div>
      `
    }
    // 长文本（URL、长描述）整行显示，避免右栏被压成竖条
    const long = typeof m.v === 'string' && m.v.length > 26
    return `
      <div class="metric-item ${long ? 'block' : ''}">
        <span class="metric-key">${m.k}</span>
        <span class="metric-val ${m.highlight ? 'status-green' : ''}">${m.v}</span>
      </div>
    `
  }).join('')

  // 渲染原始数据
  dom.drawerSnippetContent.textContent = rawSnippet || '暂无原始数据'
}

// 复制工具函数：兼容 navigator.clipboard 与非安全域/iframe 下的 fallbackCopyText
function copyToClipboard(text, btnEl) {
  if (!text) return

  const showFeedback = () => {
    if (!btnEl) return
    const origHTML = btnEl.innerHTML
    btnEl.innerHTML = ICONS.check || '✔'
    setTimeout(() => {
      btnEl.innerHTML = origHTML
    }, 1500)
  }

  // 1. 尝试现代 navigator.clipboard
  if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
    navigator.clipboard.writeText(text)
      .then(showFeedback)
      .catch(() => fallbackCopy(text, showFeedback))
    return
  }

  // 2. 降级方案：创建临时 textarea 选中执行 execCommand('copy')
  fallbackCopy(text, showFeedback)
}

function fallbackCopy(text, onSuccess) {
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.position = 'fixed'
  ta.style.top = '0'
  ta.style.left = '0'
  ta.style.opacity = '0'
  ta.style.pointerEvents = 'none'
  document.body.appendChild(ta)
  ta.focus()
  ta.select()
  try {
    const ok = document.execCommand('copy')
    if (ok && onSuccess) onSuccess()
  } catch (err) {
    console.error('fallback copy failed:', err)
  } finally {
    document.body.removeChild(ta)
  }
}



// 打开完整检测报告弹窗
function openReportModal() {
  if (!state.currentDomain) return
  dom.modalDomainTitle.textContent = `${state.currentDomain} 完整诊断报告`
  const ov = state.overviewData || {}
  dom.modalMeta.textContent = `诊断时间：${ov.timestamp ? formatTime(ov.timestamp) : formatTime(new Date())} · 主 IP：${ov.primaryIP || '--'}`

  // 组装全部 9 个模块的卡片
  let bodyHtml = ''
  MODULES_CONFIG.forEach(m => {
    const mod = state.modules[m.id]
    const badge = getStatusBadge(m.id, mod)
    const d = mod?.data || {}

    let itemsHtml = ''
    if (!mod || mod.status === 'pending') {
      itemsHtml = '<div style="color: var(--text-muted); font-size: 12px;">检测中或未开始...</div>'
    } else if (mod.error && (!d || Object.keys(d).length === 0)) {
      itemsHtml = `<div style="color: var(--error); font-size: 12px;">${mod.error}</div>`
    } else {
      itemsHtml = `<pre style="font-family: var(--font-mono); font-size: 11px; color: #9cb3c9; white-space: pre-wrap; word-break: break-all; max-height: 160px; overflow-y: auto;">${JSON.stringify(d, null, 2)}</pre>`
    }

    bodyHtml += `
      <div class="report-section">
        <div class="report-section-header">
          <div class="report-section-title">
            <span>${ICONS[m.icon] || ''}</span>
            <span>${m.name}</span>
          </div>
          <span class="node-badge ${badge.className}">${badge.label}</span>
        </div>
        <div>${itemsHtml}</div>
      </div>
    `
  })

  dom.modalBody.innerHTML = bodyHtml
  dom.reportModal.classList.add('active')
}

function closeReportModal() {
  dom.reportModal.classList.remove('active')
}

// 启动应用
document.addEventListener('DOMContentLoaded', init)
