<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Grid2X2, Search, Bell, Wifi, ChevronDown, ChevronLeft, ChevronRight, LogOut, ArrowRight, ShieldCheck, Eye, EyeOff, X, Settings2, Check, CircleHelp, Monitor, RefreshCw, Command, LockKeyhole, LoaderCircle, Power, PanelsTopLeft } from 'lucide-vue-next'
import { api, setCSRF, type User } from './api'
import { useDesktop } from './store'
import AppIcon from './components/AppIcon.vue'
import DesktopWindow from './components/DesktopWindow.vue'

const desktop = useDesktop()
const initializing = ref(true)
const loginBusy = ref(false)
const loginError = ref('')
const username = ref('')
const password = ref('')
const showPassword = ref(false)
const launcher = ref(false)
const launcherSearch = ref('')
const launcherAll = ref(false)
const calendarOffset = ref(0)
const userMenu = ref(false)
const notifications = ref(false)
const contextMenu = ref<{x:number;y:number} | null>(null)
const searchInput = ref<HTMLInputElement | null>(null)
const now = ref(new Date())
const time = computed(() => now.value.toLocaleTimeString('vi-VN', {timeZone:'Asia/Bangkok',hour:'2-digit',minute:'2-digit'}))
const date = computed(() => now.value.toLocaleDateString('vi-VN', {timeZone:'Asia/Bangkok',year:'numeric',day:'2-digit',month:'2-digit'}))
const longDate = computed(() => now.value.toLocaleDateString('vi-VN', {timeZone:'Asia/Bangkok',weekday:'long',day:'numeric',month:'long'}))
const todayParts = computed(() => Object.fromEntries(new Intl.DateTimeFormat('en-US',{timeZone:'Asia/Bangkok',year:'numeric',month:'numeric',day:'numeric'}).formatToParts(now.value).filter(p => p.type !== 'literal').map(p => [p.type,Number(p.value)])))
const calendarMonth = computed(() => new Date(todayParts.value.year,todayParts.value.month - 1 + calendarOffset.value,1))
const calendarLabel = computed(() => calendarMonth.value.toLocaleDateString('vi-VN',{month:'long',year:'numeric'}))
const calendarDays = computed(() => {
  const first = calendarMonth.value
  const start = (first.getDay() + 6) % 7
  return Array.from({length:42},(_,i) => {const value = new Date(first.getFullYear(),first.getMonth(),i-start+1);return {key:value.toISOString(),day:value.getDate(),outside:value.getMonth()!==first.getMonth(),today:value.getFullYear()===todayParts.value.year&&value.getMonth()===todayParts.value.month-1&&value.getDate()===todayParts.value.day}})
})
const recommended = computed(() => [
  {id:'notes',description:desktop.preferences.notes ? 'Tiếp tục ghi chú của bạn' : 'Một nơi cho ý tưởng mới'},
  {id:'overview',description:'Trạng thái nền tảng của bạn'},
  {id:'settings',description:'Cá nhân hóa không gian làm việc'},
  {id:'connections',description:'Khám phá các kết nối sắp tới'},
].map(item => ({...item,app:desktop.apps.find(app => app.id === item.id)!})))
const launcherApps = computed(() => desktop.apps.filter(app => `${app.title} ${app.subtitle}`.toLocaleLowerCase('vi').includes(launcherSearch.value.toLocaleLowerCase('vi'))))
const dockApps = computed(() => [...new Set([...desktop.preferences.pinned, ...desktop.preferences.windows.map(w => w.id)])].map(id => desktop.apps.find(app => app.id === id)).filter((app): app is NonNullable<typeof app> => !!app))
const anyMenu = computed(() => launcher.value || userMenu.value || notifications.value || !!contextMenu.value)
function dismiss() { launcher.value = false; userMenu.value = false; notifications.value = false; contextMenu.value = null }
async function toggleLauncher() { const open = !launcher.value; dismiss(); launcher.value = open; launcherSearch.value = ''; launcherAll.value = false; if (open) { await nextTick(); searchInput.value?.focus() } }
function toggleNotifications() { const next = !notifications.value; dismiss(); notifications.value = next; calendarOffset.value = 0 }
function toggleUserMenu() { const next = !userMenu.value; dismiss(); userMenu.value = next }
let hiddenWindows: string[] = []
function toggleDesktop() {
  dismiss()
  if (desktop.visibleWindows.length) {hiddenWindows = desktop.visibleWindows.map(w => w.id);desktop.preferences.windows.forEach(w => w.minimized = true);desktop.active = '';desktop.saveSoon()}
  else {hiddenWindows.forEach(id => desktop.open(id));hiddenWindows = []}
}
function launch(id: string) { desktop.open(id); dismiss() }
function dockClick(id: string) { dismiss(); const win = desktop.preferences.windows.find(w => w.id === id); if (win && !win.minimized && desktop.active === id) desktop.minimize(id); else desktop.open(id) }
async function login() {
  loginBusy.value = true; loginError.value = ''
  try {
    const result = await api<{user: User; csrfToken: string}>('/login', {method:'POST',body:JSON.stringify({username:username.value,password:password.value})})
    setCSRF(result.csrfToken); password.value = ''; await desktop.loadSession()
    if (desktop.preferences.mode === 'focus' && !new URLSearchParams(location.search).has('app')) launcher.value = true
  } catch(e) { loginError.value = e instanceof Error ? e.message : 'Không thể đăng nhập.' }
  finally { loginBusy.value = false }
}
async function logout() { dismiss(); try { await desktop.logout(); username.value = ''; password.value = '' } catch(e) { desktop.toast(e instanceof Error ? e.message : 'Không đăng xuất được.') } }
function keyboard(e: KeyboardEvent) {
  if (!desktop.ready) return
  if ((e.ctrlKey || e.metaKey) && e.code === 'Space') { e.preventDefault(); void toggleLauncher() }
  if (e.key === 'Escape') dismiss()
  if (e.altKey && e.key === 'Tab') { e.preventDefault(); const windows = desktop.preferences.windows; if (windows.length) desktop.open(windows[(windows.findIndex(w => w.id === desktop.active) + 1) % windows.length].id) }
}
function desktopContext(event: MouseEvent) { if ((event.target as HTMLElement).closest('.desktop-window,.desktop-shortcut,.dock,.menubar')) return; event.preventDefault(); dismiss(); contextMenu.value = {x:Math.min(event.clientX,innerWidth-240),y:Math.min(event.clientY,innerHeight-200)} }
onMounted(async () => {
  globalThis.addEventListener('keydown',keyboard)
  try { await desktop.loadSession(); if (desktop.preferences.mode === 'focus' && !new URLSearchParams(location.search).has('app')) launcher.value = true } catch(e) { if (e instanceof Error && !e.message.includes('Phiên đăng nhập')) loginError.value = e.message }
  finally { initializing.value = false }
})
const clockTimer = setInterval(() => { now.value = new Date() },10000)
const systemTimer = setInterval(() => { if (desktop.ready) void desktop.refreshSystem() },10000)
onBeforeUnmount(() => { clearInterval(clockTimer); clearInterval(systemTimer); globalThis.removeEventListener('keydown',keyboard) })
</script>

