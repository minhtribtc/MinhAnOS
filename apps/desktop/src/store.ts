import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api, setCSRF, type User, type AppDefinition, type Preferences, type DesktopWindow, type SystemInfo } from './api'

function freshPreferences():Preferences{return { theme: 'light', wallpaper: 'mountain', mode: 'focus', windows: [], notes: '', pinned: ['overview', 'files', 'apps', 'settings'] }}

export const useDesktop = defineStore('desktop', () => {
  const user = ref<User | null>(null)
  const apps = ref<AppDefinition[]>([])
  const preferences = ref<Preferences>(freshPreferences())
  const system = ref<SystemInfo | null>(null)
  const active = ref('overview')
  const order = ref<string[]>([])
  const ready = ref(false)
  const saving = ref(false)
  const saveError = ref('')
  const notice = ref('')
  const closeGuards=new Map<string,()=>boolean>()
  function registerCloseGuard(id:string,guard:()=>boolean){closeGuards.set(id,guard);return ()=>closeGuards.delete(id)}
  let timer: ReturnType<typeof setTimeout> | undefined
  let noticeTimer: ReturnType<typeof setTimeout> | undefined
  let sessionEpoch=0
  let revision = 0
  let savedRevision = 0
  let savePromise: Promise<void> | null = null
  const visibleWindows = computed(() => preferences.value.windows.filter(w => !w.minimized))
  function toast(message: string) { notice.value = message; clearTimeout(noticeTimer); noticeTimer = setTimeout(() => notice.value = '', 4500) }
  function focus(id: string) { active.value = id; order.value = [...order.value.filter(x => x !== id), id] }
  function saveSoon() { revision++; clearTimeout(timer); timer = setTimeout(() => void persist(), 500) }
  async function persist(): Promise<void> {
    clearTimeout(timer)
    const epoch=sessionEpoch
    if (savePromise) { await savePromise; if(epoch!==sessionEpoch)return; if (savedRevision < revision) await persist(); return }
    if (!user.value || savedRevision >= revision) return
    const sentRevision = revision
    const payload = JSON.stringify(preferences.value)
    saving.value = true
    savePromise = (async () => {
      try { await api('/preferences', { method: 'PUT', body: payload }); if(epoch===sessionEpoch){savedRevision = sentRevision; saveError.value = ''} }
      catch (error) { if(epoch===sessionEpoch)saveError.value = error instanceof Error ? error.message : 'Không lưu được cài đặt' }
      finally { if(epoch===sessionEpoch){saving.value = false; savePromise = null} }
    })()
    await savePromise
    if(epoch!==sessionEpoch)return
    if (savedRevision === sentRevision && revision > sentRevision) await persist()
  }
  function open(id: string) {
    if (!apps.value.some(a => a.id === id)) return
    let window = preferences.value.windows.find(w => w.id === id)
    if (!window) {
      const offset = (preferences.value.windows.length % 5) * 28
      const width = Math.min(id === 'notes' ? 680 : 1000, globalThis.innerWidth - 48)
      const height = Math.min(650, globalThis.innerHeight - 112)
      window = { id, x: Math.max(24, (globalThis.innerWidth - width) / 2 + offset - 25), y: Math.max(20, (globalThis.innerHeight - height - 58) / 2 + offset), width, height, minimized: false, maximized: false }
      preferences.value.windows.push(window)
    }
    window.minimized = false; focus(id); saveSoon()
  }
  function close(id: string) {
    if(closeGuards.get(id)?.()===false)return
    preferences.value.windows = preferences.value.windows.filter(w => w.id !== id)
    order.value = order.value.filter(x => x !== id)
    active.value = order.value.at(-1) || ''
    saveSoon()
  }
  function minimize(id: string) { const window = preferences.value.windows.find(w => w.id === id); if (window) window.minimized = true; active.value = visibleWindows.value.at(-1)?.id || ''; saveSoon() }
  function maximize(id: string) { const window = preferences.value.windows.find(w => w.id === id); if (window) window.maximized = !window.maximized; focus(id); saveSoon() }
  function setMode(mode: 'focus' | 'windows') { preferences.value.mode = mode; saveSoon() }
  async function loadSession() {
    const session = await api<{user: User; csrfToken: string}>('/session')
    user.value = session.user; setCSRF(session.csrfToken)
    const [registry, prefs] = await Promise.all([api<AppDefinition[]>('/apps'), api<Preferences>('/preferences')])
    apps.value = registry
    preferences.value = prefs
    preferences.value.mode = prefs.mode === 'windows' ? 'windows' : 'focus'
    preferences.value.windows = prefs.windows.filter(w => registry.some(a => a.id === w.id))
    order.value = preferences.value.windows.map(w => w.id)
    active.value = preferences.value.windows.filter(w => !w.minimized).at(-1)?.id || ''
    ready.value = true
    const requested = new URLSearchParams(location.search).get('app')
    if (requested && apps.value.some(a => a.id === requested)) open(requested)
    await refreshSystem()
  }
  async function refreshSystem() { try { system.value = await api<SystemInfo>('/system') } catch { /* individual panels expose disconnected state */ system.value = null } }
  // A credential change/current-session revocation already ended the server
  // session. Clear this host locally without trying another authenticated logout.
  function expireSession(){
    sessionEpoch++;savePromise=null
    clearTimeout(timer);clearTimeout(noticeTimer);user.value=null;setCSRF('');ready.value=false;apps.value=[];system.value=null
    preferences.value=freshPreferences();revision=0;savedRevision=0;order.value=[];active.value='';notice.value='';saveError.value='';saving.value=false
  }
  async function logout() {
    for(const guard of closeGuards.values()){if(!guard())return}
    await persist()
    await api('/logout', { method: 'POST' })
    expireSession()
  }
  return { expireSession, registerCloseGuard, user, apps, preferences, system, active, order, ready, saving, saveError, notice, visibleWindows, toast, focus, saveSoon, persist, open, close, minimize, maximize, setMode, loadSession, refreshSystem, logout }
})
