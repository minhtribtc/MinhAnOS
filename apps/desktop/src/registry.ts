import type { Host } from '@minhanos/app-sdk'
import catalog from '../../../contracts/apps/catalog.v1.json'
import filesManifest from '../../../contracts/manifests/files.app.json'
import settingsManifest from '../../../contracts/manifests/settings.app.json'
import notesManifest from '../../../contracts/manifests/notes.app.json'
import accountsManifest from '../../../contracts/manifests/accounts.app.json'
import storageManifest from '../../../contracts/manifests/storage.app.json'
import notificationsManifest from '../../../contracts/manifests/notifications.app.json'
import logsManifest from '../../../contracts/manifests/logs.app.json'
import filesCatalog from '../../../packages/files-app/locales/catalogs.json'
import settingsCatalog from '../../../packages/settings-app/locales/catalogs.json'
import notesCatalog from '../../../packages/notes-app/locales/catalogs.json'
import accountsCatalog from '../../../packages/accounts-app/locales/catalogs.json'
import storageCatalog from '../../../packages/storage-app/locales/catalogs.json'
import notificationsCatalog from '../../../packages/notifications-app/locales/catalogs.json'
import logsCatalog from '../../../packages/logs-app/locales/catalogs.json'

// Launcher metadata comes from each app's manifest and catalog (app standard row 3): the backend registry
// only says which App IDs this user may open. Names are <namespace>.app.name / .app.description.
type Manifest = { id: string; entrypoint: string }
type Catalog = Parameters<Host['useCatalog']>[1]
const apps: [Manifest, string, Catalog][] = [
  [filesManifest, 'files', filesCatalog], [settingsManifest, 'settings', settingsCatalog], [notesManifest, 'notes', notesCatalog],
  [accountsManifest, 'accounts', accountsCatalog], [storageManifest, 'storage', storageCatalog],
  [notificationsManifest, 'notifications', notificationsCatalog], [logsManifest, 'logs', logsCatalog],
]
export const launcherCatalogs = apps.map(([, namespace, value]) => [namespace, value] as const)

// Registry entries use the App ID, or a catalogued legacy registry ID ("files", "settings").
function manifestFor(registryId: string) {
  const appId = catalog.apps.find(a => a.appId === registryId || a.legacyRegistryIds.includes(registryId))?.appId
  return apps.find(([m]) => m.id === appId)
}
export function appNamespace(registryId: string) { return manifestFor(registryId)?.[1] }
export function nameKey(registryId: string) { const ns = appNamespace(registryId); return ns ? ns + '.app.name' : 'desktop.app.unknown' }
export function descriptionKey(registryId: string) { const ns = appNamespace(registryId); return ns ? ns + '.app.description' : 'desktop.app.unknown' }

// Entry points (/apps/<name>) are real: the Shell opens the app whose manifest declares the path.
export function registryIdForPath(pathname: string, registryIds: readonly string[]) {
  const [manifest] = apps.find(([m]) => pathname === m.entrypoint || pathname.startsWith(m.entrypoint + '/')) ?? []
  if (!manifest) return undefined
  const entry = catalog.apps.find(a => a.appId === manifest.id)
  return registryIds.find(id => id === manifest.id || !!entry?.legacyRegistryIds.includes(id))
}
