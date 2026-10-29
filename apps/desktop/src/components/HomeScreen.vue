<script setup lang="ts">
import { computed } from 'vue'
import { ArrowUpRight, Server, StickyNote, Search } from 'lucide-vue-next'
import { useDesktop } from '../store'
import { bytes } from '../api'
import AppIcon from './AppIcon.vue'
const desktop = useDesktop()
defineProps<{ time: string; date: string }>()
defineEmits<{ launch: [id: string]; search: [] }>()
const note = computed(() => desktop.preferences.notes.trim().slice(0, 160) || 'Một ý tưởng mới, một việc cần nhớ. Mở Ghi chú để bắt đầu.')
</script>

<template>
  <main class="tablet-home" aria-label="Màn hình chính">
    <div class="home-heading"><div><span>KHÔNG GIAN CỦA BẠN</span><h1>Xin chào, {{ desktop.user?.displayName }}.</h1></div><span class="home-edition">MinhAnOS <b>2</b></span></div>
    <div class="home-layout">
      <aside class="home-widgets" aria-label="Tiện ích">
        <article class="home-clock"><span>{{ date }}</span><strong>{{ time }}</strong><div><span class="widget-pill">Desktop Preview</span><span>0.2</span></div></article>
        <button class="home-system home-widget" @click="$emit('launch', 'overview')"><div class="widget-top"><span><Server :size="17" /> Nền tảng</span><ArrowUpRight :size="18" /></div><h2>{{ desktop.system ? 'Sẵn sàng cho bạn' : 'Đang mất kết nối' }}</h2><p>Trạng thái API MinhAnOS</p><div class="widget-metrics"><span><b>{{ desktop.system ? bytes(desktop.system.heapBytes) : '—' }}</b>Go heap</span><span><b>{{ desktop.system?.goroutines ?? '—' }}</b>Goroutines</span></div><div class="widget-foot"><i :class="{ offline: !desktop.system }"></i>{{ desktop.system?.hostname || 'Chưa nhận dữ liệu' }}</div></button>
        <button class="home-note home-widget" @click="$emit('launch', 'notes')"><div class="widget-top"><span><StickyNote :size="17" /> Ghi chú riêng</span><ArrowUpRight :size="18" /></div><p>{{ note }}</p><span class="note-footer">Một chút ý tưởng cho hôm nay.</span></button>
      </aside>
      <section class="home-apps" aria-label="Ứng dụng"><button v-for="app in desktop.apps" :key="app.id" class="home-app" :aria-label="`Mở ${app.title}`" @click="$emit('launch', app.id)"><AppIcon :id="app.icon" :color="app.color" /><strong>{{ app.title }}</strong><small v-if="app.status === 'planned'">Sắp có</small></button><div class="home-search-row"><button @click="$emit('search')"><Search :size="14" /> Tìm kiếm <kbd>Ctrl + Space</kbd></button></div></section>
    </div>
    <div class="home-page-indicator" aria-hidden="true"><i></i><span>MinhAnOS Desktop</span></div>
  </main>
</template>
