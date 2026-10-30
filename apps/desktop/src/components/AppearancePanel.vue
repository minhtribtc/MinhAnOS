<script setup lang="ts">
import {useHost} from '@minhanos/app-sdk'
import { Check, Layers3, Leaf, Monitor } from 'lucide-vue-next'
import { api } from '../api'
import { useDesktop } from '../store'
// Desktop's own Settings panel (com.minhanos.desktop/appearance). Values persist via profile and preferences APIs.
defineProps<{ active?: boolean }>()
const host = useHost()
const desktop = useDesktop()
function setTheme(theme: 'light' | 'dark') { void host.updateProfile(api,{theme}).catch(e=>desktop.toast(host.errorMessage(e))); desktop.preferences.theme = theme; desktop.saveSoon() }
function setWallpaper(wallpaper: 'mountain' | 'midnight' | 'sage') { desktop.preferences.wallpaper = wallpaper; desktop.saveSoon() }
</script>

<template>
  <main class="standard-content"><div class="page-eyebrow">CÁ NHÂN HÓA</div><h1>Không gian theo cách bạn.</h1><p class="muted">Những thay đổi nhỏ, cảm giác thân thuộc hơn.</p><section class="workspace-mode-setting"><h3>Cách làm việc</h3><p class="muted small-text">Chọn trải nghiệm phù hợp. Chuyển ứng dụng luôn giữ biểu mẫu đang mở.</p><div class="workspace-mode-options"><button aria-label="Chế độ tập trung" :aria-pressed="desktop.preferences.mode === 'focus'" :class="{chosen:desktop.preferences.mode === 'focus'}" @click="desktop.setMode('focus')"><Monitor :size="24"/><strong>Tập trung</strong><small>Một ứng dụng toàn màn hình · mặc định</small><Check v-if="desktop.preferences.mode === 'focus'" :size="16"/></button><button aria-label="Chế độ cửa sổ" :aria-pressed="desktop.preferences.mode === 'windows'" :class="{chosen:desktop.preferences.mode === 'windows'}" @click="desktop.setMode('windows')"><Layers3 :size="24"/><strong>Cửa sổ</strong><small>Nhiều ứng dụng song song · nâng cao</small><Check v-if="desktop.preferences.mode === 'windows'" :size="16"/></button></div></section><h3 class="settings-section">Chế độ hiển thị</h3><div class="theme-options"><button v-for="theme in (['light','dark'] as const)" :key="theme" class="theme-option" :class="{ chosen: desktop.preferences.theme === theme }" @click="setTheme(theme)"><div :class="['theme-preview',theme]"><span></span><i></i><b></b></div><span>{{ theme === 'light' ? 'Sáng' : 'Tối' }}<Check v-if="desktop.preferences.theme === theme" :size="16" /></span></button></div><h3 class="settings-section">Hình nền desktop</h3><div class="wallpaper-options"><button v-for="wallpaper in (['mountain','midnight','sage'] as const)" :key="wallpaper" class="wallpaper-option" :class="{ chosen: desktop.preferences.wallpaper === wallpaper }" @click="setWallpaper(wallpaper)"><span :class="['wallpaper-preview',wallpaper]"><Check v-if="desktop.preferences.wallpaper === wallpaper" :size="22" /></span><span>{{ {mountain:'Bình minh',midnight:'Chạng vạng',sage:'Sắc xanh'}[wallpaper] }}</span></button></div><div class="info-panel"><Leaf :size="19" /><div><strong>Một desktop, dành riêng cho bạn</strong><p>Theme, hình nền, ghi chú và vị trí cửa sổ được lưu trong tài khoản, kể cả khi bạn đăng nhập lại.</p></div></div><p v-if="desktop.saveError" class="inline-error">{{ desktop.saveError }} <button class="text-button" @click="desktop.persist">Thử lưu lại</button></p></main>
</template>
