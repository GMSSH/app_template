/* ============================================================
   站长百宝箱 —— 首页 + 10 个工具
   ------------------------------------------------------------
   纯原生 ES Module，零构建。界面与交互沿用设计稿；
   网络类工具（DNS / Ping / 端口 / HTTP / SSL / WHOIS）全部走
   后端 CLI 拿真实数据，文本类工具（JSON / Base64 / 哈希 / 时间）
   在浏览器本地计算，同样不造数据。
   ============================================================ */

import { callToolbox, envMode } from './cli.js'
import { DIGESTS } from './sha.js'

const IC = {globe:'<circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20"/>',act:'<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>',plug:'<path d="M12 22v-5M9 8V2M15 8V2M18 8v5a6 6 0 0 1-12 0V8z"/>',bars:'<path d="M5 21v-7M12 21V8M19 21V3" stroke-width="3.4"/>',shield:'<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/>',at:'<circle cx="12" cy="12" r="4"/><path d="M16 8v5a3 3 0 0 0 6 0v-1a10 10 0 1 0-4 8"/>',br:'<path d="M8 3H7a2 2 0 0 0-2 2v5a2 2 0 0 1-2 2 2 2 0 0 1 2 2v5a2 2 0 0 0 2 2h1M16 21h1a2 2 0 0 0 2-2v-5a2 2 0 0 1 2-2 2 2 0 0 1-2-2V5a2 2 0 0 0-2-2h-1"/>',bin:'<circle cx="12" cy="12" r="10"/><path d="M9 8v4M9 8 7.5 9M15 8a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3zM8 14.5h2V17H8zM14 14.5v2.5"/>',hash:'<path d="M4 9h16M4 15h16M10 3 8 21M16 3l-2 18"/>',clock:'<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',search:'<circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/>',link:'<path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/>',r:'<path d="M5 12h14M12 5l7 7-7 7"/>',l:'<path d="M19 12H5M12 19l-7-7 7-7"/>',sun:'<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',moon:'<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/>',set:'<path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6"/>',srv:'<rect x="2" y="3" width="20" height="7" rx="2"/><rect x="2" y="14" width="20" height="7" rx="2"/><path d="M6 6.5h.01M6 17.5h.01"/>',ok:'<circle cx="12" cy="12" r="10"/><path d="m8 12 3 3 5-6"/>',box:'<rect x="3" y="8" width="18" height="12" rx="3"/><path d="M8 8V5a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v3M3 13h18"/>'}
const ico = (n, s = 24) => `<svg width="${s}" height="${s}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">${IC[n]}</svg>`

// 工具实现（route() 里按 hash 查表调用）
const TOOLS = {}

const T = [
  ['dns', 'DNS 查询', '解析域名记录', 'globe', '查询域名的各类 DNS 记录'],
  ['ping', 'Ping 检测', '网络连通测试', 'act', '测试到目标主机的网络延迟'],
  ['port', '端口检测', '检查端口开放', 'plug', '扫描目标主机常用端口'],
  ['http', 'HTTP 状态', '查看响应信息', 'bars', '查看站点的响应状态与响应头'],
  ['ssl', 'SSL 证书', '证书详情查询', 'shield', '查看域名 SSL 证书详情与有效期'],
  ['whois', 'WHOIS 查询', '域名注册信息', 'at', '查询域名注册与归属信息'],
  ['json', 'JSON 工具', '格式化与校验', 'br', '格式化、压缩并校验 JSON'],
  ['b64', 'Base64 编码', '编码与解码', 'bin', '文本与 Base64 互相转换'],
  ['hash', '哈希计算', 'SHA 系列', 'hash', '实时计算文本的 SHA 哈希'],
  ['time', '时间戳转换', '日期与时间', 'clock', 'Unix 时间戳与日期互转'],
]

const $ = (s, r = document) => r.querySelector(s)
const view = $('#view')
let TM = []
let IV = 0
const later = (f, ms) => TM.push(setTimeout(f, ms))

/** 所有来自服务器/用户的数据在插入 HTML 前都要转义 */
const esc = (s) =>
  String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))

const toast = (m) => {
  const t = $('#toast')
  t.textContent = m
  t.classList.add('on')
  clearTimeout(toast.t)
  toast.t = setTimeout(() => t.classList.remove('on'), 1600)
}

