<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  Clock,
  HardDrive,
  User,
  X,
  Container,
  Settings,
  ShieldCheck,
  ShieldAlert,
  PowerOff
} from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import type { ProcessDetail, PortGroup } from '../types/port'
import { releasePort } from '../api/ports'

const props = defineProps<{ detail: ProcessDetail | null; refreshing: boolean }>()
const emit = defineEmits<{ close: []; requery: [] }>()
const { t } = useI18n()

const releasingKey = ref<string | null>(null)
const confirmAll = ref(false)
const releaseMsg = ref<{ type: 'ok' | 'err'; text: string } | null>(null)

// 22 端口、SSH 服务及 GMSSH 宿主底层核心服务 (gm_main, ga_main, gmssh 等) 属于远程管控基础，一律禁用释放与终止
// 精确保护 22 端口、SSH 守护进程以及 GMSSH 宿主底层核心进程 (ga_main / gm_main)
// 严禁误伤普通 Docker 容器 (如 GMSSH-openlist 等应用容器)
function isSystemCoreName(str: string): boolean {
  const s = str.toLowerCase().trim()
  if (!s) return false
  // 1. 精确匹配进程名
  if (s === 'ga_main' || s === 'gm_main' || s === 'sshd' || s === 'gm-agent') return true
  // 2. 匹配可执行文件二进制路径结尾 (如 /.__gmssh/bin/ga_main)
  if (/(\/|^)(ga_main|gm_main|sshd)(\s|$)/.test(s)) return true
  return false
}

const isProtectedProc = computed(() => {
  if (!props.detail) return false
  const name = props.detail.name || ''
  const sname = props.detail.service?.serviceName || ''
  const cmd = props.detail.command || ''
  if (isSystemCoreName(name) || isSystemCoreName(sname) || isSystemCoreName(cmd)) return true
  return props.detail.ports.some(p => p.port === 22)
})

const isProtectedPort = (port: number, name?: string) => {
  if (port === 22) return true
  const n = name || props.detail?.name || ''
  const cmd = props.detail?.command || ''
  return isSystemCoreName(n) || isSystemCoreName(cmd)
}

// 释放单个端口
async function releaseSingle(g: PortGroup) {
  releasingKey.value = g.key
  releaseMsg.value = null
  try {
    const res = await releasePort(g.port, g.protocol, g.pid)
    if (res.success) {
      releaseMsg.value = { type: 'ok', text: t('releasedSuccess', { p: g.port }) }
      setTimeout(() => {
        emit('requery')
        releaseMsg.value = null
      }, 700)
    } else {
      releaseMsg.value = { type: 'err', text: res.message || '释放失败' }
    }
  } catch (e: any) {
    releaseMsg.value = { type: 'err', text: e?.message || '释放失败' }
  } finally {
    releasingKey.value = null
  }
}

// 终止整个服务/进程
async function killWholeProcess() {
  if (!confirmAll.value) {
    confirmAll.value = true
    return
  }
  if (!props.detail) return
  releasingKey.value = 'all'
  releaseMsg.value = null
  try {
    const res = await releasePort(props.detail.ports[0]?.port ?? 0, 'tcp', props.detail.pid)
    if (res.success) {
      releaseMsg.value = { type: 'ok', text: t('processKilledSuccess') }
      setTimeout(() => {
        emit('requery')
        emit('close')
        releaseMsg.value = null
        confirmAll.value = false
      }, 800)
    } else {
      releaseMsg.value = { type: 'err', text: res.message || '终止进程失败' }
    }
  } catch (e: any) {
    releaseMsg.value = { type: 'err', text: e?.message || '终止进程失败' }
  } finally {
    releasingKey.value = null
  }
}

const service = computed(() => props.detail?.service)
const docker = computed(() => service.value?.docker)
const systemd = computed(() => service.value?.systemd)

const categoryLabel = computed(() => {
  switch (service.value?.category) {
    case 'web': return 'Web 服务'
    case 'db': return '数据库'
    case 'container': return '容器服务'
    case 'manager': return '进程管理'
    case 'system': return '系统服务'
    default: return '未托管进程'
  }
})

const categoryClass = computed(() => {
  switch (service.value?.category) {
    case 'web': return 'cat-web'
    case 'db': return 'cat-db'
    case 'container': return 'cat-container'
    case 'manager': return 'cat-manager'
    case 'system': return 'cat-system'
    default: return 'cat-unmanaged'
  }
})
</script>

