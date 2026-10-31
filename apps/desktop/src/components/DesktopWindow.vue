<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Minus, Square, X, Copy, Grip } from 'lucide-vue-next'
import { useHost } from '@minhanos/app-sdk'
import { useDesktop } from '../store'
import { nameKey } from '../registry'
import type { DesktopWindow, AppDefinition } from '../api'
import AppIcon from './AppIcon.vue'
import AppContent from './AppContent.vue'

const props = defineProps<{ window: DesktopWindow; app: AppDefinition; foreground?: boolean }>()
const desktop = useDesktop()
const host = useHost()
const title = computed(() => (host.profile.value.locale, host.i18n.t(nameKey(props.app.id))))
const label = (key: string) => host.i18n.t(key, { name: title.value })
const content=ref<{canClose:()=>boolean}|null>(null)
let unregisterGuard=()=>{}
onMounted(()=>{unregisterGuard=desktop.registerCloseGuard(props.window.id,()=>content.value?.canClose()??true)})
function closeWindow(){desktop.close(props.window.id)}
const viewport = ref({ width: innerWidth, height: innerHeight })
const onViewport = () => { viewport.value = { width: innerWidth, height: innerHeight } }
globalThis.addEventListener('resize', onViewport)
const geometry = computed(() => {
  const { width: vw, height: vh } = viewport.value
  if (desktop.preferences.mode === 'focus') return { width: vw - 24, height: vh - 140, left: 12, top: 38 }
  const maximized = props.window.maximized || vw < 700
  const width = maximized ? vw - 24 : Math.min(props.window.width, vw - 32)
  const height = maximized ? vh - 88 : Math.min(props.window.height, vh - 112)
  return {
    width, height,
    left: maximized ? 12 : Math.max(16, Math.min(props.window.x, vw - width - 16)),
    top: maximized ? 12 : Math.max(20, Math.min(props.window.y, vh - height - 78)),
  }
})
const style = computed(() => ({ ...Object.fromEntries(Object.entries(geometry.value).map(([key, value]) => [key, `${value}px`])), zIndex: 20 + Math.max(0, desktop.order.indexOf(props.window.id)) }))
let cleanup = () => {}
function begin(event: PointerEvent, resize = false) {
  if ((event.target as HTMLElement).closest('.window-controls') || event.button !== 0 || props.window.maximized || innerWidth < 700 || desktop.preferences.mode === 'focus') return
  event.preventDefault(); desktop.focus(props.window.id)
  const start = { x: event.clientX, y: event.clientY, ...geometry.value }
  document.body.classList.add('dragging')
  const move = (e: PointerEvent) => {
    if (resize) {
      props.window.width = Math.max(Math.min(480, innerWidth - 32), Math.min(innerWidth - start.left - 16, start.width + e.clientX - start.x))
      props.window.height = Math.max(Math.min(340, innerHeight - 112), Math.min(innerHeight - start.top - 78, start.height + e.clientY - start.y))
    } else {
      props.window.x = Math.max(16, Math.min(innerWidth - start.width - 16, start.left + e.clientX - start.x))
      props.window.y = Math.max(20, Math.min(innerHeight - start.height - 78, start.top + e.clientY - start.y))
    }
  }
  const end = () => { cleanup(); desktop.saveSoon() }
  cleanup = () => { document.body.classList.remove('dragging'); globalThis.removeEventListener('pointermove', move); globalThis.removeEventListener('pointerup', end); globalThis.removeEventListener('pointercancel', end) }
  globalThis.addEventListener('pointermove', move)
  globalThis.addEventListener('pointerup', end, { once: true })
  globalThis.addEventListener('pointercancel', end, { once: true })
}
function keyboardResize(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return
  event.preventDefault()
  props.window.width = Math.max(480, Math.min(innerWidth - 32, props.window.width + (event.key === 'ArrowRight' ? 20 : event.key === 'ArrowLeft' ? -20 : 0)))
  props.window.height = Math.max(340, Math.min(innerHeight - 112, props.window.height + (event.key === 'ArrowDown' ? 20 : event.key === 'ArrowUp' ? -20 : 0)))
  desktop.saveSoon()
}
onBeforeUnmount(() => { unregisterGuard(); cleanup(); globalThis.removeEventListener('resize', onViewport) })
</script>

<template>
  <section class="desktop-window" :class="{ focused: desktop.active === window.id, maximized: window.maximized, 'focus-mode-window': desktop.preferences.mode === 'focus' }" :style="style" role="region" :aria-label="title" :data-window="window.id" @pointerdown="desktop.focus(window.id)">
    <header class="window-titlebar" @pointerdown="begin($event)" @dblclick="desktop.preferences.mode === 'windows' && desktop.maximize(window.id)">
      <div class="window-title"><AppIcon :id="app.icon" :color="app.color" small /><span>{{ title }}</span></div>
      <div class="window-controls" @dblclick.stop>
        <button :aria-label="label('desktop.window.minimize')" @click="desktop.minimize(window.id)"><Minus :size="16" /></button>
        <button v-if="desktop.preferences.mode === 'windows'" :aria-label="label(window.maximized ? 'desktop.window.restore' : 'desktop.window.maximize')" @click="desktop.maximize(window.id)"><component :is="window.maximized ? Copy : Square" :size="13" /></button>
        <button class="close-window" :aria-label="label('desktop.window.close')" @click="closeWindow"><X :size="17" /></button>
      </div>
    </header>
    <div class="window-content"><AppContent ref="content" :foreground="foreground" :app="app" /></div>
    <button v-if="!window.maximized && desktop.preferences.mode === 'windows'" class="resize-grip" :aria-label="label('desktop.window.resize')" @pointerdown.stop="begin($event, true)" @keydown="keyboardResize"><Grip :size="13" /></button>
  </section>
</template>
