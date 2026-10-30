<script setup lang="ts">
import {useHost} from '@minhanos/app-sdk'
const host=useHost()
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { ArrowUpRight, ArrowRight, Check, ChevronRight, Circle, Database, HardDrive, Cpu, RefreshCw, ShieldCheck, Globe, Server, ExternalLink, Clock3, LockKeyhole, Plus, Search, CheckCircle2, AlertCircle, Monitor, Palette, Users, Layers3, FileText, Cloud, FolderOpen, NotebookPen, Activity } from 'lucide-vue-next'
import type { AppDefinition, AuditEntry } from '../api'
import { api, bytes } from '../api'
import { useDesktop } from '../store'
import AppIcon from './AppIcon.vue'
import FilesApp from '@minhanos/files-app'
import AccountsApp from '@minhanos/accounts-app'
import SettingsApp from '@minhanos/settings-app'
import StorageApp from '@minhanos/storage-app'
import { installedSettingsProviders } from '../settings'
const filesInstance=ref<InstanceType<typeof FilesApp>|null>(null)
const accountsInstance=ref<InstanceType<typeof AccountsApp>|null>(null)
const settingsInstance=ref<InstanceType<typeof SettingsApp>|null>(null)
const storageInstance=ref<InstanceType<typeof StorageApp>|null>(null)
defineExpose({canClose:()=>storageInstance.value?.canClose() ?? settingsInstance.value?.canClose() ?? accountsInstance.value?.canClose() ?? filesInstance.value?.canClose() ?? true})
const props = defineProps<{ app: AppDefinition; foreground?:boolean }>()
const desktop = useDesktop()
const activity = ref<AuditEntry[]>([])
const loading = ref(false)
const error = ref('')
const filter = ref('')
// Effective permissions only decide which Settings entries are listed; each panel's API still enforces access.
const permissions = ref<string[]|null>(null)
const settingsProviders = computed(() => installedSettingsProviders(desktop.apps.map(a => a.id)))
async function loadPermissions() { try { permissions.value = (await api<{ permissions: string[] }>('/v1/identity/me/access')).permissions } catch { permissions.value = [] } }
const memoryHistory = ref<number[]>([])
const filteredApps = computed(() => desktop.apps.filter(a => a.title.toLocaleLowerCase('vi').includes(filter.value.toLocaleLowerCase('vi'))))
const usedDisk = computed(() => desktop.system ? (1 - desktop.system.diskFreeBytes / Math.max(1, desktop.system.diskTotalBytes)) * 100 : 0)
const clockGreeting = computed(() => { const hour = new Date().getHours(); return hour < 11 ? 'Chào buổi sáng' : hour < 14 ? 'Chào buổi trưa' : hour < 18 ? 'Chào buổi chiều' : 'Chào buổi tối' })
const uptime = computed(() => { const seconds = desktop.system?.uptimeSeconds || 0; return seconds < 60 ? `${seconds} giây` : seconds < 3600 ? `${Math.floor(seconds / 60)} phút` : `${Math.floor(seconds / 3600)} giờ ${Math.floor(seconds % 3600 / 60)} phút` })
const memoryPath = computed(() => { const values = memoryHistory.value; const maximum = Math.max(...values, 1024 ** 2); return values.map((value, i) => `${i ? 'L' : 'M'}${(i / Math.max(1, values.length - 1)) * 260},${64 - value / maximum * 45}`).join(' ') })
const roadmap = [{ version: '0.2', title: 'Desktop V2', detail: 'Home, Dock và chuyển ứng dụng giữ trạng thái', current: true }, { version: '0.3', title: 'Kết nối dịch vụ', detail: 'NetBird, Authentik và 1Panel', current: false }, { version: '0.4', title: 'File Station', detail: 'Quản lý và chia sẻ dữ liệu', current: false }, { version: '0.7', title: 'Nền tảng NAS', detail: 'Lưu trữ, agent và dịch vụ file', current: false }]
async function refresh() {
  loading.value = true; error.value = ''
  try {
    if (props.app.id === 'activity') activity.value = await api<AuditEntry[]>('/audit')
    if (props.app.id === 'settings') await loadPermissions()
    if (['overview', 'about', 'connections'].includes(props.app.id)) await desktop.refreshSystem()
  } catch (e) { error.value = e instanceof Error ? e.message : 'Không tải được dữ liệu.' }
  finally { loading.value = false }
}
function togglePin(id: string) { const pinned = desktop.preferences.pinned; desktop.preferences.pinned = pinned.includes(id) ? pinned.filter(a => a !== id) : [...pinned, id]; desktop.saveSoon() }
function sample() { if (desktop.system) memoryHistory.value = [...memoryHistory.value, desktop.system.heapBytes].slice(-18) }
onMounted(() => { void refresh(); sample() })
const sampler = setInterval(sample, 5000)
onBeforeUnmount(() => clearInterval(sampler))
</script>