/** 宿主 iframe 或 http 源里 navigator.clipboard 常常不可用，必须回退到 execCommand */
const copy = async (s) => {
  try {
    await navigator.clipboard.writeText(s)
    return toast('已复制')
  } catch (e) {
    /* 继续走回退方案 */
  }
  const ta = document.createElement('textarea')
  ta.value = s
  ta.style.position = 'fixed'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  ta.select()
  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch (e) {
    ok = false
  }
  document.body.removeChild(ta)
  toast(ok ? '已复制' : '复制失败，请手动选择')
}

$('#s1').innerHTML = ico('sun', 14)
$('#s2').innerHTML = ico('moon', 14)


function setTheme(t) {
  document.documentElement.dataset.theme = t
  try {
    localStorage.setItem('gm-theme', t)
  } catch (e) {
    /* 隐私模式下不可写，忽略 */
  }
}
try {
  setTheme(localStorage.getItem('gm-theme') || 'dark')
} catch (e) {
  setTheme('dark')
}
$('#sw').onclick = () => setTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark')

const bar = (ph, btn, ic = 'link') =>
  `<div class="bar2"><label class="inp">${ico(ic, 20)}<input id="q" placeholder="${esc(ph)}" autocomplete="off" spellcheck="false"></label><button class="go" id="go">${ico('search', 18)}<span>${esc(btn)}</span></button></div>`
const skel = '<div class="panel"><div class="skel" style="width:40%"></div><div class="skel"></div><div class="skel" style="width:80%"></div><div class="skel" style="width:60%"></div></div>'
const rows = (a, cls = '') =>
  `<div class="rows">${a.map((r, i) => `<div class="row ${cls}" style="--i:${i}"><span>${esc(r[0])}</span><span>${r[1]}</span></div>`).join('')}</div>`
const head = (t, right = '') => `<div class="ph">${ico('act', 20)}<b>${esc(t)}</b>${right}</div>`
const badge = (ok, text) => `<span class="ok"${ok ? '' : ' style="color:#f87171"'}>${ico(ok ? 'ok' : 'act', 18)}${esc(text)}</span>`
const notice = (msg) => `<div class="panel"><div class="ph">${ico('act', 20)}<b>检测失败</b></div><div class="msg bad">✗ ${esc(msg)}</div></div>`
const meta = (a) => `<div class="meta">${a.map((s) => `<span>${s}</span>`).join('')}</div>`

/**
 * 统一的「输入 → 执行 → 渲染」流程。
 * opt.full：保留完整 URL（HTTP 检测需要）；否则只取主机名。
 */
