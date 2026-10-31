<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Search, Wifi, LogOut, ArrowRight, ShieldCheck, Eye, EyeOff, X, Monitor, LoaderCircle, Home, Layers, SlidersHorizontal, Palette, Settings, Check, Bell, Pin, PinOff, ChevronUp } from 'lucide-vue-next'
import {useHost,ApiError} from '@minhanos/app-sdk'
import { api, setCSRF, onAuthFailure, type User } from './api'
import { useDesktop } from './store'
import desktopCatalogs from '../locales/catalogs.json'
import { launcherCatalogs, nameKey, descriptionKey } from './registry'
import AppIcon from './components/AppIcon.vue'
import DesktopWindow from './components/DesktopWindow.vue'
import HomeScreen from './components/HomeScreen.vue'
import AccountsApp from '@minhanos/accounts-app'
import AppSwitcher from './components/AppSwitcher.vue'
import {NotificationPanel,createInbox,notificationCatalogs} from '@minhanos/notifications-app/panel'
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
// Launcher names follow the user's locale: every listed app's catalog stays registered while the Shell runs.
const releaseLauncherCatalogs=launcherCatalogs.map(([namespace,value])=>host.useCatalog(namespace,value))
onBeforeUnmount(()=>releaseLauncherCatalogs.forEach(release=>release()))
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t(key,params)}
const appName=(id:string)=>t(nameKey(id))
const deepLinked=()=>(location.pathname.startsWith('/apps/')&&location.pathname!=='/apps/desktop')||new URLSearchParams(location.search).has('app')
const stopHostEvents=[host.on('session-ended',()=>desktop.expireSession()),host.on('profile-changed',({displayName})=>{if(desktop.user)desktop.user.displayName=displayName}),host.setSettingsOpener(desktop.openSettings)]
onBeforeUnmount(()=>{stopHostEvents.forEach(stop=>stop());releaseDesktopCatalog()})
onBeforeUnmount(stopAuthFailure)
watch(()=>host.profile.value.theme,theme=>{if(theme!=='system'){desktop.preferences.theme=theme}})
const initializing = ref(true), loginBusy = ref(false), loginError = ref(''), username = ref(''), password = ref(''), showPassword = ref(false)
const home = ref(true)
const panel = ref<'search' | 'switcher' | 'control' | 'notifications' | null>(null)
// Notification bell (Notification Center): unread count polled every minute while the page is visible.
const releaseNotificationCatalog=host.useCatalog('notifications',notificationCatalogs)
const inbox=createInbox('/api'), hasInbox=computed(()=>desktop.apps.some(a=>a.id==='com.minhanos.notifications'))
const refreshInbox=()=>{ if (desktop.ready && hasInbox.value && document.visibilityState==='visible' && panel.value!=='notifications') void inbox.load({limit:8}) }
watch(hasInbox,available=>{ if (available) refreshInbox(); else { inbox.items.value=[]; inbox.unread.value=0 } })
const inboxTimer=setInterval(refreshInbox,60000)
document.addEventListener('visibilitychange',refreshInbox)
onBeforeUnmount(()=>{clearInterval(inboxTimer);document.removeEventListener('visibilitychange',refreshInbox);releaseNotificationCatalog()})
function openNotifications(){ launch('com.minhanos.notifications') }
const query = ref(''), searchInput = ref<HTMLInputElement | null>(null), now = ref(new Date())
const time = computed(() => (host.profile.value.locale,host.i18n.formatDate(now.value,{hour:'2-digit',minute:'2-digit'})))
const longDate = computed(() => (host.profile.value.locale,host.i18n.formatDate(now.value,{weekday:'long',day:'numeric',month:'long'})))
const results = computed(() => { const q = query.value.toLocaleLowerCase(host.profile.value.locale); return desktop.apps.filter(a => `${appName(a.id)} ${t(descriptionKey(a.id))}`.toLocaleLowerCase(host.profile.value.locale).includes(q)) })
function togglePin(id: string) { desktop.togglePin(id) }
// Dock (ADR 0013): pinned apps in the user's order, then running apps that are not pinned.
const dockApps = computed(() => [...new Set([...desktop.preferences.pinned, ...desktop.preferences.windows.map(w => w.id)])].map(id => desktop.apps.find(a => a.id === id)).filter((a): a is NonNullable<typeof a> => !!a))
const dockGlyph = computed(() => Math.round(desktop.dock.icon * 0.48))
const unreadText = computed(() => inbox.unread.value > 99 ? '99+' : String(inbox.unread.value))
const bellLabel = computed(() => inbox.unread.value ? host.i18n.t('notifications.bellUnread',{count:inbox.unread.value}) : host.i18n.t('notifications.bell'))
const shortDate = computed(() => (host.profile.value.locale,host.i18n.formatDate(now.value,{day:'2-digit',month:'2-digit'})))
// A maximized app in front takes the whole screen: the dock slides away, a pill with the bell stays on its edge,
// and pointing at that edge (or the pill's handle) shows the dock until the pointer leaves it.
const immersive = computed(() => !home.value && !!desktop.preferences.windows.find(w => w.id === desktop.active && !w.minimized && w.maximized))
const dockPeek = ref(false), menu = ref<{ x: number; y: number; app?: string } | null>(null), menuEl = ref<HTMLElement | null>(null)
const dockShown = computed(() => !immersive.value || dockPeek.value || !!panel.value || !!menu.value)
let peekTimer: ReturnType<typeof setTimeout> | undefined
function peekDock(show: boolean) { clearTimeout(peekTimer); if (show) dockPeek.value = true; else peekTimer = setTimeout(() => { dockPeek.value = false }, 500) }
watch(immersive, () => { dockPeek.value = false })
onBeforeUnmount(() => clearTimeout(peekTimer))
const styleVars = computed(() => ({ '--dock-icon': desktop.dock.icon + 'px', '--dock-reserve': (desktop.dockThickness + 16) + 'px' }))
// Right-click (or Shift+F10): an app → open, pin or unpin, close; Home or empty dock space → position and size.
async function openMenu(event: MouseEvent | KeyboardEvent, app?: string) {
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  const x = event instanceof MouseEvent && event.clientX ? event.clientX : rect.left + rect.width / 2, y = event instanceof MouseEvent && event.clientY ? event.clientY : rect.top
  menu.value = { x, y, app }
  await nextTick()
  const el = menuEl.value; if (!el) return
  const box = el.getBoundingClientRect(), above = desktop.dock.position === 'bottom' && rect.top > innerHeight / 2
  menu.value = { app, x: Math.max(8, Math.min(x, innerWidth - box.width - 8)), y: Math.max(8, Math.min(above ? y - box.height - 8 : y, innerHeight - box.height - 8)) }
  el.querySelector<HTMLElement>('button')?.focus()
}
function moveMenu(step: number) { const items = [...(menuEl.value?.querySelectorAll<HTMLElement>('button') ?? [])]; const at = items.indexOf(document.activeElement as HTMLElement); items[(at + step + items.length) % items.length]?.focus() }
const dragId = ref('')
function dropOn(target: string) { if (dragId.value && desktop.preferences.pinned.includes(target)) desktop.movePin(dragId.value, target); dragId.value = '' }
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
function togglePanel(value: 'switcher' | 'control' | 'notifications') { panel.value = panel.value === value ? null : value }
function openAppearance() { panel.value = null; home.value = false; desktop.openSettings('com.minhanos.desktop','appearance') }
async function login() {
  loginBusy.value = true; loginError.value = ''
  try { const result = await api<{user: User; csrfToken: string}>('/login', {method:'POST',body:JSON.stringify({username:username.value,password:password.value})}); setCSRF(result.csrfToken); password.value = ''; await desktop.loadSession(); await host.loadProfile(api); home.value = !deepLinked() }
  catch(e) { loginError.value = host.errorMessage(e) }
  finally { loginBusy.value = false }
}
// First-run setup: a fresh install has no account; step 1 creates the System Admin (setup code from the server's
// secrets/bootstrap_password), step 2 names the first organization. One request does both, then signs in.
const setupRequired = ref(false), setupStep = ref<1 | 2>(1), setup = ref({code:'',displayName:'',username:'',password:'',repeat:'',organization:''})
async function checkSetup() { try { setupRequired.value = (await api<{required:boolean}>('/setup')).required } catch { setupRequired.value = false } }
function setupNext() {
  loginError.value = ''
  if ([...setup.value.password].length < 12) { loginError.value = t('desktop.setup.passwordShort'); return }
  if (setup.value.password !== setup.value.repeat) { loginError.value = t('desktop.setup.mismatch'); return }
  setupStep.value = 2; void nextTick(() => document.getElementById('setup-organization')?.focus())
}
async function finishSetup() {
  loginBusy.value = true; loginError.value = ''
  try {
    const s = setup.value
    await api('/setup', {method:'POST',body:JSON.stringify({setupCode:s.code,username:s.username,displayName:s.displayName,password:s.password,organizationName:s.organization})})
    username.value = s.username; password.value = s.password; setup.value = {code:'',displayName:'',username:'',password:'',repeat:'',organization:''}; setupRequired.value = false; setupStep.value = 1
    loginBusy.value = false; await login()
  } catch(e) { loginError.value = host.errorMessage(e); if (e instanceof ApiError && e.code === 'SETUP_DONE') { setupRequired.value = false; setupStep.value = 1 } else if (e instanceof ApiError && /CODE|USERNAME|NAME$|PASSWORD/.test(e.code)) setupStep.value = 1 }
  finally { loginBusy.value = false }
}
async function logout() { panel.value = null; try { await desktop.logout(); username.value = ''; password.value = ''; home.value = true } catch(e) { desktop.toast(host.errorMessage(e)) } }
function keyboard(e: KeyboardEvent) {
  if (!desktop.ready) return
  if ((e.ctrlKey || e.metaKey) && e.code === 'Space') { e.preventDefault(); void search() }
  if (e.key === 'Escape') { panel.value = null; menu.value = null }
  if (panel.value && e.key === 'Tab') {
    const nodes = [...document.querySelectorAll<HTMLElement>('[aria-modal="true"] button, [aria-modal="true"] input')].filter(el => !el.hasAttribute('disabled') && el.getClientRects().length)
    const first = nodes[0], last = nodes.at(-1)
    if (first && (e.shiftKey ? document.activeElement === first : document.activeElement === last)) { e.preventDefault(); (e.shiftKey ? last : first)?.focus() }
  }
  if (e.altKey && e.key === 'Tab') { e.preventDefault(); const wins = desktop.preferences.windows; if (wins.length) launch(wins[(wins.findIndex(w => w.id === desktop.active) + 1) % wins.length].id) }
}
onMounted(async () => { globalThis.addEventListener('keydown', keyboard); try { await desktop.loadSession(); await host.loadProfile(api); home.value = !deepLinked() } catch(e) { if (e instanceof Error && !(e instanceof ApiError && e.code==='AUTH_REQUIRED')) loginError.value = host.errorMessage(e); else await checkSetup() } finally { initializing.value = false } })
const clockTimer = setInterval(() => { now.value = new Date() }, 10000)
const systemTimer = setInterval(() => { if (desktop.ready) void desktop.refreshSystem() }, 10000)
onBeforeUnmount(() => { clearInterval(clockTimer); clearInterval(systemTimer); globalThis.removeEventListener('keydown', keyboard) })
</script>
<template>
  <div :style="desktop.ready ? styleVars : undefined" :class="['platform', 'tablet-shell', desktop.ready ? 'dock-' + desktop.dock.position : '', {'dock-immersive': desktop.ready && immersive}, desktop.ready ? desktop.preferences.wallpaper : 'mountain', {'dark-mode':desktop.ready && (host.profile.value.theme==='dark'||(host.profile.value.theme==='system'&&systemDark)),'focus-workspace':desktop.ready && desktop.preferences.mode === 'focus'}]">
    <div class="wallpaper"></div><div class="wallpaper-shade"></div>
    <div v-if="initializing" class="initial-loader"><img src="/mark.svg" alt="MinhAnOS" width="58" height="58"><LoaderCircle :size="24" class="spinning" /><span>{{ t('desktop.loading') }}</span></div>
    <main v-else-if="desktop.passwordChangeRequired" class="password-change-screen" role="dialog" aria-modal="true" :aria-label="host.i18n.t('desktop.passwordChange.title')"><section class="password-change-card"><header><h1>{{ host.i18n.t('desktop.passwordChange.title') }}</h1><p>{{ host.i18n.t('desktop.passwordChange.hint') }}</p></header><AccountsApp embedded panel section="security" :active="true" /><button class="secondary-button" @click="desktop.logout().catch(e => desktop.toast(host.errorMessage(e)))"><LogOut :size="15" /> {{ host.i18n.t('desktop.passwordChange.logout') }}</button></section></main>
    <template v-else-if="!desktop.ready">
      <div class="login-brand"><img src="/mark.svg" alt="" width="32" height="32"><strong>MinhAn<span>OS</span></strong><span class="brand-divider"></span><span>{{ t('desktop.brand.tagline') }}</span></div>
      <main class="login-screen"><section class="login-story"><div class="login-eyebrow"><span></span> {{ t('desktop.login.eyebrow') }}</div><h1>{{ t('desktop.login.headline') }}</h1><p>{{ t('desktop.login.story') }}</p><div class="login-story-footer"><span class="story-line"></span><span>{{ t('desktop.login.storyFooter') }}</span></div></section><section v-if="setupRequired" class="login-card setup-card" :aria-label="t('desktop.setup.eyebrow')"><div class="login-symbol"><ShieldCheck :size="25" :stroke-width="1.5" /></div><span class="page-eyebrow">{{ t('desktop.setup.eyebrow') }} · {{ t('desktop.setup.step', { step: setupStep }) }}</span>
        <form v-if="setupStep === 1" @submit.prevent="setupNext"><h2>{{ t('desktop.setup.adminTitle') }}</h2><p class="muted">{{ t('desktop.setup.adminHint') }}</p><label for="setup-code">{{ t('desktop.setup.code') }}</label><input id="setup-code" v-model="setup.code" type="password" autocomplete="off" required maxlength="256" autofocus><small class="setup-help">{{ t('desktop.setup.codeHint') }}</small><label for="setup-name">{{ t('desktop.setup.displayName') }}</label><input id="setup-name" v-model="setup.displayName" autocomplete="name" required maxlength="100"><label for="setup-username">{{ t('desktop.setup.username') }}</label><input id="setup-username" v-model="setup.username" autocomplete="username" required minlength="3" maxlength="32" pattern="[a-z][a-z0-9_\-]{2,31}" aria-describedby="setup-username-hint"><small id="setup-username-hint" class="setup-help">{{ t('desktop.setup.usernameHint') }}</small><label for="setup-password">{{ t('desktop.setup.password') }}</label><input id="setup-password" v-model="setup.password" type="password" autocomplete="new-password" required minlength="12" maxlength="256" aria-describedby="setup-password-hint"><small id="setup-password-hint" class="setup-help">{{ t('desktop.setup.passwordHint') }}</small><label for="setup-repeat">{{ t('desktop.setup.repeat') }}</label><input id="setup-repeat" v-model="setup.repeat" type="password" autocomplete="new-password" required maxlength="256"><p v-if="loginError" class="login-error" role="alert">{{ loginError }}</p><button class="login-submit">{{ t('desktop.setup.next') }}<ArrowRight :size="18" /></button></form>
        <form v-else @submit.prevent="finishSetup"><h2>{{ t('desktop.setup.orgTitle') }}</h2><p class="muted">{{ t('desktop.setup.orgHint') }}</p><label for="setup-organization">{{ t('desktop.setup.orgName') }}</label><input id="setup-organization" v-model="setup.organization" required maxlength="100" :placeholder="t('desktop.setup.orgPlaceholder')"><p class="setup-summary">{{ t('desktop.setup.summary', { name: setup.displayName, username: setup.username }) }}</p><p v-if="loginError" class="login-error" role="alert">{{ loginError }}</p><button class="login-submit" :disabled="loginBusy"><LoaderCircle v-if="loginBusy" :size="18" class="spinning" />{{ loginBusy ? t('desktop.setup.busy') : t('desktop.setup.submit') }}<Check v-if="!loginBusy" :size="18" /></button><button type="button" class="setup-back" :disabled="loginBusy" @click="setupStep = 1; loginError = ''">{{ t('desktop.setup.back') }}</button></form>
        <div class="login-security"><ShieldCheck :size="15" /><span>{{ t('desktop.setup.security') }}</span></div></section><section v-else class="login-card">
