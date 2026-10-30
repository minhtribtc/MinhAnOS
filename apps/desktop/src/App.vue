<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Search, Wifi, LogOut, ArrowRight, ShieldCheck, Eye, EyeOff, X, Monitor, LoaderCircle, Home, Layers, SlidersHorizontal, Moon, Sun, Settings, Check } from 'lucide-vue-next'
import {useHost,ApiError} from '@minhanos/app-sdk'
import { api, setCSRF, onAuthFailure, type User } from './api'
import { useDesktop } from './store'
import desktopCatalogs from '../locales/catalogs.json'
import AppIcon from './components/AppIcon.vue'
import DesktopWindow from './components/DesktopWindow.vue'
import HomeScreen from './components/HomeScreen.vue'
import AccountsApp from '@minhanos/accounts-app'
import AppSwitcher from './components/AppSwitcher.vue'
const systemDark=ref(globalThis.matchMedia('(prefers-color-scheme: dark)').matches)
const themeMedia=globalThis.matchMedia('(prefers-color-scheme: dark)')
const themeChanged=(e:MediaQueryListEvent)=>systemDark.value=e.matches
themeMedia.addEventListener('change',themeChanged)
onBeforeUnmount(()=>themeMedia.removeEventListener('change',themeChanged))
const host=useHost()
const desktop = useDesktop()
const stopAuthFailure=onAuthFailure(()=>desktop.expireSession())
// App-to-Shell bridge (app-sdk): apps report session/profile changes and deep-link into Settings.
const releaseDesktopCatalog=host.useCatalog('desktop',desktopCatalogs)
const stopHostEvents=[host.on('session-ended',()=>desktop.expireSession()),host.on('profile-changed',({displayName})=>{if(desktop.user)desktop.user.displayName=displayName}),host.setSettingsOpener(desktop.openSettings)]
onBeforeUnmount(()=>{stopHostEvents.forEach(stop=>stop());releaseDesktopCatalog()})
onBeforeUnmount(stopAuthFailure)
watch(()=>host.profile.value.theme,theme=>{if(theme!=='system'){desktop.preferences.theme=theme}})
const initializing = ref(true), loginBusy = ref(false), loginError = ref(''), username = ref(''), password = ref(''), showPassword = ref(false)
const home = ref(true)
const panel = ref<'search' | 'switcher' | 'control' | null>(null)
const query = ref(''), searchInput = ref<HTMLInputElement | null>(null), now = ref(new Date())
const time = computed(() => (host.profile.value.locale,host.i18n.formatDate(now.value,{hour:'2-digit',minute:'2-digit'})))
const longDate = computed(() => (host.profile.value.locale,host.i18n.formatDate(now.value,{weekday:'long',day:'numeric',month:'long'})))
const dockApps = computed(() => [...new Set([...desktop.preferences.pinned, ...desktop.preferences.windows.map(w => w.id)])].map(id => desktop.apps.find(a => a.id === id)).filter((a): a is NonNullable<typeof a> => !!a))
const results = computed(() => desktop.apps.filter(a => `${a.title} ${a.subtitle}`.toLocaleLowerCase('vi').includes(query.value.toLocaleLowerCase('vi'))))
let restoreFocus: HTMLElement | null = null
watch(panel, async (value, previous) => {
  if (value && !previous) restoreFocus = document.activeElement as HTMLElement | null
  await nextTick()
  if (value) document.querySelector<HTMLElement>('[aria-modal="true"] input, [aria-modal="true"] button')?.focus()
  else if (restoreFocus?.isConnected) restoreFocus.focus()
})
function launch(id: string) { desktop.open(id); home.value = false; panel.value = null }
function goHome() { home.value = true; panel.value = null }
async function search() { panel.value = panel.value === 'search' ? null : 'search'; query.value = ''; if (panel.value) { await nextTick(); searchInput.value?.focus() } }
function togglePanel(value: 'switcher' | 'control') { panel.value = panel.value === value ? null : value }
function setTheme(theme: 'light' | 'dark') { void host.updateProfile(api,{theme}).catch(e=>desktop.toast(e.message)); desktop.preferences.theme = theme; desktop.saveSoon() }
async function login() {
  loginBusy.value = true; loginError.value = ''
  try { const result = await api<{user: User; csrfToken: string}>('/login', {method:'POST',body:JSON.stringify({username:username.value,password:password.value})}); setCSRF(result.csrfToken); password.value = ''; await desktop.loadSession(); await host.loadProfile(api); home.value = !new URLSearchParams(location.search).has('app') }
  catch(e) { loginError.value = host.errorMessage(e) }
  finally { loginBusy.value = false }
}
async function logout() { panel.value = null; try { await desktop.logout(); username.value = ''; password.value = ''; home.value = true } catch(e) { desktop.toast(host.errorMessage(e)) } }
function keyboard(e: KeyboardEvent) {
  if (!desktop.ready) return
  if ((e.ctrlKey || e.metaKey) && e.code === 'Space') { e.preventDefault(); void search() }
  if (e.key === 'Escape') panel.value = null
  if (panel.value && e.key === 'Tab') {
    const nodes = [...document.querySelectorAll<HTMLElement>('[aria-modal="true"] button, [aria-modal="true"] input')].filter(el => !el.hasAttribute('disabled') && el.getClientRects().length)
    const first = nodes[0], last = nodes.at(-1)
    if (first && (e.shiftKey ? document.activeElement === first : document.activeElement === last)) { e.preventDefault(); (e.shiftKey ? last : first)?.focus() }
  }
  if (e.altKey && e.key === 'Tab') { e.preventDefault(); const wins = desktop.preferences.windows; if (wins.length) launch(wins[(wins.findIndex(w => w.id === desktop.active) + 1) % wins.length].id) }
}
onMounted(async () => { globalThis.addEventListener('keydown', keyboard); try { await desktop.loadSession(); await host.loadProfile(api); home.value = !new URLSearchParams(location.search).has('app') } catch(e) { if (e instanceof Error && !(e instanceof ApiError && e.code==='AUTH_REQUIRED')) loginError.value = host.errorMessage(e) } finally { initializing.value = false } })
const clockTimer = setInterval(() => { now.value = new Date() }, 10000)
const systemTimer = setInterval(() => { if (desktop.ready) void desktop.refreshSystem() }, 10000)
onBeforeUnmount(() => { clearInterval(clockTimer); clearInterval(systemTimer); globalThis.removeEventListener('keydown', keyboard) })
</script>
<template>
  <div :class="['platform', 'tablet-shell', desktop.ready ? desktop.preferences.wallpaper : 'mountain', {'dark-mode':desktop.ready && (host.profile.value.theme==='dark'||(host.profile.value.theme==='system'&&systemDark)),'focus-workspace':desktop.ready && desktop.preferences.mode === 'focus'}]">
    <div class="wallpaper"></div><div class="wallpaper-shade"></div>
    <div v-if="initializing" class="initial-loader"><img src="/mark.svg" alt="MinhAnOS" width="58" height="58"><LoaderCircle :size="24" class="spinning" /><span>Đang mở không gian của bạn…</span></div>
    <main v-else-if="desktop.passwordChangeRequired" class="password-change-screen" role="dialog" aria-modal="true" :aria-label="host.i18n.t('desktop.passwordChange.title')"><section class="password-change-card"><header><h1>{{ host.i18n.t('desktop.passwordChange.title') }}</h1><p>{{ host.i18n.t('desktop.passwordChange.hint') }}</p></header><AccountsApp embedded panel section="security" :active="true" /><button class="secondary-button" @click="desktop.logout().catch(e => desktop.toast(host.errorMessage(e)))"><LogOut :size="15" /> {{ host.i18n.t('desktop.passwordChange.logout') }}</button></section></main>
    <template v-else-if="!desktop.ready">
      <div class="login-brand"><img src="/mark.svg" alt="" width="32" height="32"><strong>MinhAn<span>OS</span></strong><span class="brand-divider"></span><span>YOUR SPACE. YOUR POSSIBILITIES.</span></div>
      <main class="login-screen"><section class="login-story"><div class="login-eyebrow"><span></span> KHÔNG GIAN CỦA BẠN</div><h1>Mọi thứ,<br>trong tầm tay.</h1><p>Một desktop cho máy chủ, ứng dụng<br>và những điều bạn đang xây dựng.</p><div class="login-story-footer"><span class="story-line"></span><span>TỪNG BƯỚC, MỞ RA NHIỀU KHẢ NĂNG.</span></div></section><section class="login-card"><div class="login-symbol"><Monitor :size="25" :stroke-width="1.5" /></div><span class="page-eyebrow">MINHANOS DESKTOP</span><h2>Chào mừng trở lại.</h2><p class="muted">Đăng nhập để vào không gian của bạn.</p><form @submit.prevent="login"><label for="username">Tên đăng nhập</label><input id="username" v-model="username" name="username" autocomplete="username" placeholder="Nhập tên đăng nhập" required maxlength="64" autofocus><label for="password">Mật khẩu</label><div class="password-input"><input id="password" v-model="password" name="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" placeholder="Nhập mật khẩu" required maxlength="256"><button type="button" :aria-label="showPassword ? 'Ẩn mật khẩu' : 'Hiện mật khẩu'" @click="showPassword = !showPassword"><component :is="showPassword ? EyeOff : Eye" :size="17" /></button></div><p v-if="loginError" class="login-error" role="alert">{{ loginError }}</p><button class="login-submit" :disabled="loginBusy"><LoaderCircle v-if="loginBusy" :size="18" class="spinning" />{{ loginBusy ? 'Đang đăng nhập…' : 'Vào desktop' }}<ArrowRight v-if="!loginBusy" :size="18" /></button></form><div class="login-security"><ShieldCheck :size="15" /><span>Tài khoản cục bộ · Phiên riêng tư</span></div><div class="login-version">DESKTOP PREVIEW <span>0.2.0</span></div></section></main>
      <footer class="login-footer"><span>MinhAnOS © {{ now.getFullYear() }}</span><span><span class="connection-dot"></span> 10.1.1.20 · Môi trường phát triển</span></footer>
    </template>

    <template v-else>
      <header class="tablet-status" :inert="!!panel"><button @click="goHome">{{ time }} <span>{{ longDate }}</span></button><span class="status-brand">MinhAnOS</span><button aria-label="Mở trung tâm điều khiển" @click="togglePanel('control')"><span :class="['status-dot', {offline:!desktop.system}]"></span><span>{{ desktop.system ? 'Đã kết nối' : 'Mất kết nối' }}</span><Wifi :size="16" /><SlidersHorizontal :size="17" /></button></header>
      <HomeScreen :inert="!!panel" v-show="home || !desktop.visibleWindows.length" :time="time" :date="longDate" @launch="launch" @search="search" />
      <!-- Keep instances mounted: Home, Spotlight and switching must retain local drafts. -->
      <DesktopWindow :foreground="!home && desktop.active===window.id && !window.minimized && !panel" :inert="!!panel" v-for="window in desktop.preferences.windows" v-show="!home && !window.minimized && (desktop.preferences.mode === 'windows' || desktop.active === window.id)" :key="window.id" :window="window" :app="desktop.apps.find(a => a.id === window.id)!" />
      <div v-if="panel" class="tablet-scrim" @click="panel = null"></div>
      <section v-if="panel === 'search'" class="tablet-spotlight" role="dialog" aria-modal="true" aria-label="Tìm ứng dụng" @keydown.esc="panel = null"><label><Search :size="23" /><input ref="searchInput" v-model="query" placeholder="Tìm ứng dụng…" aria-label="Tìm trong danh sách ứng dụng"><button class="tablet-round" aria-label="Đóng tìm kiếm" @click="panel = null"><X :size="18" /></button></label><div class="spotlight-results"><button v-for="app in results" :key="app.id" @click="launch(app.id)"><AppIcon :id="app.icon" :color="app.color" small /><span><strong>{{ app.title }}</strong><small>{{ app.subtitle }}</small></span><ArrowRight :size="16" /></button><p v-if="!results.length">Không tìm thấy ứng dụng phù hợp.</p></div><footer>Ứng dụng trên MinhAnOS <kbd>Esc để đóng</kbd></footer></section>
      <AppSwitcher v-if="panel === 'switcher'" @launch="launch" @dismiss="panel = null" />
      <section v-if="panel === 'control'" class="tablet-control" role="dialog" aria-modal="true" aria-label="Trung tâm điều khiển"><header><span>Trung tâm điều khiển</span><button class="tablet-round" aria-label="Đóng trung tâm điều khiển" @click="panel = null"><X :size="18" /></button></header><div class="control-profile"><span class="avatar">{{ desktop.user?.displayName.charAt(0) }}</span><div><strong>{{ desktop.user?.displayName }}</strong><small>@{{ desktop.user?.username }} · {{ desktop.user?.role }}</small></div></div><div class="control-connection"><Wifi :size="23" /><div><strong>{{ desktop.system ? 'API đã kết nối' : 'API chưa kết nối' }}</strong><small>NetBird, Authentik, 1Panel chưa cấu hình</small></div></div><div class="control-tiles"><button :class="{selected:desktop.preferences.theme === 'light'}" @click="setTheme('light')"><Sun :size="24" />Giao diện sáng</button><button :class="{selected:desktop.preferences.theme === 'dark'}" @click="setTheme('dark')"><Moon :size="24" />Giao diện tối</button></div><button class="control-settings" @click="launch('settings')"><Settings :size="18" /> Cài đặt không gian<ArrowRight :size="16" /></button><button class="control-settings" @click="logout"><LogOut :size="18" /> Đăng xuất</button><p>Điều khiển desktop web · Chưa có quyền quản trị Debian qua giao diện.</p></section>
      <nav :inert="!!panel" class="tablet-dock" aria-label="Thanh ứng dụng"><button class="tablet-dock-action" :class="{selected:home}" aria-label="Màn hình chính" title="Màn hình chính" @click="goHome"><Home :size="25" /></button><div class="tablet-dock-separator"></div><button v-for="app in dockApps" :key="app.id" class="tablet-dock-app" :class="{active:!home && desktop.active === app.id && desktop.preferences.windows.some(w => w.id === app.id && !w.minimized)}" :aria-label="`Mở ${app.title}`" @click="launch(app.id)"><AppIcon :id="app.icon" :color="app.color" /><span class="tablet-dock-tooltip">{{ app.title }}</span><i v-if="desktop.preferences.windows.some(w => w.id === app.id)"></i></button><div class="tablet-dock-separator"></div><button class="tablet-dock-action" aria-label="Tìm ứng dụng" title="Tìm ứng dụng · Ctrl+Space" @click="search"><Search :size="25" /></button><button class="tablet-dock-action" aria-label="Ứng dụng đang mở" title="Ứng dụng đang mở" @click="togglePanel('switcher')"><Layers :size="25" /><span v-if="desktop.preferences.windows.length" class="dock-count">{{ desktop.preferences.windows.length }}</span></button></nav>
      <button v-if="desktop.saveError" class="tablet-save-error" @click="desktop.persist">Chưa lưu — thử lại</button><span v-else-if="desktop.saving" class="tablet-saving" role="status">Đang lưu…</span>
      <Transition name="toast"><div v-if="desktop.notice" class="toast-message" role="status"><Check :size="17" />{{ desktop.notice }}</div></Transition>
    </template>
  </div>
</template>