<template>
  <div :class="['platform', desktop.ready ? desktop.preferences.wallpaper : 'mountain', { 'dark-mode': desktop.ready && desktop.preferences.theme === 'dark', 'focus-workspace': desktop.ready && desktop.preferences.mode === 'focus' }]" @contextmenu="desktopContext">
    <div class="wallpaper"></div><div class="wallpaper-shade"></div>
    <div v-if="initializing" class="initial-loader"><img src="/mark.svg" alt="MinhAnOS" width="58" height="58"><LoaderCircle :size="24" class="spinning" /><span>Đang mở không gian của bạn…</span></div>
    <template v-else-if="!desktop.ready">
      <div class="login-brand"><img src="/mark.svg" alt="" width="32" height="32"><strong>MinhAn<span>OS</span></strong><span class="brand-divider"></span><span>YOUR SPACE. YOUR POSSIBILITIES.</span></div>
      <main class="login-screen"><section class="login-story"><div class="login-eyebrow"><span></span> KHÔNG GIAN CỦA BẠN</div><h1>Mọi thứ,<br>trong tầm tay.</h1><p>Một desktop cho máy chủ, ứng dụng<br>và những điều bạn đang xây dựng.</p><div class="login-story-footer"><span class="story-line"></span><span>TỪNG BƯỚC, MỞ RA NHIỀU KHẢ NĂNG.</span></div></section><section class="login-card"><div class="login-symbol"><Monitor :size="25" :stroke-width="1.5" /></div><span class="page-eyebrow">MINHANOS DESKTOP</span><h2>Chào mừng trở lại.</h2><p class="muted">Đăng nhập để vào không gian của bạn.</p><form @submit.prevent="login"><label for="username">Tên đăng nhập</label><input id="username" v-model="username" name="username" autocomplete="username" placeholder="Nhập tên đăng nhập" required maxlength="64" autofocus><label for="password">Mật khẩu</label><div class="password-input"><input id="password" v-model="password" name="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" placeholder="Nhập mật khẩu" required maxlength="256"><button type="button" :aria-label="showPassword ? 'Ẩn mật khẩu' : 'Hiện mật khẩu'" @click="showPassword = !showPassword"><component :is="showPassword ? EyeOff : Eye" :size="17" /></button></div><p v-if="loginError" class="login-error" role="alert">{{ loginError }}</p><button class="login-submit" :disabled="loginBusy"><LoaderCircle v-if="loginBusy" :size="18" class="spinning" />{{ loginBusy ? 'Đang đăng nhập…' : 'Vào desktop' }}<ArrowRight v-if="!loginBusy" :size="18" /></button></form><div class="login-security"><ShieldCheck :size="15" /><span>Tài khoản cục bộ · Phiên riêng tư</span></div><div class="login-version">DESKTOP PREVIEW <span>0.1.0</span></div></section></main>
      <footer class="login-footer"><span>MinhAnOS © {{ now.getFullYear() }}</span><span><span class="connection-dot"></span> 10.1.1.20 · Môi trường phát triển</span></footer>
    </template>
    <template v-else>
      <div class="desktop-surface" @pointerdown.self="dismiss"><div class="desktop-shortcuts"><button v-for="id in ['files','apps','connections','settings','notes']" :key="id" class="desktop-shortcut" :aria-label="`Mở ${desktop.apps.find(a => a.id === id)?.title}`" @dblclick="launch(id)" @keydown.enter="launch(id)" @click="dismiss"><AppIcon :id="id" :color="desktop.apps.find(a => a.id === id)?.color" /><span>{{ desktop.apps.find(a => a.id === id)?.title }}</span></button></div><div class="desktop-watermark"><span>MAKE ROOM FOR WHAT'S NEXT.</span><strong>MinhAnOS</strong><small>Desktop Preview · 0.1.0</small></div></div>
      <DesktopWindow v-for="window in desktop.preferences.windows" v-show="!window.minimized && (desktop.preferences.mode === 'windows' || desktop.active === window.id)" :key="window.id" :window="window" :app="desktop.apps.find(app => app.id === window.id)!" />
      <nav class="dock" aria-label="Thanh ứng dụng">
        <button class="taskbar-brand" aria-label="Mở Tổng quan hệ thống" @click="launch('overview')"><img src="/mark.svg" alt="" width="30" height="30"><span><strong>MinhAnOS</strong><small><i class="connection-dot"></i>{{ desktop.system ? 'Desktop đã sẵn sàng' : 'Đang kết nối lại' }}</small></span></button>
        <div class="taskbar-center">
          <button class="dock-launcher" :class="{selected:launcher}" aria-label="Tất cả ứng dụng" title="Start · Ctrl+Space" @click="toggleLauncher"><span class="start-mark" aria-hidden="true"><i></i><i></i><i></i><i></i></span></button>
          <button class="taskbar-search" aria-label="Tìm ứng dụng" title="Tìm ứng dụng · Ctrl+Space" @click="toggleLauncher"><Search :size="18" /><span>Tìm kiếm</span></button>
          <button class="taskbar-taskview" aria-label="Hiện desktop" title="Hiện desktop" @click="toggleDesktop"><PanelsTopLeft :size="23" :stroke-width="1.5" /></button>
          <button v-for="app in dockApps" :key="app.id" class="dock-app" :class="{ active: desktop.active === app.id, running: desktop.preferences.windows.some(w => w.id === app.id) }" :aria-label="`Mở ${app.title}`" @click="dockClick(app.id)"><AppIcon :id="app.icon" :color="app.color" /><span class="dock-tooltip">{{ app.title }}</span><i></i></button>
        </div>
        <div class="taskbar-tray">
          <button v-if="desktop.saveError" class="save-error" @click="desktop.persist">Chưa lưu</button>
          <span v-else-if="desktop.saving" class="tray-saving" title="Đang lưu bố cục">Đang lưu…</span>
          <button class="tray-network" :title="desktop.system ? 'API MinhAnOS đã kết nối' : 'API MinhAnOS chưa kết nối'" aria-label="Xem kết nối" @click="launch('connections')"><Wifi :size="18" /><ShieldCheck :size="17" /></button>
          <button class="taskbar-clock" aria-label="Mở lịch và thông báo" @click="toggleNotifications"><strong>{{ time }}</strong><span>{{ date }}</span></button>
          <button class="tray-bell" aria-label="Thông báo" @click="toggleNotifications"><Bell :size="17" /></button>
          <button class="show-desktop-edge" aria-label="Ẩn hoặc khôi phục tất cả cửa sổ" @click="toggleDesktop"></button>
        </div>
      </nav>
      <div v-if="anyMenu" class="dismiss-layer" @click="dismiss"></div>
      <section v-if="launcher" class="launcher-panel" :class="{'fullscreen-launcher':desktop.preferences.mode === 'focus'}" role="dialog" aria-label="Danh sách ứng dụng">
        <div v-if="desktop.preferences.mode === 'focus'" class="focus-launcher-heading"><div><img src="/mark.svg" alt="" width="36" height="36"><span><strong>MinhAnOS</strong><small>Không gian ứng dụng của bạn</small></span></div><button class="icon-button" aria-label="Đóng danh sách ứng dụng" @click="dismiss"><X :size="21" /></button></div>
        <label class="launcher-search"><Search :size="18" /><input ref="searchInput" v-model="launcherSearch" placeholder="Tìm kiếm ứng dụng trong MinhAnOS" aria-label="Tìm trong danh sách ứng dụng"><kbd>Ctrl + Space</kbd></label>
        <div class="start-scroll">
          <div class="start-section-heading"><strong>{{ launcherSearch ? 'Kết quả tìm kiếm' : launcherAll ? 'Tất cả ứng dụng' : 'Ứng dụng' }}</strong><button v-if="!launcherSearch" @click="launcherAll = !launcherAll">{{ launcherAll ? 'Quay lại' : 'Tất cả' }}<ChevronRight :size="13" /></button><button v-else class="icon-button" aria-label="Xóa tìm kiếm" @click="launcherSearch = ''"><X :size="15" /></button></div>
          <div class="launcher-grid" :class="{'all-apps-list':launcherAll}"><button v-for="app in launcherApps" :key="app.id" @click="launch(app.id)"><AppIcon :id="app.icon" :color="app.color" /><strong>{{ app.title }}</strong><span v-if="launcherAll">{{ app.status === 'planned' ? 'Đang phát triển' : app.category }}</span></button></div>
          <p v-if="!launcherApps.length" class="launcher-empty">Chưa tìm thấy ứng dụng phù hợp.</p>
          <template v-if="!launcherSearch && !launcherAll"><div class="start-section-heading recommended-heading"><div><strong>Đề xuất</strong><p>Tiếp tục những điều bạn đang làm.</p></div></div><div class="recommended-grid"><button v-for="item in recommended" :key="item.id" @click="launch(item.id)"><AppIcon :id="item.app.icon" :color="item.app.color" small /><span><strong>{{ item.app.title }}</strong><small>{{ item.description }}</small></span></button></div></template>
        </div>
        <footer class="start-footer"><button class="start-user" aria-label="Menu tài khoản" @click="toggleUserMenu"><span class="avatar">{{ desktop.user?.displayName.charAt(0) }}</span><span>{{ desktop.user?.displayName }}</span><ChevronDown :size="12" /></button><button class="start-power" aria-label="Đăng xuất" title="Đăng xuất tài khoản" @click="logout"><Power :size="20" /></button></footer>
      </section>
      <section v-if="userMenu" class="user-popover"><div class="popover-profile"><span class="avatar">{{ desktop.user?.displayName.charAt(0) }}</span><div><strong>{{ desktop.user?.displayName }}</strong><span>@{{ desktop.user?.username }}</span></div><span class="role-label">{{ desktop.user?.role === 'admin' ? 'ADMIN' : 'USER' }}</span></div><button @click="launch('settings')"><Settings2 :size="16" /> Cài đặt cá nhân</button><button @click="launch('about')"><CircleHelp :size="16" /> Về MinhAnOS</button><div class="popover-separator"></div><button @click="logout"><LogOut :size="16" /> Đăng xuất</button></section>
      <section v-if="notifications" class="notifications-panel" role="dialog" aria-label="Lịch và thông báo">
        <div class="flyout-notifications"><div class="launcher-heading"><strong>Thông báo</strong><button class="icon-button" aria-label="Đóng thông báo" @click="dismiss"><X :size="16" /></button></div><div class="notification-item"><span class="notification-icon"><Check :size="20" /></span><div><strong>Không gian làm việc đã sẵn sàng</strong><p>Chào {{ desktop.user?.displayName }}. Ghi chú và bố cục của bạn được lưu riêng.</p></div></div><div class="notification-item"><span class="notification-icon neutral"><LockKeyhole :size="19" /></span><div><strong>Các dịch vụ đang chờ kết nối</strong><p>NetBird, Authentik và 1Panel chưa được cấu hình.</p><button class="text-button" @click="launch('connections')">Xem kết nối <ArrowRight :size="13" /></button></div></div></div>
        <div class="calendar-panel"><div class="calendar-date">{{ longDate }}</div><div class="calendar-heading"><strong>{{ calendarLabel }}</strong><div><button aria-label="Tháng trước" @click="calendarOffset--"><ChevronLeft :size="16" /></button><button aria-label="Tháng sau" @click="calendarOffset++"><ChevronRight :size="16" /></button></div></div><div class="calendar-weekdays"><span v-for="day in ['T2','T3','T4','T5','T6','T7','CN']" :key="day">{{ day }}</span></div><div class="calendar-grid"><span v-for="day in calendarDays" :key="day.key" :class="{outside:day.outside,today:day.today}">{{ day.day }}</span></div><button v-if="calendarOffset" class="text-button" @click="calendarOffset = 0">Quay về hôm nay</button></div>
      </section>
      <section v-if="contextMenu" class="context-menu" :style="{left:`${contextMenu.x}px`,top:`${contextMenu.y}px`}"><button @click="launch('apps')"><Grid2X2 :size="16" /> Tất cả ứng dụng</button><button @click="launch('settings')"><Settings2 :size="16" /> Cá nhân hóa desktop</button><button @click="desktop.refreshSystem(); dismiss(); desktop.toast('Đã yêu cầu cập nhật trạng thái.')"><RefreshCw :size="16" /> Làm mới trạng thái</button></section>
      <Transition name="toast"><div v-if="desktop.notice" class="toast-message" role="status"><Check :size="17" />{{ desktop.notice }}</div></Transition>
    </template>
  </div>
</template>