function wire(b, render, opt = {}) {
  const q = $('#q', b)
  const go = $('#go', b)
  const out = $('#out', b)

  const norm = (v) => {
    v = (v || '').trim()
    if (!opt.full) v = v.replace(/^https?:\/\//, '').replace(/[\/?#].*$/, '')
    return v
  }

  const run = async () => {
    // 进入工具不自动执行，也不需要默认目标：必须由用户输入
    const v = norm(q.value)
    if (!v) {
      out.innerHTML = `<div class="panel"><div class="msg">请先输入检测目标</div></div>`
      return
    }
    q.value = v
    go.classList.add('load')
    out.innerHTML = skel
    try {
      await render(out, v)
    } catch (e) {
      out.innerHTML = notice(e && e.message ? e.message : String(e))
    } finally {
      go.classList.remove('load')
    }
  }

  go.onclick = run
  q.onkeydown = (e) => e.key === 'Enter' && run()
  return run
}

const kb = (n) => (n / 1024).toFixed(1) + ' KB'

/* ------------------------------------------------------------------ DNS */
TOOLS.dns = (b) => {
  let ty = 'A'
  b.innerHTML =
    bar('请输入域名，例如：example.com', '查询') +
    `<div class="chips">${['A', 'AAAA', 'MX', 'NS', 'TXT', 'CNAME', 'ALL']
      .map((c, i) => `<button class="chip${i ? '' : ' on'}">${c}</button>`)
      .join('')}</div><div id="out"></div>`

  const run = wire(
    b,
    async (o, d) => {
      const r = await callToolbox('dns', { target: d, type: ty, timeout: 12 })
      const server = r.server ? 'DNS 服务器：' + r.server : '使用本机默认 DNS'
      const tail = meta([ico('clock', 18) + '查询耗时：' + (r.elapsed_ms || 0) + ' ms', ico('srv', 18) + esc(server + (r.tool ? ' · ' + r.tool : ''))])

      if (ty === 'ALL') {
        const groups = (r.groups || []).filter((g) => g.records && g.records.length)
        if (!groups.length) {
          const why = r.error_text || (r.groups || []).map((g) => g.error_text).filter(Boolean)[0] || '未查到任何记录'
          o.innerHTML = notice(why) + tail
          return
        }
        o.innerHTML =
          `<div class="panel">${head('查询结果', badge(true, '查询成功'))}` +
          rows(groups.map((g) => [g.type + ' 记录', g.records.map((x) => esc(x.value)).join('<br>')])) +
          '</div>' + tail
        return
      }

      if (r.error_kind) {
        o.innerHTML = notice(r.error_text || '查询失败') + tail
        return
      }
      o.innerHTML =
        `<div class="panel">${head('查询结果')}` +
        rows(r.records.map((x) => [x.type + ' 记录', esc(x.value) + (x.ttl ? ` <span style="color:var(--tx2)">TTL ${x.ttl}</span>` : '')])) +
        '</div>' + tail
    },
  )

  b.querySelectorAll('.chip').forEach(
    (c) =>
      (c.onclick = () => {
        b.querySelectorAll('.chip').forEach((x) => x.classList.remove('on'))
        c.classList.add('on')
        ty = c.textContent
        run()
      }),
  )
}

/* ----------------------------------------------------------------- Ping */
TOOLS.ping = (b) => {
  b.innerHTML = bar('请输入域名或 IP，例如：example.com', '开始 Ping', 'link') + '<div id="out"></div>'
  wire(
    b,
    async (o, h) => {
      const r = await callToolbox('ping', { target: h, count: 5, timeout: 12 })
      if (!r.supported) throw new Error(r.error_text || '当前服务器无法执行 ping')
      const done = !r.error_kind
      const list = (r.samples || [])
        .map(
          (s, i) =>
            `<div class="row pr" style="--i:${i}"><span>#${i + 1}</span><span>${
              s.loss ? '请求超时' : `TTL ${s.ttl || '-'} · ${s.bytes || '?'} bytes`
            }</span><div class="lat"><i style="width:${s.loss ? 0 : Math.min(100, s.ms * 1.8)}%"></i></div><b>${
              s.loss ? '-' : s.ms + ' ms'
            }</b></div>`,
        )
        .join('')
      const stats = [
        [r.min_ms + ' ms', '最小'],
        [r.avg_ms + ' ms', '平均'],
        [r.max_ms + ' ms', '最大'],
        [Math.round(r.loss_pct) + '%', '丢包'],
      ]
        .map((s, i) => `<div style="animation-delay:${i * 80}ms"><b>${esc(s[0])}</b><small>${s[1]}</small></div>`)
        .join('')
      o.innerHTML =
        `<div class="panel">${head('Ping ' + h, badge(done, done ? '测试完成' : '无响应'))}<div class="rows">${list}</div></div>` +
        `<div class="stats">${stats}</div>` +
        (done ? '' : meta([esc(r.error_text)]))
    },
  )
}

/* ----------------------------------------------------------------- 端口 */
TOOLS.port = (b) => {
  b.innerHTML = bar('请输入域名或 IP，例如：example.com', '开始扫描') + '<div id="out"></div>'
  wire(
    b,
    async (o, h) => {
      const r = await callToolbox('port', { target: h, timeout: 10 })
      const TAG = {
        open: ['t-o', '开放'],
        closed: ['t-c', '关闭'],
        filtered: ['t-f', '已过滤'],
        error: ['t-c', '失败'],
      }
      const list = r.checks
        .map((c, i) => {
          const [cls, label] = TAG[c.state] || TAG.error
          const text = c.state === 'error' && c.error_text ? c.error_text : label
          return `<div class="row pt" style="--i:${i}"><b>${c.port}</b><span>${esc(c.service)}</span><span class="tag ${cls}" title="${esc(c.error_text || '')}">${esc(text)}</span></div>`
        })
        .join('')
      o.innerHTML =
        `<div class="panel">${head('扫描 ' + h, badge(true, '扫描完成'))}<div class="rows">${list}</div></div>` +
        meta([
          ico('clock', 18) + '耗时 ' + r.elapsed_ms + ' ms',
          ico('srv', 18) + `开放 ${r.open} · 关闭 ${r.closed} · 过滤 ${r.filtered}`,
        ])
    },
  )
}

/* ----------------------------------------------------------------- HTTP */
const STATUS_TEXT = {
  200: 'OK',
  201: 'Created',
  204: 'No Content',
  301: 'Moved Permanently',
  302: 'Found',
  304: 'Not Modified',
  307: 'Temporary Redirect',
  308: 'Permanent Redirect',
  400: 'Bad Request',
  401: 'Unauthorized',
  403: 'Forbidden',
  404: 'Not Found',
  429: 'Too Many Requests',
  500: 'Internal Server Error',
  502: 'Bad Gateway',
  503: 'Service Unavailable',
  504: 'Gateway Timeout',
}

TOOLS.http = (b) => {
  b.innerHTML = bar('请输入网址，例如：https://example.com', '检查') + '<div id="out"></div>'
  wire(
    b,
    async (o, h) => {
      const r = await callToolbox('http', { target: h, timeout: 15 })
      if (r.error_kind) throw new Error(r.error_detail || r.error_kind)
      const ok = r.status_code > 0 && r.status_code < 400
      const head_ = (r.headers || []).map((x) => [x.name, esc(x.value)])
      o.innerHTML =
        `<div class="panel">${head('响应信息', badge(ok, ok ? '请求成功' : 'HTTP ' + r.status_code))}` +
        `<div class="ssl"><div class="badge"${ok ? '' : ' style="color:#f87171"'}>${r.status_code}</div><div><b>${esc(
          STATUS_TEXT[r.status_code] || 'HTTP 响应',
        )}</b><br><span style="color:var(--tx2)">${esc(r.protocol || '-')} · ${r.stage.total_ms} ms · ${kb(
          r.size_bytes,
        )}</span></div></div>` +
        rows(head_) +
        '</div>' +
        meta([
          ico('srv', 18) + '远程 IP：' + esc(r.remote_ip || '-'),
          ico('link', 18) + '重定向 ' + r.redirects + ' 次',
        ])
    },
    { full: true },
  )
}

/* ------------------------------------------------------------------ SSL */
TOOLS.ssl = (b) => {
  b.innerHTML = bar('请输入域名，例如：example.com', '查询') + '<div id="out"></div>'
  wire(
    b,
    async (o, d) => {
      const r = await callToolbox('ssl', { target: d, port: 443, timeout: 15 })
      const c = r.cert
      if (!c) throw new Error(r.error_text || '未获取到证书')
      const days = c.days_left
      const C = 2 * Math.PI * 45
      const frac = Math.max(0, Math.min(1, days / 180))
      const color = days < 0 ? '#f87171' : days <= 30 ? '#d9a10b' : 'var(--ok)'
      const state = r.expired ? '证书已过期' : r.verify_ok ? '证书有效' : '证书校验未通过'
      o.innerHTML =
        `<div class="panel">${head('证书详情', badge(!r.expired && r.verify_ok, state))}` +
        `<div class="ssl"><svg class="ring" viewBox="0 0 110 110"><circle cx="55" cy="55" r="45" stroke="var(--card2)"/><circle id="rg" cx="55" cy="55" r="45" stroke="${color}" stroke-dasharray="${C}" stroke-dashoffset="${C}" transform="rotate(-90 55 55)" style="transition:stroke-dashoffset 1.2s cubic-bezier(.2,.8,.2,1)"/><text x="55" y="53" text-anchor="middle" fill="currentColor" font-size="26" font-weight="800">${days}</text><text x="55" y="72" text-anchor="middle" fill="var(--tx2)" font-size="11">${days < 0 ? '已过期' : '天后到期'}</text></svg>` +
        `<div><b style="font-size:18px">${esc(c.subject)}</b><br><span style="color:var(--tx2)">${esc(
          r.verify_text || '',
        )} · ${esc(r.protocol || '')} · ${esc(r.cipher_suite || '')}</span></div></div>` +
        rows([
          ['颁发给', esc(c.subject)],
          ['颁发者', esc(c.issuer)],
          ['有效期', esc(c.not_before + ' ~ ' + c.not_after)],
          ['签名算法', esc(c.signature_alg)],
          ['公钥', esc(c.public_key_alg + ' ' + c.public_key_bits + ' bit')],
          ['域名匹配', esc(r.matched_by)],
          ['证书链', esc((c.chain_subjects || []).join(' → ') || '-')],
          ['SHA-256 指纹', `<span style="font-family:ui-monospace,Menlo,monospace;font-size:13px;word-break:break-all">${esc(
            c.fingerprint_sha256 || '-',
          )}</span>`],
        ]) +
        '</div>'
      later(() => {
        const ring = $('#rg', o)
        if (ring) ring.style.strokeDashoffset = C * (1 - frac)
      }, 60)
    },
  )
}

/* ---------------------------------------------------------------- WHOIS */
TOOLS.whois = (b) => {
  b.innerHTML = bar('请输入域名，例如：example.com', '查询', 'at') + '<div id="out"></div>'
  wire(
    b,
    async (o, d) => {
      const r = await callToolbox('whois', { target: d, timeout: 6 })
      if (!r.found) throw new Error(r.error_text || '未查到注册信息')
      o.innerHTML =
        `<div class="panel">${head('注册信息')}` + rows(r.fields.map((f) => [f.name, esc(f.value)])) + '</div>' +
        meta([
          ico('srv', 18) + esc(r.servers.join(' → ')),
          ico('clock', 18) + '耗时 ' + r.elapsed_ms + ' ms',
        ])
    },
  )
}

/* ----------------------------------------------------------------- JSON */
TOOLS.json = (b) => {
  b.innerHTML = `<textarea id="in" placeholder='在此粘贴 JSON，例如：{"name":"GM Tools"}'></textarea><div class="btns"><button class="chip on" data-a="f">格式化</button><button class="chip on" data-a="m">压缩</button><button class="chip on" data-a="v">校验</button><button class="chip" data-a="s">示例</button></div><div class="panel" style="margin-top:0"><div id="msg" class="msg">等待输入…</div><pre id="o"></pre></div>`
  const i = $('#in', b)
  const doit = (a) => {
    if (a === 's') {
      i.value = '{"name":"GM Tools","version":"1.0.0","tools":["dns","ping","ssl"],"author":{"id":7,"active":true}}'
      a = 'f'
    }
    try {
      const j = JSON.parse(i.value)
      $('#msg').className = 'msg good'
      $('#msg').textContent = '✓ JSON 合法'
      $('#o').textContent = a === 'm' ? JSON.stringify(j) : JSON.stringify(j, null, 2)
    } catch (e) {
      $('#msg').className = 'msg bad'
      $('#msg').textContent = '✗ ' + (i.value.trim() ? e.message : '请先输入 JSON')
      $('#o').textContent = ''
    }
  }
  b.querySelectorAll('[data-a]').forEach((x) => (x.onclick = () => doit(x.dataset.a)))
}

/* --------------------------------------------------------------- Base64 */
TOOLS.b64 = (b) => {
  b.innerHTML = `<textarea id="in" placeholder="输入文本或 Base64"></textarea><div class="btns"><button class="chip on" data-a="e">编码</button><button class="chip on" data-a="d">解码</button><button class="chip" data-a="c">复制结果</button></div><div class="panel" style="margin-top:0"><div id="msg" class="msg">结果</div><pre id="o"></pre></div>`
  const enc = (s) => {
    const bytes = new TextEncoder().encode(s)
    let bin = ''
    for (const b of bytes) bin += String.fromCharCode(b)
    return btoa(bin)
  }
  const dec = (s) => {
    const bin = atob(s)
    const bytes = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
    return new TextDecoder().decode(bytes)
  }
  b.querySelectorAll('[data-a]').forEach(
    (x) =>
      (x.onclick = () => {
        const a = x.dataset.a
        const v = $('#in', b).value
        if (a === 'c') return $('#o').textContent && copy($('#o').textContent)
        try {
          $('#o').textContent = a === 'e' ? enc(v) : dec(v.trim())
          $('#msg').className = 'msg good'
          $('#msg').textContent = a === 'e' ? '✓ 已编码' : '✓ 已解码'
        } catch (e) {
          $('#msg').className = 'msg bad'
          $('#msg').textContent = '✗ 不是有效的 Base64'
          $('#o').textContent = ''
        }
      }),
  )
}

/* ----------------------------------------------------------------- 哈希 */
TOOLS.hash = (b) => {
  b.innerHTML = `<textarea id="in" placeholder="输入要计算哈希的文本" style="min-height:120px">GM Tools</textarea><div class="panel"><div class="ph">${ico(
    'hash',
    20,
  )}<b>哈希结果</b><span class="ok" style="color:var(--tx2)">点击复制</span></div><div class="rows" id="hr"></div></div>`
  const input = $('#in', b)
  let timer = 0
  const calc = () => {
    const text = input.value
    $('#hr', b).innerHTML = DIGESTS.map(([name, fn]) => {
      let v = '计算失败'
      try {
        v = fn(text)
      } catch (e) {
        /* 保留失败提示 */
      }
      return `<div class="row hs"><span>${name}</span><span class="copy" data-v="${v}">${v}</span></div>`
    }).join('')
    b.querySelectorAll('.copy').forEach((c) => (c.onclick = () => copy(c.dataset.v || '')))
  }
  input.oninput = () => {
    clearTimeout(timer)
    timer = setTimeout(calc, 120)
  }
  calc()
}

/* ----------------------------------------------------------------- 时间 */
TOOLS.time = (b) => {
  b.innerHTML = `<div class="panel" style="margin-top:0"><div class="ph">${ico(
    'clock',
    20,
  )}<b>当前时间</b></div><div class="rows" id="now"></div></div><div class="panel"><div class="ph"><b>时间戳 → 日期</b></div><div class="bar2"><label class="inp"><input id="ts" placeholder="例如：1767225600"></label><button class="go" id="g1"><span>转换</span></button></div><div id="r1" class="msg" style="margin-top:12px"></div></div><div class="panel"><div class="ph"><b>日期 → 时间戳</b></div><div class="bar2"><label class="inp"><input id="dt" type="datetime-local" step="1"></label><button class="go" id="g2"><span>转换</span></button></div><div id="r2" class="msg" style="margin-top:12px"></div></div>`
  const f = (d) => d.toLocaleString('zh-CN', { hour12: false })
  const tick = () => {
    const n = Date.now()
    $('#now', b).innerHTML =
      `<div class="row hs"><span>日期时间</span><span>${f(new Date(n))}</span></div>` +
      `<div class="row hs"><span>秒</span><span class="copy" data-v="${(n / 1000) | 0}">${(n / 1000) | 0}</span></div>` +
      `<div class="row hs"><span>毫秒</span><span class="copy" data-v="${n}">${n}</span></div>`
    b.querySelectorAll('.copy').forEach((c) => (c.onclick = () => copy(c.dataset.v || '')))
  }
  tick()
  IV = setInterval(tick, 1000)
  $('#g1', b).onclick = () => {
    let v = +$('#ts', b).value
    if (!v) return ($('#r1', b).textContent = '请输入有效的时间戳')
    if (v < 1e11) v *= 1000
    $('#r1', b).textContent = '→ ' + f(new Date(v))
  }
  $('#g2', b).onclick = () => {
    const v = $('#dt', b).value
    $('#r2', b).textContent = v ? '→ ' + Math.floor(new Date(v).getTime() / 1000) : '请选择日期'
  }
}

/* ----------------------------------------------------------------- 首页 */
const envNote = () =>
  envMode() === 'none'
    ? `<div class="panel" style="margin-top:20px"><div class="ph">${ico('act', 20)}<b>当前环境不能执行服务器命令</b></div><div class="msg">网络类工具需要极目桌面宿主，或本地运行 <code>node web/dev-server.mjs</code> 后访问 http://dev.adde.cn:5185 。文本类工具可正常使用。</div></div>`
    : ''

function home() {
  return (
    `<div class="hero"><div></div></div><div class="hh"><h1>站长百宝箱</h1><p>开发与运维必备工具集</p></div>` +
    `<div class="grid">${T.map(
      (t, i) =>
        `<button class="card" style="--i:${i}" onclick="location.hash='${t[0]}'"><span class="big">${ico(
          t[3],
          44,
        )}</span><b>${t[1]}</b><small>${t[2]}</small><span class="go2">${ico('r', 16)}</span></button>`,
    ).join('')}</div>` +
    '<div class="foot">让服务器管理更简单</div>' +
    envNote()
  )
}

function route() {
  const t = T.find((x) => x[0] === location.hash.slice(1))
  TM.forEach(clearTimeout)
  TM = []
  clearInterval(IV)
  view.style.animation = 'none'
  void view.offsetWidth
  view.style.animation = ''
  view.scrollTop = 0

  if (!t) {
    view.innerHTML = home()
    return
  }
  view.innerHTML = `<button class="back" onclick="location.hash=''">${ico(
    'l',
    16,
  )}返回</button><div class="head"><div class="hi">${ico(t[3], 38)}</div><div><h2>${t[1]}</h2><p>${t[4]}</p></div></div><div id="body"></div>`
  TOOLS[t[0]]($('#body'))
}

addEventListener('hashchange', route)
route()

