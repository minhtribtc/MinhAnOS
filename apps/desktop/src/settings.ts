import {defineSettingsProvider,type SettingsProvider} from '@minhanos/app-sdk'
import {settingsProvider as accounts} from '@minhanos/accounts-app/settings'
import catalog from '../../../contracts/apps/catalog.v1.json'
import desktopManifest from '../../../contracts/manifests/desktop.app.json'
import filesManifest from '../../../contracts/manifests/files.app.json'
import AppearancePanel from './components/AppearancePanel.vue'

// First-party apps compiled into this Shell. Settings navigation is derived from each manifest;
// a new app adds its provider here (tests/settings-standard.mjs checks manifests and panels).
const providers:SettingsProvider[]=[
 defineSettingsProvider({manifest:desktopManifest,panels:{appearance:AppearancePanel}}),
 accounts,
 defineSettingsProvider({manifest:filesManifest,panels:{}}),
]

// The Shell itself is always present; other apps only when the backend registry lists them
// under their App ID or a catalogued legacy registry ID.
export function installedSettingsProviders(registryIds:readonly string[]){
 return providers.filter(provider=>{
  if(provider.manifest.id==='com.minhanos.desktop')return true
  const app=catalog.apps.find(a=>a.appId===provider.manifest.id)
  return registryIds.includes(provider.manifest.id)||!!app?.legacyRegistryIds.some(id=>registryIds.includes(id))
 })
}
