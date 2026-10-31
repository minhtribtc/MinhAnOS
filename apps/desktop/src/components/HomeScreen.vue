<script setup lang="ts">
import { computed } from 'vue'
import { ArrowUpRight, Server, StickyNote, Search } from 'lucide-vue-next'
import { useHost } from '@minhanos/app-sdk'
import { useDesktop } from '../store'
import { bytes } from '../api'
import { nameKey } from '../registry'
import AppIcon from './AppIcon.vue'
const desktop = useDesktop()
const host = useHost()
const t = (key: string, params?: Record<string, string | number>) => { host.profile.value.locale; return host.i18n.t(key, params) }
defineProps<{ time: string; date: string }>()
defineEmits<{ launch: [id: string]; search: []; menu: [event: MouseEvent | KeyboardEvent, id: string] }>()
// Apps pinned in the dock are not repeated here (ADR 0013); every app stays reachable from Search.
const homeApps = computed(() => desktop.apps.filter(a => !desktop.preferences.pinned.includes(a.id)))
const hasNotes = computed(() => desktop.apps.some(a => a.id === 'com.minhanos.notes'))
const note = computed(() => desktop.preferences.notes.trim().slice(0, 160) || t('desktop.home.noteEmpty'))
</script>

<template>
  <main class="tablet-home" :aria-label="t('desktop.home.label')">
    <div class="home-heading"><div><span>{{ t('desktop.home.eyebrow') }}</span><h1>{{ t('desktop.home.greeting', { name: desktop.user?.displayName ?? '' }) }}</h1></div></div>
    <div class="home-layout">
      <aside class="home-widgets" :aria-label="t('desktop.home.widgets')">
        <article class="home-clock"><span>{{ date }}</span><strong>{{ time }}</strong><div v-if="desktop.system"><span class="widget-pill">{{ t('desktop.home.version', { version: desktop.system.version }) }}</span></div></article>
        <article class="home-system home-widget" role="status"><div class="widget-top"><span><Server :size="17" /> {{ t('desktop.home.platform') }}</span></div><h2>{{ desktop.system ? t('desktop.home.ready') : t('desktop.home.offline') }}</h2><p>{{ t('desktop.home.platformHint') }}</p><div class="widget-metrics"><span><b>{{ desktop.system ? bytes(desktop.system.heapBytes) : '—' }}</b>{{ t('desktop.home.heap') }}</span><span><b>{{ desktop.system?.goroutines ?? '—' }}</b>{{ t('desktop.home.goroutines') }}</span></div><div class="widget-foot"><i :class="{ offline: !desktop.system }"></i>{{ desktop.system?.hostname || t('desktop.home.noData') }}</div></article>
        <button v-if="hasNotes" class="home-note home-widget" @click="$emit('launch', 'com.minhanos.notes')"><div class="widget-top"><span><StickyNote :size="17" /> {{ t('desktop.home.note') }}</span><ArrowUpRight :size="18" /></div><p>{{ note }}</p><span class="note-footer">{{ t('desktop.home.noteFooter') }}</span></button>
      </aside>
      <section class="home-apps" :aria-label="t('desktop.home.apps')"><button v-for="app in homeApps" :key="app.id" class="home-app" :aria-label="t('desktop.home.open', { name: t(nameKey(app.id)) })" aria-haspopup="menu" @click="$emit('launch', app.id)" @contextmenu.prevent="$emit('menu', $event, app.id)" @keydown.shift.f10.prevent="$emit('menu', $event, app.id)"><AppIcon :id="app.icon" :color="app.color" /><strong>{{ t(nameKey(app.id)) }}</strong></button><p v-if="!homeApps.length" class="home-all-pinned">{{ t('desktop.home.allPinned') }}</p><div class="home-search-row"><button @click="$emit('search')"><Search :size="14" /> {{ t('desktop.home.search') }} <kbd>Ctrl + Space</kbd></button></div></section>
    </div>
    <div class="home-page-indicator" aria-hidden="true"><i></i><span>{{ t('desktop.app.name') }}</span></div>
  </main>
</template>
