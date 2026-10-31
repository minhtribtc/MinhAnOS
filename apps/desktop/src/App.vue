<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Search, Wifi, LogOut, ArrowRight, ShieldCheck, Eye, EyeOff, X, Monitor, LoaderCircle, Home, Layers, SlidersHorizontal, Palette, Settings, Check, Bell, Pin, PinOff } from 'lucide-vue-next'
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
const dockApps = computed(() => [...new Set([...desktop.preferences.pinned, ...desktop.preferences.windows.map(w => w.id)])].map(id => desktop.apps.find(a => a.id === id)).filter((a): a is NonNullable<typeof a> => !!a))
const results = computed(() => { const q = query.value.toLocaleLowerCase(host.profile.value.locale); return desktop.apps.filter(a => `${appName(a.id)} ${t(descriptionKey(a.id))}`.toLocaleLowerCase(host.profile.value.locale).includes(q)) })
function togglePin(id: string) { const pinned = desktop.preferences.pinned; desktop.preferences.pinned = pinned.includes(id) ? pinned.filter(a => a !== id) : [...pinned, id]; desktop.saveSoon() }
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
  if (e.key === 'Escape') panel.value = null
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
  <div :class="['platform', 'tablet-shell', desktop.ready ? desktop.preferences.wallpaper : 'mountain', {'dark-mode':desktop.ready && (host.profile.value.theme==='dark'||(host.profile.value.theme==='system'&&systemDark)),'focus-workspace':desktop.ready && desktop.preferences.mode === 'focus'}]">
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
      <header class="tablet-status" :inert="!!panel"><button :aria-label="t('desktop.status.home')" @click="goHome">{{ time }} <span>{{ longDate }}</span></button><span class="status-brand">MinhAnOS</span><span class="status-actions"><button v-if="hasInbox" class="status-bell" :aria-label="inbox.unread.value ? host.i18n.t('notifications.bellUnread',{count:inbox.unread.value}) : host.i18n.t('notifications.bell')" :title="host.i18n.t('notifications.bell')" @click="togglePanel('notifications')"><Bell :size="17" /><b v-if="inbox.unread.value" class="status-badge">{{ inbox.unread.value > 99 ? '99+' : inbox.unread.value }}</b></button><button :aria-label="t('desktop.status.control')" @click="togglePanel('control')"><span :class="['status-dot', {offline:!desktop.system}]"></span><span>{{ desktop.system ? t('desktop.status.online') : t('desktop.status.offline') }}</span><Wifi :size="16" /><SlidersHorizontal :size="17" /></button></span></header>
      <HomeScreen :inert="!!panel" v-show="home || !desktop.visibleWindows.length" :time="time" :date="longDate" @launch="launch" @search="search" />
      <!-- Keep instances mounted: Home, Spotlight and switching must retain local drafts. -->
      <DesktopWindow :foreground="!home && desktop.active===window.id && !window.minimized && !panel" :inert="!!panel" v-for="window in desktop.preferences.windows" v-show="!home && !window.minimized && (desktop.preferences.mode === 'windows' || desktop.active === window.id)" :key="window.id" :window="window" :app="desktop.apps.find(a => a.id === window.id)!" />
      <div v-if="panel" class="tablet-scrim" @click="panel = null"></div>
      <section v-if="panel === 'search'" class="tablet-spotlight" role="dialog" aria-modal="true" :aria-label="t('desktop.search.label')" @keydown.esc="panel = null"><label><Search :size="23" /><input ref="searchInput" v-model="query" :placeholder="t('desktop.search.placeholder')" :aria-label="t('desktop.search.input')"><button class="tablet-round" :aria-label="t('desktop.search.close')" @click="panel = null"><X :size="18" /></button></label><div class="spotlight-results"><div v-for="app in results" :key="app.id" class="spotlight-row"><button class="spotlight-open" @click="launch(app.id)"><AppIcon :id="app.icon" :color="app.color" small /><span><strong>{{ appName(app.id) }}</strong><small>{{ t(descriptionKey(app.id)) }}</small></span><ArrowRight :size="16" /></button><button class="tablet-round spotlight-pin" :aria-pressed="desktop.preferences.pinned.includes(app.id)" :aria-label="t(desktop.preferences.pinned.includes(app.id) ? 'desktop.search.unpin' : 'desktop.search.pin', { name: appName(app.id) })" :title="t(desktop.preferences.pinned.includes(app.id) ? 'desktop.search.unpin' : 'desktop.search.pin', { name: appName(app.id) })" @click="togglePin(app.id)"><component :is="desktop.preferences.pinned.includes(app.id) ? PinOff : Pin" :size="16" /></button></div><p v-if="!results.length">{{ t('desktop.search.empty') }}</p></div><footer>{{ t('desktop.search.footer') }} <kbd>{{ t('desktop.search.esc') }}</kbd></footer></section>
      <AppSwitcher v-if="panel === 'switcher'" @launch="launch" @dismiss="panel = null" />
      <section v-if="panel === 'notifications'" class="tablet-control tablet-notifications" role="dialog" aria-modal="true" aria-labelledby="nc-panel-title"><NotificationPanel :inbox="inbox" @open="openNotifications" @close="panel = null" /></section>
      <section v-if="panel === 'control'" class="tablet-control" role="dialog" aria-modal="true" aria-labelledby="control-title"><header><span id="control-title">{{ t('desktop.control.title') }}</span><button class="tablet-round" :aria-label="t('desktop.control.close')" @click="panel = null"><X :size="18" /></button></header><div class="control-profile"><span class="avatar">{{ desktop.user?.displayName.charAt(0) }}</span><div><strong>{{ desktop.user?.displayName }}</strong><small>@{{ desktop.user?.username }}</small></div></div><div class="control-connection"><Wifi :size="23" /><div><strong>{{ desktop.system ? t('desktop.control.apiOnline') : t('desktop.control.apiOffline') }}</strong><small v-if="desktop.system">{{ t('desktop.control.host', { host: desktop.system.hostname }) }}</small></div></div><button class="control-settings" @click="openAppearance"><Palette :size="18" /> {{ t('desktop.control.appearance') }}<ArrowRight :size="16" /></button><button class="control-settings" @click="launch('settings')"><Settings :size="18" /> {{ t('desktop.control.settings') }}<ArrowRight :size="16" /></button><button class="control-settings" @click="logout"><LogOut :size="18" /> {{ t('desktop.control.logout') }}</button></section>
      <nav :inert="!!panel" class="tablet-dock" :aria-label="t('desktop.dock.label')"><button class="tablet-dock-action" :class="{selected:home}" :aria-label="t('desktop.dock.home')" :title="t('desktop.dock.home')" @click="goHome"><Home :size="25" /></button><div class="tablet-dock-separator"></div><button v-for="app in dockApps" :key="app.id" class="tablet-dock-app" :class="{active:!home && desktop.active === app.id && desktop.preferences.windows.some(w => w.id === app.id && !w.minimized)}" :aria-label="t('desktop.dock.open', { name: appName(app.id) })" @click="launch(app.id)"><AppIcon :id="app.icon" :color="app.color" /><span class="tablet-dock-tooltip">{{ appName(app.id) }}</span><i v-if="desktop.preferences.windows.some(w => w.id === app.id)"></i></button><div class="tablet-dock-separator"></div><button class="tablet-dock-action" :aria-label="t('desktop.dock.search')" :title="t('desktop.dock.searchHint')" @click="search"><Search :size="25" /></button><button class="tablet-dock-action" :aria-label="t('desktop.dock.running')" :title="t('desktop.dock.running')" @click="togglePanel('switcher')"><Layers :size="25" /><span v-if="desktop.preferences.windows.length" class="dock-count">{{ desktop.preferences.windows.length }}</span></button></nav>
      <button v-if="desktop.saveError" class="tablet-save-error" @click="desktop.persist">{{ t('desktop.save.retry') }}</button><span v-else-if="desktop.saving" class="tablet-saving" role="status">{{ t('desktop.save.saving') }}</span>
      <Transition name="toast"><div v-if="desktop.notice" class="toast-message" role="status"><Check :size="17" />{{ desktop.notice }}</div></Transition>
    </template>
  </div>
</template>