<template>
  <aside class="panel" :class="{ open: !!detail }" aria-label="服务与端口详情">
    <div class="panel-inner" v-if="detail">
      <!-- 头部：服务名与关闭 -->
      <div class="panel-head">
        <div class="proc-title-area">
          <div class="name-row">
            <span class="category-pill" :class="categoryClass">{{ categoryLabel }}</span>
            <span class="proc-name mono">{{ service?.serviceName || detail.name }}</span>
            <span v-if="detail.pid" class="pid-badge mono">PID {{ detail.pid }}</span>
          </div>
          <!-- 服务真实简介（不强行捏造，有才显示） -->
          <p v-if="service?.summary" class="service-summary">{{ service.summary }}</p>
          <div class="tags-row">
            <span v-if="service?.uptime" class="meta-tag">
              <Clock :size="12" class="meta-icon" />
              <span>运行 {{ service.uptime }}</span>
            </span>
            <span v-if="service?.memoryRss" class="meta-tag">
              <HardDrive :size="12" class="meta-icon" />
              <span>内存 {{ service.memoryRss }}</span>
            </span>
            <span v-if="detail.user" class="meta-tag">
              <User :size="12" class="meta-icon" />
              <span>{{ detail.user }}</span>
            </span>
          </div>
        </div>
        <button class="close-btn" @click="emit('close')" title="关闭面板">
          <X :size="16" />
        </button>
      </div>

      <!-- 操作结果反馈 Toast -->
      <div v-if="releaseMsg" class="feedback-bar" :class="releaseMsg.type">
        {{ releaseMsg.text }}
      </div>

      <!-- 主体滚动区 -->
      <div class="panel-scroll">
        <div class="section">
          <!-- 1. Docker 容器深度信息 -->
          <div v-if="docker" class="docker-card">
            <div class="sec-label">
              <Container :size="13" style="margin-right: 4px;" />
              Docker 容器详情
            </div>
            <div class="prop-grid">
              <div v-if="docker.containerName" class="prop-item">
                <span class="k">容器名称</span>
                <span class="v mono highlight">{{ docker.containerName }}</span>
              </div>
              <div v-if="docker.image" class="prop-item">
                <span class="k">容器镜像</span>
                <span class="v mono">{{ docker.image }}</span>
              </div>
              <div v-if="docker.containerId" class="prop-item">
                <span class="k">容器 ID</span>
                <span class="v mono">{{ docker.containerId }}</span>
              </div>
              <div v-if="docker.internalPort" class="prop-item">
                <span class="k">容器内部端口</span>
                <span class="v mono">{{ docker.internalPort }}</span>
              </div>
            </div>
            <div v-if="docker.cmd" class="cmd-block" style="margin-top: 8px;">
              <span class="k">容器启动命令</span>
              <pre class="mono">{{ docker.cmd }}</pre>
            </div>
          </div>

          <!-- 2. Systemd 服务深度属性 -->
          <div v-if="systemd" class="systemd-card">
            <div class="sec-label">
              <Settings :size="13" style="margin-right: 4px;" />
              Systemd Unit 属性
            </div>
            <div class="prop-grid">
              <div v-if="systemd.unitName" class="prop-item">
                <span class="k">Unit 服务名称</span>
                <span class="v mono highlight">{{ systemd.unitName }}</span>
              </div>
              <div v-if="systemd.activeState" class="prop-item">
                <span class="k">ActiveState / SubState</span>
                <span class="v mono">{{ systemd.activeState }} ({{ systemd.subState }})</span>
              </div>
              <div v-if="systemd.fragmentPath" class="prop-item full-width">
                <span class="k">配置文件 (FragmentPath)</span>
                <span class="v mono selectable">{{ systemd.fragmentPath }}</span>
              </div>
              <div v-if="systemd.memoryCurrent" class="prop-item">
                <span class="k">当前内存占用</span>
                <span class="v mono">{{ systemd.memoryCurrent }}</span>
              </div>
              <div v-if="systemd.cpuUsageNSec" class="prop-item">
                <span class="k">CPU 累计耗时</span>
                <span class="v mono">{{ systemd.cpuUsageNSec }}</span>
              </div>
            </div>
            <div v-if="systemd.execStart" class="cmd-block" style="margin-top: 8px;">
              <span class="k">ExecStart 执行命令</span>
              <pre class="mono">{{ systemd.execStart }}</pre>
            </div>
          </div>

          <!-- 3. 第三层：服务占用的端口关系 -->
          <div class="ports-header-row">
            <span class="sec-label">服务监听端口清单 ({{ detail.ports.length }})</span>
          </div>

          <div class="port-list">
            <div
              v-for="g in detail.ports"
              :key="g.key"
              class="port-row-card"
              :class="{ highlighted: detail.selectedPortKey === g.key }"
            >
              <div class="port-meta-left">
                <div class="port-line">
                  <span class="proto-badge mono" :class="g.protocol.toLowerCase()">{{ g.protocol }}</span>
                  <span class="port-num mono">{{ g.port }}</span>
                  <span v-if="docker?.internalPort && g.port === 8080" class="container-map mono">
                    → 容器 {{ docker.internalPort }}
                  </span>
                  <span class="state-pill mono">{{ g.state }}</span>
                </div>
                <div class="addr-line mono">
                  <span class="addr-scope-dot" :class="g.external ? 'external' : 'local'"></span>
                  {{ g.addresses.join(', ') }}
                </div>
              </div>

              <!-- 单端口释放按钮 -->
              <button
                v-if="!isProtectedPort(g.port, detail.name)"
                class="release-single-btn"
                :disabled="releasingKey === g.key"
                @click="releaseSingle(g)"
                title="释放此端口"
              >
                {{ releasingKey === g.key ? '释放中…' : '释放端口' }}
              </button>
              <span v-else class="protected-pill" title="系统核心连接端口，系统保护禁止释放">
                <ShieldCheck :size="11" style="flex-shrink: 0;" />
                核心保护
              </span>
            </div>
          </div>

          <!-- 4. 未托管/通用进程启动信息 -->
          <div v-if="!docker && !systemd" class="process-meta-box">
            <div class="sec-label">进程执行信息</div>
            <div class="cmd-block" v-if="detail.command">
              <span class="k">启动命令</span>
              <pre class="mono">{{ detail.command }}</pre>
            </div>
            <div class="prop-item" v-if="detail.workingDirectory">
              <span class="k">工作目录</span>
              <span class="v mono selectable">{{ detail.workingDirectory }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部终极操作：终止整个服务 / 杀死进程 -->
      <div class="panel-foot">
        <button
          v-if="!isProtectedProc"
          class="kill-proc-btn"
          :class="{ confirm: confirmAll }"
          :disabled="releasingKey === 'all' || !detail.pid"
          @click="killWholeProcess"
        >
          <PowerOff :size="14" />
          {{ confirmAll ? '确认终止进程' : '终止进程' }}
        </button>
        <div v-else class="protected-foot-notice">
          <ShieldAlert :size="14" style="flex-shrink: 0;" />
          <span>系统核心保护：远程管控服务 (SSH / GMSSH)，禁止被终止</span>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  height: 100%;
  width: 420px;
  background: var(--panel);
  border-left: 1px solid var(--line);
  box-shadow: var(--sh);
  transform: translateX(100%);
  transition: transform 0.28s cubic-bezier(0.16, 1, 0.3, 1);
  z-index: 20;
  overflow: hidden;
}
.panel.open {
  transform: translateX(0);
}
.panel-inner {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-height: 100%;
  min-height: 0;
  overflow: hidden;
}
.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 16px;
  border-bottom: 1px solid var(--line);
  background: var(--p2);
}
.proc-title-area {
  flex: 1;
  min-width: 0;
  padding-right: 8px;
}
.name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.category-pill {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 6px;
  text-transform: uppercase;
}
.cat-web { background: rgba(59, 130, 246, 0.15); color: #2563eb; }
.cat-db { background: rgba(234, 88, 12, 0.15); color: #ea580c; }
.cat-container { background: rgba(13, 148, 136, 0.15); color: #0d9488; }
.cat-manager { background: rgba(124, 58, 237, 0.15); color: #7c3aed; }
.cat-system { background: rgba(100, 116, 139, 0.15); color: #475569; }
.cat-unmanaged { background: rgba(16, 185, 129, 0.15); color: #059669; }

:root[data-theme="dark"] .cat-web { color: #60a5fa; }
:root[data-theme="dark"] .cat-db { color: #fb923c; }
:root[data-theme="dark"] .cat-container { color: #2dd4bf; }
:root[data-theme="dark"] .cat-manager { color: #a78bfa; }
:root[data-theme="dark"] .cat-system { color: #94a3b8; }
:root[data-theme="dark"] .cat-unmanaged { color: #34d399; }

.proc-name {
  font-size: 16px;
  font-weight: 700;
  color: var(--tx);
}
.pid-badge {
  font-size: 11px;
  font-weight: 600;
  background: var(--line);
  color: var(--mu);
  padding: 2px 6px;
  border-radius: 4px;
}
.service-summary {
  margin: 6px 0 0 0;
  font-size: 12.5px;
  line-height: 1.4;
  color: var(--mu);
}
.tags-row {
  display: flex;
  gap: 8px;
  margin-top: 10px;
  flex-wrap: wrap;
}
.meta-tag {
  font-size: 11px;
  color: var(--mu);
  background: var(--panel);
  border: 1px solid var(--line);
  padding: 4px 10px;
  border-radius: 100px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  line-height: 1;
}
.meta-icon {
  flex-shrink: 0;
  color: var(--mu);
  opacity: 0.85;
}
.close-btn {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 6px;
  color: var(--mu);
  transition: all 0.2s;
}
.close-btn:hover {
  background: var(--line);
  color: var(--tx);
}

.feedback-bar {
  padding: 8px 16px;
  font-size: 12px;
  font-weight: 600;
  text-align: center;
  transition: all 0.2s;
}
.feedback-bar.ok { background: rgba(40, 193, 123, 0.15); color: #16a34a; }
.feedback-bar.err { background: rgba(229, 71, 27, 0.15); color: #dc2626; }

.section {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.docker-card, .systemd-card, .process-meta-box {
  background: var(--p2);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 14px 16px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
}
.sec-label {
  font-size: 11.5px;
  font-weight: 700;
  color: var(--tx);
  letter-spacing: 0.04em;
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px dashed var(--line);
}
.prop-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px 16px;
}
.prop-item.full-width {
  grid-column: span 2;
}
.prop-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.prop-item .k, .cmd-block .k {
  font-size: 11px;
  color: var(--mu);
  font-weight: 500;
}
.prop-item .v {
  font-size: 12.5px;
  color: var(--tx);
  word-break: break-all;
  line-height: 1.35;
}
.prop-item .v.highlight {
  font-weight: 600;
  color: var(--ac);
}
.prop-item.full-width .v {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 11.5px;
}
.selectable {
  user-select: all;
}
.cmd-block {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px dashed var(--line);
}
.cmd-block pre {
  margin: 6px 0 0 0;
  padding: 10px 12px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 6px;
  font-size: 11.5px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--tx);
}

.ports-header-row {
  margin-top: 4px;
}
.port-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.port-row-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  background: var(--p2);
  border: 1px solid var(--line);
  border-radius: 8px;
  transition: all 0.2s;
}
.port-row-card.highlighted {
  border-color: var(--ac);
  background: color-mix(in srgb, var(--ac) 8%, var(--p2));
}
.port-meta-left {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.port-line {
  display: flex;
  align-items: center;
  gap: 6px;
}
.proto-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  text-transform: uppercase;
}
.proto-badge.tcp { background: rgba(61, 139, 255, 0.15); color: #2563eb; }
.proto-badge.udp { background: rgba(240, 168, 28, 0.15); color: #d97706; }
.port-num {
  font-size: 14px;
  font-weight: 700;
  color: var(--tx);
}
.container-map {
  font-size: 11px;
  font-weight: 600;
  color: #0d9488;
}
.state-pill {
  font-size: 10px;
  color: var(--mu);
  padding: 1px 4px;
  border-radius: 3px;
  background: var(--line);
}
.addr-line {
  font-size: 11px;
  color: var(--mu);
  display: flex;
  align-items: center;
  gap: 5px;
}
.addr-scope-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}
.addr-scope-dot.external { background: #ff5230; }
.addr-scope-dot.local { background: #28c17b; }

.release-single-btn {
  font-size: 12px;
  font-weight: 600;
  padding: 5px 10px;
  border-radius: 6px;
  background: var(--panel);
  border: 1px solid var(--line);
  color: var(--tx);
  transition: all 0.2s;
}
.release-single-btn:hover:not(:disabled) {
  border-color: #ef4444;
  color: #ef4444;
}

.panel-foot {
  flex-shrink: 0;
  padding: 16px 16px calc(28px + env(safe-area-inset-bottom, 0px));
  margin-bottom: 8px;
  border-top: 1px solid var(--line);
  background: var(--p2);
  z-index: 2;
}
.kill-proc-btn {
  width: 100%;
  padding: 9px 14px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.2);
  transition: all 0.2s;
}
.kill-proc-btn:hover:not(:disabled) {
  background: #ef4444;
  color: #fff;
}
.kill-proc-btn.confirm {
  background: #ef4444;
  color: #fff;
  animation: pulseGlow 1.2s infinite;
}

.panel-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  overscroll-behavior: contain;
}
.panel-scroll::-webkit-scrollbar {
  width: 5px;
}
.panel-scroll::-webkit-scrollbar-thumb {
  background: var(--line2);
  border-radius: 4px;
}

.protected-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 600;
  color: #10b981;
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.25);
  padding: 3px 8px;
  border-radius: 6px;
  user-select: none;
}

.protected-foot-notice {
  width: 100%;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: rgba(16, 185, 129, 0.08);
  color: #10b981;
  border: 1px dashed rgba(16, 185, 129, 0.3);
  user-select: none;
}

</style>
