import {defineSettingsProvider,type SettingsProvider} from '@minhanos/app-sdk'
import {settingsProvider as accounts} from '@minhanos/accounts-app/settings'
import {createSystemSettingsProvider} from '@minhanos/settings-app/system'
import {settingsProvider as storage} from '@minhanos/storage-app/settings'
import {settingsProvider as files} from '@minhanos/files-app/settings'
import catalog from '../../../contracts/apps/catalog.v1.json'
import desktopManifest from '../../../contracts/manifests/desktop.app.json'
import notificationsManifest from '../../../contracts/manifests/notifications.app.json'
import logsManifest from '../../../contracts/manifests/logs.app.json'
import notesManifest from '../../../contracts/manifests/notes.app.json'
import AppearancePanel from './components/AppearancePanel.vue'
import {api} from './api'

// First-party apps compiled into this Shell. Settings navigation is derived from each manifest;
// a new app adds its provider here (tests/settings-standard.mjs checks manifests and panels).
const providers:SettingsProvider[]=[
 createSystemSettingsProvider(api),
 defineSettingsProvider({manifest:desktopManifest,panels:{appearance:AppearancePanel}}),
 accounts,
 storage,
 files,
 defineSettingsProvider({manifest:notificationsManifest,panels:{}}),
 defineSettingsProvider({manifest:logsManifest,panels:{}}),
 defineSettingsProvider({manifest:notesManifest,panels:{}}),
]

// System settings and the Shell itself are always present; other apps only when the backend registry lists them
// under their App ID or a catalogued legacy registry ID.
export function installedSettingsProviders(registryIds:readonly string[]){
 return providers.filter(provider=>{
  if(['com.minhanos.settings','com.minhanos.desktop'].includes(provider.manifest.id))return true
  const app=catalog.apps.find(a=>a.appId===provider.manifest.id)
  return registryIds.includes(provider.manifest.id)||!!app?.legacyRegistryIds.some(id=>registryIds.includes(id))
 })
}