<div class="login-symbol"><Monitor :size="25" :stroke-width="1.5" /></div><span class="page-eyebrow">{{ t('desktop.app.name') }}</span><h2>{{ t('desktop.login.welcome') }}</h2><p class="muted">{{ t('desktop.login.hint') }}</p><form @submit.prevent="login"><label for="username">{{ t('desktop.login.username') }}</label><input id="username" v-model="username" name="username" autocomplete="username" :placeholder="t('desktop.login.usernamePlaceholder')" required maxlength="64" autofocus><label for="password">{{ t('desktop.login.password') }}</label><div class="password-input"><input id="password" v-model="password" name="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" :placeholder="t('desktop.login.passwordPlaceholder')" required maxlength="256"><button type="button" :aria-label="showPassword ? t('desktop.login.hidePassword') : t('desktop.login.showPassword')" :aria-pressed="showPassword" @click="showPassword = !showPassword"><component :is="showPassword ? EyeOff : Eye" :size="17" /></button></div><p v-if="loginError" class="login-error" role="alert">{{ loginError }}</p><button class="login-submit" :disabled="loginBusy"><LoaderCircle v-if="loginBusy" :size="18" class="spinning" />{{ loginBusy ? t('desktop.login.busy') : t('desktop.login.submit') }}<ArrowRight v-if="!loginBusy" :size="18" /></button></form><div class="login-security"><ShieldCheck :size="15" /><span>{{ t('desktop.login.security') }}</span></div></section></main>
      <footer class="login-footer"><span>{{ t('desktop.login.footer', { year: now.getFullYear() }) }}</span></footer>
    </template>

    <template v-else>
      <HomeScreen :inert="!!panel" v-show="home || !desktop.visibleWindows.length" :time="time" :date="longDate" @launch="launch" @search="search" @menu="(event, id) => openMenu(event, id)" />
      <!-- Keep instances mounted: Home, Spotlight and switching must retain local drafts. -->
      <DesktopWindow :foreground="!home && desktop.active===window.id && !window.minimized && !panel" :inert="!!panel" v-for="window in desktop.preferences.windows" v-show="!home && !window.minimized && (desktop.preferences.mode === 'windows' || desktop.active === window.id)" :key="window.id" :window="window" :app="desktop.apps.find(a => a.id === window.id)!" />
      <div v-if="panel" class="tablet-scrim" @click="panel = null"></div>
      <section v-if="panel === 'search'" class="tablet-spotlight" role="dialog" aria-modal="true" :aria-label="t('desktop.search.label')" @keydown.esc="panel = null"><label><Search :size="23" /><input ref="searchInput" v-model="query" :placeholder="t('desktop.search.placeholder')" :aria-label="t('desktop.search.input')"><button class="tablet-round" :aria-label="t('desktop.search.close')" @click="panel = null"><X :size="18" /></button></label><div class="spotlight-results"><div v-for="app in results" :key="app.id" class="spotlight-row"><button class="spotlight-open" @click="launch(app.id)"><AppIcon :id="app.icon" :color="app.color" small /><span><strong>{{ appName(app.id) }}</strong><small>{{ t(descriptionKey(app.id)) }}</small></span><ArrowRight :size="16" /></button><button class="tablet-round spotlight-pin" :aria-pressed="desktop.preferences.pinned.includes(app.id)" :aria-label="t(desktop.preferences.pinned.includes(app.id) ? 'desktop.search.unpin' : 'desktop.search.pin', { name: appName(app.id) })" :title="t(desktop.preferences.pinned.includes(app.id) ? 'desktop.search.unpin' : 'desktop.search.pin', { name: appName(app.id) })" @click="togglePin(app.id)"><component :is="desktop.preferences.pinned.includes(app.id) ? PinOff : Pin" :size="16" /></button></div><p v-if="!results.length">{{ t('desktop.search.empty') }}</p></div><footer>{{ t('desktop.search.footer') }} <kbd>{{ t('desktop.search.esc') }}</kbd></footer></section>
      <AppSwitcher v-if="panel === 'switcher'" @launch="launch" @dismiss="panel = null" />
      <section v-if="panel === 'notifications'" class="tablet-control tablet-notifications" role="dialog" aria-modal="true" aria-labelledby="nc-panel-title"><NotificationPanel :inbox="inbox" @open="openNotifications" @close="panel = null" /></section>
      <section v-if="panel === 'control'" class="tablet-control" role="dialog" aria-modal="true" aria-labelledby="control-title"><header><span id="control-title">{{ t('desktop.control.title') }}</span><button class="tablet-round" :aria-label="t('desktop.control.close')" @click="panel = null"><X :size="18" /></button></header><div class="control-profile"><span class="avatar">{{ desktop.user?.displayName.charAt(0) }}</span><div><strong>{{ desktop.user?.displayName }}</strong><small>@{{ desktop.user?.username }}</small></div></div><div class="control-connection"><Wifi :size="23" /><div><strong>{{ desktop.system ? t('desktop.control.apiOnline') : t('desktop.control.apiOffline') }}</strong><small v-if="desktop.system">{{ t('desktop.control.host', { host: desktop.system.hostname }) }}</small></div></div><button class="control-settings" @click="openAppearance"><Palette :size="18" /> {{ t('desktop.control.appearance') }}<ArrowRight :size="16" /></button><button class="control-settings" @click="launch('settings')"><Settings :size="18" /> {{ t('desktop.control.settings') }}<ArrowRight :size="16" /></button><button class="control-settings" @click="logout"><LogOut :size="18" /> {{ t('desktop.control.logout') }}</button></section>
      <div v-if="immersive && !dockShown" class="dock-hotzone" aria-hidden="true" @pointerenter="peekDock(true)"></div>
      <div v-if="immersive && !dockShown" class="dock-pill" :inert="!!panel">
        <button v-if="hasInbox" class="dock-pill-bell" :aria-label="bellLabel" :title="host.i18n.t('notifications.bell')" @click="togglePanel('notifications')"><Bell :size="15" /><b v-if="inbox.unread.value" class="status-badge">{{ unreadText }}</b></button>
        <button class="dock-pill-show" :aria-label="t('desktop.dock.show')" :title="t('desktop.dock.show')" @click="peekDock(true)"><ChevronUp :size="15" /></button>
      </div>
      <nav :inert="!!panel" class="tablet-dock" :class="{ 'dock-away': !dockShown }" :aria-label="t('desktop.dock.label')" @contextmenu.self.prevent="openMenu($event)" @pointerenter="peekDock(true)" @pointerleave="peekDock(false)">
        <button class="tablet-dock-action" :class="{selected:home}" :aria-label="t('desktop.dock.home')" :title="t('desktop.dock.home')" aria-haspopup="menu" @click="goHome" @contextmenu.prevent="openMenu($event)" @keydown.shift.f10.prevent="openMenu($event)"><Home :size="dockGlyph" /></button>
        <div class="tablet-dock-separator"></div>
        <button v-for="app in dockApps" :key="app.id" class="tablet-dock-app" :class="{active:!home && desktop.active === app.id && desktop.preferences.windows.some(w => w.id === app.id && !w.minimized), dragging: dragId === app.id}" :draggable="desktop.preferences.pinned.includes(app.id)" :aria-label="t('desktop.dock.open', { name: appName(app.id) })" aria-haspopup="menu" @click="launch(app.id)" @contextmenu.prevent="openMenu($event, app.id)" @keydown.shift.f10.prevent="openMenu($event, app.id)" @dragstart="dragId = app.id" @dragend="dragId = ''" @dragover.prevent @drop.prevent="dropOn(app.id)"><AppIcon :id="app.icon" :color="app.color" /><span class="tablet-dock-tooltip">{{ appName(app.id) }}</span><i v-if="desktop.preferences.windows.some(w => w.id === app.id)"></i></button>
        <div class="tablet-dock-separator"></div>
        <button class="tablet-dock-action" :aria-label="t('desktop.dock.search')" :title="t('desktop.dock.searchHint')" @click="search"><Search :size="dockGlyph" /></button>
        <button class="tablet-dock-action" :aria-label="t('desktop.dock.running')" :title="t('desktop.dock.running')" @click="togglePanel('switcher')"><Layers :size="dockGlyph" /><span v-if="desktop.preferences.windows.length" class="dock-count">{{ desktop.preferences.windows.length }}</span></button>
        <div class="tablet-dock-separator"></div>
        <div class="dock-tray">
          <button v-if="hasInbox" class="tablet-dock-action dock-bell" :class="{selected:panel==='notifications'}" :aria-label="bellLabel" :title="host.i18n.t('notifications.bell')" @click="togglePanel('notifications')"><Bell :size="dockGlyph - 3" /><b v-if="inbox.unread.value" class="status-badge">{{ unreadText }}</b></button>
          <button class="tablet-dock-action dock-status" :class="{selected:panel==='control'}" :aria-label="t('desktop.status.control')" :title="desktop.system ? t('desktop.status.online') : t('desktop.status.offline')" @click="togglePanel('control')"><span :class="['status-dot', {offline:!desktop.system}]"></span><SlidersHorizontal :size="dockGlyph - 5" /></button>
          <button class="dock-clock" :aria-label="t('desktop.dock.clock', { time, date: longDate })" :title="longDate" @click="togglePanel('control')"><strong>{{ time }}</strong><small v-if="desktop.dock.position === 'bottom'">{{ shortDate }}</small></button>
        </div>
      </nav>
      <div v-if="menu" class="dock-menu-scrim" @click="menu = null" @contextmenu.prevent="menu = null"></div>
      <div v-if="menu" ref="menuEl" class="dock-menu" role="menu" :aria-label="menu.app ? appName(menu.app) : t('desktop.dock.settings')" :style="{ left: menu.x + 'px', top: menu.y + 'px' }" @keydown.esc.stop="menu = null" @keydown.down.prevent="moveMenu(1)" @keydown.up.prevent="moveMenu(-1)">
        <template v-if="menu.app">
          <button role="menuitem" @click="launch(menu.app); menu = null"><ArrowRight :size="15" />{{ t('desktop.dock.menuOpen') }}</button>
          <button role="menuitem" @click="desktop.togglePin(menu.app); menu = null"><component :is="desktop.preferences.pinned.includes(menu.app) ? PinOff : Pin" :size="15" />{{ t(desktop.preferences.pinned.includes(menu.app) ? 'desktop.dock.unpin' : 'desktop.dock.pin') }}</button>
          <button v-if="desktop.preferences.windows.some(w => w.id === menu?.app)" role="menuitem" @click="desktop.close(menu.app); menu = null"><X :size="15" />{{ t('desktop.dock.menuClose') }}</button>
        </template>
        <template v-else>
          <span class="dock-menu-label">{{ t('desktop.dock.position') }}</span>
          <button v-for="p in (['bottom','left','right'] as const)" :key="p" role="menuitemradio" :aria-checked="desktop.preferences.dock?.position === p" @click="desktop.setDock({ position: p }); menu = null"><Check v-if="desktop.preferences.dock?.position === p" :size="15" /><span v-else class="dock-menu-gap"></span>{{ t('desktop.dock.positions.' + p) }}</button>
          <span class="dock-menu-label">{{ t('desktop.dock.size') }}</span>
          <button v-for="s in (['small','medium','large'] as const)" :key="s" role="menuitemradio" :aria-checked="desktop.preferences.dock?.size === s" @click="desktop.setDock({ size: s }); menu = null"><Check v-if="desktop.preferences.dock?.size === s" :size="15" /><span v-else class="dock-menu-gap"></span>{{ t('desktop.dock.sizes.' + s) }}</button>
          <button role="menuitem" @click="menu = null; openAppearance()"><Settings :size="15" />{{ t('desktop.dock.settings') }}</button>
        </template>
      </div>
      <button v-if="desktop.saveError" class="tablet-save-error" @click="desktop.persist">{{ t('desktop.save.retry') }}</button><span v-else-if="desktop.saving" class="tablet-saving" role="status">{{ t('desktop.save.saving') }}</span>
      <Transition name="toast"><div v-if="desktop.notice" class="toast-message" role="status"><Check :size="17" />{{ desktop.notice }}</div></Transition>
    </template>
  </div>
</template>
