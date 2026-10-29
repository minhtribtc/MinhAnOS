<script setup lang="ts">
import { X, Layers } from 'lucide-vue-next'
import { useDesktop } from '../store'
import AppIcon from './AppIcon.vue'
const desktop = useDesktop()
defineEmits<{ launch: [id: string]; dismiss: [] }>()
</script>
<template>
  <section class="tablet-switcher tablet-overlay" role="dialog" aria-modal="true" aria-label="Ứng dụng đang mở">
    <header><div><span>KHÔNG GIAN LÀM VIỆC</span><h1>Ứng dụng đang mở</h1><p>Chọn ứng dụng để tiếp tục công việc.</p></div><button class="tablet-round" aria-label="Đóng trình chuyển ứng dụng" @click="$emit('dismiss')"><X :size="22" /></button></header>
    <div v-if="desktop.preferences.windows.length" class="switcher-grid"><article v-for="win in [...desktop.preferences.windows].reverse()" :key="win.id" class="switcher-card"><button class="switcher-activate" :aria-label="`Tiếp tục ${desktop.apps.find(a => a.id === win.id)?.title}`" @click="$emit('launch', win.id)"><AppIcon :id="desktop.apps.find(a => a.id === win.id)?.icon || win.id" :color="desktop.apps.find(a => a.id === win.id)?.color" /><h2>{{ desktop.apps.find(a => a.id === win.id)?.title }}</h2><p>{{ win.minimized ? 'Đã thu nhỏ' : 'Đang mở' }} · Giữ trạng thái trong phiên</p><span>Tiếp tục →</span></button><button class="switcher-close" :aria-label="`Đóng ${desktop.apps.find(a => a.id === win.id)?.title}`" @click="desktop.close(win.id)"><X :size="18" /></button></article></div>
    <div v-else class="switcher-empty"><Layers :size="42" /><h2>Không gian đang trống</h2><p>Mở một ứng dụng từ Home hoặc Dock.</p></div>
    <p class="switcher-hint">Đóng ứng dụng sẽ xóa biểu mẫu chưa lưu của ứng dụng đó. Tác vụ Terminal và upload chưa được triển khai.</p>
  </section>
</template>
