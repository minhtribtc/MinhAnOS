<script setup lang="ts">
import { X, Layers } from 'lucide-vue-next'
import { useHost } from '@minhanos/app-sdk'
import { useDesktop } from '../store'
import { nameKey } from '../registry'
import AppIcon from './AppIcon.vue'
const desktop = useDesktop()
const host = useHost()
const t = (key: string, params?: Record<string, string | number>) => { host.profile.value.locale; return host.i18n.t(key, params) }
const name = (id: string) => t(nameKey(id))
defineEmits<{ launch: [id: string]; dismiss: [] }>()
</script>
<template>
  <section class="tablet-switcher tablet-overlay" role="dialog" aria-modal="true" aria-labelledby="switcher-title">
    <header><div><span>{{ t('desktop.switcher.eyebrow') }}</span><h1 id="switcher-title">{{ t('desktop.switcher.title') }}</h1><p>{{ t('desktop.switcher.hint') }}</p></div><button class="tablet-round" :aria-label="t('desktop.switcher.close')" @click="$emit('dismiss')"><X :size="22" /></button></header>
    <div v-if="desktop.preferences.windows.length" class="switcher-grid"><article v-for="win in [...desktop.preferences.windows].reverse()" :key="win.id" class="switcher-card"><button class="switcher-activate" :aria-label="t('desktop.switcher.resume', { name: name(win.id) })" @click="$emit('launch', win.id)"><AppIcon :id="desktop.apps.find(a => a.id === win.id)?.icon || 'app'" :color="desktop.apps.find(a => a.id === win.id)?.color" /><h2>{{ name(win.id) }}</h2><p>{{ win.minimized ? t('desktop.switcher.minimized') : t('desktop.switcher.open') }}</p><span>{{ t('desktop.switcher.continue') }}</span></button><button class="switcher-close" :aria-label="t('desktop.switcher.closeApp', { name: name(win.id) })" @click="desktop.close(win.id)"><X :size="18" /></button></article></div>
    <div v-else class="switcher-empty"><Layers :size="42" /><h2>{{ t('desktop.switcher.empty') }}</h2><p>{{ t('desktop.switcher.emptyHint') }}</p></div>
    <p class="switcher-hint">{{ t('desktop.switcher.note') }}</p>
  </section>
</template>
