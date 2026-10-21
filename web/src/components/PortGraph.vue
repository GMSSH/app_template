<script setup lang="ts">
import { nextTick, watch, ref } from 'vue'
import { VueFlow, useVueFlow, Handle, Position, type Node, type Edge, type NodeMouseEvent } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'
import { useI18n } from 'vue-i18n'
import type { PortGroup } from '../types/port'

const props = defineProps<{
  nodes: Node[]
  edges: Edge[]
  selectedProcKey: string | null
  selectedPortKey: string | null
  layoutKey: string
  scanning?: boolean
}>()

const emit = defineEmits<{
  selectProc: [ports: PortGroup[], portKey?: string]
  paneClick: []
}>()

const { t } = useI18n()
const { fitView, setCenter, findNode } = useVueFlow()
const anim = ref(true)

watch(() => props.layoutKey, async () => {
  anim.value = true
  setTimeout(() => (anim.value = false), 900)
  await nextTick()
  setTimeout(() => fitView({ padding: 0.16, duration: 450, maxZoom: 1.1 }), 60)
}, { immediate: true })

/**
 * 平滑聚焦节点：将选中的节点居中并向左平移避开 380px 的右侧抽屉
 */
function focusNodeById(nodeId: string) {
  const targetZoom = 1.05
  const drawerWidth = 380 // 抽屉宽度
  // 为了不让抽屉遮挡，将视图中心在坐标系中向右偏移 (使得节点在屏幕中向左偏移至可视区居中)
  const shiftX = (drawerWidth / 2) / targetZoom

  const n = findNode(nodeId)
  if (n) {
    const w = n.dimensions?.width || 280
    const h = n.dimensions?.height || 100
    const cx = n.position.x + w / 2 + shiftX
    const cy = n.position.y + h / 2
    setCenter(cx, cy, { zoom: targetZoom, duration: 500 })
  }
}

const onProcCardClick = (nodeId: string, ports: PortGroup[]) => {
  emit('selectProc', ports)
  focusNodeById(nodeId)
}

const onPortChipClick = (nodeId: string, ports: PortGroup[], g: PortGroup) => {
  emit('selectProc', ports, g.key)
  focusNodeById(nodeId)
}

const onAddrCardClick = (nodeId: string) => {
  focusNodeById(nodeId)
}

const onPaneClick = () => {
  emit('paneClick')
  fitView({ padding: 0.16, duration: 450, maxZoom: 1.1 })
}

const isSelectedProc = (data: any) => {
  if (!props.selectedProcKey) return false
  const pk = `${data.name || 'unknown'}#${data.pid ?? 'x'}`
  return props.selectedProcKey === pk
}

// 端口雷达专有色彩映射
const getKindColor = (type?: string) => {
  switch (type) {
    case 'docker': return 'var(--c-docker)'
    case 'systemd': return 'var(--c-systemd)'
    case 'daemon': return 'var(--c-daemon)'
    default: return 'var(--c-native)'
  }
}

const getOwnerLabel = (type?: string) => {
  const key = ['docker', 'systemd', 'daemon'].includes(type || '') ? type! : 'native'
  return t('legend.' + key)
}
</script>

<template>
  <div class="stage">
    <!-- 雷达扫描线动效 -->
    <div class="scan" :class="{ go: scanning }" />

    <VueFlow
      :nodes="nodes"
      :edges="edges"
      :nodes-connectable="false"
      :elements-selectable="false"
      :zoom-on-scroll="true"
      :zoom-on-double-click="false"
      :min-zoom="0.3"
      :max-zoom="1.6"
      @pane-click="onPaneClick"
    >
      <!-- 雷达背景点阵 -->
      <Background variant="dots" :gap="22" :size="2" pattern-color="var(--dot)" />

      <!-- 地址中心 Hub 节点 (端口雷达原版样式) -->
      <template #node-addr="{ id, data }">
        <div class="hub" @click.stop="onAddrCardClick(id)" :class="{ pub: data.scope === 'external' }">
          <Handle type="source" :position="Position.Right" />
          <div class="t">
            <i />
            <span>{{ data.scope === 'external' ? t('hub.external') : t('hub.local') }}</span>
          </div>
          <div class="ip mono">{{ data.scope === 'external' ? '0.0.0.0' : '127.0.0.1' }}</div>
          <div class="s">
            {{ t('hub.ports', { n: data.count }) }} · {{ data.scope === 'external' ? t('hub.accessExternal') : t('hub.accessLocal') }}
          </div>
        </div>
      </template>

      <!-- 进程 Proc 节点 (端口雷达原版微渐变卡片 + 头像 + 药丸) -->
      <template #node-proc="{ id, data }">
        <div
          class="node"
          :class="{
            in: anim,
            sel: isSelectedProc(data),
            dim: selectedProcKey !== null && !isSelectedProc(data),
          }"
          :style="{
            '--c': getKindColor(data.ownerType),
          }"
          @click.stop="onProcCardClick(id, data.ports)"
        >
          <Handle type="target" :position="Position.Left" />
          <Handle v-if="data.solo" type="source" :position="Position.Right" />

          <div class="nh">
            <div class="av">{{ (data.name || '?')[0].toUpperCase() }}</div>
            <div>
              <b>{{ data.name || t('unknown') }}</b>
              <small><i class="kd" />{{ getOwnerLabel(data.ownerType) }}</small>
            </div>
            <span class="pid mono">PID {{ data.pid ?? '-' }}</span>
          </div>

          <div class="chips">
            <span
              v-for="g in data.ports"
              :key="g.key"
              class="chip mono"
              :class="{
                'chip-active': selectedPortKey === g.key,
              }"
              @click.stop="onPortChipClick(id, data.ports, g)"
            >
              <i class="k" :class="g.protocol" />
              <span>{{ g.port }}/{{ g.protocol }}</span>
            </span>
          </div>
        </div>
      </template>

      <!-- 搜索模式下的独立端口中继节点 -->
      <template #node-port="{ id, data }">
        <div
          class="node port-solo-node"
          :class="{
            sel: selectedPortKey === data.group.key,
          }"
          :style="{
            '--c': data.group.protocol === 'tcp' ? 'var(--c-tcp)' : 'var(--c-udp)',
          }"
          @click.stop="onPortChipClick(id, [data.group], data.group)"
        >
          <Handle type="target" :position="Position.Left" />
          <Handle type="source" :position="Position.Right" />
          <div class="nh">
            <div class="av mono">{{ data.group.port }}</div>
            <div>
              <b class="mono">{{ data.group.port }}</b>
              <small>{{ data.group.protocol.toUpperCase() }} · {{ data.group.state || 'LISTEN' }}</small>
            </div>
          </div>
        </div>
      </template>

      <!-- 控件组与小地图 -->
      <Controls position="top-left" />
      <MiniMap position="bottom-left" class="custom-minimap" :mask-color="'var(--minimap-mask)'" />
    </VueFlow>

    <!-- 画布操作提示 -->
    <div class="hint mono">{{ t('canvasHint') }}</div>
  </div>