<template>
  <div v-if="app.id === 'overview'" class="overview-layout">
    <aside class="app-sidebar">
      <div class="sidebar-caption">KHÔNG GIAN LÀM VIỆC</div>
      <button class="sidebar-item selected"><Monitor :size="17" /> Tổng quan</button>
      <button class="sidebar-item" @click="desktop.open('connections')"><Globe :size="17" /> Kết nối dịch vụ <span class="tiny-count">3</span></button>
      <button class="sidebar-item" @click="desktop.open('activity')"><Clock3 :size="17" /> Nhật ký hoạt động</button>
      <div class="sidebar-caption second">CÁ NHÂN</div>
      <button class="sidebar-item" @click="desktop.open('notes')"><NotebookPen :size="17" /> Ghi chú</button>
      <button class="sidebar-item" @click="desktop.openSettings('com.minhanos.desktop','appearance')"><Palette :size="17" /> Cá nhân hóa</button>
      <div class="sidebar-bottom"><span class="connection-dot"></span><div><strong>Desktop đã sẵn sàng</strong><small>MinhAnOS 0.2.0 · Bản thử nghiệm</small></div></div>
    </aside>
    <main class="overview-main">
      <div class="page-eyebrow"><span>DESKTOP / TỔNG QUAN</span><span class="soft-badge"><span class="connection-dot"></span> Phiên cục bộ</span></div>
      <div class="welcome-heading"><div><p>{{ clockGreeting }}, {{ desktop.user?.displayName }} <span class="greeting-sun">✦</span></p><h1>Mọi thứ, trong tầm tay.</h1><p class="muted">Một không gian riêng. Nhiều điều để khám phá.</p></div><button class="icon-button subtle" aria-label="Làm mới tổng quan" :disabled="loading" @click="refresh"><RefreshCw :size="17" :class="{ spinning: loading }" /></button></div>
      <div class="welcome-banner"><div><span class="eyebrow">CHÀO MỪNG ĐẾN MINHANOS</span><h2>Nhẹ nhàng hơn.<br>Chủ động hơn.</h2><p>Máy chủ, dữ liệu và ứng dụng của bạn,<br>trong một desktop thống nhất.</p><button class="banner-link" @click="desktop.open('apps')">Khám phá không gian mới <ArrowUpRight :size="17" /></button></div><div class="banner-art" aria-hidden="true"><div class="orbit orbit-one"></div><div class="orbit orbit-two"></div><div class="art-stack stack-back"></div><div class="art-stack stack-mid"></div><div class="art-stack stack-front"><Layers3 :size="52" :stroke-width="1.1" /></div><span class="art-star">✧</span><span class="art-dot"></span></div></div>
      <div class="section-label"><h3>Truy cập nhanh</h3><button class="text-button" @click="desktop.open('apps')">Tất cả ứng dụng <ArrowRight :size="14" /></button></div>
      <div class="quick-grid"><button v-for="id in ['files','connections','settings','notes']" :key="id" class="quick-card" @click="desktop.open(id)"><AppIcon :id="id" :color="desktop.apps.find(a => a.id === id)?.color" small /><span>{{ desktop.apps.find(a => a.id === id)?.title }}</span><ChevronRight :size="14" /></button></div>
      <div class="section-label"><h3>Nhịp hoạt động</h3><span class="muted small-text">Dữ liệu thật · Tiến trình & container</span></div>
      <div v-if="desktop.system" class="metric-grid">
        <div class="metric-card"><div class="metric-label"><Cpu :size="15" /> Bộ nhớ ứng dụng <span class="connection-dot"></span></div><strong>{{ bytes(desktop.system.heapBytes) }}<span>Go heap</span></strong><svg class="sparkline" viewBox="0 0 260 70" preserveAspectRatio="none" aria-label="Mẫu bộ nhớ tiến trình từ khi mở cửa sổ"><path :d="memoryPath" fill="none" stroke="currentColor" stroke-width="2" /></svg></div>
        <div class="metric-card"><div class="metric-label"><HardDrive :size="15" /> Filesystem /data</div><strong>{{ bytes(desktop.system.diskFreeBytes) }}<span>còn trống</span></strong><div class="storage-track"><i :style="{ width: `${usedDisk}%` }"></i></div><div class="metric-foot">Tổng {{ bytes(desktop.system.diskTotalBytes) }} · {{ usedDisk.toFixed(1) }}% đã dùng</div></div>
        <div class="metric-card"><div class="metric-label"><Database :size="15" /> Nền tảng</div><div class="db-status"><span class="connection-dot" :class="{ offline: desktop.system.database !== 'connected' }"></span> {{ desktop.system.database === 'connected' ? 'Đang hoạt động' : 'Mất kết nối DB' }}</div><div class="metric-foot">PostgreSQL · Go API</div><div class="uptime"><Clock3 :size="13" /> Uptime ứng dụng: {{ uptime }}</div></div>
      </div>
      <div v-else class="inline-error"><AlertCircle :size="17" /> Chưa lấy được trạng thái máy chủ. <button class="text-button" @click="refresh">Thử lại</button></div>
      <div class="overview-footer"><ShieldCheck :size="14" /><span>Bố cục và ghi chú được lưu riêng theo tài khoản.</span><button class="text-button" @click="desktop.open('about')">Về bản thử nghiệm <ArrowUpRight :size="13" /></button></div>
    </main>
  </div>

  <SettingsApp v-else-if="app.id === 'settings'" ref="settingsInstance" :providers="settingsProviders" :permissions="permissions" :active="props.foreground!==false" :target="desktop.settingsTarget" />

  <main v-else-if="app.id === 'apps'" class="standard-content">
    <div class="page-eyebrow">APP CENTER / DESKTOP 0.2</div><h1>Mở ra nhiều khả năng.</h1><p class="muted">Các ứng dụng tích hợp trong không gian MinhAnOS.</p><div class="app-search"><Search :size="17" /><input v-model="filter" placeholder="Tìm ứng dụng…" aria-label="Tìm ứng dụng trong trung tâm"></div><div class="catalog-grid"><article v-for="item in filteredApps" :key="item.id" class="catalog-card"><AppIcon :id="item.icon" :color="item.color" /><span class="catalog-status" :class="{ planned: item.status === 'planned' }">{{ item.status === 'ready' ? 'Sẵn sàng' : 'Sắp có' }}</span><h3>{{ item.title }}</h3><p>{{ item.subtitle }}</p><div><button class="text-button" @click="desktop.open(item.id)">Mở ứng dụng <ArrowUpRight :size="14" /></button><button class="pin-button" :aria-label="`${desktop.preferences.pinned.includes(item.id) ? 'Bỏ ghim' : 'Ghim'} ${item.title}`" @click="togglePin(item.id)"><Check v-if="desktop.preferences.pinned.includes(item.id)" :size="15" /><Plus v-else :size="15" /></button></div></article></div><div v-if="!filteredApps.length" class="empty-state"><Search :size="32" /><h3>Không tìm thấy ứng dụng</h3><p>Thử tìm bằng một từ khác.</p></div><p class="muted small-text">Đây là danh mục ứng dụng nội bộ. Cài đặt package bên ngoài chưa được hỗ trợ.</p>
  </main>

  <main v-else-if="app.id === 'connections'" class="standard-content">
    <div class="page-eyebrow">INTEGRATIONS</div><h1>Một nơi. Mọi kết nối.</h1><p class="muted">Nền tảng cho mạng riêng, danh tính và máy chủ của bạn.</p><div class="integration-list"><article v-for="(item,index) in (desktop.system?.integrations || [{id:'netbird',name:'NetBird',description:'Quản lý thiết bị và mạng riêng'},{id:'authentik',name:'Authentik',description:'Đăng nhập tập trung và danh tính'},{id:'onepanel',name:'1Panel',description:'Máy chủ, container và ứng dụng'}])" :key="item.id" class="integration-card"><span :class="['integration-logo', ['blue','amber','teal'][index]]"><component :is="[Globe,ShieldCheck,Server][index]" :size="27" /></span><div><h2>{{ item.name }}</h2><p>{{ item.description }}</p></div><span class="soft-badge neutral"><Circle :size="8" /> Chưa kết nối</span></article></div><div class="info-panel"><LockKeyhole :size="21" /><div><strong>Kết nối thật, trong bước tiếp theo</strong><p>Bản này chưa gọi API của ba dịch vụ. Cần xác minh endpoint, phiên bản và quyền trước khi cấu hình. Không có dữ liệu dịch vụ giả hoặc khóa API lưu ở trình duyệt.</p></div></div><button class="secondary-button" @click="desktop.open('about')">Xem lộ trình phát triển <ArrowRight :size="15" /></button>
  </main>

  <AccountsApp v-else-if="app.id === 'com.minhanos.account-center'" ref="accountsInstance" embedded :active="props.foreground!==false" />
  <StorageApp v-else-if="app.id === 'com.minhanos.storage'" ref="storageInstance" :active="props.foreground!==false" />
  <FilesApp v-else-if="app.id === 'files'" ref="filesInstance" :active="props.foreground!==false" embedded api-base="/api/drive" />

  <main v-else-if="app.id === 'activity'" class="standard-content"><div class="page-eyebrow">ACTIVITY LOG</div><div class="section-label"><h1>Những gì đã diễn ra.</h1><button class="secondary-button" :disabled="loading" @click="refresh"><RefreshCw :size="15" :class="{spinning:loading}" /> Làm mới</button></div><p class="muted">{{ desktop.user?.role === 'admin' ? '60 hoạt động xác thực và quản trị gần nhất.' : 'Hoạt động xác thực của tài khoản bạn.' }}</p><p v-if="error" class="inline-error">{{ error }}</p><div class="audit-list"><div v-for="entry in activity" :key="entry.id" class="audit-row"><span class="audit-icon"><component :is="entry.action === 'user.create' ? Users : ShieldCheck" :size="18" /></span><div><strong>{{ entry.detail }}</strong><p>{{ entry.actor }} <span>· {{ entry.action }}</span></p></div><time>{{ new Date(entry.createdAt).toLocaleString('vi-VN',{timeZone:'Asia/Bangkok',hour:'2-digit',minute:'2-digit',day:'2-digit',month:'2-digit'}) }}</time></div></div><div v-if="!activity.length && !loading && !error" class="empty-state"><Activity :size="36" /><h3>Chưa có hoạt động</h3></div></main>

  <main v-else-if="app.id === 'notes'" class="notes-content"><div class="notes-toolbar"><span><NotebookPen :size="16" /> Ghi chú riêng của {{ desktop.user?.displayName }}</span><span :class="{ 'error-text': desktop.saveError }">{{ desktop.saveError ? 'Chưa lưu được' : desktop.saving ? 'Đang lưu…' : 'Tự động lưu' }}</span></div><textarea v-model="desktop.preferences.notes" maxlength="32000" aria-label="Nội dung ghi chú" placeholder="Một ý tưởng mới bắt đầu từ đây…&#10;&#10;Ghi chú này chỉ hiển thị trong tài khoản của bạn." @input="desktop.saveSoon()"></textarea><div class="notes-footer">{{ desktop.preferences.notes.length.toLocaleString('vi-VN') }} / 32.000 ký tự <button v-if="desktop.saveError" class="text-button" @click="desktop.persist">Thử lưu lại</button></div></main>

  <main v-else class="standard-content about-content"><div class="about-brand"><img src="/mark.svg" alt="" width="64" height="64"><span>MinhAn<span>OS</span></span></div><span class="soft-badge">DESKTOP PREVIEW · 0.2.0</span><h1>Không gian riêng.<br>Khả năng rộng mở.</h1><p class="muted">Được xây mới bằng Vue, TypeScript và Go.<br>Từng bước hướng đến nền tảng quản trị máy chủ và NAS.</p><div class="roadmap"><div v-for="step in roadmap" :key="step.version" :class="{current:step.current}"><span>{{ step.version }}</span><div><strong>{{ step.title }}</strong><p>{{ step.detail }}</p></div><CheckCircle2 v-if="step.current" :size="19" /></div></div><div v-if="desktop.system" class="detail-list compact"><div><span>Backend</span><strong>{{ desktop.system.goVersion }} · {{ desktop.system.architecture }}</strong></div><div><span>Database</span><strong>PostgreSQL</strong></div><div><span>Node</span><strong>{{ desktop.system.node }}</strong></div><div><span>Phạm vi số liệu</span><strong>Tiến trình & filesystem container</strong></div></div><p class="small-text muted">Bản thử nghiệm chưa có SSO, agent Linux, File Station hay quản lý package. Đây là nền tảng mới, độc lập với bản kodbox đang chạy.</p></main>
</template>