</template>

<style scoped>
.stage {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 20px;
  box-shadow: var(--sh);
}

.stage:before {
  content: "";
  position: absolute;
  inset: 0;
  background-image: radial-gradient(var(--dot) 1.2px, transparent 1.2px);
  background-size: 22px 22px;
  pointer-events: none;
  z-index: 0;
}

/* 提示文本 */
.hint {
  position: absolute;
  left: 20px;
  bottom: 16px;
  z-index: 5;
  font-size: 12px;
  color: var(--mu);
  pointer-events: none;
}

/* 监听枢纽节点 (Hub) */
.hub {
  position: relative;
  width: 204px;
  padding: 16px 18px;
  border-radius: 14px;
  background: var(--panel);
  border: 1px solid var(--line2);
  box-shadow: var(--sh);
  transition: border-color 0.25s, box-shadow 0.25s;
}

.hub.pub {
  border-color: var(--ac);
}

.hub .t {
  font-size: 13px;
  color: var(--mu);
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.hub .t i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--mu);
}

.hub.pub .t i {
  background: var(--ac);
  animation: pulseGlow 2s infinite;
}

.hub .ip {
  font-size: 22px;
  font-weight: 600;
  margin: 8px 0 4px;
  letter-spacing: -0.02em;
  color: var(--tx);
}

.hub .s {
  font-size: 12px;
  color: var(--mu);
  line-height: 1.4;
}

/* 进程节点 (Proc Node) - 端口雷达原版样式 */
.node {
  --c: var(--mu);
  width: 278px;
  padding: 13px 14px 13px 18px;
  border-radius: 14px;
  position: relative;
  background: linear-gradient(100deg, color-mix(in srgb, var(--c) 13%, var(--panel)), var(--panel) 75%);
  border: 1px solid color-mix(in srgb, var(--c) 30%, var(--line));
  cursor: pointer;
  box-shadow: var(--sh);
  transition: border-color 0.2s, opacity 0.25s, transform 0.25s, box-shadow 0.25s;
}

.node.in {
  animation: nodeSlideIn 0.45s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}

.node:hover {
  border-color: var(--line2);
  transform: translateX(-2px);
}

.node.sel {
  border-color: var(--tx) !important;
  box-shadow: 0 0 0 1.5px var(--tx), var(--sh) !important;
}

.node.dim {
  opacity: 0.35;
}

.node:before {
  content: "";
  position: absolute;
  left: 0;
  top: 14px;
  bottom: 14px;
  width: 3.5px;
  border-radius: 0 3px 3px 0;
  background: var(--c);
}

.nh {
  display: flex;
  align-items: center;
  gap: 10px;
}

.av {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: grid;
  place-items: center;
  font-weight: 600;
  font-size: 13px;
  background: color-mix(in srgb, var(--c, var(--mu)) 18%, var(--p2));
  color: var(--c, var(--tx));
  border: 1px solid color-mix(in srgb, var(--c, var(--mu)) 38%, transparent);
  flex: none;
}

.nh b {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--tx);
}

.nh small {
  display: flex;
  align-items: center;
  color: color-mix(in srgb, var(--c) 70%, var(--mu));
  font-size: 12px;
  margin-top: 2px;
}

.kd {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c);
  margin-right: 6px;
  flex: none;
}

.pid {
  margin-left: auto;
  font-size: 11px;
  color: var(--mu);
}

/* 端口药丸 (Chips) */
.chips {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-top: 11px;
}

.chip {
  font-size: 12px;
  padding: 4px 9px 4px 8px;
  border-radius: 7px;
  background: var(--p2);
  border: 1px solid var(--line);
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--tx);
  transition: all 0.2s;
}

.chip:hover {
  border-color: var(--line2);
  background: var(--panel);
}

.chip-active {
  background: var(--tx) !important;
  color: var(--bg) !important;
  border-color: var(--tx) !important;
}

.k {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--tx);
  flex: none;
}

.k.udp {
  background: none;
  border: 1.5px solid var(--tx);
}

.chip-active .k {
  background: var(--bg) !important;
}
.chip-active .k.udp {
  background: none !important;
  border-color: var(--bg) !important;
}

.port-solo-node {
  width: 170px;
}
</style>
